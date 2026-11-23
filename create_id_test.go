package autotask_test

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	autotask "github.com/tphakala/go-autotask"
	"github.com/tphakala/go-autotask/autotasktest"
	"github.com/tphakala/go-autotask/entities"
)

// createIDCases lists every entity type in entities/, keyed by Go type name.
// TestCreateIDCoversAllEntities fails when a type is added there without a row.
// A row checks id assignment through the mock's root create, not that the API
// offers a root create for the type.
var createIDCases = map[string]func(*testing.T){
	"BillingCode":                 createSetsID[entities.BillingCode],
	"BillingItemApprovalLevel":    createSetsID[entities.BillingItemApprovalLevel],
	"BillingItem":                 createSetsID[entities.BillingItem],
	"ChangeRequestLink":           createSetsID[entities.ChangeRequestLink],
	"CompanyNote":                 createSetsID[entities.CompanyNote],
	"Company":                     createSetsID[entities.Company],
	"ConfigurationItem":           createSetsID[entities.ConfigurationItem],
	"ConfigurationItemAttachment": createSetsID[entities.ConfigurationItemAttachment],
	"ConfigurationItemBillingProductAssociation": createSetsID[entities.ConfigurationItemBillingProductAssociation],
	"ConfigurationItemCategoryUdfAssociation":    createSetsID[entities.ConfigurationItemCategoryUdfAssociation],
	"ConfigurationItemCategory":                  createSetsID[entities.ConfigurationItemCategory],
	"ConfigurationItemDnsRecord":                 createSetsID[entities.ConfigurationItemDnsRecord],
	"ConfigurationItemNoteAttachment":            createSetsID[entities.ConfigurationItemNoteAttachment],
	"ConfigurationItemNote":                      createSetsID[entities.ConfigurationItemNote],
	"ConfigurationItemRelatedItem":               createSetsID[entities.ConfigurationItemRelatedItem],
	"ConfigurationItemSslSubjectAlternativeName": createSetsID[entities.ConfigurationItemSslSubjectAlternativeName],
	"ConfigurationItemType":                      createSetsID[entities.ConfigurationItemType],
	"TicketAdditionalConfigurationItem":          createSetsID[entities.TicketAdditionalConfigurationItem],
	"Contact":                                    createSetsID[entities.Contact],
	"Contract":                                   createSetsID[entities.Contract],
	"Department":                                 createSetsID[entities.Department],
	"ExpenseItem":                                createSetsID[entities.ExpenseItem],
	"ExpenseReport":                              createSetsID[entities.ExpenseReport],
	"Invoice":                                    createSetsID[entities.Invoice],
	"Opportunity":                                createSetsID[entities.Opportunity],
	"Product":                                    createSetsID[entities.Product],
	"ProjectNote":                                createSetsID[entities.ProjectNote],
	"Project":                                    createSetsID[entities.Project],
	"QuoteItem":                                  createSetsID[entities.QuoteItem],
	"Quote":                                      createSetsID[entities.Quote],
	"Resource":                                   createSetsID[entities.Resource],
	"ServiceBundle":                              createSetsID[entities.ServiceBundle],
	"Service":                                    createSetsID[entities.Service],
	"Task":                                       createSetsID[entities.Task],
	"TicketAttachment":                           createSetsID[entities.TicketAttachment],
	"TicketNoteAttachment":                       createSetsID[entities.TicketNoteAttachment],
	"TicketNote":                                 createSetsID[entities.TicketNote],
	"Ticket":                                     createSetsID[entities.Ticket],
	"TimeEntry":                                  createSetsID[entities.TimeEntry],
	"ClientPortalUser":                           createSetsID[entities.ClientPortalUser],
	"ContactGroup":                               createSetsID[entities.ContactGroup],
	"ContactGroupContact":                        createSetsID[entities.ContactGroupContact],
	"ResourceAttachment":                         createSetsID[entities.ResourceAttachment],
	"ResourceRole":                               createSetsID[entities.ResourceRole],
	"ResourceRoleDepartment":                     createSetsID[entities.ResourceRoleDepartment],
	"ResourceRoleQueue":                          createSetsID[entities.ResourceRoleQueue],
	"ResourceServiceDeskRole":                    createSetsID[entities.ResourceServiceDeskRole],
	"ResourceSkill":                              createSetsID[entities.ResourceSkill],
	"Role":                                       createSetsID[entities.Role],
	"Skill":                                      createSetsID[entities.Skill],
}

// idOf returns the value of the entity's "id" JSON member, or false when absent.
func idOf(t *testing.T, entity any) (int64, bool) {
	t.Helper()
	raw, err := json.Marshal(entity)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	v, ok := m["id"]
	if !ok {
		return 0, false
	}
	var id int64
	if err := json.Unmarshal(v, &id); err != nil {
		t.Fatalf("id member %s: %v", v, err)
	}
	return id, true
}

// createSetsID creates a zero T through the mock server and checks that Create
// returns it with the id the server stored, that the id reads back, and that no
// other member changed.
func createSetsID[T autotask.Entity](t *testing.T) {
	t.Helper()
	_, client := autotasktest.NewServer(t)

	var in T
	before, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	out, err := autotask.Create(t.Context(), client, &in)
	if err != nil {
		t.Fatal(err)
	}
	id, ok := idOf(t, out)
	if !ok || id <= 0 {
		t.Fatalf("created entity has no id (id=%d set=%v)", id, ok)
	}
	if out != &in {
		t.Fatal("Create returned a different pointer than its argument")
	}

	// Create must not populate any member except id.
	var sent, got map[string]json.RawMessage
	if err := json.Unmarshal(before, &sent); err != nil {
		t.Fatal(err)
	}
	after, err := json.Marshal(out)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(after, &got); err != nil {
		t.Fatal(err)
	}
	delete(got, "id")
	if !maps.EqualFunc(sent, got, func(a, b json.RawMessage) bool { return string(a) == string(b) }) {
		t.Fatalf("members other than id changed: sent %s, got %s", before, after)
	}

	read, err := autotask.Get[T](t.Context(), client, id)
	if err != nil {
		t.Fatalf("Get(%d) after Create: %v", id, err)
	}
	if readID, _ := idOf(t, read); readID != id {
		t.Fatalf("stored id = %d, Create returned %d", readID, id)
	}
}

func TestCreateSetsIDForEveryEntity(t *testing.T) {
	t.Parallel()
	for _, name := range slices.Sorted(maps.Keys(createIDCases)) {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			createIDCases[name](t)
		})
	}
}

func TestCreateIDCoversAllEntities(t *testing.T) {
	t.Parallel()
	files, err := filepath.Glob(filepath.Join("entities", "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	var found []string
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
			if id, ok := fn.Recv.List[0].Type.(*ast.Ident); ok {
				found = append(found, id.Name)
			}
		}
	}
	slices.Sort(found)
	if len(found) == 0 {
		t.Fatal("found no entity types in entities/")
	}
	if want := slices.Sorted(maps.Keys(createIDCases)); !slices.Equal(found, want) {
		t.Fatalf("entity types in entities/ = %v, createIDCases = %v", found, want)
	}
}

func TestCreateSetsTicketID(t *testing.T) {
	t.Parallel()
	_, client := autotasktest.NewServer(t)

	in := entities.Ticket{
		Title:      autotask.Set("created ticket"),
		ExternalID: autotask.Set("ext-1"),
	}
	out, err := autotask.Create(t.Context(), client, &in)
	if err != nil {
		t.Fatal(err)
	}
	id, ok := out.ID.Get()
	if !ok {
		t.Fatal("ID not set after Create")
	}
	stored, err := autotask.Get[entities.Ticket](t.Context(), client, id)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := stored.ExternalID.Get(); got != "ext-1" {
		t.Fatalf("ticket stored under id %d has externalID %q, want ext-1", id, got)
	}
	if got, _ := out.Title.Get(); got != "created ticket" {
		t.Fatalf("Title = %q after Create, want it unchanged", got)
	}
}

func TestCreateChildSetsAttachmentID(t *testing.T) {
	t.Parallel()
	_, client := autotasktest.NewServer(t)

	in := entities.TicketAttachment{
		Title: autotask.Set("report.xlsx"),
		Data:  autotask.Set("AAAA"),
	}
	out, err := autotask.CreateChild[entities.Ticket](t.Context(), client, 42, &in)
	if err != nil {
		t.Fatal(err)
	}
	id, ok := out.ID.Get()
	if !ok {
		t.Fatal("ID not set after CreateChild")
	}
	stored, err := autotask.GetChild[entities.Ticket, entities.TicketAttachment](t.Context(), client, 42)
	if err != nil {
		t.Fatal(err)
	}
	if len(stored) != 1 {
		t.Fatalf("stored %d attachments, want 1", len(stored))
	}
	if storedID, _ := stored[0].ID.Get(); storedID != id {
		t.Fatalf("stored attachment id = %d, CreateChild returned %d", storedID, id)
	}
	if got, _ := out.Title.Get(); got != "report.xlsx" {
		t.Fatalf("Title = %q after CreateChild, want it unchanged", got)
	}
}
