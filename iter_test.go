package autotask

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListIter(t *testing.T) {
	page := 0
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1.0/TestEntities/query", func(w http.ResponseWriter, r *http.Request) {
		page++
		if page == 1 {
			json.NewEncoder(w).Encode(map[string]any{
				"items":       []any{map[string]any{"id": 1}, map[string]any{"id": 2}},
				"pageDetails": map[string]any{"count": 2, "nextPageUrl": "/v1.0/TestEntities/query?page=2"},
			})
		} else {
			json.NewEncoder(w).Encode(map[string]any{
				"items":       []any{map[string]any{"id": 3}},
				"pageDetails": map[string]any{"count": 1},
			})
		}
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	client := testClient(t, srv)
	var ids []int64
	for entity, err := range ListIter[testEntity](context.Background(), client, NewQuery()) {
		if err != nil {
			t.Fatal(err)
		}
		v, _ := entity.ID.Get()
		ids = append(ids, v)
	}
	if len(ids) != 3 {
		t.Fatalf("got %d items; want 3", len(ids))
	}
}

func TestListIterBreakEarly(t *testing.T) {
	reqs := 0
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1.0/TestEntities/query", func(w http.ResponseWriter, r *http.Request) {
		reqs++
		json.NewEncoder(w).Encode(map[string]any{
			"items":       []any{map[string]any{"id": 1}, map[string]any{"id": 2}, map[string]any{"id": 3}},
			"pageDetails": map[string]any{"count": 3, "nextPageUrl": "/v1.0/TestEntities/query?page=2"},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	client := testClient(t, srv)
	count := 0
	for _, err := range ListIter[testEntity](context.Background(), client, NewQuery()) {
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
