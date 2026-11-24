package autotasktest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"math"
	"net/http"
	"path/filepath"
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
	if _, err := autotask.CreateChild[entities.TicketNote](ctx, client, 2, &noteAtt); err != nil {
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

	// Both attachment types use the segment Attachments and parent id 2;
	// neither sees the other, so the stores are separate per type.
	noteAtts, err := autotask.ListChild[entities.TicketNote, entities.TicketNoteAttachment](ctx, client, 2)
	if err != nil {
		t.Fatal(err)
	}
	assertTitles(t, "note 2 attachments", titlesOf(noteAtts, noteAttachmentTitle), []string{"note attachment"})
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

// contactNames returns the first names of the contacts, sorted.
func contactNames(items []*entities.Contact) []string {
	return titlesOf(items, func(c *entities.Contact) autotask.Optional[string] { return c.FirstName })
}

// TestChildItemsBelongToParentCollection pins that a child item belongs to the
// parent collection and id of the URL it was created at, or of the route
// WithChildEntity registered. Companies/9/Contacts and Opportunities/9/Contacts
// reach one Contacts store, so the parent id alone cannot tell them apart.
func TestChildItemsBelongToParentCollection(t *testing.T) {
	t.Run("created through child URLs", func(t *testing.T) {
		_, client := NewServer(t, WithDeleteSupport("Contacts"))
		ctx := t.Context()
		company := entities.Contact{FirstName: autotask.Set("company")}
		if _, err := autotask.CreateChild[entities.Company](ctx, client, 9, &company); err != nil {
			t.Fatal(err)
		}
		if _, err := autotask.CreateChild[entities.Opportunity](ctx, client, 9, &entities.Contact{FirstName: autotask.Set("opportunity")}); err != nil {
			t.Fatal(err)
		}
		listCompany := func() []string {
			t.Helper()
			list, err := autotask.ListChild[entities.Company, entities.Contact](ctx, client, 9)
			if err != nil {
				t.Fatal(err)
			}
			return contactNames(list)
		}
		listOpportunity := func() []string {
			t.Helper()
			list, err := autotask.ListChild[entities.Opportunity, entities.Contact](ctx, client, 9)
			if err != nil {
				t.Fatal(err)
			}
			return contactNames(list)
		}
		assertTitles(t, "Companies/9/Contacts", listCompany(), []string{"company"})
		assertTitles(t, "Opportunities/9/Contacts", listOpportunity(), []string{"opportunity"})

		id, _ := company.ID.Get()
		var resp struct {
			Item json.RawMessage `json:"item"`
		}
		if err := client.Do(ctx, http.MethodGet, fmt.Sprintf("/v1.0/Opportunities/9/Contacts/%d", id), nil, &resp); err != nil {
			t.Fatal(err)
		}
		if string(resp.Item) != "null" {
			t.Errorf("GET of a Companies contact under Opportunities = %s, want null", resp.Item)
		}
		_, err := autotask.UpdateChild[entities.Opportunity](ctx, client, 9, &entities.Contact{ID: autotask.Set(id), FirstName: autotask.Set("hijack")})
		if _, ok := errors.AsType[*autotask.NotFoundError](err); !ok {
			t.Errorf("UpdateChild under the wrong parent collection err = %v, want *NotFoundError", err)
		}
		err = autotask.DeleteChild[entities.Opportunity, entities.Contact](ctx, client, 9, id)
		if _, ok := errors.AsType[*autotask.NotFoundError](err); !ok {
			t.Errorf("DeleteChild under the wrong parent collection err = %v, want *NotFoundError", err)
		}
		assertTitles(t, "Companies/9/Contacts after the refused writes", listCompany(), []string{"company"})
	})

	t.Run("parent id 0 means no parent", func(t *testing.T) {
		_, client := NewServer(t,
			WithChildEntity[entities.Company](0, entities.Contact{ID: autotask.Set(int64(1)), FirstName: autotask.Set("seeded at 0")}),
		)
		ctx := t.Context()
		if _, err := autotask.CreateChild[entities.Company](ctx, client, 0, &entities.Contact{FirstName: autotask.Set("created at 0")}); err != nil {
			t.Fatal(err)
		}
		list, err := autotask.ListChild[entities.Opportunity, entities.Contact](ctx, client, 9)
		if err != nil {
			t.Fatal(err)
		}
		assertTitles(t, "Opportunities/9/Contacts", contactNames(list), []string{"created at 0", "seeded at 0"})
	})

	t.Run("seeded through two routes", func(t *testing.T) {
		contact := func(id int64, name string) entities.Contact {
			return entities.Contact{ID: autotask.Set(id), FirstName: autotask.Set(name)}
		}
		_, client := NewServer(t,
			WithChildEntity[entities.Company](9, contact(1, "company")),
			WithChildEntity[entities.Project](9, contact(2, "project")),
			WithEntity(contact(3, "seeded")),
		)
		ctx := t.Context()
		companies, err := autotask.ListChild[entities.Company, entities.Contact](ctx, client, 9)
		if err != nil {
			t.Fatal(err)
		}
		assertTitles(t, "Companies/9/Contacts", contactNames(companies), []string{"company", "seeded"})
		projects, err := autotask.ListChild[entities.Project, entities.Contact](ctx, client, 9)
		if err != nil {
			t.Fatal(err)
		}
		assertTitles(t, "Projects/9/Contacts", contactNames(projects), []string{"project", "seeded"})
	})
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
	// WithChildOnlyWrites registers its route with the same conflict check.
	WithChildOnlyWrites[entities.Ticket, entities.ProjectNote]()(ts)
	if len(rec.fatal) != 2 || !strings.Contains(rec.fatal[1], "Tickets/Notes") {
		t.Fatalf("fatal messages = %v, want a second naming Tickets/Notes", rec.fatal)
	}
	if got := ts.childRoutes["Tickets/Notes"]; got != "TicketNotes" {
		t.Errorf("route Tickets/Notes after WithChildOnlyWrites = %q, want TicketNotes", got)
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

// TestChildQueryAndCountSeeChildListItems pins that a query or count on a child
// URL sees the items the child list at that URL shows (those under its parent
// plus ownerless ones) and applies its filter to them, and that root URLs still
// see every item. A child URL that resolves to no store answers an empty page
// and a count of 0.
func TestChildQueryAndCountSeeChildListItems(t *testing.T) {
	seeded := TicketNoteFixture(func(n *entities.TicketNote) { n.Title = autotask.Set("seeded") })
	note := TicketNoteFixture(func(n *entities.TicketNote) { n.Title = autotask.Set("ticket 1") })
	contact := ContactFixture()
	_, client := NewServer(t,
		WithEntity(seeded),
		WithChildEntity[entities.Ticket](1, note),
		WithChildEntity[entities.Company](7, contact),
	)
	ctx := t.Context()
	seededID, _ := seeded.ID.Get()
	noteID, _ := note.ID.Get()
	contactID, _ := contact.ID.Get()
	all := `{"filter":[]}`

	for _, tt := range []struct {
		name string
		path string
		body string
		want []int64
	}{
		{"child derived store", "/v1.0/Tickets/1/Notes", all, []int64{seededID, noteID}},
		{"filter on the child set", "/v1.0/Tickets/1/Notes", fmt.Sprintf(`{"filter":[{"field":"id","op":"eq","value":%d}]}`, noteID), []int64{noteID}},
		{"other parent", "/v1.0/Companies/9/Contacts", all, nil},
		{"own parent", "/v1.0/Companies/7/Contacts", all, []int64{contactID}},
		{"unknown segment", "/v1.0/Gadgets/1/Zzz", all, nil},
		{"root keeps every item", "/v1.0/TicketNotes", all, []int64{seededID, noteID}},
		{"root of a store holding only owned items", "/v1.0/Contacts", all, []int64{contactID}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var page struct {
				Items []struct {
					ID int64 `json:"id"`
				} `json:"items"`
			}
			if err := client.Do(ctx, http.MethodPost, tt.path+"/query", json.RawMessage(tt.body), &page); err != nil {
				t.Fatal(err)
			}
			if page.Items == nil {
				t.Error("items is null, want an array")
			}
			got := make([]int64, 0, len(page.Items))
			for _, it := range page.Items {
				got = append(got, it.ID)
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("query ids = %v, want %v", got, tt.want)
			}

			var count struct {
				QueryCount int `json:"queryCount"`
			}
			if err := client.Do(ctx, http.MethodPost, tt.path+"/query/count", json.RawMessage(tt.body), &count); err != nil {
				t.Fatal(err)
			}
			if count.QueryCount != len(tt.want) {
				t.Errorf("queryCount = %d, want %d", count.QueryCount, len(tt.want))
			}
		})
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

// segmentNamedEntity is a type outside this module whose entity name equals the
// child segment of TicketNote ("Notes").
type segmentNamedEntity struct {
	ID autotask.Optional[int64] `json:"id,omitzero"`
}

func (segmentNamedEntity) EntityName() string { return "Notes" }

// TestKnownChildTypeIgnoresSegmentStore pins that a child URL of a module type
// does not fall back to a store named after the segment: a store "Notes" seeded
// for another type must not receive or serve TicketNote items.
func TestKnownChildTypeIgnoresSegmentStore(t *testing.T) {
	_, client := NewServer(t, WithEntity(segmentNamedEntity{ID: autotask.Set(int64(100))}))
	ctx := t.Context()

	// Nothing was created under the ticket, so a read must not be answered by
	// the seeded "Notes" store.
	before, err := autotask.ListChild[entities.Ticket, entities.TicketNote](ctx, client, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(before) != 0 {
		t.Errorf("ticket notes before any create = %d items, want 0", len(before))
	}

	note := entities.TicketNote{Title: autotask.Set("child note")}
	if _, err := autotask.CreateChild[entities.Ticket](ctx, client, 1, &note); err != nil {
		t.Fatal(err)
	}
	id, _ := note.ID.Get()
	got, err := autotask.Get[entities.TicketNote](ctx, client, id)
	if err != nil {
		t.Fatalf("Get[TicketNote] after CreateChild: %v", err)
	}
	if title, _ := got.Title.Get(); title != "child note" {
		t.Errorf("Get[TicketNote] title = %q, want %q", title, "child note")
	}
	other, err := autotask.List[segmentNamedEntity](ctx, client, autotask.NewQuery().Where("id", autotask.OpGt, 0))
	if err != nil {
		t.Fatal(err)
	}
	if len(other) != 1 {
		t.Errorf("List of the segment-named type = %d items, want 1 (the seeded one)", len(other))
	}
}

// TestOutsideTypeUsesExistingSegmentStore pins that on a URL no type in package
// entities claims, an existing store named after the segment serves a child list.
func TestOutsideTypeUsesExistingSegmentStore(t *testing.T) {
	_, client := NewServer(t, WithEntity(segmentNamedEntity{ID: autotask.Set(int64(100))}))
	var resp struct {
		Items []json.RawMessage `json:"items"`
	}
	if err := client.Do(t.Context(), http.MethodGet, "/v1.0/Gadgets/1/Notes", nil, &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Items) != 1 {
		t.Errorf("child list under an unclaimed URL = %d items, want the 1 seeded in the Notes store", len(resp.Items))
	}
}

// derivedNamedEntity is a type outside this module named singular(Gadgets)+Notes.
type derivedNamedEntity struct {
	ID autotask.Optional[int64] `json:"id,omitzero"`
}

func (derivedNamedEntity) EntityName() string { return "GadgetNotes" }

// TestOutsideTypePrefersDerivedStoreOverSegmentStore pins the order on an
// unclaimed URL: an existing singular(Parent)+Segment store wins over an existing
// store named after the segment.
func TestOutsideTypePrefersDerivedStoreOverSegmentStore(t *testing.T) {
	_, client := NewServer(t,
		WithEntity(segmentNamedEntity{ID: autotask.Set(int64(100))}),
		WithEntity(derivedNamedEntity{ID: autotask.Set(int64(200))}, derivedNamedEntity{ID: autotask.Set(int64(201))}),
	)
	var resp struct {
		Items []json.RawMessage `json:"items"`
	}
	if err := client.Do(t.Context(), http.MethodGet, "/v1.0/Gadgets/1/Notes", nil, &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Items) != 2 {
		t.Errorf("child list under an unclaimed URL = %d items, want the 2 seeded in the GadgetNotes store", len(resp.Items))
	}
}

// TestChildEntityRouteClaimsKnownURL pins the escape hatch the docs name: a type
// from outside this module registered with WithChildEntity reads back on a URL
// that otherwise belongs to a type in package entities (Tickets/Notes).
func TestChildEntityRouteClaimsKnownURL(t *testing.T) {
	_, client := NewServer(t, WithChildEntity[entities.Ticket, segmentNamedEntity](1))
	ctx := t.Context()

	item := segmentNamedEntity{}
	if _, err := autotask.CreateChild[entities.Ticket](ctx, client, 1, &item); err != nil {
		t.Fatal(err)
	}
	id, _ := item.ID.Get()
	if _, err := autotask.Get[segmentNamedEntity](ctx, client, id); err != nil {
		t.Fatalf("Get after CreateChild through the route: %v", err)
	}
	for parent, want := range map[int64]int{1: 1, 2: 0} {
		list, err := autotask.ListChild[entities.Ticket, segmentNamedEntity](ctx, client, parent)
		if err != nil {
			t.Fatal(err)
		}
		if len(list) != want {
			t.Errorf("items under ticket %d = %d, want %d", parent, len(list), want)
		}
	}
}

// TestUnseededChildCreateUsesSegmentEntity pins that a child create of a type
// whose entity name is the segment (Contact under Companies) lands in that
// type's store, which Get and List read, not in CompanyContacts.
func TestUnseededChildCreateUsesSegmentEntity(t *testing.T) {
	_, client := NewServer(t)
	ctx := t.Context()

	contact := entities.Contact{FirstName: autotask.Set("Ada")}
	if _, err := autotask.CreateChild[entities.Company](ctx, client, 1, &contact); err != nil {
		t.Fatal(err)
	}
	id, _ := contact.ID.Get()
	if _, err := autotask.Get[entities.Contact](ctx, client, id); err != nil {
		t.Fatalf("Get[Contact] after CreateChild: %v", err)
	}
	for parent, want := range map[int64]int{1: 1, 2: 0} {
		list, err := autotask.ListChild[entities.Company, entities.Contact](ctx, client, parent)
		if err != nil {
			t.Fatal(err)
		}
		if len(list) != want {
			t.Errorf("contacts under company %d = %d, want %d", parent, len(list), want)
		}
	}
}

// TestKnownEntityNamesCoverEntities pins that knownEntityNames holds the entity
// name of every type declared in the entities package, so a new entity type
// cannot silently fall back to URL-shape resolution.
func TestKnownEntityNamesCoverEntities(t *testing.T) {
	found := entityNamesDeclared(t, filepath.Join("..", "entities"))
	if len(found) == 0 {
		t.Fatal("found no entity types in ../entities")
	}
	if got, want := slices.Sorted(maps.Keys(knownEntityNames)), slices.Sorted(maps.Keys(found)); !slices.Equal(got, want) {
		t.Fatalf("knownEntityNames = %v, entity names in ../entities = %v", got, want)
	}
}

// entityNamesDeclared returns the string each EntityName method in the non-test
// files of dir returns.
func entityNamesDeclared(t *testing.T, dir string) map[string]bool {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]bool{}
	fset := token.NewFileSet()
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, file, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Recv == nil || fn.Name.Name != "EntityName" {
				continue
			}
			name, ok := returnedStringLiteral(fn)
			if !ok {
				t.Errorf("%s: EntityName does not return a single string literal, so the registry test cannot read it", file)
				continue
			}
			found[name] = true
		}
	}
	return found
}

// returnedStringLiteral returns the value of fn's body when it is a single
// "return <string literal>" statement.
func returnedStringLiteral(fn *ast.FuncDecl) (string, bool) {
	if fn.Body == nil || len(fn.Body.List) != 1 {
		return "", false
	}
	ret, ok := fn.Body.List[0].(*ast.ReturnStmt)
	if !ok || len(ret.Results) != 1 {
		return "", false
	}
	lit, ok := ret.Results[0].(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	name, err := strconv.Unquote(lit.Value)
	return name, err == nil
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

// TestQueryFiltersCompareNumbersExactly pins that the numeric operators compare
// by exact value: the two seeded ids differ only above 2^53, where a float64
// comparison or a formatted text comparison cannot tell them apart. The text
// operators see a number as it was stored, and the last rows pin that text
// values still compare as text and that an ordering operator matches nothing
// when either side is not a number.
func TestQueryFiltersCompareNumbersExactly(t *testing.T) {
	const low = int64(1) << 53
	const high = low + 1
	_, client := NewServer(t, WithEntity(
		TicketNoteFixture(func(n *entities.TicketNote) { n.ID = autotask.Set(low); n.Title = autotask.Set("low") }),
		TicketNoteFixture(func(n *entities.TicketNote) { n.ID = autotask.Set(high); n.Title = autotask.Set("high") }),
	))
	ctx := t.Context()

	for _, tt := range []struct {
		name   string
		filter string
		want   []int64
	}{
		{"eq", `{"field":"id","op":"eq","value":9007199254740993}`, []int64{high}},
		{"eq written with a fraction", `{"field":"id","op":"eq","value":9007199254740993.0}`, []int64{high}},
		{"noteq", `{"field":"id","op":"noteq","value":9007199254740993}`, []int64{low}},
		{"in", `{"field":"id","op":"in","value":[9007199254740993]}`, []int64{high}},
		{"notIn", `{"field":"id","op":"notIn","value":[9007199254740993]}`, []int64{low}},
		{"gt", `{"field":"id","op":"gt","value":9007199254740992}`, []int64{high}},
		{"gte", `{"field":"id","op":"gte","value":9007199254740993}`, []int64{high}},
		{"lt", `{"field":"id","op":"lt","value":9007199254740993}`, []int64{low}},
		{"lte", `{"field":"id","op":"lte","value":9007199254740992}`, []int64{low}},
		{"contains on a number", `{"field":"id","op":"contains","value":"199254740993"}`, []int64{high}},
		{"beginsWith on a number", `{"field":"id","op":"beginsWith","value":"9007199254740993"}`, []int64{high}},
		{"endsWith on a number", `{"field":"id","op":"endsWith","value":"992"}`, []int64{low}},
		{"eq on text", `{"field":"title","op":"eq","value":"high"}`, []int64{high}},
		{"in on text", `{"field":"title","op":"in","value":["low"]}`, []int64{low}},
		{"gt on a text field", `{"field":"title","op":"gt","value":1}`, nil},
		{"gt against a text value", `{"field":"id","op":"gt","value":"1"}`, nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			body := json.RawMessage(`{"filter":[` + tt.filter + `]}`)
			var resp struct {
				Items []struct {
					ID int64 `json:"id"`
				} `json:"items"`
			}
			if err := client.Do(ctx, http.MethodPost, "/v1.0/TicketNotes/query", body, &resp); err != nil {
				t.Fatal(err)
			}
			got := make([]int64, 0, len(resp.Items))
			for _, it := range resp.Items {
				got = append(got, it.ID)
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("ids = %v, want %v", got, tt.want)
			}
		})
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

// widgetReport is a parent type whose EntityName ("WidgetReports") is not the
// parent segment its child URLs use ("Widgets").
type widgetReport struct{}

func (widgetReport) EntityName() string { return "WidgetReports" }

// gadget is a type outside this module whose child URL is
// /v1.0/Widgets/{id}/Items although its parent type is named WidgetReports.
type gadget struct {
	ID autotask.Optional[int64] `json:"id,omitzero"`
}

func (gadget) EntityName() string      { return "Gadgets" }
func (gadget) ChildEntityName() string { return "Items" }
func (gadget) ChildParentName() string { return "Widgets" }

// noteLike has a ChildParentName but no ChildEntityName, so its child segment is
// its entity name, "Notes", which is also the child segment of TicketNote:
// /v1.0/Tickets/{id}/Notes.
type noteLike struct {
	ID autotask.Optional[int64] `json:"id,omitzero"`
}

func (noteLike) EntityName() string      { return "Notes" }
func (noteLike) ChildParentName() string { return "Tickets" }

// TestWithChildEntityRoutesChildParentName pins that WithChildEntity keys its
// route on the same parent segment the typed child calls send: ChildParentName
// when the child type has one, with the child segment from ChildEntityName or
// else the entity name. Keyed on the parent's EntityName instead, the request
// /Widgets/42/Items would derive the store WidgetItems, find none and answer an
// empty list, and /Tickets/42/Notes would derive TicketNotes, a known entity
// type without a store, and answer an empty list too.
func TestWithChildEntityRoutesChildParentName(t *testing.T) {
	tests := []struct {
		name    string
		option  ServerOption
		list    func(ctx context.Context, c *autotask.Client, parent int64) (int, error)
		parent  string
		segment string
	}{
		{
			name:   "ChildEntityName and ChildParentName",
			option: WithChildEntity[widgetReport](42, gadget{ID: autotask.Set(int64(5))}),
			list: func(ctx context.Context, c *autotask.Client, parent int64) (int, error) {
				items, err := autotask.ListChild[widgetReport, gadget](ctx, c, parent)
				return len(items), err
			},
			parent:  "Widgets",
			segment: "Items",
		},
		{
			name:   "ChildParentName only",
			option: WithChildEntity[widgetReport](42, noteLike{ID: autotask.Set(int64(5))}),
			list: func(ctx context.Context, c *autotask.Client, parent int64) (int, error) {
				items, err := autotask.ListChild[widgetReport, noteLike](ctx, c, parent)
				return len(items), err
			},
			parent:  "Tickets",
			segment: "Notes",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, client := NewServer(t, tt.option)
			ctx := t.Context()
			for parent, want := range map[int64]int{42: 1, 43: 0} {
				got, err := tt.list(ctx, client, parent)
				if err != nil {
					t.Fatal(err)
				}
				if got != want {
					t.Errorf("typed items under parent %d = %d, want %d", parent, got, want)
				}
				raw, err := autotask.ListChildRaw(ctx, client, tt.parent, parent, tt.segment)
				if err != nil {
					t.Fatal(err)
				}
				if len(raw) != want {
					t.Errorf("raw items under /%s/%d/%s = %d, want %d", tt.parent, parent, tt.segment, len(raw), want)
				}
			}
		})
	}
}
