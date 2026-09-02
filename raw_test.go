package autotask

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newCRUDTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1.0/Tickets/{id}", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"item": map[string]any{"id": 123, "title": "Test Ticket"},
		})
	})
	mux.HandleFunc("POST /v1.0/Tickets/query", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items":       []any{map[string]any{"id": 1}, map[string]any{"id": 2}},
			"pageDetails": map[string]any{"count": 2, "nextPageUrl": nil},
		})
	})
	mux.HandleFunc("POST /v1.0/Tickets", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"itemId": 456})
	})
	mux.HandleFunc("PATCH /v1.0/Tickets", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"item": map[string]any{"id": 123}})
	})
	mux.HandleFunc("DELETE /v1.0/Tickets/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func testClient(t *testing.T, srv *httptest.Server) *Client {
	t.Helper()
	auth := AuthConfig{Username: "u", Secret: "s", IntegrationCode: "c"}
	client, err := NewClient(t.Context(), auth, WithBaseURL(srv.URL))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func TestGetRaw(t *testing.T) {
	srv := newCRUDTestServer(t)
	client := testClient(t, srv)
	result, err := GetRaw(t.Context(), client, "Tickets", 123)
	if err != nil {
		t.Fatal(err)
	}
	if result["title"] != "Test Ticket" {
		t.Fatalf("title = %v", result["title"])
	}
}

func TestListRaw(t *testing.T) {
	srv := newCRUDTestServer(t)
	client := testClient(t, srv)
	results, err := ListRaw(t.Context(), client, "Tickets", NewQuery().Where("status", OpEq, 1))
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("results = %d; want 2", len(results))
	}
}

func TestCreateRaw(t *testing.T) {
	srv := newCRUDTestServer(t)
	client := testClient(t, srv)
	data := map[string]any{"title": "New Ticket"}
	result, err := CreateRaw(t.Context(), client, "Tickets", data)
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestUpdateRaw(t *testing.T) {
	srv := newCRUDTestServer(t)
	client := testClient(t, srv)
	data := map[string]any{"id": 123, "title": "Updated"}
	result, err := UpdateRaw(t.Context(), client, "Tickets", data)
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestDeleteRaw(t *testing.T) {
	srv := newCRUDTestServer(t)
	client := testClient(t, srv)
	err := DeleteRaw(t.Context(), client, "Tickets", 123)
	if err != nil {
		t.Fatal(err)
	}
}

func TestListRawMaxPagesGuard(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1.0/Tickets/query", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items":       []any{map[string]any{"id": 1}},
			"pageDetails": map[string]any{"count": 1, "nextPageUrl": "/v1.0/Tickets/query?page=next"},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	client := testClient(t, srv)
	_, err := ListRaw(t.Context(), client, "Tickets", NewQuery())
	if err == nil {
		t.Fatal("expected MaxPagesExceededError")
	}
	if _, ok := errors.AsType[*MaxPagesExceededError](err); !ok {
		t.Fatalf("expected MaxPagesExceededError, got: %v", err)
	}
}

func TestListRawWithMaxRecords(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1.0/Tickets/query", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items": []any{
				map[string]any{"id": 1}, map[string]any{"id": 2}, map[string]any{"id": 3},
			},
			"pageDetails": map[string]any{"count": 3, "nextPageUrl": "/v1.0/Tickets/query?page=2"},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	client := testClient(t, srv)
	results, err := ListRaw(t.Context(), client, "Tickets", NewQuery().Limit(2))
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("results = %d; want 2 (capped by MaxRecords)", len(results))
	}
}

func TestListRawIter(t *testing.T) {
	page := 0
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1.0/Tickets/query", func(w http.ResponseWriter, r *http.Request) {
		page++
		if page == 1 {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"items": []any{
					map[string]any{"id": 10}, map[string]any{"id": 11},
				},
				"pageDetails": map[string]any{"count": 2, "nextPageUrl": "/v1.0/Tickets/query?page=2"},
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items":       []any{map[string]any{"id": 12}},
			"pageDetails": map[string]any{"count": 1},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	client := testClient(t, srv)
	var ids []float64
	for item, err := range ListRawIter(t.Context(), client, "Tickets", NewQuery()) {
		if err != nil {
			t.Fatal(err)
		}
		id, ok := item["id"].(float64)
		if !ok {
			t.Fatalf("item %v missing numeric id", item)
		}
		ids = append(ids, id)
	}
	if len(ids) != 3 {
		t.Fatalf("got %d items; want 3", len(ids))
	}
	if ids[2] != 12 {
		t.Fatalf("last id = %v; want 12", ids[2])
	}
}

func TestListRawIterBreakEarly(t *testing.T) {
	reqs := 0
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1.0/Tickets/query", func(w http.ResponseWriter, r *http.Request) {
		reqs++
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items": []any{
				map[string]any{"id": 10}, map[string]any{"id": 11}, map[string]any{"id": 12},
			},
			"pageDetails": map[string]any{"count": 3, "nextPageUrl": "/v1.0/Tickets/query?page=2"},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	client := testClient(t, srv)
	count := 0
	for _, err := range ListRawIter(t.Context(), client, "Tickets", NewQuery()) {
		if err != nil {
			t.Fatal(err)
		}
		count++
		if count == 2 {
			break
		}
	}
	if count != 2 {
		t.Fatalf("count = %d; want 2", count)
	}
	if reqs != 1 {
		t.Fatalf("requests = %d; want 1 when breaking early", reqs)
	}
}

func TestListRawIterAPIError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1.0/Tickets/query", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"errors": []any{map[string]any{"message": "server error"}},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	client := testClient(t, srv)
	for _, err := range ListRawIter(t.Context(), client, "Tickets", NewQuery()) {
		if err == nil {
			t.Fatal("expected error from API")
		}
		return
	}
	t.Fatal("iterator should have yielded an error")
}

func TestListRawIterErrorOnSecondPage(t *testing.T) {
	page := 0
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1.0/Tickets/query", func(w http.ResponseWriter, r *http.Request) {
		page++
		if page == 1 {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"items":       []any{map[string]any{"id": 10}},
				"pageDetails": map[string]any{"count": 1, "nextPageUrl": "/v1.0/Tickets/query?page=2"},
			})
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"errors": []any{map[string]any{"message": "page 2 error"}},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	client := testClient(t, srv)
	var gotItems int
	var gotError bool
	for _, err := range ListRawIter(t.Context(), client, "Tickets", NewQuery()) {
		if err != nil {
			gotError = true
			break
		}
		gotItems++
	}
	if gotItems != 1 {
		t.Fatalf("items = %d; want 1 before error", gotItems)
	}
	if !gotError {
		t.Fatal("expected error on second page")
	}
}

func TestListRawIterMaxPagesGuard(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1.0/Tickets/query", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items":       []any{map[string]any{"id": 1}},
			"pageDetails": map[string]any{"count": 1, "nextPageUrl": "/v1.0/Tickets/query?page=next"},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	client := testClient(t, srv)
	var gotError bool
	for _, err := range ListRawIter(t.Context(), client, "Tickets", NewQuery()) {
		if err != nil {
			if _, ok := errors.AsType[*MaxPagesExceededError](err); !ok {
				t.Fatalf("expected MaxPagesExceededError, got: %v", err)
			}
			gotError = true
			break
		}
	}
	if !gotError {
		t.Fatal("expected MaxPagesExceededError from iterator")
	}
}
