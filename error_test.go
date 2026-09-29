package autotask

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestErrorImplementsError(t *testing.T) {
	err := &Error{StatusCode: 400, Message: "bad request"}
	if err.Error() == "" {
		t.Fatal("Error() should return non-empty string")
	}
}

func TestTypedErrorsAsType(t *testing.T) {
	base := Error{StatusCode: http.StatusNotFound, Message: "not found"}
	err := &NotFoundError{Err: base}
	nf, ok := errors.AsType[*NotFoundError](err)
	if !ok {
		t.Fatal("errors.AsType should match NotFoundError")
	}
	if nf.Err.StatusCode != http.StatusNotFound {
		t.Fatalf("StatusCode = %d; want %d", nf.Err.StatusCode, http.StatusNotFound)
	}
}

func TestRateLimitErrorRetryAfter(t *testing.T) {
	err := &RateLimitError{
		Err:        Error{StatusCode: http.StatusTooManyRequests, Message: "too many requests"},
		RetryAfter: 60 * time.Second,
	}
	if err.RetryAfter != 60*time.Second {
		t.Fatalf("RetryAfter = %v; want 60s", err.RetryAfter)
	}
}

func TestParseResponse400(t *testing.T) {
	body := `{"errors":["Field Title is required"]}`
	resp := &http.Response{
		StatusCode: http.StatusBadRequest,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{},
	}
	err := parseResponse(resp, nil)
	if _, ok := errors.AsType[*ValidationError](err); !ok {
		t.Fatalf("expected ValidationError, got %T: %v", err, err)
	}
}

func TestParseResponse401(t *testing.T) {
	resp := &http.Response{
		StatusCode: http.StatusUnauthorized,
		Body:       io.NopCloser(strings.NewReader(`{"errors":["Invalid credentials"]}`)),
		Header:     http.Header{},
	}
	err := parseResponse(resp, nil)
	if _, ok := errors.AsType[*AuthenticationError](err); !ok {
		t.Fatalf("expected AuthenticationError, got %T: %v", err, err)
	}
}

func TestParseResponse429WithRetryAfter(t *testing.T) {
	resp := &http.Response{ //nolint:gocritic // EmbeddedFieldLiteral false positive: http.Response.Header is a named field of type http.Header, not embedded, so it cannot be keyed directly
		StatusCode: http.StatusTooManyRequests,
		Body:       io.NopCloser(strings.NewReader(`{"errors":["Rate limit exceeded"]}`)),
		Header:     http.Header{"Retry-After": []string{"120"}},
	}
	err := parseResponse(resp, nil)
	rle, ok := errors.AsType[*RateLimitError](err)
	if !ok {
		t.Fatalf("expected RateLimitError, got %T: %v", err, err)
	}
	if rle.RetryAfter != 120*time.Second {
		t.Fatalf("RetryAfter = %v; want 120s", rle.RetryAfter)
	}
}

func TestParseResponse500(t *testing.T) {
	resp := &http.Response{
		StatusCode: http.StatusInternalServerError,
		Body:       io.NopCloser(strings.NewReader(`{"errors":["Internal error"]}`)),
		Header:     http.Header{},
	}
	err := parseResponse(resp, nil)
	if _, ok := errors.AsType[*ServerError](err); !ok {
		t.Fatalf("expected ServerError, got %T: %v", err, err)
	}
}

// truncatedServer answers with status and a Content-Length larger than the body
// it writes, so the client's body read fails with io.ErrUnexpectedEOF.
func truncatedServer(t *testing.T, status int, header http.Header) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		for k, v := range header {
			w.Header()[k] = v
		}
		w.Header().Set("Content-Length", "100")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{"errors":["par`))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestDoBodyReadFailureKeepsStatusType(t *testing.T) {
	t.Run("429 keeps RetryAfter", func(t *testing.T) {
		srv := truncatedServer(t, http.StatusTooManyRequests, http.Header{"Retry-After": {"120"}})
		c := &Client{httpClient: srv.Client(), baseURL: srv.URL}
		err := c.do(t.Context(), http.MethodGet, "/x", nil, nil)
		rle, ok := errors.AsType[*RateLimitError](err)
		if !ok {
			t.Fatalf("expected RateLimitError, got %T: %v", err, err)
		}
		if rle.RetryAfter != 120*time.Second {
			t.Fatalf("RetryAfter = %v; want 120s", rle.RetryAfter)
		}
		if !errors.Is(err, io.ErrUnexpectedEOF) {
			t.Fatalf("read error not preserved: %v", err)
		}
	})
	t.Run("503", func(t *testing.T) {
		srv := truncatedServer(t, http.StatusServiceUnavailable, nil)
		c := &Client{httpClient: srv.Client(), baseURL: srv.URL}
		err := c.do(t.Context(), http.MethodGet, "/x", nil, nil)
		se, ok := errors.AsType[*ServerError](err)
		if !ok {
			t.Fatalf("expected ServerError, got %T: %v", err, err)
		}
		if se.Err.StatusCode != http.StatusServiceUnavailable || len(se.Err.Errors) != 0 {
			t.Fatalf("unexpected Err: %+v", se.Err)
		}
		if !errors.Is(err, io.ErrUnexpectedEOF) {
			t.Fatalf("read error not preserved: %v", err)
		}
		want := "autotask: 503 Service Unavailable: autotask: reading response body: unexpected EOF"
		if err.Error() != want {
			t.Fatalf("Error() = %q; want %q", err.Error(), want)
		}
	})
	t.Run("2xx keeps the read error only", func(t *testing.T) {
		srv := truncatedServer(t, http.StatusOK, nil)
		c := &Client{httpClient: srv.Client(), baseURL: srv.URL}
		err := c.do(t.Context(), http.MethodGet, "/x", nil, nil)
		if !errors.Is(err, io.ErrUnexpectedEOF) {
			t.Fatalf("expected read error, got %T: %v", err, err)
		}
		if _, ok := errors.AsType[*Error](err); ok {
			t.Fatalf("2xx read failure must not be status-typed: %v", err)
		}
	})
}

func TestParseResponse200Success(t *testing.T) {
	body := `{"item":{"id":123}}`
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{},
	}
	var result struct {
		Item struct {
			ID int `json:"id"`
		} `json:"item"`
	}
	err := parseResponse(resp, &result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Item.ID != 123 {
		t.Fatalf("ID = %d; want 123", result.Item.ID)
	}
}

func TestParseResponse200WithErrors(t *testing.T) {
	body := `{"errors":["Something went wrong"]}`
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{},
	}
	err := parseResponse(resp, nil)
	if err == nil {
		t.Fatal("expected error for 200 response with errors array")
	}
}

func TestParseRetryAfterSeconds(t *testing.T) {
	d := parseRetryAfter("30")
	if d != 30*time.Second {
		t.Fatalf("parseRetryAfter('30') = %v; want 30s", d)
	}
}

func TestParseRetryAfterDefault(t *testing.T) {
	d := parseRetryAfter("")
	if d != 60*time.Second {
		t.Fatalf("parseRetryAfter('') = %v; want 60s", d)
	}
}
