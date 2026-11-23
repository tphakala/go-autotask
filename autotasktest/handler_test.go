package autotasktest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"testing"

	autotask "github.com/tphakala/go-autotask"
	"github.com/tphakala/go-autotask/entities"
)

// TestHandleVersionInfoUsesAPIVersionsKey pins the wire key of the mock version
// endpoint. discoverZone decodes the response under the "apiVersions" key, so
// returning any other key (the old "versions") yields an empty version list.
func TestHandleVersionInfoUsesAPIVersionsKey(t *testing.T) {
	ts, _ := NewServer(t)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, ts.URL+"/atservicesrest/versioninformation", http.NoBody)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	// No credentials: zone discovery is answered without the auth check.
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	// Decode with exactly the shape discoverZone uses.
	var decoded struct {
		Versions []string `json:"apiVersions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(decoded.Versions) == 0 {
		t.Fatal("apiVersions is empty; the mock must use the \"apiVersions\" key that discoverZone decodes")
	}
	if !slices.Contains(decoded.Versions, "V1.0") {
		t.Fatalf("apiVersions = %v; want it to advertise the supported V1.0", decoded.Versions)
	}
}

// TestHandleQueryNoMatchSendsEmptyItemsArray pins the wire shape of a query that
// matches no stored item: "items": [] like the Autotask API, not null.
func TestHandleQueryNoMatchSendsEmptyItemsArray(t *testing.T) {
	ts, _ := NewServer(t, WithEntity(CompanyFixture()))

	body := strings.NewReader(`{"filter":[{"op":"eq","field":"id","value":-1}]}`)
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, ts.URL+"/v1.0/Companies/query", body)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("UserName", "test-user")
	req.Header.Set("Secret", "test-secret")
	req.Header.Set("ApiIntegrationCode", "test-code")

	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var decoded struct {
		Items json.RawMessage `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got := string(decoded.Items); got != "[]" {
		t.Errorf("items = %s, want []", got)
	}
}

// TestNewClientDiscoversZoneFromServer runs real zone discovery against the
// test server. Discovery requests carry no credentials, so the server must
// answer them without the auth check; every other path stays auth-checked.
func TestNewClientDiscoversZoneFromServer(t *testing.T) {
	ts, _ := NewServer(t, WithEntity(CompanyFixture(func(c *entities.Company) { c.ID = autotask.Set(int64(7)) })))
	auth := autotask.AuthConfig{Username: "test-user", Secret: "test-secret", IntegrationCode: "test-code"}

	client, err := autotask.NewClient(t.Context(), auth, autotask.WithZoneBaseURL(ts.URL))
	if err != nil {
		t.Fatalf("NewClient with zone discovery: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	zone, ok := client.Zone()
	if !ok || zone.URL != ts.URL {
		t.Fatalf("Zone() = %+v, %v; want URL %q, true", zone, ok, ts.URL)
	}
	if _, err := autotask.Get[entities.Company](t.Context(), client, 7); err != nil {
		t.Fatalf("Get through the discovered zone: %v", err)
	}

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, ts.URL+"/v1.0/Companies/7", http.NoBody)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("entity GET without credentials: status = %d, want 401", resp.StatusCode)
	}
}

// TestDiscoveryRulesRunBeforeDiscovery pins that a rule on a discovery path
// still answers first, so a test can make zone discovery fail.
func TestDiscoveryRulesRunBeforeDiscovery(t *testing.T) {
	ts, _ := NewServer(t, WithErrorOn(http.MethodGet, "/versioninformation", http.StatusServiceUnavailable, []string{"maintenance"}))
	auth := autotask.AuthConfig{Username: "test-user", Secret: "test-secret", IntegrationCode: "test-code"}

	client, err := autotask.NewClient(t.Context(), auth, autotask.WithZoneBaseURL(ts.URL))
	if err == nil {
		_ = client.Close()
		t.Fatal("NewClient succeeded; want the 503 the rule serves for the version request")
	}
	if !strings.Contains(err.Error(), "503") {
		t.Fatalf("NewClient error = %v; want the rule's 503", err)
	}
}

// TestDiscoveryBypassIsLimitedToDiscoveryGETs pins that only the two discovery
// GETs at the server root skip the auth check: another method, the same suffix
// on another path, or an extra path segment still needs credentials.
func TestDiscoveryBypassIsLimitedToDiscoveryGETs(t *testing.T) {
	ts, _ := NewServer(t)
	for _, tc := range []struct{ method, path string }{
		{http.MethodPost, "/atservicesrest/versioninformation"},
		{http.MethodGet, "/v1.0/Companies/7/zoneInformation"},
		{http.MethodGet, "/v1.0/Companies/7/atservicesrest/versioninformation"},
		{http.MethodGet, "/atservicesrest/V1.0/x/zoneInformation"},
	} {
		req, err := http.NewRequestWithContext(t.Context(), tc.method, ts.URL+tc.path, http.NoBody)
		if err != nil {
			t.Fatalf("new request: %v", err)
		}
		resp, err := ts.Client().Do(req)
		if err != nil {
			t.Fatalf("do: %v", err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("%s %s without credentials: status = %d, want 401", tc.method, tc.path, resp.StatusCode)
		}
	}
}

// titlesOf returns the titles of the items, for comparing lists across entity
// types whose ids come from separate sequences.
func titlesOf[T any](items []*T, title func(*T) autotask.Optional[string]) []string {
	got := make([]string, 0, len(items))
	for _, it := range items {
		v, _ := title(it).Get()
		got = append(got, v)
	}
	slices.Sort(got)
	return got
}

func noteTitle(n *entities.TicketNote) autotask.Optional[string]                   { return n.Title }
func ticketAttachmentTitle(a *entities.TicketAttachment) autotask.Optional[string] { return a.Title }
func noteAttachmentTitle(a *entities.TicketNoteAttachment) autotask.Optional[string] {
	return a.Title
}

func assertTitles(t *testing.T, what string, got, want []string) {
	t.Helper()
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("%s = %v, want %v", what, got, want)
	}
}

// TestChildStoresArePerParentAndType pins that a child URL resolves to the
// store of the child's entity type (not one store per URL segment) and that
// child items stay under the parent named in their create URL.
func TestChildStoresArePerParentAndType(t *testing.T) {
	seeded := TicketNoteFixture(func(n *entities.TicketNote) { n.Title = autotask.Set("seeded") })
	forOne := TicketNoteFixture(func(n *entities.TicketNote) { n.Title = autotask.Set("for ticket 1") })
	_, client := NewServer(t,
		WithEntity(seeded),
		WithChildEntity[entities.Ticket](1, forOne),
	)
	ctx := t.Context()

	// A child type nothing was seeded for answers an empty page, not 404.
	empty, err := autotask.ListChild[entities.TicketNote, entities.TicketNoteAttachment](ctx, client, 10)
	if err != nil {
		t.Fatalf("list of an unpopulated child type: %v", err)
	}
	if len(empty) != 0 {
		t.Fatalf("unpopulated child list = %d items, want 0", len(empty))
	}

	created := TicketNoteFixture(func(n *entities.TicketNote) {
		n.ID = autotask.Optional[int64]{}
		n.Title = autotask.Set("for ticket 2")
	})
	if _, err := autotask.CreateChild[entities.Ticket](ctx, client, 2, &created); err != nil {
		t.Fatal(err)
	}
	noteAtt := entities.TicketNoteAttachment{Title: autotask.Set("note attachment")}
	if _, err := autotask.CreateChild[entities.TicketNote](ctx, client, 10, &noteAtt); err != nil {
		t.Fatal(err)
	}
	ticketAtt := entities.TicketAttachment{Title: autotask.Set("ticket attachment")}
	if _, err := autotask.CreateChild[entities.Ticket](ctx, client, 2, &ticketAtt); err != nil {
		t.Fatal(err)
	}

	// Items seeded without a parent are listed under every parent.
	notes1, err := autotask.ListChild[entities.Ticket, entities.TicketNote](ctx, client, 1)
	if err != nil {
		t.Fatal(err)
	}
	assertTitles(t, "ticket 1 notes", titlesOf(notes1, noteTitle), []string{"seeded", "for ticket 1"})
	notes2, err := autotask.ListChild[entities.Ticket, entities.TicketNote](ctx, client, 2)
	if err != nil {
		t.Fatal(err)
	}
	assertTitles(t, "ticket 2 notes", titlesOf(notes2, noteTitle), []string{"seeded", "for ticket 2"})

	// Both attachment types use the segment Attachments; neither sees the other.
	noteAtts, err := autotask.ListChild[entities.TicketNote, entities.TicketNoteAttachment](ctx, client, 10)
	if err != nil {
		t.Fatal(err)
	}
	assertTitles(t, "note 10 attachments", titlesOf(noteAtts, noteAttachmentTitle), []string{"note attachment"})
	ticketAtts, err := autotask.ListChild[entities.Ticket, entities.TicketAttachment](ctx, client, 2)
	if err != nil {
		t.Fatal(err)
	}
	assertTitles(t, "ticket 2 attachments", titlesOf(ticketAtts, ticketAttachmentTitle), []string{"ticket attachment"})

	// The child URL and the root URL share one store per entity type.
	id, _ := noteAtt.ID.Get()
	direct, err := autotask.Get[entities.TicketNoteAttachment](ctx, client, id)
	if err != nil {
		t.Fatalf("Get after CreateChild: %v", err)
	}
	if title, _ := direct.Title.Get(); title != "note attachment" {
		t.Errorf("Get title = %q, want %q", title, "note attachment")
	}
}

func TestChildGetByID(t *testing.T) {
	_, client := NewServer(t)
	ctx := t.Context()

	att := entities.TicketNoteAttachment{Title: autotask.Set("file")}
	if _, err := autotask.CreateChild[entities.TicketNote](ctx, client, 10, &att); err != nil {
		t.Fatal(err)
	}
	attID, _ := att.ID.Get()

	getItem := func(parent int64) string {
		t.Helper()
		var resp struct {
			Item json.RawMessage `json:"item"`
		}
		path := fmt.Sprintf("/v1.0/TicketNotes/%d/Attachments/%d", parent, attID)
		if err := client.Do(ctx, http.MethodGet, path, nil, &resp); err != nil {
			t.Fatal(err)
		}
		return string(resp.Item)
	}
	if got := getItem(10); !strings.Contains(got, `"title":"file"`) {
		t.Errorf("child GET by id under its parent = %s, want the attachment", got)
	}
	if got := getItem(11); got != "null" {
		t.Errorf("child GET by id under another parent = %s, want null", got)
	}
}

func TestChildUpdateKeepsParent(t *testing.T) {
	_, client := NewServer(t)
	ctx := t.Context()

	note := TicketNoteFixture(func(n *entities.TicketNote) {
		n.ID = autotask.Optional[int64]{}
		n.Title = autotask.Set("before")
	})
	if _, err := autotask.CreateChild[entities.Ticket](ctx, client, 2, &note); err != nil {
		t.Fatal(err)
	}
	note.Title = autotask.Set("after")
	if _, err := autotask.UpdateChild[entities.Ticket](ctx, client, 2, &note); err != nil {
		t.Fatalf("UpdateChild: %v", err)
	}
	noteID, _ := note.ID.Get()
	got, err := autotask.Get[entities.TicketNote](ctx, client, noteID)
	if err != nil {
		t.Fatal(err)
	}
	if title, _ := got.Title.Get(); title != "after" {
		t.Errorf("title after UpdateChild = %q, want %q", title, "after")
	}
	for parent, want := range map[int64][]string{2: {"after"}, 3: {}} {
		list, err := autotask.ListChild[entities.Ticket, entities.TicketNote](ctx, client, parent)
		if err != nil {
			t.Fatal(err)
		}
		assertTitles(t, fmt.Sprintf("ticket %d notes", parent), titlesOf(list, noteTitle), want)
	}

	other := entities.TicketNote{ID: note.ID, Title: autotask.Set("hijack")}
	_, err = autotask.UpdateChild[entities.Ticket](ctx, client, 3, &other)
	if _, ok := errors.AsType[*autotask.NotFoundError](err); !ok {
		t.Errorf("UpdateChild under the wrong parent err = %v, want *NotFoundError", err)
	}
}

func TestChildDeleteChecksParent(t *testing.T) {
	_, client := NewServer(t, WithDeleteSupport("TicketNoteAttachments"))
	ctx := t.Context()

	att := entities.TicketNoteAttachment{Title: autotask.Set("file")}
	if _, err := autotask.CreateChild[entities.TicketNote](ctx, client, 10, &att); err != nil {
		t.Fatal(err)
	}
	attID, _ := att.ID.Get()
	del := func(parent int64) error {
		return autotask.DeleteChild[entities.TicketNote, entities.TicketNoteAttachment](ctx, client, parent, attID)
	}
	remaining := func() int {
		t.Helper()
		list, err := autotask.ListChild[entities.TicketNote, entities.TicketNoteAttachment](ctx, client, 10)
		if err != nil {
			t.Fatal(err)
		}
		return len(list)
	}

	if _, ok := errors.AsType[*autotask.NotFoundError](del(11)); !ok {
		t.Error("DeleteChild under the wrong parent did not return *NotFoundError")
	}
	if got := remaining(); got != 1 {
		t.Fatalf("attachments after a refused delete = %d, want 1", got)
	}
	if err := del(10); err != nil {
		t.Fatalf("DeleteChild: %v", err)
	}
	if got := remaining(); got != 0 {
		t.Fatalf("attachments after delete = %d, want 0", got)
	}
	if _, ok := errors.AsType[*autotask.NotFoundError](del(10)); !ok {
		t.Error("second DeleteChild did not return *NotFoundError")
	}
}

// TestChildDeleteNeedsDeleteSupport pins that a child DELETE honors the store's
// WithDeleteSupport gate like a root DELETE does.
func TestChildDeleteNeedsDeleteSupport(t *testing.T) {
	_, client := NewServer(t)
	ctx := t.Context()

	att := entities.TicketAttachment{Title: autotask.Set("kept")}
	if _, err := autotask.CreateChild[entities.Ticket](ctx, client, 2, &att); err != nil {
		t.Fatal(err)
	}
	id, _ := att.ID.Get()
	err := autotask.DeleteChild[entities.Ticket, entities.TicketAttachment](ctx, client, 2, id)
	apiErr, ok := errors.AsType[*autotask.Error](err)
	if !ok || apiErr.StatusCode != http.StatusBadRequest {
		t.Errorf("DeleteChild without delete support err = %v, want a 400 *Error", err)
	}
}

// TestWithChildEntityReusesExistingStore pins that option order does not
// matter: WithChildEntity adds to a store WithDeleteSupport or WithEntity
// created first, and a store it creates is reused by a later WithDeleteSupport
// or WithEntity. Each case ends with one store, delete support kept, and every
// seeded item listed under the parent.
func TestWithChildEntityReusesExistingStore(t *testing.T) {
	att := entities.TicketNoteAttachment{Title: autotask.Set("child")}
	root := entities.TicketNoteAttachment{Title: autotask.Set("root")}
	const store = "TicketNoteAttachments"
	for _, tt := range []struct {
		name string
		opts []ServerOption
		want []string
	}{
		{"delete support first", []ServerOption{WithDeleteSupport(store), WithChildEntity[entities.TicketNote](10, att)}, []string{"child"}},
		{"child first", []ServerOption{WithChildEntity[entities.TicketNote](10, att), WithDeleteSupport(store), WithEntity(root)}, []string{"child", "root"}},
		{"entity first", []ServerOption{WithEntity(root), WithChildEntity[entities.TicketNote](10, att), WithDeleteSupport(store)}, []string{"child", "root"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ts, client := NewServer(t, tt.opts...)
			stores := 0
			for key, st := range ts.entities {
				if st.name == store {
					stores++
					if key != store {
						t.Errorf("store registered under %q", key)
					}
				}
			}
			if stores != 1 {
				t.Fatalf("stores named %s = %d, want 1", store, stores)
			}
			if !ts.entities[store].canDelete {
				t.Error("delete support was lost")
			}
			list, err := autotask.ListChild[entities.TicketNote, entities.TicketNoteAttachment](t.Context(), client, 10)
			if err != nil {
				t.Fatal(err)
			}
			assertTitles(t, "note 10 attachments", titlesOf(list, noteAttachmentTitle), tt.want)
		})
	}
}

// TestWithChildEntityRouteConflict pins that registering one parent and segment
// for two entity types fails the test and keeps the first route.
func TestWithChildEntityRouteConflict(t *testing.T) {
	rec := &recordingTB{TB: t}
	ts := &TestServer{
		t:           rec,
		entities:    map[string]*entityStore{},
		childRoutes: map[string]string{},
	}
	WithChildEntity[entities.Ticket, entities.TicketNote](1)(ts)
	if len(rec.fatal) != 0 {
		t.Fatalf("first registration failed: %v", rec.fatal)
	}
	// ProjectNote also has the segment Notes, so under Tickets it conflicts.
	WithChildEntity[entities.Ticket, entities.ProjectNote](1)(ts)
	if len(rec.fatal) != 1 || !strings.Contains(rec.fatal[0], "Tickets/Notes") {
		t.Fatalf("fatal messages = %v, want one naming Tickets/Notes", rec.fatal)
	}
	if got := ts.childRoutes["Tickets/Notes"]; got != "TicketNotes" {
		t.Errorf("route Tickets/Notes = %q, want TicketNotes", got)
	}
}

// recordingTB records Fatalf calls instead of stopping the test. Other methods
// fall through to the embedded TB.
type recordingTB struct {
	testing.TB
	fatal []string
}

func (r *recordingTB) Fatalf(format string, args ...any) {
	r.fatal = append(r.fatal, fmt.Sprintf(format, args...))
}

// TestSeededItemsAreVisibleUnderEveryParent pins that an item seeded without a
// parent answers child GET by id, UpdateChild and DeleteChild under any parent.
func TestSeededItemsAreVisibleUnderEveryParent(t *testing.T) {
	seeded := entities.TicketNoteAttachment{ID: autotask.Set(int64(5)), Title: autotask.Set("seeded")}
	_, client := NewServer(t, WithEntity(seeded), WithDeleteSupport("TicketNoteAttachments"))
	ctx := t.Context()

	var resp struct {
		Item json.RawMessage `json:"item"`
	}
	if err := client.Do(ctx, http.MethodGet, "/v1.0/TicketNotes/77/Attachments/5", nil, &resp); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(resp.Item), `"title":"seeded"`) {
		t.Errorf("child GET by id of a seeded item under another parent = %s, want the item", resp.Item)
	}
	edited := entities.TicketNoteAttachment{ID: autotask.Set(int64(5)), Title: autotask.Set("edited")}
	if _, err := autotask.UpdateChild[entities.TicketNote](ctx, client, 78, &edited); err != nil {
		t.Fatalf("UpdateChild of a seeded item: %v", err)
	}
	got, err := autotask.Get[entities.TicketNoteAttachment](ctx, client, 5)
	if err != nil {
		t.Fatal(err)
	}
	if title, _ := got.Title.Get(); title != "edited" {
		t.Errorf("title after UpdateChild = %q, want edited", title)
	}
	if err := autotask.DeleteChild[entities.TicketNote, entities.TicketNoteAttachment](ctx, client, 79, 5); err != nil {
		t.Fatalf("DeleteChild of a seeded item: %v", err)
	}
}

// TestChildListPages pins that child lists page by ?page=N and filter by parent
// before paging: the other parent's item is stored first, so paging the whole
// store and filtering each page would need more requests than the two pages of
// parent 1.
func TestChildListPages(t *testing.T) {
	mk := func(title string) entities.TicketNote {
		return TicketNoteFixture(func(n *entities.TicketNote) { n.Title = autotask.Set(title) })
	}
	ts, client := NewServer(t,
		WithPageSize(1),
		WithChildEntity[entities.Ticket](2, mk("b1")),
		WithChildEntity[entities.Ticket](1, mk("a1"), mk("a2")),
	)
	before := ts.RequestCount()
	got, err := autotask.ListChild[entities.Ticket, entities.TicketNote](t.Context(), client, 1)
	if err != nil {
		t.Fatal(err)
	}
	assertTitles(t, "ticket 1 notes", titlesOf(got, noteTitle), []string{"a1", "a2"})
	if requests := ts.RequestCount() - before; requests != 2 {
		t.Errorf("requests = %d, want 2 (one page per item of parent 1)", requests)
	}
}

// TestPageSizeZeroPutsEverythingOnOnePage pins that a page size of 0 or less
// answers every item on one page, with no next link, for a child list and a query.
func TestPageSizeZeroPutsEverythingOnOnePage(t *testing.T) {
	mk := func(title string) entities.TicketNote {
		return TicketNoteFixture(func(n *entities.TicketNote) { n.Title = autotask.Set(title) })
	}
	_, client := NewServer(t, WithPageSize(0), WithChildEntity[entities.Ticket](1, mk("n1"), mk("n2")))
	ctx := t.Context()

	// An empty list must not divide by a zero page size.
	var empty struct {
		Items []json.RawMessage `json:"items"`
	}
	if err := client.Do(ctx, http.MethodGet, "/v1.0/Tickets/1/Attachments", nil, &empty); err != nil {
		t.Fatalf("empty child list: %v", err)
	}
	if len(empty.Items) != 0 {
		t.Errorf("empty child list: %d items, want 0", len(empty.Items))
	}

	for _, path := range []string{"/v1.0/Tickets/1/Notes", "/v1.0/TicketNotes/query"} {
		method := http.MethodGet
		var body any
		if strings.HasSuffix(path, "/query") {
			method = http.MethodPost
			body = map[string]any{"filter": []any{}}
		}
		var resp struct {
			Items       []json.RawMessage `json:"items"`
			PageDetails struct {
				NextPageURL *string `json:"nextPageUrl"`
			} `json:"pageDetails"`
		}
		if err := client.Do(ctx, method, path, body, &resp); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		if len(resp.Items) != 2 || resp.PageDetails.NextPageURL != nil {
			t.Errorf("%s: %d items, next page %v; want 2 items and no next page", path, len(resp.Items), resp.PageDetails.NextPageURL)
		}
	}
}

// TestChildSegmentResolvesThroughSingularParent pins the store a child URL
// reaches for each suffix rule of singularEntityName: the singular parent name
// plus the segment must be the entity name the library's child types use.
func TestChildSegmentResolvesThroughSingularParent(t *testing.T) {
	for _, tt := range []struct{ parent, segment, want string }{
		{"Tickets", "Attachments", "TicketAttachments"},
		{"Companies", "Notes", "CompanyNotes"},
		{"Statuses", "Items", "StatusItems"},
		{"Addresses", "Lines", "AddressLines"},
	} {
		ts, client := NewServer(t)
		path := "/v1.0/" + tt.parent + "/5/" + tt.segment
		if err := client.Do(t.Context(), http.MethodPost, path, map[string]any{"title": "x"}, nil); err != nil {
			t.Fatal(err)
		}
		if _, ok := ts.entities[tt.want]; !ok {
			t.Errorf("POST %s did not create store %s", path, tt.want)
		}
	}
}

// TestChildListOfUnknownTypeSendsEmptyItemsArray pins the wire shape of a child
// list nothing was stored for: "items": [] like a known type with no match, not null.
func TestChildListOfUnknownTypeSendsEmptyItemsArray(t *testing.T) {
	_, client := NewServer(t)
	var resp struct {
		Items json.RawMessage `json:"items"`
	}
	if err := client.Do(t.Context(), http.MethodGet, "/v1.0/Tickets/1/Attachments", nil, &resp); err != nil {
		t.Fatal(err)
	}
	if got := string(resp.Items); got != "[]" {
		t.Errorf("items = %s, want []", got)
	}
}

// TestHugePageNumberAnswersEmptyPage pins that a ?page=N whose offset would
// overflow int answers an empty page instead of panicking.
func TestHugePageNumberAnswersEmptyPage(t *testing.T) {
	mk := func(title string) entities.TicketNote {
		return TicketNoteFixture(func(n *entities.TicketNote) { n.Title = autotask.Set(title) })
	}
	_, client := NewServer(t, WithPageSize(2), WithChildEntity[entities.Ticket](1, mk("n1"), mk("n2")))
	var resp struct {
		Items []json.RawMessage `json:"items"`
	}
	path := "/v1.0/Tickets/1/Notes?page=" + strconv.Itoa(math.MaxInt)
	if err := client.Do(t.Context(), http.MethodGet, path, nil, &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Items) != 0 {
		t.Errorf("items = %d, want 0", len(resp.Items))
	}
}

// TestCreateAfterSeedDoesNotReuseSeededID pins that a create never returns the
// id of a seeded item: a root create after WithEntity and a child create after
// WithChildEntity.
func TestCreateAfterSeedDoesNotReuseSeededID(t *testing.T) {
	seeded := TicketNoteFixture(func(n *entities.TicketNote) { n.ID = autotask.Set(int64(1)) })
	create := map[string]func(context.Context, *autotask.Client) (*entities.TicketNote, error){
		"root after WithEntity": func(ctx context.Context, c *autotask.Client) (*entities.TicketNote, error) {
			return autotask.Create(ctx, c, &entities.TicketNote{Title: autotask.Set("new")})
		},
		"child after WithChildEntity": func(ctx context.Context, c *autotask.Client) (*entities.TicketNote, error) {
			return autotask.CreateChild[entities.Ticket](ctx, c, 1, &entities.TicketNote{Title: autotask.Set("new")})
		},
	}
	opts := map[string]ServerOption{
		"root after WithEntity":       WithEntity(seeded),
		"child after WithChildEntity": WithChildEntity[entities.Ticket](1, seeded),
	}
	for name, fn := range create {
		t.Run(name, func(t *testing.T) {
			_, client := NewServer(t, opts[name])
			got, err := fn(t.Context(), client)
			if err != nil {
				t.Fatal(err)
			}
			if id, _ := got.ID.Get(); id == 1 {
				t.Errorf("create got id 1, the seeded item's id")
			}
		})
	}
}

// TestIDsAbove2To53StayExact pins that a seeded id above 2^53, and the id a
// create gets after it, update and read back exactly.
func TestIDsAbove2To53StayExact(t *testing.T) {
	const big = int64(1)<<53 + 1
	seeded := TicketNoteFixture(func(n *entities.TicketNote) {
		n.ID = autotask.Set(big)
		n.Title = autotask.Set("seeded")
	})
	_, client := NewServer(t, WithEntity(seeded))
	ctx := t.Context()

	created, err := autotask.Create(ctx, client, &entities.TicketNote{Title: autotask.Set("created")})
	if err != nil {
		t.Fatal(err)
	}
	if id, _ := created.ID.Get(); id != big+1 {
		t.Fatalf("created id = %d, want %d", id, big+1)
	}
	// big is odd and above 2^53, so a float64 round trip would change it.
	for _, id := range []int64{big, big + 1} {
		if _, err := autotask.Update(ctx, client, &entities.TicketNote{ID: autotask.Set(id), Title: autotask.Set(fmt.Sprintf("edited %d", id))}); err != nil {
			t.Fatalf("update %d: %v", id, err)
		}
	}
	for id, want := range map[int64]string{big: fmt.Sprintf("edited %d", big), big + 1: fmt.Sprintf("edited %d", big+1)} {
		got, err := autotask.Get[entities.TicketNote](ctx, client, id)
		if err != nil {
			t.Fatalf("get %d: %v", id, err)
		}
		if gotID, _ := got.ID.Get(); gotID != id {
			t.Errorf("get %d returned id %d", id, gotID)
		}
		if title, _ := got.Title.Get(); title != want {
			t.Errorf("get %d title = %q, want %q", id, title, want)
		}
	}
}

// TestCreateWithInvalidBodyAnswers400 pins that a create body that is not one
// JSON object answers 400: null (a nil map that used to panic on the id
// assignment), a truncated object, and an object followed by more data.
func TestCreateWithInvalidBodyAnswers400(t *testing.T) {
	ts, _ := NewServer(t)
	for _, body := range []string{"null", `{"title":`, `{"title":"a"} x`} {
		req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, ts.URL+"/v1.0/TicketNotes", strings.NewReader(body))
		if err != nil {
			t.Fatalf("new request: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("UserName", "test-user")
		req.Header.Set("Secret", "test-secret")
		req.Header.Set("ApiIntegrationCode", "test-code")
		resp, err := ts.Client().Do(req)
		if err != nil {
			t.Fatalf("%s: do: %v", body, err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", body, resp.StatusCode)
		}
	}
}
