package autotask

import (
	"context"
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
		json.NewEncoder(w).Encode(map[string]any{
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
	children, err := GetChild[testEntity, testChildEntity](context.Background(), client, 42)
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
		json.NewEncoder(w).Encode(map[string]any{"itemId": 12})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	client := testClient(t, srv)
	child := &testChildEntity{Message: Set("new note")}
	result, err := CreateChild[testEntity](context.Background(), client, 42, child)
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}
