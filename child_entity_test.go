package autotask_test

import (
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
	const wantPath = "/Tickets/100/TicketNotes"

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
