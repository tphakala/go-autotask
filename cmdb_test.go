package autotask_test

import (
	"context"
	"net/http"
	"slices"
	"testing"

	autotask "github.com/tphakala/go-autotask"
	"github.com/tphakala/go-autotask/autotasktest"
	"github.com/tphakala/go-autotask/entities"
)

// cmdbCase is one typed call that reaches a parent-URL operation, with the
// request it must send.
type cmdbCase struct {
	name       string
	opts       []autotasktest.ServerOption
	wantMethod string
	wantPath   string
	run        func(ctx context.Context, c *autotask.Client) error
}

// cmdbCreate builds a case that creates child under parent parentID with
// CreateChild.
func cmdbCreate[P, C autotask.Entity](name string, parentID int64, wantPath string, child C) cmdbCase {
	return cmdbCase{
		name:       name,
		wantMethod: http.MethodPost,
		wantPath:   wantPath,
		run: func(ctx context.Context, c *autotask.Client) error {
			_, err := autotask.CreateChild[P](ctx, c, parentID, &child)
			return err
		},
	}
}

// cmdbUpdate builds a case that updates a seeded child with UpdateChild. The
// child carries its id, which UpdateChild sends in the body.
func cmdbUpdate[P, C autotask.Entity](name string, parentID int64, wantPath string, child C) cmdbCase {
	return cmdbCase{
		name:       name,
		opts:       []autotasktest.ServerOption{autotasktest.WithChildEntity[P](parentID, child)},
		wantMethod: http.MethodPatch,
		wantPath:   wantPath,
		run: func(ctx context.Context, c *autotask.Client) error {
			_, err := autotask.UpdateChild[P](ctx, c, parentID, &child)
			return err
		},
	}
}

// cmdbDelete builds a case that deletes the seeded child with id childID using
// DeleteChild.
func cmdbDelete[P, C autotask.Entity](name string, parentID, childID int64, wantPath string, child C) cmdbCase {
	var zero C
	return cmdbCase{
		name: name,
		opts: []autotasktest.ServerOption{
			autotasktest.WithChildEntity[P](parentID, child),
			autotasktest.WithDeleteSupport(zero.EntityName()),
		},
		wantMethod: http.MethodDelete,
		wantPath:   wantPath,
		run: func(ctx context.Context, c *autotask.Client) error {
			return autotask.DeleteChild[P, C](ctx, c, parentID, childID)
		},
	}
}

// TestConfigurationItemChildOperations checks that each operation the REST
// swagger lists on a parent URL of a configuration item type is reachable with
// the typed child call, and that the call sends the expected request. DNS
// records and SSL subject alternative names have no create in the API, so they
// are seeded.
func TestConfigurationItemChildOperations(t *testing.T) {
	t.Parallel()
	const id = int64(5)
	setID := autotask.Set(id)

	tests := []cmdbCase{
		cmdbCreate[entities.ConfigurationItemCategory]("UdfAssociation create", 1,
			"/v1.0/ConfigurationItemCategories/1/UdfAssociations",
			entities.ConfigurationItemCategoryUdfAssociation{IsRequired: autotask.Set(true)}),
		cmdbUpdate[entities.ConfigurationItemCategory]("UdfAssociation update", 1,
			"/v1.0/ConfigurationItemCategories/1/UdfAssociations",
			entities.ConfigurationItemCategoryUdfAssociation{ID: setID, IsRequired: autotask.Set(false)}),
		cmdbDelete[entities.ConfigurationItemCategory]("UdfAssociation delete", 1, id,
			"/v1.0/ConfigurationItemCategories/1/UdfAssociations/5",
			entities.ConfigurationItemCategoryUdfAssociation{ID: setID}),

		cmdbCreate[entities.ConfigurationItem]("Note create", 1,
			"/v1.0/ConfigurationItems/1/Notes",
			entities.ConfigurationItemNote{Title: autotask.Set("note")}),
		cmdbUpdate[entities.ConfigurationItem]("Note update", 1,
			"/v1.0/ConfigurationItems/1/Notes",
			entities.ConfigurationItemNote{ID: setID, Title: autotask.Set("edited")}),

		cmdbCreate[entities.ConfigurationItem]("Attachment create", 1,
			"/v1.0/ConfigurationItems/1/Attachments",
			entities.ConfigurationItemAttachment{Title: autotask.Set("a.txt"), Data: autotask.Set("AAAA")}),
		cmdbDelete[entities.ConfigurationItem]("Attachment delete", 1, id,
			"/v1.0/ConfigurationItems/1/Attachments/5",
			entities.ConfigurationItemAttachment{ID: setID}),

		cmdbCreate[entities.ConfigurationItemNote]("NoteAttachment create", 2,
			"/v1.0/ConfigurationItemNotes/2/Attachments",
			entities.ConfigurationItemNoteAttachment{Title: autotask.Set("a.txt"), Data: autotask.Set("AAAA")}),
		cmdbDelete[entities.ConfigurationItemNote]("NoteAttachment delete", 2, id,
			"/v1.0/ConfigurationItemNotes/2/Attachments/5",
			entities.ConfigurationItemNoteAttachment{ID: setID}),

		cmdbCreate[entities.ConfigurationItem]("RelatedItem create", 1,
			"/v1.0/ConfigurationItems/1/RelatedItems",
			entities.ConfigurationItemRelatedItem{RelatedConfigurationItemID: autotask.Set(int64(9))}),
		cmdbDelete[entities.ConfigurationItem]("RelatedItem delete", 1, id,
			"/v1.0/ConfigurationItems/1/RelatedItems/5",
			entities.ConfigurationItemRelatedItem{ID: setID}),

		cmdbCreate[entities.ConfigurationItem]("BillingProductAssociation create", 1,
			"/v1.0/ConfigurationItems/1/BillingProductAssociations",
			entities.ConfigurationItemBillingProductAssociation{BillingProductID: autotask.Set(int64(7))}),
		cmdbUpdate[entities.ConfigurationItem]("BillingProductAssociation update", 1,
			"/v1.0/ConfigurationItems/1/BillingProductAssociations",
			entities.ConfigurationItemBillingProductAssociation{ID: setID, BillingProductID: autotask.Set(int64(8))}),
		cmdbDelete[entities.ConfigurationItem]("BillingProductAssociation delete", 1, id,
			"/v1.0/ConfigurationItems/1/BillingProductAssociations/5",
			entities.ConfigurationItemBillingProductAssociation{ID: setID}),

		cmdbDelete[entities.ConfigurationItem]("DnsRecord delete", 1, id,
			"/v1.0/ConfigurationItems/1/DnsRecords/5",
			entities.ConfigurationItemDnsRecord{ID: setID}),
		cmdbDelete[entities.ConfigurationItem]("SslSubjectAlternativeName delete", 1, id,
			"/v1.0/ConfigurationItems/1/SslSubjectAlternativeNames/5",
			entities.ConfigurationItemSslSubjectAlternativeName{ID: setID}),

		cmdbCreate[entities.Ticket]("TicketAdditionalConfigurationItem create", 3,
			"/v1.0/Tickets/3/AdditionalConfigurationItems",
			entities.TicketAdditionalConfigurationItem{ConfigurationItemID: autotask.Set(int64(1))}),
		cmdbDelete[entities.Ticket]("TicketAdditionalConfigurationItem delete", 3, id,
			"/v1.0/Tickets/3/AdditionalConfigurationItems/5",
			entities.TicketAdditionalConfigurationItem{ID: setID}),
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			srv, client := autotasktest.NewServer(t, tt.opts...)
			if err := tt.run(t.Context(), client); err != nil {
				t.Fatal(err)
			}
			req := srv.LastRequest()
			if req.Method != tt.wantMethod || req.Path != tt.wantPath {
				t.Fatalf("sent %s %s, want %s %s", req.Method, req.Path, tt.wantMethod, tt.wantPath)
			}
		})
	}
}

// TestConfigurationItemReadOnlyChildren lists DNS records and SSL subject
// alternative names, which the API only returns, under their own parent.
func TestConfigurationItemReadOnlyChildren(t *testing.T) {
	t.Parallel()
	srv, client := autotasktest.NewServer(t,
		autotasktest.WithChildEntity[entities.ConfigurationItem](1,
			entities.ConfigurationItemDnsRecord{ID: autotask.Set(int64(1)), Data: autotask.Set("one.example")}),
		autotasktest.WithChildEntity[entities.ConfigurationItem](2,
			entities.ConfigurationItemDnsRecord{ID: autotask.Set(int64(2)), Data: autotask.Set("two.example")}),
		autotasktest.WithChildEntity[entities.ConfigurationItem](1,
			entities.ConfigurationItemSslSubjectAlternativeName{ID: autotask.Set(int64(1)), Name: autotask.Set("san.example")}),
	)

	dns, err := autotask.ListChild[entities.ConfigurationItem, entities.ConfigurationItemDnsRecord](t.Context(), client, 1)
	if err != nil {
		t.Fatal(err)
	}
	if req := srv.LastRequest(); req.Method != http.MethodGet || req.Path != "/v1.0/ConfigurationItems/1/DnsRecords" {
		t.Fatalf("sent %s %s, want GET /v1.0/ConfigurationItems/1/DnsRecords", req.Method, req.Path)
	}
	if len(dns) != 1 {
		t.Fatalf("listed %d DNS records under CI 1, want 1", len(dns))
	}
	if got, _ := dns[0].Data.Get(); got != "one.example" {
		t.Fatalf("DNS record data = %q, want one.example", got)
	}

	sans, err := autotask.ListChild[entities.ConfigurationItem, entities.ConfigurationItemSslSubjectAlternativeName](t.Context(), client, 1)
	if err != nil {
		t.Fatal(err)
	}
	if req := srv.LastRequest(); req.Method != http.MethodGet || req.Path != "/v1.0/ConfigurationItems/1/SslSubjectAlternativeNames" {
		t.Fatalf("sent %s %s, want GET /v1.0/ConfigurationItems/1/SslSubjectAlternativeNames", req.Method, req.Path)
	}
	if len(sans) != 1 {
		t.Fatalf("listed %d subject alternative names under CI 1, want 1", len(sans))
	}
}

// attachmentTitles returns the titles of the listed attachments.
func attachmentTitles[T any](items []*T, title func(*T) autotask.Optional[string]) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		v, _ := title(item).Get()
		out = append(out, v)
	}
	return out
}

// TestConfigurationItemAttachmentsStayApart checks that attachments of the
// configuration item, configuration item note and ticket note types, which
// share the Attachments segment, are each listed only for their own type. All
// three sit under the parent id 1, so a list that mixed types would show it.
func TestConfigurationItemAttachmentsStayApart(t *testing.T) {
	t.Parallel()
	_, client := autotasktest.NewServer(t)
	ctx := t.Context()

	if _, err := autotask.CreateChild[entities.ConfigurationItem](ctx, client, 1,
		&entities.ConfigurationItemAttachment{Title: autotask.Set("ci attachment")}); err != nil {
		t.Fatal(err)
	}
	if _, err := autotask.CreateChild[entities.ConfigurationItemNote](ctx, client, 1,
		&entities.ConfigurationItemNoteAttachment{Title: autotask.Set("ci note attachment")}); err != nil {
		t.Fatal(err)
	}
	if _, err := autotask.CreateChild[entities.TicketNote](ctx, client, 1,
		&entities.TicketNoteAttachment{Title: autotask.Set("ticket note attachment")}); err != nil {
		t.Fatal(err)
	}

	ciAtt, err := autotask.ListChild[entities.ConfigurationItem, entities.ConfigurationItemAttachment](ctx, client, 1)
	if err != nil {
		t.Fatal(err)
	}
	ciNoteAtt, err := autotask.ListChild[entities.ConfigurationItemNote, entities.ConfigurationItemNoteAttachment](ctx, client, 1)
	if err != nil {
		t.Fatal(err)
	}
	ticketNoteAtt, err := autotask.ListChild[entities.TicketNote, entities.TicketNoteAttachment](ctx, client, 1)
	if err != nil {
		t.Fatal(err)
	}
	otherCI, err := autotask.ListChild[entities.ConfigurationItem, entities.ConfigurationItemAttachment](ctx, client, 2)
	if err != nil {
		t.Fatal(err)
	}

	checks := []struct {
		name string
		got  []string
		want []string
	}{
		{"CI 1 attachments", attachmentTitles(ciAtt, func(a *entities.ConfigurationItemAttachment) autotask.Optional[string] { return a.Title }), []string{"ci attachment"}},
		{"CI note 1 attachments", attachmentTitles(ciNoteAtt, func(a *entities.ConfigurationItemNoteAttachment) autotask.Optional[string] { return a.Title }), []string{"ci note attachment"}},
		{"ticket note 1 attachments", attachmentTitles(ticketNoteAtt, func(a *entities.TicketNoteAttachment) autotask.Optional[string] { return a.Title }), []string{"ticket note attachment"}},
		{"CI 2 attachments", attachmentTitles(otherCI, func(a *entities.ConfigurationItemAttachment) autotask.Optional[string] { return a.Title }), []string{}},
	}
	for _, c := range checks {
		if !slices.Equal(c.got, c.want) {
			t.Errorf("%s = %v, want %v", c.name, c.got, c.want)
		}
	}
}

// TestConfigurationItemFixtureSetsProductID checks that the fixture carries
// ProductID, the field the API requires when creating a configuration item,
// through the mock server's JSON round trip.
func TestConfigurationItemFixtureSetsProductID(t *testing.T) {
	t.Parallel()
	ci := autotasktest.ConfigurationItemFixture()
	_, client := autotasktest.NewServer(t, autotasktest.WithEntity(ci))
	id, _ := ci.ID.Get()

	got, err := autotask.Get[entities.ConfigurationItem](t.Context(), client, id)
	if err != nil {
		t.Fatal(err)
	}
	if productID, ok := got.ProductID.Get(); !ok || productID == 0 {
		t.Fatalf("ProductID = %d (set=%v), want the fixture's value", productID, ok)
	}
}
