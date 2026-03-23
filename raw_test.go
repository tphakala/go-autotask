package autotask

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func newCRUDTestServer(t *testing.T) (*httptest.Server, *[]http.Request) {
	t.Helper()
	var mu sync.Mutex
	var requests []http.Request
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1.0/Tickets/{id}", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requests = append(requests, *r)
		mu.Unlock()
		json.NewEncoder(w).Encode(map[string]any{
			"item": map[string]any{"id": 123, "title": "Test Ticket"},
		})
	})
	mux.HandleFunc("POST /v1.0/Tickets/query", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requests = append(requests, *r)
		mu.Unlock()
		json.NewEncoder(w).Encode(map[string]any{
			"items":       []any{map[string]any{"id": 1}, map[string]any{"id": 2}},
			"pageDetails": map[string]any{"count": 2, "nextPageUrl": nil},
		})
	})
	mux.HandleFunc("POST /v1.0/Tickets", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requests = append(requests, *r)
		mu.Unlock()
		json.NewEncoder(w).Encode(map[string]any{"itemId": 456})
	})
	mux.HandleFunc("PATCH /v1.0/Tickets", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requests = append(requests, *r)
		mu.Unlock()
		json.NewEncoder(w).Encode(map[string]any{"item": map[string]any{"id": 123}})
	})
	mux.HandleFunc("DELETE /v1.0/Tickets/{id}", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requests = append(requests, *r)
		mu.Unlock()
		w.WriteHeader(200)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, &requests
}

func testClient(t *testing.T, srv *httptest.Server) *Client {
	t.Helper()
	auth := AuthConfig{Username: "u", Secret: "s", IntegrationCode: "c"}
	client, err := NewClient(context.Background(), auth, WithBaseURL(srv.URL))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { client.Close() })
	return client
}

func TestGetRaw(t *testing.T) {
	srv, _ := newCRUDTestServer(t)
	client := testClient(t, srv)
	result, err := GetRaw(context.Background(), client, "Tickets", 123)
	if err != nil {
		t.Fatal(err)
	}
	if result["title"] != "Test Ticket" {
		t.Fatalf("title = %v", result["title"])
	}
}

func TestListRaw(t *testing.T) {
	srv, _ := newCRUDTestServer(t)
	client := testClient(t, srv)
	results, err := ListRaw(context.Background(), client, "Tickets", NewQuery().Where("status", OpEq, 1))
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("results = %d; want 2", len(results))
	}
}

func TestCreateRaw(t *testing.T) {
	srv, _ := newCRUDTestServer(t)
	client := testClient(t, srv)
	data := map[string]any{"title": "New Ticket"}
	result, err := CreateRaw(context.Background(), client, "Tickets", data)
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestUpdateRaw(t *testing.T) {
	srv, _ := newCRUDTestServer(t)
	client := testClient(t, srv)
	data := map[string]any{"id": 123, "title": "Updated"}
	result, err := UpdateRaw(context.Background(), client, "Tickets", data)
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestDeleteRaw(t *testing.T) {
	srv, _ := newCRUDTestServer(t)
	client := testClient(t, srv)
	err := DeleteRaw(context.Background(), client, "Tickets", 123)
	if err != nil {
		t.Fatal(err)
	}
}
