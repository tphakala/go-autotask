package autotasktest

import (
	"context"
	"errors"
	"net/http"
	"testing"

	autotask "github.com/tphakala/go-autotask"
	"github.com/tphakala/go-autotask/entities"
	"github.com/tphakala/go-autotask/metadata"
)

// wantBadRequest fails the test unless err is a 400 *autotask.Error.
func wantBadRequest(t *testing.T, what string, err error) {
	t.Helper()
	apiErr, ok := errors.AsType[*autotask.Error](err)
	if !ok || apiErr.StatusCode != http.StatusBadRequest {
		t.Errorf("%s: err = %v, want a 400 *autotask.Error", what, err)
	}
}

// wantOK fails the test when err is not nil.
func wantOK(t *testing.T, what string, err error) {
	t.Helper()
	if err != nil {
		t.Errorf("%s: %v", what, err)
	}
}

// allItems matches every item of an entity.
func allItems() *autotask.Query { return autotask.NewQuery().Where("id", autotask.OpGt, 0) }

// noStore fails the test when the server holds a store named name.
func noStore(t *testing.T, ts *TestServer, names ...string) {
	t.Helper()
	for _, name := range names {
		if _, ok := ts.entities[name]; ok {
			t.Errorf("a rejected write created the store %s", name)
		}
	}
}

// TestChildOnlyWritesAllowOnlyTheTypedChildRoute pins that a marked type is
// written only on the URL the typed child calls send: root writes, and child
// writes that resolve to its store under another parent or segment, answer 400,
// without creating a store or an item, and reads are not gated.
func TestChildOnlyWritesAllowOnlyTheTypedChildRoute(t *testing.T) {
	t.Run("QuoteItem under Quote", func(t *testing.T) {
		ts, client := NewServer(t,
			WithChildOnlyWrites[entities.Quote, entities.QuoteItem](),
			WithDeleteSupport("QuoteItems"),
		)
		ctx := t.Context()
		item := entities.QuoteItem{Name: autotask.Set("line")}
		if _, err := autotask.CreateChild[entities.Quote](ctx, client, 5, &item); err != nil {
			t.Fatalf("CreateChild: %v", err)
		}
		id, _ := item.ID.Get()
		item.Name = autotask.Set("renamed")
		_, err := autotask.UpdateChild[entities.Quote](ctx, client, 5, &item)
		wantOK(t, "UpdateChild", err)
		extra := entities.QuoteItem{Name: autotask.Set("extra")}
		_, err = autotask.CreateChild[entities.Quote](ctx, client, 5, &extra)
		wantOK(t, "second CreateChild", err)
		extraID, _ := extra.ID.Get()
		wantOK(t, "DeleteChild", autotask.DeleteChild[entities.Quote, entities.QuoteItem](ctx, client, 5, extraID))

		_, err = autotask.Create(ctx, client, &entities.QuoteItem{Name: autotask.Set("root")})
		wantBadRequest(t, "root Create", err)
		_, err = autotask.Update(ctx, client, &entities.QuoteItem{ID: autotask.Set(id), Name: autotask.Set("root")})
		wantBadRequest(t, "root Update", err)
		wantBadRequest(t, "root Delete", autotask.Delete[entities.QuoteItem](ctx, client, id))
		_, err = autotask.CreateChildRaw(ctx, client, "Quotes", 5, "QuoteItems", map[string]any{"name": "old url"})
		wantBadRequest(t, "Quotes/QuoteItems create", err)
		_, err = autotask.CreateChildRaw(ctx, client, "Quote", 5, "Items", map[string]any{"name": "singular parent"})
		wantBadRequest(t, "Quote/Items create", err)
		_, err = autotask.UpdateChildRaw(ctx, client, "Quotes", 5, "QuoteItems", map[string]any{"id": id, "name": "old url"})
		wantBadRequest(t, "Quotes/QuoteItems update", err)
		wantBadRequest(t, "Quotes/QuoteItems delete", autotask.DeleteChildRaw(ctx, client, "Quotes", 5, "QuoteItems", id))

		got, err := autotask.List[entities.QuoteItem](ctx, client, allItems())
		if err != nil {
			t.Fatalf("root List: %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("root List = %d items, want the 1 written through the allowed route", len(got))
		}
		if name, _ := got[0].Name.Get(); name != "renamed" {
			t.Errorf("stored name = %q, want renamed", name)
		}
		noStore(t, ts, "QuoteQuoteItems")
	})

	t.Run("ExpenseItem under ExpenseReport", func(t *testing.T) {
		ts, client := NewServer(t, WithChildOnlyWrites[entities.ExpenseReport, entities.ExpenseItem]())
		ctx := t.Context()
		item := entities.ExpenseItem{Origin: autotask.Set("home")}
		if _, err := autotask.CreateChild[entities.ExpenseReport](ctx, client, 3, &item); err != nil {
			t.Fatalf("CreateChild: %v", err)
		}
		_, err := autotask.UpdateChild[entities.ExpenseReport](ctx, client, 3, &item)
		wantOK(t, "UpdateChild", err)

		_, err = autotask.CreateChildRaw(ctx, client, "ExpenseReports", 3, "ExpenseItems", map[string]any{"origin": "v1.9.0 url"})
		wantBadRequest(t, "ExpenseReports/ExpenseItems create", err)
		_, err = autotask.CreateChildRaw(ctx, client, "ExpenseReports", 3, "Items", map[string]any{"origin": "wrong parent"})
		wantBadRequest(t, "ExpenseReports/Items create", err)
		_, err = autotask.Create(ctx, client, &entities.ExpenseItem{Origin: autotask.Set("root")})
		wantBadRequest(t, "root Create", err)

		got, err := autotask.List[entities.ExpenseItem](ctx, client, allItems())
		if err != nil {
			t.Fatalf("root List: %v", err)
		}
		if len(got) != 1 {
			t.Errorf("root List = %d items, want 1", len(got))
		}
		noStore(t, ts, "ExpenseReportItems")
	})

	t.Run("Contact under Company and Project", func(t *testing.T) {
		_, client := NewServer(t,
			WithChildOnlyWrites[entities.Company, entities.Contact](),
			WithChildOnlyWrites[entities.Project, entities.Contact](),
		)
		ctx := t.Context()
		contact := entities.Contact{FirstName: autotask.Set("Ada")}
		if _, err := autotask.CreateChild[entities.Company](ctx, client, 9, &contact); err != nil {
			t.Fatalf("CreateChild under Company: %v", err)
		}
		second := entities.Contact{FirstName: autotask.Set("Grace")}
		if _, err := autotask.CreateChild[entities.Project](ctx, client, 4, &second); err != nil {
			t.Fatalf("CreateChild under Project: %v", err)
		}
		_, err := autotask.CreateChild[entities.Ticket](ctx, client, 2, &entities.Contact{FirstName: autotask.Set("other")})
		wantBadRequest(t, "Tickets/Contacts create", err)
		_, err = autotask.Create(ctx, client, &entities.Contact{FirstName: autotask.Set("root")})
		wantBadRequest(t, "root Create", err)
		got, err := autotask.List[entities.Contact](ctx, client, allItems())
		if err != nil {
			t.Fatalf("root List: %v", err)
		}
		if len(got) != 2 {
			t.Errorf("root List = %d items, want the 2 written under Company and Project", len(got))
		}
	})
}

// TestChildOnlyWritesLeaveOtherURLsLenient pins that the gate follows the
// resolved store: a URL that resolves to an unmarked store, a segment no marked
// type uses, and a root write of an unmarked type keep their behavior without
// the option.
func TestChildOnlyWritesLeaveOtherURLsLenient(t *testing.T) {
	ctx := t.Context()
	ts, client := NewServer(t,
		WithChildOnlyWrites[entities.Company, entities.Contact](),
		WithChildOnlyWrites[entities.Quote, entities.QuoteItem](),
	)

	// /ContactGroups/{id}/Contacts shares the segment of the marked Contact.
	member := entities.ContactGroupContact{ContactID: autotask.Set[int64](4)}
	if _, err := autotask.CreateChild[entities.ContactGroup](ctx, client, 2, &member); err != nil {
		t.Errorf("CreateChild of ContactGroupContact: %v", err)
	}
	if store, ok := ts.entities["ContactGroupContacts"]; !ok || len(store.items) != 1 {
		t.Errorf("ContactGroupContacts store = %v, want one item", ok)
	}
	// No marked type uses the segment Bogus, so the junk-store fallback stays.
	_, err := autotask.CreateChildRaw(ctx, client, "Quotes", 5, "Bogus", map[string]any{"name": "x"})
	wantOK(t, "Quotes/Bogus create", err)
	_, err = autotask.Create(ctx, client, &entities.Ticket{Title: autotask.Set("root")})
	wantOK(t, "root Create of an unmarked type", err)
}

// TestWriteSupportGatesAndEntityInfo pins that WithoutUpdateSupport and
// WithoutCreateSupport reject the write at root and child URLs, that the default
// entityInformation answer reports the flags, and that explicit metadata wins.
func TestWriteSupportGatesAndEntityInfo(t *testing.T) {
	type flags struct{ create, update, delete bool }
	seedLink := entities.ChangeRequestLink{ID: autotask.Set[int64](1)}
	seedResource := entities.Resource{ID: autotask.Set[int64](1)}
	seedSkill := entities.ResourceSkill{ID: autotask.Set[int64](7)}
	for _, tt := range []struct {
		name     string
		opts     []ServerOption
		fail     func(context.Context, *autotask.Client) error // nil when the row has none
		succeed  func(context.Context, *autotask.Client) error // nil when the row has none
		infoName string
		want     flags
	}{
		{
			name: "without update",
			opts: []ServerOption{WithEntity(seedLink), WithoutUpdateSupport("ChangeRequestLinks")},
			fail: func(ctx context.Context, c *autotask.Client) error {
				_, err := autotask.Update(ctx, c, &entities.ChangeRequestLink{ID: autotask.Set[int64](1)})
				return err
			},
			succeed: func(ctx context.Context, c *autotask.Client) error {
				_, err := autotask.Create(ctx, c, &entities.ChangeRequestLink{})
				return err
			},
			infoName: "ChangeRequestLinks",
			want:     flags{create: true},
		},
		{
			name: "without create",
			opts: []ServerOption{WithEntity(seedResource), WithoutCreateSupport("Resources")},
			fail: func(ctx context.Context, c *autotask.Client) error {
				_, err := autotask.Create(ctx, c, &entities.Resource{})
				return err
			},
			succeed: func(ctx context.Context, c *autotask.Client) error {
				_, err := autotask.Update(ctx, c, &entities.Resource{ID: autotask.Set[int64](1)})
				return err
			},
			infoName: "Resources",
			want:     flags{update: true},
		},
		{
			name: "without create on a child URL",
			opts: []ServerOption{
				WithChildOnlyWrites[entities.Resource, entities.ResourceSkill](),
				WithoutCreateSupport("ResourceSkills"),
				WithChildEntity[entities.Resource](1, seedSkill),
			},
			fail: func(ctx context.Context, c *autotask.Client) error {
				_, err := autotask.CreateChild[entities.Resource](ctx, c, 1, &entities.ResourceSkill{})
				return err
			},
			succeed: func(ctx context.Context, c *autotask.Client) error {
				_, err := autotask.UpdateChild[entities.Resource](ctx, c, 1, &entities.ResourceSkill{ID: autotask.Set[int64](7)})
				return err
			},
			infoName: "ResourceSkills",
			want:     flags{update: true},
		},
		{
			name: "without update on a child URL",
			opts: []ServerOption{
				WithoutUpdateSupport("ResourceSkills"),
				WithChildEntity[entities.Resource](1, seedSkill),
			},
			fail: func(ctx context.Context, c *autotask.Client) error {
				_, err := autotask.UpdateChild[entities.Resource](ctx, c, 1, &entities.ResourceSkill{ID: autotask.Set[int64](7)})
				return err
			},
			succeed: func(ctx context.Context, c *autotask.Client) error {
				_, err := autotask.CreateChild[entities.Resource](ctx, c, 1, &entities.ResourceSkill{})
				return err
			},
			infoName: "ResourceSkills",
			want:     flags{create: true},
		},
		{
			name:     "delete support",
			opts:     []ServerOption{WithDeleteSupport("QuoteItems")},
			infoName: "QuoteItems",
			want:     flags{create: true, update: true, delete: true},
		},
		{
			name: "explicit metadata wins",
			opts: []ServerOption{
				WithEntity(seedLink),
				WithEntityMetadata("ChangeRequestLinks", EntityInfoResponse{Name: "ChangeRequestLinks", CanUpdate: true}, nil, nil),
				WithoutUpdateSupport("ChangeRequestLinks"),
			},
			fail: func(ctx context.Context, c *autotask.Client) error {
				_, err := autotask.Update(ctx, c, &entities.ChangeRequestLink{ID: autotask.Set[int64](1)})
				return err
			},
			infoName: "ChangeRequestLinks",
			want:     flags{update: true},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, client := NewServer(t, tt.opts...)
			ctx := t.Context()
			if tt.fail != nil {
				wantBadRequest(t, "gated write", tt.fail(ctx, client))
			}
			if tt.succeed != nil {
				wantOK(t, "ungated write", tt.succeed(ctx, client))
			}
			info, err := metadata.GetEntityInfo(ctx, client, tt.infoName)
			if err != nil {
				t.Fatal(err)
			}
			got := flags{info.CanCreate, info.CanUpdate, info.CanDelete}
			if got != tt.want {
				t.Errorf("entityInformation flags (create, update, delete) = %+v, want %+v", got, tt.want)
			}
		})
	}
}

// TestChildMetadataAnswersForChildEntity pins that the metadata paths under a
// child URL answer for the entity that URL resolves to, and that a metadata
// read creates no store.
func TestChildMetadataAnswersForChildEntity(t *testing.T) {
	const childPath = "/v1.0/Tickets/5/AdditionalConfigurationItems/entityInformation"
	const entityName = "TicketAdditionalConfigurationItems"

	for _, tt := range []struct {
		name       string
		opts       []ServerOption
		path       string
		wantName   string
		wantDelete bool
		noStore    string // a store the read must not create
	}{
		{name: "default answer names the child entity and creates no store", path: childPath, wantName: entityName, noStore: entityName},
		{
			name: "segment store answers with its delete flag", opts: []ServerOption{WithDeleteSupport("Gadgets")},
			path: "/v1.0/Widgets/42/Gadgets/entityInformation", wantName: "Gadgets", wantDelete: true,
		},
		{
			name: "a WithChildEntity route names its store", opts: []ServerOption{WithChildEntity[widgetReport](42, gadget{ID: autotask.Set(int64(5))})},
			path: "/v1.0/Widgets/42/Items/entityInformation", wantName: "Gadgets",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ts, client := NewServer(t, tt.opts...)
			ctx := t.Context()
			var resp struct {
				Info struct {
					Name      string `json:"name"`
					CanDelete bool   `json:"canDelete"`
				} `json:"info"`
			}
			if err := client.Do(ctx, http.MethodGet, tt.path, nil, &resp); err != nil {
				t.Fatal(err)
			}
			if resp.Info.Name != tt.wantName || resp.Info.CanDelete != tt.wantDelete {
				t.Errorf("entityInformation = %+v, want name %q and canDelete %v", resp.Info, tt.wantName, tt.wantDelete)
			}
			if tt.noStore != "" {
				noStore(t, ts, tt.noStore)
			}
		})
	}

	t.Run("explicit metadata applies to all three paths", func(t *testing.T) {
		_, client := NewServer(t, WithEntityMetadata(entityName,
			EntityInfoResponse{Name: entityName, CanDelete: true},
			[]FieldInfoResponse{{Name: "ticketID"}},
			[]UDFInfoResponse{{Name: "udf"}},
		))
		ctx := t.Context()
		var info struct {
			Info struct {
				CanDelete bool `json:"canDelete"`
			} `json:"info"`
		}
		if err := client.Do(ctx, http.MethodGet, childPath, nil, &info); err != nil {
			t.Fatal(err)
		}
		if !info.Info.CanDelete {
			t.Error("child entityInformation canDelete = false, want true from WithEntityMetadata")
		}
		var fields, udfs struct {
			Fields []struct {
				Name string `json:"name"`
			} `json:"fields"`
		}
		if err := client.Do(ctx, http.MethodGet, childPath+"/fields", nil, &fields); err != nil {
			t.Fatal(err)
		}
		if len(fields.Fields) != 1 || fields.Fields[0].Name != "ticketID" {
			t.Errorf("child fields = %+v, want only ticketID", fields.Fields)
		}
		if err := client.Do(ctx, http.MethodGet, childPath+"/userDefinedFields", nil, &udfs); err != nil {
			t.Fatal(err)
		}
		if len(udfs.Fields) != 1 || udfs.Fields[0].Name != "udf" {
			t.Errorf("child userDefinedFields = %+v, want only udf", udfs.Fields)
		}
	})
}
