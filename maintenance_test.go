package autotask

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"unicode/utf8"
)

// maintenancePage mimics the page Autotask serves with HTTP 200 during planned
// maintenance.
const maintenancePage = `<!DOCTYPE html><html><head><meta http-equiv="refresh" content="60"><title>Unavailable</title></head><body>Autotask is down for maintenance.</body></html>`

// newMaintenanceServer answers every request with 200, text/html and the
// maintenance page.
func newMaintenanceServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, maintenancePage)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func requireContentTypeError(t *testing.T, err error) {
	t.Helper()
	ct, ok := errors.AsType[*UnexpectedContentTypeError](err)
	if !ok {
		t.Fatalf("expected UnexpectedContentTypeError, got %T: %v", err, err)
	}
	if ct.Err.StatusCode != http.StatusOK {
		t.Fatalf("Err.StatusCode = %d; want %d", ct.Err.StatusCode, http.StatusOK)
	}
	if _, ok := errors.AsType[*Error](err); !ok {
		t.Fatalf("errors.AsType[*Error] does not match %T: %v", err, err)
	}
	const wantText = `autotask: 200 response body is not JSON (Content-Type "text/html; charset=utf-8")`
	if got := ct.Error(); got != wantText {
		t.Fatalf("Error() = %q; want %q", got, wantText)
	}
	if ct.ContentType != "text/html; charset=utf-8" {
		t.Fatalf("ContentType = %q; want %q", ct.ContentType, "text/html; charset=utf-8")
	}
	if !strings.HasPrefix(ct.Snippet, "<!DOCTYPE html>") {
		t.Fatalf("Snippet = %q; want the start of the page", ct.Snippet)
	}
}

func TestMaintenancePageIsTypedError(t *testing.T) {
	srv := newMaintenanceServer(t)
	client := testClient(t, srv)
	ctx := t.Context()

	calls := map[string]func() error{
		"Get": func() error {
			_, err := Get[testEntity](ctx, client, 1)
			return err
		},
		"List": func() error {
			_, err := List[testEntity](ctx, client, NewQuery())
			return err
		},
		"Update": func() error {
			_, err := Update(ctx, client, &testEntity{ID: Set(int64(1)), Title: Set("x")})
			return err
		},
		// Delete and DeleteRaw decode no result, so before the check they reported success.
		"Delete": func() error {
			return Delete[testEntity](ctx, client, 1)
		},
		"DeleteRaw": func() error {
			return DeleteRaw(ctx, client, "TestEntities", 1)
		},
		"GetRaw": func() error {
			_, err := GetRaw(ctx, client, "TestEntities", 1)
			return err
		},
	}
	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			requireContentTypeError(t, call())
		})
	}
}

func TestZoneDiscoveryMaintenancePage(t *testing.T) {
	auth := AuthConfig{Username: "u", Secret: "s", IntegrationCode: "c"}

	t.Run("version endpoint", func(t *testing.T) {
		srv := newMaintenanceServer(t)
		_, err := NewClient(t.Context(), auth, WithZoneBaseURL(srv.URL))
		requireContentTypeError(t, err)
		if !strings.Contains(err.Error(), "decoding version response") {
			t.Fatalf("error %q lost its version-response context", err)
		}
	})

	t.Run("zone endpoint", func(t *testing.T) {
		mux := http.NewServeMux()
		mux.HandleFunc("GET /atservicesrest/versioninformation", func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, `{"apiVersions":["V1.0"]}`)
		})
		mux.HandleFunc("GET /atservicesrest/V1.0/zoneInformation", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = io.WriteString(w, maintenancePage)
		})
		srv := httptest.NewServer(mux)
		t.Cleanup(srv.Close)
		_, err := NewClient(t.Context(), auth, WithZoneBaseURL(srv.URL))
		requireContentTypeError(t, err)
		if !strings.Contains(err.Error(), "decoding zone response") {
			t.Fatalf("error %q lost its zone-response context", err)
		}
	})
}

func TestZoneDiscoveryTruncatedBody(t *testing.T) {
	// The declared length exceeds what is written, so reading the body fails.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "100")
		_, _ = io.WriteString(w, `{"apiVersions":`)
	}))
	t.Cleanup(srv.Close)
	auth := AuthConfig{Username: "u", Secret: "s", IntegrationCode: "c"}
	_, err := NewClient(t.Context(), auth, WithZoneBaseURL(srv.URL))
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("err = %v; want it to wrap io.ErrUnexpectedEOF", err)
	}
	if !strings.Contains(err.Error(), "decoding version response") {
		t.Fatalf("error %q lost its version-response context", err)
	}
}

func TestParseResponseContentType(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		body        string
		wantTyped   bool // want *UnexpectedContentTypeError
		wantErr     bool
		// decodeErr: an untyped decode error, only when there is a result to decode.
		decodeErr bool
	}{
		{name: "html page", contentType: "text/html", body: maintenancePage, wantTyped: true, wantErr: true},
		{name: "no content type, not json", body: "Service Unavailable", wantTyped: true, wantErr: true},
		{name: "json", contentType: "application/json; charset=utf-8", body: `{"id":1}`},
		{name: "json mixed case", contentType: "Application/JSON", body: `{"id":1}`},
		{name: "structured +json", contentType: "application/problem+json", body: `{"id":1}`},
		{name: "json sniffed as text/plain", contentType: "text/plain; charset=utf-8", body: `{"id":1}`},
		{name: "json without content type", body: `{"id":1}`},
		{name: "empty html body", contentType: "text/html"},
		// Passes the check; decoding it fails as it did before the check existed.
		{name: "whitespace html body", contentType: "text/html", body: " \r\n", decodeErr: true},
		// Labelled JSON but malformed: stays a decode error, not a content-type error.
		{name: "malformed json", contentType: "application/json", body: `{"id":`, decodeErr: true},
		{name: "malformed json mixed case", contentType: "Application/JSON", body: `{"id":`, decodeErr: true},
		{name: "malformed structured +json", contentType: "application/problem+json", body: `{"id":`, decodeErr: true},
	}
	for _, tt := range tests {
		for _, withResult := range []bool{true, false} {
			t.Run(tt.name, func(t *testing.T) {
				resp := &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{},
					Body:       io.NopCloser(strings.NewReader(tt.body)),
				}
				if tt.contentType != "" {
					resp.Header.Set("Content-Type", tt.contentType)
				}
				var result struct {
					ID int `json:"id"`
				}
				var target any
				if withResult {
					target = &result
				}
				err := parseResponse(resp, target)
				wantErr := tt.wantErr || (withResult && tt.decodeErr)
				if (err != nil) != wantErr {
					t.Fatalf("withResult=%v: err = %v; wantErr %v", withResult, err, wantErr)
				}
				if _, typed := errors.AsType[*UnexpectedContentTypeError](err); typed != tt.wantTyped {
					t.Fatalf("withResult=%v: typed = %v; want %v (err %v)", withResult, typed, tt.wantTyped, err)
				}
				if withResult && !wantErr && tt.body != "" && result.ID != 1 {
					t.Fatalf("ID = %d; want 1", result.ID)
				}
			})
		}
	}
}

func TestUnexpectedContentTypeSnippetBounded(t *testing.T) {
	// "é" is two bytes; an odd-length prefix puts a character across the cut.
	body := "x" + strings.Repeat("é", maxSnippetBytes)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
	resp.Header.Set("Content-Type", "text/html")
	ct, ok := errors.AsType[*UnexpectedContentTypeError](parseResponse(resp, nil))
	if !ok {
		t.Fatal("expected UnexpectedContentTypeError")
	}
	if len(ct.Snippet) > maxSnippetBytes {
		t.Fatalf("len(Snippet) = %d; want <= %d", len(ct.Snippet), maxSnippetBytes)
	}
	if !utf8.ValidString(ct.Snippet) {
		t.Fatalf("Snippet is not valid UTF-8: %q", ct.Snippet)
	}
	if want := body[:maxSnippetBytes-1]; ct.Snippet != want {
		t.Fatalf("Snippet = %q; want %q", ct.Snippet, want)
	}
}

func TestUnexpectedContentTypeNon200Status(t *testing.T) {
	resp := &http.Response{
		StatusCode: http.StatusAccepted,
		Header:     http.Header{},
		Body:       io.NopCloser(strings.NewReader(maintenancePage)),
	}
	resp.Header.Set("Content-Type", "text/html")
	ct, ok := errors.AsType[*UnexpectedContentTypeError](parseResponse(resp, nil))
	if !ok {
		t.Fatal("expected UnexpectedContentTypeError")
	}
	if ct.Err.StatusCode != http.StatusAccepted {
		t.Fatalf("Err.StatusCode = %d; want %d", ct.Err.StatusCode, http.StatusAccepted)
	}
}
