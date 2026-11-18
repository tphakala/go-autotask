package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tphakala/go-autotask/autotasktest"
	"github.com/tphakala/go-autotask/metadata"
)

func TestToSnakeCase(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"Tickets", "tickets"},
		{"TicketNotes", "ticket_notes"},
		{"ConfigurationItems", "configuration_items"},
		{"BillingItemApprovalLevels", "billing_item_approval_levels"},
		{"Companies", "companies"},
		{"TimeEntries", "time_entries"},
		{"ServiceBundles", "service_bundles"},
		{"QuoteItems", "quote_items"},
		{"IDs", "ids"},
		{"HTTPServer", "http_server"},
		{"APIKey", "api_key"},
		{"HTMLParser", "html_parser"},
		{"SimpleURL", "simple_url"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := toSnakeCase(tt.input); got != tt.want {
				t.Errorf("toSnakeCase(%q) = %q; want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSingular(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"Tickets", "Ticket"},
		{"Companies", "Company"},
		{"TimeEntries", "TimeEntry"},
		{"ConfigurationItems", "ConfigurationItem"},
		{"Statuses", "Status"},
		{"Services", "Service"},
		{"Addresses", "Address"},
		{"Resources", "Resource"},
		{"Departments", "Department"},
		{"TicketNotes", "TicketNote"},
		{"BillingCodes", "BillingCode"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := singular(tt.input); got != tt.want {
				t.Errorf("singular(%q) = %q; want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestGoName(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		// Basic uppercasing
		{"title", "Title"},
		{"companyName", "CompanyName"},
		// Standalone acronyms
		{"id", "ID"},
		{"url", "URL"},
		{"api", "API"},
		{"html", "HTML"},
		{"sql", "SQL"},
		{"ip", "IP"},
		{"ssl", "SSL"},
		{"cpu", "CPU"},
		{"sku", "SKU"},
		// Compound fields with acronyms already correct from API
		{"companyID", "CompanyID"},
		{"ticketID", "TicketID"},
		{"nextPageUrl", "NextPageUrl"},
		// Already uppercase acronyms pass through
		{"URL", "URL"},
		{"ID", "ID"},
		// Edge cases
		{"", ""},
		{"a", "A"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := goName(tt.input); got != tt.want {
				t.Errorf("goName(%q) = %q; want %q", tt.input, got, tt.want)
			}
		})
	}
}

// autotask.Create and CreateChild return the server-assigned id by decoding
// {"id": N} into the entity, so a generated entity must carry its id as an
// Optional[int64] member with the JSON name "id".
func TestGenerateEntityIDMember(t *testing.T) {
	dir := t.TempDir()
	g := &Generator{OutputDir: dir}
	fields := []metadata.FieldInfo{
		{Name: "id", Type: "long"},
		{Name: "title", Type: "string"},
	}
	if skip, err := g.generateEntity("Widgets", fields, nil); err != nil || skip != "" {
		t.Fatalf("skip = %q, err = %v", skip, err)
	}
	src, err := os.ReadFile(filepath.Join(dir, "widgets.go"))
	if err != nil {
		t.Fatal(err)
	}
	var idLine string
	for line := range strings.SplitSeq(string(src), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "ID ") {
			idLine = strings.Join(strings.Fields(line), " ")
		}
	}
	if want := "ID autotask.Optional[int64] `json:\"id,omitzero\"`"; idLine != want {
		t.Fatalf("id member = %q, want %q\n%s", idLine, want, src)
	}
}

const (
	handWrittenTicket = "package entities\n\ntype Ticket struct{}\n"
	// staleGeneratedTickets is what an older generator left next to ticket.go.
	staleGeneratedTickets = generatedHeader + "\npackage entities\n\ntype Ticket struct{}\n"
)

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), filePerm); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, dir, name string) string {
	t.Helper()
	src, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	return string(src)
}

func requireNoFile(t *testing.T, dir, name string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(err) {
		t.Fatalf("%s must not exist (stat err = %v)", name, err)
	}
}

// The reported case: Tickets would go to tickets.go next to a hand-written
// ticket.go that declares Ticket, and the package would stop building.
func TestGenerateEntitySkipsTypeDeclaredInAnotherFile(t *testing.T) {
	tests := []struct {
		name  string
		files map[string]string
	}{
		{"hand-written file only", map[string]string{"ticket.go": handWrittenTicket}},
		// A target that has the header must still go through the type check.
		{"stale generated target", map[string]string{"ticket.go": handWrittenTicket, "tickets.go": staleGeneratedTickets}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, content := range tt.files {
				writeFile(t, dir, name, content)
			}
			skip, err := (&Generator{OutputDir: dir}).generateEntity("Tickets", nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(skip, "ticket.go") {
				t.Fatalf("skip reason = %q; want it to name ticket.go", skip)
			}
			for name, content := range tt.files {
				if got := readFile(t, dir, name); got != content {
					t.Fatalf("%s changed:\n%s", name, got)
				}
			}
			if _, stale := tt.files["tickets.go"]; !stale {
				requireNoFile(t, dir, "tickets.go")
			}
		})
	}
}

func TestGenerateEntityKeepsFileWithoutGeneratedHeader(t *testing.T) {
	dir := t.TempDir()
	const handWritten = "package entities\n\n// Widget is maintained by hand.\ntype Gadget struct{}\n"
	writeFile(t, dir, "widgets.go", handWritten)
	skip, err := (&Generator{OutputDir: dir}).generateEntity("Widgets", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if skip == "" {
		t.Fatal("want a skip reason for a file without the generated header")
	}
	if got := readFile(t, dir, "widgets.go"); got != handWritten {
		t.Fatalf("widgets.go was overwritten:\n%s", got)
	}
}

// Each step runs against what the earlier steps left in the directory.
func TestGenerateEntityAcrossRuns(t *testing.T) {
	dir := t.TempDir()
	g := &Generator{OutputDir: dir}
	run := func(name, field string) string {
		t.Helper()
		skip, err := g.generateEntity(name, []metadata.FieldInfo{{Name: field, Type: "string"}}, nil)
		if err != nil {
			t.Fatal(err)
		}
		return skip
	}
	if skip := run("Widgets", "first"); skip != "" {
		t.Fatalf("new entity skipped: %s", skip)
	}
	if skip := run("Widgets", "second"); skip != "" {
		t.Fatalf("regenerating a generated file skipped: %s", skip)
	}
	if got := readFile(t, dir, "widgets.go"); !strings.Contains(got, "Second ") || strings.Contains(got, "First ") {
		t.Fatalf("widgets.go was not regenerated:\n%s", got)
	}
	// Widget and Widgets both name the Go type Widget.
	if skip := run("Widget", "third"); !strings.Contains(skip, "widgets.go") {
		t.Fatalf("skip reason = %q; want it to name widgets.go", skip)
	}
	requireNoFile(t, dir, "widget.go")
	if err := os.Remove(filepath.Join(dir, "widgets.go")); err != nil {
		t.Fatal(err)
	}
	if skip := run("Widget", "third"); skip != "" {
		t.Fatalf("skipped after the other declaration was removed: %s", skip)
	}
}

func TestGenerateSkipsCollisionsAndFailsWhenNothingIsGenerated(t *testing.T) {
	_, client := autotasktest.NewServer(t)
	dir := t.TempDir()
	writeFile(t, dir, "ticket.go", handWrittenTicket)

	g := &Generator{Client: client, OutputDir: dir, Entities: []string{"Tickets", "Widgets"}}
	if err := g.Generate(t.Context()); err != nil {
		t.Fatalf("one entity was generated, so Generate must succeed: %v", err)
	}
	requireNoFile(t, dir, "tickets.go")
	if got := readFile(t, dir, "widgets.go"); !strings.HasPrefix(got, generatedHeader) {
		t.Fatalf("widgets.go does not start with the generated header:\n%s", got)
	}

	g.Entities = []string{"Tickets"}
	if err := g.Generate(t.Context()); err == nil {
		t.Fatal("want an error when every entity is skipped")
	}
}

// The go tool ignores names that start with "." or "_", so they cannot collide
// with a generated type, and an editor lock file must not stop the run.
func TestGenerateEntityIgnoresFilesTheGoToolIgnores(t *testing.T) {
	dir := t.TempDir()
	if err := os.Symlink("missing-target", filepath.Join(dir, ".#ticket.go")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	writeFile(t, dir, "_old_ticket.go", handWrittenTicket)
	skip, err := (&Generator{OutputDir: dir}).generateEntity("Tickets", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if skip != "" {
		t.Fatalf("skipped because of a file the go tool ignores: %s", skip)
	}
	if got := readFile(t, dir, "tickets.go"); !strings.HasPrefix(got, generatedHeader) {
		t.Fatalf("tickets.go was not generated:\n%s", got)
	}
}
