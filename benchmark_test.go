package autotask

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

var benchSink any

func BenchmarkQueryMarshal(b *testing.B) {
	q := NewQuery().
		Where("status", OpEq, 1).
		Where("queueID", OpEq, 8).
		Or(Field("priority", OpEq, 1), Field("priority", OpEq, 2)).
		Fields("id", "title", "status").
		Limit(100)
	for b.Loop() {
		benchSink, _ = json.Marshal(q)
	}
}

func BenchmarkParseResponseSuccess(b *testing.B) {
	body := `{"item":{"id":123,"title":"Test Ticket","status":1}}`
	for b.Loop() {
		resp := &http.Response{
			StatusCode: 200,
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
			StatusCode: 404,
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
		benchSink, _ = json.Marshal(s)
	}
}
