package autotask

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestListIter(t *testing.T) {
	page := 0
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1.0/TestEntities/query", func(w http.ResponseWriter, r *http.Request) {
		page++
		if page == 1 {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"items":       []any{map[string]any{"id": 1}, map[string]any{"id": 2}},
				"pageDetails": map[string]any{"count": 2, "nextPageUrl": "/v1.0/TestEntities/query?page=2"},
			})
		} else {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"items":       []any{map[string]any{"id": 3}},
				"pageDetails": map[string]any{"count": 1},
			})
		}
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	client := testClient(t, srv)
	ids := make([]int64, 0, 3)
	for entity, err := range ListIter[testEntity](t.Context(), client, NewQuery()) {
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
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items":       []any{map[string]any{"id": 1}, map[string]any{"id": 2}, map[string]any{"id": 3}},
			"pageDetails": map[string]any{"count": 3, "nextPageUrl": "/v1.0/TestEntities/query?page=2"},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	client := testClient(t, srv)
	count := 0
	for _, err := range ListIter[testEntity](t.Context(), client, NewQuery()) {
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

func TestListIterMaxPagesGuard(t *testing.T) {
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
	var gotError bool
	for _, err := range ListIter[testEntity](t.Context(), client, NewQuery()) {
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

// TestListIterDecodeError pins the decode-error message for a top-level entity
// and the partial-page behavior: a malformed item yields a wrapped error but
// iteration continues, so a valid item after it is still yielded.
func TestListIterDecodeError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1.0/TestEntities/query", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items":       []any{"not-an-object", map[string]any{"id": 2}},
			"pageDetails": map[string]any{"count": 2},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	client := testClient(t, srv)
	var decodeErr error
	var goodIDs []int64
	for entity, err := range ListIter[testEntity](t.Context(), client, NewQuery()) {
		if err != nil {
			decodeErr = err
			continue
		}
		v, _ := entity.ID.Get()
		goodIDs = append(goodIDs, v)
	}
	if decodeErr == nil {
		t.Fatal("expected a decode error for the malformed item")
	}
	if got, want := decodeErr.Error(), "autotask: decoding TestEntities:"; !strings.Contains(got, want) {
		t.Fatalf("decode error = %q; want it to contain %q", got, want)
	}
	if len(goodIDs) != 1 || goodIDs[0] != 2 {
		t.Fatalf("good ids = %v; want [2] (item after the malformed one must still be yielded)", goodIDs)
	}
}
