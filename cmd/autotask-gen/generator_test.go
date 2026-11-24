package main

import (
	"errors"
	"io"
	"io/fs"
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
	var genErr error
	stderr := captureStderr(t, func() { genErr = g.Generate(t.Context()) })
	if genErr == nil {
		t.Fatal("want an error when every entity is skipped")
	}
	if want := "warning: skipping Tickets: Ticket is already declared in ticket.go"; !strings.Contains(stderr, want) {
		t.Fatalf("stderr = %q; want it to contain %q", stderr, want)
	}
}

// An entity that would be skipped costs no metadata request: the check runs
// before GetFields and GetUDFs.
func TestGenerateSkipsBeforeFetchingMetadata(t *testing.T) {
	srv, client := autotasktest.NewServer(t)
	dir := t.TempDir()
	writeFile(t, dir, "ticket.go", handWrittenTicket)

	g := &Generator{Client: client, OutputDir: dir, Entities: []string{"Tickets", "Widgets"}}
	if err := g.Generate(t.Context()); err != nil {
		t.Fatal(err)
	}
	fetchedWidgets := false
	for _, r := range srv.Requests() {
		if strings.Contains(r.Path, "/Tickets/") {
			t.Errorf("skipped entity Tickets still sent %s %s", r.Method, r.Path)
		}
		fetchedWidgets = fetchedWidgets || strings.Contains(r.Path, "/Widgets/")
	}
	if !fetchedWidgets {
		t.Fatal("no request for Widgets was recorded, so the check above proves nothing")
	}
}

// A collision check that cannot read the target stops the run before any
// metadata request.
func TestGenerateStopsWhenTheCollisionCheckFails(t *testing.T) {
	srv, client := autotasktest.NewServer(t)
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "tickets.go"), 0o755); err != nil {
		t.Fatal(err)
	}

	g := &Generator{Client: client, OutputDir: dir, Entities: []string{"Tickets"}}
	err := g.Generate(t.Context())
	if err == nil || !strings.Contains(err.Error(), "generating Tickets") {
		t.Fatalf("Generate() error = %v; want one naming Tickets", err)
	}
	if n := srv.RequestCount(); n != 0 {
		t.Fatalf("%d requests were sent; want none", n)
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
	// What a run killed between creating the temp file and renaming it leaves.
	writeFile(t, dir, strings.Replace(tempPattern("tickets.go"), "*", "123456", 1), handWrittenTicket)
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

// Only Go source files that are not tests can collide with a generated type;
// a sibling that does not parse still counts as far as it parses.
func TestGenerateEntityScansOnlyGoSourceFiles(t *testing.T) {
	const declaresTicket = "package entities\n\ntype Ticket struct{}\n"
	tests := []struct {
		name  string
		files map[string]string // content by file name
		dirs  []string
		links map[string]string // link name to target, relative to the directory
		want  string            // file that must be named in the skip reason, "" for no skip
	}{
		{name: "non-Go file", files: map[string]string{"ticket.go.bak": declaresTicket}},
		{name: "test file", files: map[string]string{"ticket_test.go": declaresTicket}},
		{name: "directory named like a Go file", dirs: []string{"ticket.go"}},
		{name: "symlink to a directory named like a Go file", dirs: []string{"sub"}, links: map[string]string{"ticket.go": "sub"}},
		{name: "dangling symlink named like a Go file", links: map[string]string{"ticket.go": "missing"}},
		{name: "symlink to a file that declares the type", files: map[string]string{"ticket.txt": declaresTicket}, links: map[string]string{"ticket.go": "ticket.txt"}, want: "ticket.go"},
		{name: "syntax error after the declaration", files: map[string]string{"broken.go": declaresTicket + "\nfunc (\n"}, want: "broken.go"},
		{name: "syntax error and no declaration", files: map[string]string{"broken.go": "package entities\n\nfunc (\n"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, content := range tt.files {
				writeFile(t, dir, name, content)
			}
			for _, name := range tt.dirs {
				if err := os.Mkdir(filepath.Join(dir, name), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			for name, target := range tt.links {
				if err := os.Symlink(target, filepath.Join(dir, name)); err != nil {
					t.Skipf("symlinks unavailable: %v", err)
				}
			}
			skip, err := (&Generator{OutputDir: dir}).generateEntity("Tickets", nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			if tt.want == "" && skip != "" {
				t.Fatalf("skipped: %s", skip)
			}
			if tt.want != "" && !strings.Contains(skip, tt.want) {
				t.Fatalf("skip reason = %q; want it to name %s", skip, tt.want)
			}
		})
	}
}

// A field name that does not format must not leave anything behind: the
// earlier generated file stays as it was and no temporary file remains.
func TestGenerateEntityWritesNothingWhenFormattingFails(t *testing.T) {
	dir := t.TempDir()
	const earlier = generatedHeader + "\npackage entities\n\ntype Widget struct{}\n"
	writeFile(t, dir, "widgets.go", earlier)

	_, err := (&Generator{OutputDir: dir}).generateEntity("Widgets", []metadata.FieldInfo{{Name: "a b", Type: "string"}}, nil)
	if err == nil || !strings.Contains(err.Error(), "formatting generated code") {
		t.Fatalf("generateEntity() error = %v; want a formatting error", err)
	}
	if got := readFile(t, dir, "widgets.go"); got != earlier {
		t.Fatalf("widgets.go changed:\n%s", got)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("directory has %d entries; want only widgets.go", len(entries))
	}
}

// A regeneration replaces the file instead of rewriting it in place, so an
// interrupted run cannot leave a truncated file that later runs skip as
// hand-written.
func TestGenerateEntityReplacesTheTargetByRename(t *testing.T) {
	generate := func(t *testing.T, g *Generator, field string) {
		t.Helper()
		skip, err := g.generateEntity("Widgets", []metadata.FieldInfo{{Name: field, Type: "string"}}, nil)
		if err != nil || skip != "" {
			t.Fatalf("skip = %q, err = %v", skip, err)
		}
	}
	t.Run("earlier file is not rewritten in place", func(t *testing.T) {
		dir := t.TempDir()
		g := &Generator{OutputDir: dir}
		generate(t, g, "first")
		linkDir := t.TempDir()
		if err := os.Link(filepath.Join(dir, "widgets.go"), filepath.Join(linkDir, "earlier.go")); err != nil {
			t.Skipf("hard links unavailable: %v", err)
		}
		generate(t, g, "second")

		earlier := readFile(t, linkDir, "earlier.go")
		if !strings.HasPrefix(earlier, generatedHeader) || !strings.Contains(earlier, "First ") || strings.Contains(earlier, "Second ") {
			t.Fatalf("the earlier file was rewritten in place:\n%s", earlier)
		}
		info, err := os.Stat(filepath.Join(dir, "widgets.go"))
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != filePerm {
			t.Fatalf("widgets.go mode = %v; want %v", got, os.FileMode(filePerm))
		}
	})
	t.Run("symlinked target stays a link", func(t *testing.T) {
		dir, linkDir := t.TempDir(), t.TempDir()
		writeFile(t, linkDir, "real.go", generatedHeader+"\n")
		if err := os.Symlink(filepath.Join(linkDir, "real.go"), filepath.Join(dir, "widgets.go")); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
		generate(t, &Generator{OutputDir: dir}, "second")

		info, err := os.Lstat(filepath.Join(dir, "widgets.go"))
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode()&os.ModeSymlink == 0 {
			t.Fatal("widgets.go is no longer a symbolic link")
		}
		if got := readFile(t, linkDir, "real.go"); !strings.Contains(got, "Second ") {
			t.Fatalf("the linked file was not updated:\n%s", got)
		}
		entries, err := os.ReadDir(linkDir)
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 1 {
			t.Fatalf("link target directory has %d entries; want only real.go", len(entries))
		}
	})
}

// A target that is a symbolic link to a file that does not exist stops the
// entity and creates nothing.
func TestGenerateEntityStopsOnASymlinkedTargetToAMissingFile(t *testing.T) {
	dir, linkDir := t.TempDir(), t.TempDir()
	missing := filepath.Join(linkDir, "missing.go")
	if err := os.Symlink(missing, filepath.Join(dir, "widgets.go")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	_, err := (&Generator{OutputDir: dir}).generateEntity("Widgets", []metadata.FieldInfo{{Name: "second", Type: "string"}}, nil)
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("generateEntity() error = %v; want one wrapping fs.ErrNotExist", err)
	}
	requireNoFile(t, linkDir, "missing.go")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("directory has %d entries; want only the link", len(entries))
	}
}

// A failed replacement removes its temporary file: the rename onto a
// directory fails after the temporary file was written.
func TestReplaceFileRemovesItsTempFileWhenTheRenameFails(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "widgets.go"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := replaceFile(filepath.Join(dir, "widgets.go"), []byte("data")); err == nil {
		t.Fatal("replaceFile() onto a directory succeeded; want an error")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("directory has %d entries; want only the directory widgets.go", len(entries))
	}
}

// A path the generator cannot inspect must stop the entity, not fall through
// to a write: both modes below leave the write itself possible.
func TestGenerateEntityReturnsReadErrors(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("file modes do not restrict root")
	}
	t.Run("unreadable target", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "tickets.go", staleGeneratedTickets)
		path := filepath.Join(dir, "tickets.go")
		if err := os.Chmod(path, 0o200); err != nil {
			t.Fatal(err)
		}
		if _, err := (&Generator{OutputDir: dir}).generateEntity("Tickets", nil, nil); err == nil {
			t.Fatal("want an error for a target that cannot be read")
		}
		if err := os.Chmod(path, 0o600); err != nil {
			t.Fatal(err)
		}
		if got := readFile(t, dir, "tickets.go"); got != staleGeneratedTickets {
			t.Fatalf("tickets.go changed:\n%s", got)
		}
	})
	t.Run("unlistable directory", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.Chmod(dir, 0o300); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
		if _, err := (&Generator{OutputDir: dir}).generateEntity("Tickets", nil, nil); err == nil {
			t.Fatal("want an error for a directory that cannot be listed")
		}
		if err := os.Chmod(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		requireNoFile(t, dir, "tickets.go")
	})
}

// captureStderr runs fn and returns what it wrote to os.Stderr.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stderr
	os.Stderr = w
	defer func() { os.Stderr = orig }()
	fn()
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}
