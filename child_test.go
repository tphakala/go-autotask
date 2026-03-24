package autotask

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

type testChildEntity struct {
	ID      Optional[int64]  `json:"id,omitzero"`
	Message Optional[string] `json:"message,omitzero"`
}

func (testChildEntity) EntityName() string { return "Notes" }

func TestGetChild(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1.0/TestEntities/{parentID}/Notes", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items": []any{
				map[string]any{"id": 10, "message": "note 1"},
				map[string]any{"id": 11, "message": "note 2"},
			},
			"pageDetails": map[string]any{"count": 2},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	client := testClient(t, srv)
	children, err := GetChild[testEntity, testChildEntity](t.Context(), client, 42)
	if err != nil {
		t.Fatal(err)
	}
	if len(children) != 2 {
		t.Fatalf("len = %d; want 2", len(children))
	}
}

func TestCreateChild(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1.0/TestEntities/{parentID}/Notes", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"itemId": 12})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	client := testClient(t, srv)
	child := &testChildEntity{Message: Set("new note")}
	result, err := CreateChild[testEntity](t.Context(), client, 42, child)
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestListChild(t *testing.T) {
	page := 0
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1.0/TestEntities/{parentID}/Notes", func(w http.ResponseWriter, r *http.Request) {
		page++
		if page == 1 {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"items": []any{
					map[string]any{"id": 10, "message": "note 1"},
					map[string]any{"id": 11, "message": "note 2"},
				},
				"pageDetails": map[string]any{"count": 2, "nextPageUrl": "/v1.0/TestEntities/42/Notes?page=2"},
			})
		} else {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"items": []any{
					map[string]any{"id": 12, "message": "note 3"},
				},
				"pageDetails": map[string]any{"count": 1},
			})
		}
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	client := testClient(t, srv)
	children, err := ListChild[testEntity, testChildEntity](t.Context(), client, 42)
	if err != nil {
		t.Fatal(err)
	}
	if len(children) != 3 {
		t.Fatalf("len = %d; want 3", len(children))
	}
	id, _ := children[2].ID.Get()
	if id != 12 {
		t.Fatalf("last child id = %d; want 12", id)
	}
}

func TestListChildSinglePage(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1.0/TestEntities/{parentID}/Notes", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items": []any{
				map[string]any{"id": 10, "message": "only note"},
			},
			"pageDetails": map[string]any{"count": 1},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	client := testClient(t, srv)
	children, err := ListChild[testEntity, testChildEntity](t.Context(), client, 42)
	if err != nil {
		t.Fatal(err)
	}
	if len(children) != 1 {
		t.Fatalf("len = %d; want 1", len(children))
	}
}

func TestListChildEmpty(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1.0/TestEntities/{parentID}/Notes", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items":       []any{},
			"pageDetails": map[string]any{"count": 0},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	client := testClient(t, srv)
	children, err := ListChild[testEntity, testChildEntity](t.Context(), client, 42)
	if err != nil {
		t.Fatal(err)
	}
	if len(children) != 0 {
		t.Fatalf("len = %d; want 0", len(children))
	}
}

func TestListChildAPIError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1.0/TestEntities/{parentID}/Notes", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"errors": []any{map[string]any{"message": "server error"}},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	client := testClient(t, srv)
	_, err := ListChild[testEntity, testChildEntity](t.Context(), client, 42)
	if err == nil {
		t.Fatal("expected error from API")
	}
}
