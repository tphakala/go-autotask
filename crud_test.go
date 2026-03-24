package autotask

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type testEntity struct {
	ID    Optional[int64]  `json:"id,omitzero"`
	Title Optional[string] `json:"title,omitzero"`
}

func (testEntity) EntityName() string { return "TestEntities" }

func newTypedTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1.0/TestEntities/{id}", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"item": map[string]any{"id": 42, "title": "Hello"},
		})
	})
	mux.HandleFunc("POST /v1.0/TestEntities/query", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items": []any{
				map[string]any{"id": 1, "title": "First"},
				map[string]any{"id": 2, "title": "Second"},
			},
			"pageDetails": map[string]any{"count": 2},
		})
	})
	mux.HandleFunc("POST /v1.0/TestEntities/query/count", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"queryCount": 42})
	})
	mux.HandleFunc("POST /v1.0/TestEntities", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"itemId": 99})
	})
	mux.HandleFunc("PATCH /v1.0/TestEntities", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"item": map[string]any{"id": 42, "title": "Updated"},
		})
	})
	mux.HandleFunc("DELETE /v1.0/TestEntities/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestGet(t *testing.T) {
	srv := newTypedTestServer(t)
	client := testClient(t, srv)
	entity, err := Get[testEntity](t.Context(), client, 42)
	if err != nil {
		t.Fatal(err)
	}
	if v, ok := entity.ID.Get(); !ok || v != 42 {
		t.Fatalf("ID = %v, %v; want 42", v, ok)
	}
	if v, ok := entity.Title.Get(); !ok || v != "Hello" {
		t.Fatalf("Title = %v, %v; want Hello", v, ok)
	}
}

func TestList(t *testing.T) {
	srv := newTypedTestServer(t)
	client := testClient(t, srv)
	entities, err := List[testEntity](t.Context(), client, NewQuery().Where("status", OpEq, 1))
	if err != nil {
		t.Fatal(err)
	}
	if len(entities) != 2 {
		t.Fatalf("len = %d; want 2", len(entities))
	}
}

func TestCount(t *testing.T) {
	srv := newTypedTestServer(t)
	client := testClient(t, srv)
	count, err := Count[testEntity](t.Context(), client, NewQuery())
	if err != nil {
		t.Fatal(err)
	}
	if count != 42 {
		t.Fatalf("count = %d; want 42", count)
	}
}

func TestCreate(t *testing.T) {
	srv := newTypedTestServer(t)
	client := testClient(t, srv)
	entity := &testEntity{Title: Set("New")}
	result, err := Create(t.Context(), client, entity)
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestUpdate(t *testing.T) {
	srv := newTypedTestServer(t)
	client := testClient(t, srv)
	entity := &testEntity{ID: Set(int64(42)), Title: Set("Updated")}
	result, err := Update(t.Context(), client, entity)
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestDelete(t *testing.T) {
	srv := newTypedTestServer(t)
	client := testClient(t, srv)
	err := Delete[testEntity](t.Context(), client, 42)
	if err != nil {
		t.Fatal(err)
	}
}

func TestListMaxPagesGuard(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1.0/TestEntities/query", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items":       []any{map[string]any{"id": 1}},
			"pageDetails": map[string]any{"count": 1, "nextPageUrl": "/v1.0/TestEntities/query?page=next"},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	client := testClient(t, srv)
	_, err := List[testEntity](t.Context(), client, NewQuery())
	if err == nil {
		t.Fatal("expected ErrMaxPagesExceeded")
	}
	var maxErr *ErrMaxPagesExceeded
	if !errors.As(err, &maxErr) {
		t.Fatalf("expected ErrMaxPagesExceeded, got: %v", err)
	}
}
