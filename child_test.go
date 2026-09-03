package autotask

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type testChildEntity struct { //nolint:recvcheck // EntityName uses value receiver (Entity interface), SetID uses pointer receiver (EntityWithID)
	ID      Optional[int64]  `json:"id,omitzero"`
	Message Optional[string] `json:"message,omitzero"`
}

func (testChildEntity) EntityName() string { return "Notes" }

func (e *testChildEntity) SetID(id int64) { e.ID = Set(id) }

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
	const nextPath = "/v1.0/TestEntities/42/Notes?page=2"
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1.0/TestEntities/{parentID}/Notes", func(w http.ResponseWriter, r *http.Request) {
		page++
		if page == 1 {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"items": []any{
					map[string]any{"id": 10, "message": "note 1"},
					map[string]any{"id": 11, "message": "note 2"},
				},
				"pageDetails": map[string]any{"count": 2, "nextPageUrl": nextPath},
			})
			return
		}
		if got := r.URL.RequestURI(); got != nextPath {
			t.Fatalf("request URI = %q; want %q", got, nextPath)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items": []any{
				map[string]any{"id": 12, "message": "note 3"},
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
	if len(children) != 3 {
		t.Fatalf("len = %d; want 3", len(children))
	}
	id, _ := children[2].ID.Get()
	if id != 12 {
		t.Fatalf("last child id = %d; want 12", id)
	}
}

// TestListChildIterDecodeError pins the child decode-error message (the " child"
// suffix) and the partial-page behavior: a malformed item yields a wrapped
// error but iteration continues, so a valid item after it is still yielded.
func TestListChildIterDecodeError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1.0/TestEntities/{parentID}/Notes", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items":       []any{"not-an-object", map[string]any{"id": 11}},
			"pageDetails": map[string]any{"count": 2},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	client := testClient(t, srv)
	var decodeErr error
	var goodIDs []int64
	for entity, err := range ListChildIter[testEntity, testChildEntity](t.Context(), client, 42) {
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
	if got, want := decodeErr.Error(), "autotask: decoding Notes child:"; !strings.Contains(got, want) {
		t.Fatalf("decode error = %q; want it to contain %q", got, want)
	}
	if len(goodIDs) != 1 || goodIDs[0] != 11 {
		t.Fatalf("good ids = %v; want [11] (item after the malformed one must still be yielded)", goodIDs)
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

func TestListChildIter(t *testing.T) {
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
	ids := make([]int64, 0, 3)
	for entity, err := range ListChildIter[testEntity, testChildEntity](t.Context(), client, 42) {
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

func TestListChildIterBreakEarly(t *testing.T) {
	reqs := 0
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1.0/TestEntities/{parentID}/Notes", func(w http.ResponseWriter, r *http.Request) {
		reqs++
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items": []any{
				map[string]any{"id": 10, "message": "a"},
				map[string]any{"id": 11, "message": "b"},
				map[string]any{"id": 12, "message": "c"},
			},
			"pageDetails": map[string]any{"count": 3, "nextPageUrl": "/v1.0/TestEntities/42/Notes?page=2"},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	client := testClient(t, srv)
	count := 0
	for _, err := range ListChildIter[testEntity, testChildEntity](t.Context(), client, 42) {
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

func TestListChildIterAPIError(t *testing.T) {
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
	for _, err := range ListChildIter[testEntity, testChildEntity](t.Context(), client, 42) {
		if err == nil {
			t.Fatal("expected error from API")
		}
		return
	}
	t.Fatal("iterator should have yielded an error")
}

func TestListChildMaxPagesGuard(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1.0/TestEntities/{parentID}/Notes", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items":       []any{map[string]any{"id": 1, "message": "note"}},
			"pageDetails": map[string]any{"count": 1, "nextPageUrl": "/v1.0/TestEntities/42/Notes?page=next"},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	client := testClient(t, srv)
	_, err := ListChild[testEntity, testChildEntity](t.Context(), client, 42)
	if err == nil {
		t.Fatal("expected MaxPagesExceededError")
	}
	if _, ok := errors.AsType[*MaxPagesExceededError](err); !ok {
		t.Fatalf("expected MaxPagesExceededError, got: %v", err)
	}
}

func TestListChildIterMaxPagesGuard(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1.0/TestEntities/{parentID}/Notes", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items":       []any{map[string]any{"id": 1, "message": "note"}},
			"pageDetails": map[string]any{"count": 1, "nextPageUrl": "/v1.0/TestEntities/42/Notes?page=next"},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	client := testClient(t, srv)
	var gotError bool
	for _, err := range ListChildIter[testEntity, testChildEntity](t.Context(), client, 42) {
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

func TestCreateChildSetsItemID(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1.0/TestEntities/{parentID}/Notes", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"itemId": 55})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	client := testClient(t, srv)
	child := &testChildEntity{Message: Set("new note")}
	result, err := CreateChild[testEntity](t.Context(), client, 42, child)
	if err != nil {
		t.Fatal(err)
	}
	id, ok := result.ID.Get()
	if !ok || id != 55 {
		t.Fatalf("ID = %v, %v; want 55, true", id, ok)
	}
}

func TestListChildIterErrorOnSecondPage(t *testing.T) {
	page := 0
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1.0/TestEntities/{parentID}/Notes", func(w http.ResponseWriter, r *http.Request) {
		page++
		if page == 1 {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"items": []any{
					map[string]any{"id": 10, "message": "note 1"},
				},
				"pageDetails": map[string]any{"count": 1, "nextPageUrl": "/v1.0/TestEntities/42/Notes?page=2"},
			})
		} else {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"errors": []any{map[string]any{"message": "page 2 error"}},
			})
		}
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	client := testClient(t, srv)
	var gotItems int
	var gotError bool
	for _, err := range ListChildIter[testEntity, testChildEntity](t.Context(), client, 42) {
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

func TestListChildRaw(t *testing.T) {
	page := 0
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1.0/Tickets/42/TicketNotes", func(w http.ResponseWriter, r *http.Request) {
		page++
		if page == 1 {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"items": []any{
					map[string]any{"id": 10, "message": "note 1"},
				},
				"pageDetails": map[string]any{"count": 1, "nextPageUrl": "/v1.0/Tickets/42/TicketNotes?page=2"},
			})
		} else {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"items": []any{
					map[string]any{"id": 11, "message": "note 2"},
				},
				"pageDetails": map[string]any{"count": 1},
			})
		}
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	client := testClient(t, srv)
	items, err := ListChildRaw(t.Context(), client, "Tickets", 42, "TicketNotes")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("len = %d; want 2", len(items))
	}
	// Pin accumulate order/content across pages, not just the count.
	wantIDs := []float64{10, 11}
	for i, want := range wantIDs {
		got, ok := items[i]["id"].(float64)
		if !ok || got != want {
			t.Fatalf("items[%d][id] = %v (ok=%v); want %v", i, items[i]["id"], ok, want)
		}
	}
}

func TestListChildRawError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1.0/Tickets/42/TicketNotes", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"errors": []any{map[string]any{"message": "server error"}},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	client := testClient(t, srv)
	items, err := ListChildRaw(t.Context(), client, "Tickets", 42, "TicketNotes")
	if err == nil {
		t.Fatal("expected error from API")
	}
	if items != nil {
		t.Fatalf("items = %v; want nil on error", items)
	}
}

func TestListChildRawIter(t *testing.T) {
	page := 0
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1.0/Tickets/42/TicketNotes", func(w http.ResponseWriter, r *http.Request) {
		page++
		if page == 1 {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"items": []any{
					map[string]any{"id": 10, "message": "note 1"},
					map[string]any{"id": 11, "message": "note 2"},
				},
				"pageDetails": map[string]any{"count": 2, "nextPageUrl": "/v1.0/Tickets/42/TicketNotes?page=2"},
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items": []any{
				map[string]any{"id": 12, "message": "note 3"},
			},
			"pageDetails": map[string]any{"count": 1},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	client := testClient(t, srv)
	var ids []float64
	for item, err := range ListChildRawIter(t.Context(), client, "Tickets", 42, "TicketNotes") {
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

func TestListChildRawIterBreakEarly(t *testing.T) {
	reqs := 0
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1.0/Tickets/42/TicketNotes", func(w http.ResponseWriter, r *http.Request) {
		reqs++
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items": []any{
				map[string]any{"id": 10, "message": "a"},
				map[string]any{"id": 11, "message": "b"},
				map[string]any{"id": 12, "message": "c"},
			},
			"pageDetails": map[string]any{"count": 3, "nextPageUrl": "/v1.0/Tickets/42/TicketNotes?page=2"},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	client := testClient(t, srv)
	count := 0
	for _, err := range ListChildRawIter(t.Context(), client, "Tickets", 42, "TicketNotes") {
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

func TestListChildRawIterAPIError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1.0/Tickets/42/TicketNotes", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"errors": []any{map[string]any{"message": "server error"}},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	client := testClient(t, srv)
	for _, err := range ListChildRawIter(t.Context(), client, "Tickets", 42, "TicketNotes") {
		if err == nil {
			t.Fatal("expected error from API")
		}
		return
	}
	t.Fatal("iterator should have yielded an error")
}

func TestListChildRawIterErrorOnSecondPage(t *testing.T) {
	page := 0
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1.0/Tickets/42/TicketNotes", func(w http.ResponseWriter, r *http.Request) {
		page++
		if page == 1 {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"items": []any{
					map[string]any{"id": 10, "message": "note 1"},
				},
				"pageDetails": map[string]any{"count": 1, "nextPageUrl": "/v1.0/Tickets/42/TicketNotes?page=2"},
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
	for _, err := range ListChildRawIter(t.Context(), client, "Tickets", 42, "TicketNotes") {
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

func TestListChildRawIterMaxPagesGuard(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1.0/Tickets/42/TicketNotes", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items":       []any{map[string]any{"id": 1, "message": "note"}},
			"pageDetails": map[string]any{"count": 1, "nextPageUrl": "/v1.0/Tickets/42/TicketNotes?page=next"},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	client := testClient(t, srv)
	var gotError bool
	for _, err := range ListChildRawIter(t.Context(), client, "Tickets", 42, "TicketNotes") {
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

func TestCreateChildRaw(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1.0/Tickets/42/TicketNotes", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"itemId": 99})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	client := testClient(t, srv)
	data := map[string]any{"message": "new note"}
	result, err := CreateChildRaw(t.Context(), client, "Tickets", 42, "TicketNotes", data)
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result["itemId"] != float64(99) {
		t.Fatalf("itemId = %v; want 99", result["itemId"])
	}
}
