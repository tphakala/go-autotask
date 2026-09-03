package autotask

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

var benchSink any

// benchPageBody returns a single-page list/query response body carrying n items.
// Writing pre-encoded bytes keeps the mock handler off the hot path so the
// benchmark measures the client-side decode-and-accumulate work.
func benchPageBody(tb testing.TB, n int) []byte {
	tb.Helper()
	items := make([]map[string]any, n)
	for i := range items {
		items[i] = map[string]any{"id": i + 1, "title": "Item"}
	}
	body, err := json.Marshal(map[string]any{
		"items":       items,
		"pageDetails": map[string]any{"count": n},
	})
	if err != nil {
		tb.Fatal(err)
	}
	return body
}

func BenchmarkList(b *testing.B) {
	body := benchPageBody(b, 100)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1.0/TestEntities/query", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	})
	srv := httptest.NewServer(mux)
	b.Cleanup(srv.Close)
	client := testClient(b, srv)
	q := NewQuery()
	b.ReportAllocs()
	for b.Loop() {
		items, err := List[testEntity](b.Context(), client, q)
		if err != nil {
			b.Fatal(err)
		}
		if len(items) != 100 {
			b.Fatalf("got %d items; want 100", len(items))
		}
		benchSink = items
	}
}

func BenchmarkListChild(b *testing.B) {
	body := benchPageBody(b, 100)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1.0/TestEntities/{parentID}/Notes", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	})
	srv := httptest.NewServer(mux)
	b.Cleanup(srv.Close)
	client := testClient(b, srv)
	b.ReportAllocs()
	for b.Loop() {
		items, err := ListChild[testEntity, testChildEntity](b.Context(), client, 42)
		if err != nil {
			b.Fatal(err)
		}
		if len(items) != 100 {
			b.Fatalf("got %d items; want 100", len(items))
		}
		benchSink = items
	}
}

func BenchmarkListRaw(b *testing.B) {
	body := benchPageBody(b, 100)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1.0/Tickets/query", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	})
	srv := httptest.NewServer(mux)
	b.Cleanup(srv.Close)
	client := testClient(b, srv)
	q := NewQuery()
	b.ReportAllocs()
	for b.Loop() {
		items, err := ListRaw(b.Context(), client, "Tickets", q)
		if err != nil {
			b.Fatal(err)
		}
		if len(items) != 100 {
			b.Fatalf("got %d items; want 100", len(items))
		}
		benchSink = items
	}
}

func BenchmarkQueryMarshal(b *testing.B) {
	q := NewQuery().
		Where("status", OpEq, 1).
		Where("queueID", OpEq, 8).
		Or(Field("priority", OpEq, 1), Field("priority", OpEq, 2)).
		Fields("id", "title", "status").
		Limit(100)
	for b.Loop() {
		benchSink, _ = json.Marshal(q) //nolint:errchkjson // benchmark intentionally discards marshal error
	}
}

func BenchmarkParseResponseSuccess(b *testing.B) {
	body := `{"item":{"id":123,"title":"Test Ticket","status":1}}`
	for b.Loop() {
		resp := &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{},
		}
		var result map[string]any
		benchSink = parseResponse(resp, &result)
	}
}

func BenchmarkParseResponseError(b *testing.B) {
	body := `{"errors":["Not found"]}`
	for b.Loop() {
		resp := &http.Response{
			StatusCode: http.StatusNotFound,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{},
		}
		benchSink = parseResponse(resp, nil)
	}
}

func BenchmarkOptionalMarshal(b *testing.B) {
	type S struct {
		Name  Optional[string] `json:"name,omitzero"`
		Value Optional[int]    `json:"value,omitzero"`
		Clear Optional[string] `json:"clear,omitzero"`
	}
	s := S{Name: Set("test"), Clear: Null[string]()}
	for b.Loop() {
		benchSink, _ = json.Marshal(s) //nolint:errchkjson // benchmark intentionally discards marshal error
	}
}
