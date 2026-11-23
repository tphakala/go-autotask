package autotask_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	autotask "github.com/tphakala/go-autotask"
	"github.com/tphakala/go-autotask/autotasktest"
	"github.com/tphakala/go-autotask/entities"
)

// assertChildGetPath is a test helper that checks the last request path contains wantPath.
func assertChildGetPath(t *testing.T, srv *autotasktest.TestServer, wantPath string) {
	t.Helper()
	req := srv.LastRequest()
	if !strings.Contains(req.Path, wantPath) {
		t.Fatalf("path = %q, want %s", req.Path, wantPath)
	}
}

// assertChildCreateRequest is a test helper that checks the last request was a POST to wantPath.
func assertChildCreateRequest(t *testing.T, srv *autotasktest.TestServer, wantPath string) {
	t.Helper()
	req := srv.LastRequest()
	if req.Method != http.MethodPost {
		t.Fatalf("method = %s, want POST", req.Method)
	}
	if !strings.Contains(req.Path, wantPath) {
		t.Fatalf("path = %q, want %s", req.Path, wantPath)
	}
}

//nolint:dupl // TestChildTicketNote and TestChildProjectTask test different generic type combinations
func TestChildTicketNote(t *testing.T) {
	t.Parallel()
	note := autotasktest.TicketNoteFixture()
	srv, client := autotasktest.NewServer(t, autotasktest.WithEntity(note))
	const wantPath = "/Tickets/100/Notes"

	t.Run("GetChild", func(t *testing.T) {
		children, err := autotask.GetChild[entities.Ticket, entities.TicketNote](t.Context(), client, 100)
		if err != nil {
			t.Fatal(err)
		}
		if len(children) == 0 {
			t.Fatal("expected at least 1 child")
		}
		assertChildGetPath(t, srv, wantPath)
	})

	t.Run("CreateChild", func(t *testing.T) {
		newNote := autotasktest.TicketNoteFixture(func(n *entities.TicketNote) {
			n.ID = autotask.Optional[int64]{} // unset for create
		})
		result, err := autotask.CreateChild[entities.Ticket](t.Context(), client, 100, &newNote)
		if err != nil {
			t.Fatal(err)
		}
		if result == nil {
			t.Fatal("expected non-nil result")
		}
		assertChildCreateRequest(t, srv, wantPath)
	})
}

//nolint:dupl // TestChildProjectTask and TestChildTicketNote test different generic type combinations
func TestChildProjectTask(t *testing.T) {
	t.Parallel()
	task := autotasktest.TaskFixture()
	srv, client := autotasktest.NewServer(t, autotasktest.WithEntity(task))
	const wantPath = "/Projects/200/Tasks"

	t.Run("GetChild", func(t *testing.T) {
		children, err := autotask.GetChild[entities.Project, entities.Task](t.Context(), client, 200)
		if err != nil {
			t.Fatal(err)
		}
		if len(children) == 0 {
			t.Fatal("expected at least 1 child")
		}
		assertChildGetPath(t, srv, wantPath)
	})

	t.Run("CreateChild", func(t *testing.T) {
		newTask := autotasktest.TaskFixture(func(tk *entities.Task) {
			tk.ID = autotask.Optional[int64]{} // unset for create
		})
		result, err := autotask.CreateChild[entities.Project](t.Context(), client, 200, &newTask)
		if err != nil {
			t.Fatal(err)
		}
		if result == nil {
			t.Fatal("expected non-nil result")
		}
		assertChildCreateRequest(t, srv, wantPath)
	})
}

func TestChildCreateNilEntity(t *testing.T) {
	t.Parallel()
	_, client := autotasktest.NewServer(t)
	_, err := autotask.CreateChild[entities.Ticket](t.Context(), client, 1, (*entities.TicketNote)(nil))
	if err == nil {
		t.Fatal("expected error for nil child")
	}
	if !strings.Contains(err.Error(), "must not be nil") {
		t.Fatalf("error = %v, want 'must not be nil'", err)
	}
}

func TestChildTicketNoteAttachment(t *testing.T) {
	t.Parallel()
	srv, client := autotasktest.NewServer(t)
	const wantPath = "/v1.0/TicketNotes/300/Attachments"

	in := entities.TicketNoteAttachment{
		AttachmentType: autotask.Set("FILE_ATTACHMENT"),
		FullPath:       autotask.Set("report.xlsx"),
		Publish:        autotask.Set(int64(1)),
		Title:          autotask.Set("report.xlsx"),
		Data:           autotask.Set("AAAA"),
	}
	out, err := autotask.CreateChild[entities.TicketNote](t.Context(), client, 300, &in)
	if err != nil {
		t.Fatal(err)
	}
	if req := srv.LastRequest(); req.Method != http.MethodPost || req.Path != wantPath {
		t.Fatalf("CreateChild sent %s %s, want POST %s", req.Method, req.Path, wantPath)
	}
	id, ok := out.ID.Get()
	if !ok || id <= 0 {
		t.Fatalf("ID = %d (set=%v) after CreateChild, want the server's id", id, ok)
	}

	listed, err := autotask.ListChild[entities.TicketNote, entities.TicketNoteAttachment](t.Context(), client, 300)
	if err != nil {
		t.Fatal(err)
	}
	if req := srv.LastRequest(); req.Method != http.MethodGet || req.Path != wantPath {
		t.Fatalf("ListChild sent %s %s, want GET %s", req.Method, req.Path, wantPath)
	}
	if len(listed) != 1 {
		t.Fatalf("listed %d attachments, want 1", len(listed))
	}
	if gotID, _ := listed[0].ID.Get(); gotID != id {
		t.Fatalf("listed attachment id = %d, CreateChild returned %d", gotID, id)
	}
	if got, _ := listed[0].Title.Get(); got != "report.xlsx" {
		t.Fatalf("listed Title = %q, want report.xlsx", got)
	}
}

// assertLastRequest checks that the last request the mock saw had the exact
// method and path.
func assertLastRequest(t *testing.T, srv *autotasktest.TestServer, wantMethod, wantPath string) {
	t.Helper()
	req := srv.LastRequest()
	if req.Method != wantMethod || req.Path != wantPath {
		t.Fatalf("request = %s %s, want %s %s", req.Method, req.Path, wantMethod, wantPath)
	}
}

// TestQuoteItemAndExpenseItemChildRoutes pins the child URLs of the two item
// entities the REST swagger serves under a shorter parent and child segment
// than their root entity names: QuoteItem under /Quotes/{id}/Items and
// ExpenseItem under /Expenses/{id}/Items.
func TestQuoteItemAndExpenseItemChildRoutes(t *testing.T) {
	t.Parallel()
	srv, client := autotasktest.NewServer(t, autotasktest.WithDeleteSupport("QuoteItems"))
	ctx := t.Context()

	quoteItem := entities.QuoteItem{Quantity: autotask.Set(2.0)}
	if _, err := autotask.CreateChild[entities.Quote](ctx, client, 101, &quoteItem); err != nil {
		t.Fatal(err)
	}
	assertLastRequest(t, srv, http.MethodPost, "/v1.0/Quotes/101/Items")
	quoteItemID, _ := quoteItem.ID.Get()

	quoteItem.Quantity = autotask.Set(3.0)
	if _, err := autotask.UpdateChild[entities.Quote](ctx, client, 101, &quoteItem); err != nil {
		t.Fatal(err)
	}
	assertLastRequest(t, srv, http.MethodPatch, "/v1.0/Quotes/101/Items")

	if err := autotask.DeleteChild[entities.Quote, entities.QuoteItem](ctx, client, 101, quoteItemID); err != nil {
		t.Fatal(err)
	}
	assertLastRequest(t, srv, http.MethodDelete, fmt.Sprintf("/v1.0/Quotes/101/Items/%d", quoteItemID))

	expenseItem := entities.ExpenseItem{Description: autotask.Set("taxi")}
	if _, err := autotask.CreateChild[entities.ExpenseReport](ctx, client, 7, &expenseItem); err != nil {
		t.Fatal(err)
	}
	assertLastRequest(t, srv, http.MethodPost, "/v1.0/Expenses/7/Items")

	listed, err := autotask.ListChild[entities.ExpenseReport, entities.ExpenseItem](ctx, client, 7)
	if err != nil {
		t.Fatal(err)
	}
	assertLastRequest(t, srv, http.MethodGet, "/v1.0/Expenses/7/Items")
	if len(listed) != 1 {
		t.Fatalf("expense items under report 7 = %d, want the 1 created through the child URL", len(listed))
	}
}
