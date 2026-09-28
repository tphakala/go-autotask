package autotask_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	autotask "github.com/tphakala/go-autotask"
	"github.com/tphakala/go-autotask/autotasktest"
	"github.com/tphakala/go-autotask/entities"
	"github.com/tphakala/go-autotask/middleware"
)

func TestAuthHeadersPresent(t *testing.T) {
	t.Parallel()
	comp := autotasktest.CompanyFixture()
	srv, client := autotasktest.NewServer(t,
		autotasktest.WithAuth("myuser", "mysecret", "mycode"),
		autotasktest.WithEntity(comp),
	)

	id, _ := comp.ID.Get()
	_, err := autotask.Get[entities.Company](t.Context(), client, id)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	req := srv.LastRequest()
	if got := req.Headers.Get("UserName"); got != "myuser" {
		t.Errorf("UserName header = %q; want %q", got, "myuser")
	}
	if got := req.Headers.Get("Secret"); got != "mysecret" {
		t.Errorf("Secret header = %q; want %q", got, "mysecret")
	}
	if got := req.Headers.Get("ApiIntegrationCode"); got != "mycode" {
		t.Errorf("ApiIntegrationCode header = %q; want %q", got, "mycode")
	}
}

func TestAuthImpersonationPresent(t *testing.T) {
	t.Parallel()
	var gotHeaders http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeaders = r.Header.Clone()
		_ = json.NewEncoder(w).Encode(map[string]any{"item": map[string]any{"id": 1}})
	}))
	t.Cleanup(srv.Close)

	auth := autotask.AuthConfig{Username: "user", Secret: "secret", IntegrationCode: "code"}
	client, err := autotask.NewClient(t.Context(), auth,
		autotask.WithBaseURL(srv.URL),
		autotask.WithImpersonation(12345),
	)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	_, err = autotask.Get[entities.Company](t.Context(), client, 1)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if got := gotHeaders.Get("ImpersonationResourceId"); got != "12345" {
		t.Errorf("ImpersonationResourceId = %q; want %q", got, "12345")
	}
}

func TestAuthImpersonationAbsent(t *testing.T) {
	t.Parallel()
	var gotHeaders http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeaders = r.Header.Clone()
		_ = json.NewEncoder(w).Encode(map[string]any{"item": map[string]any{"id": 1}})
	}))
	t.Cleanup(srv.Close)

	auth := autotask.AuthConfig{Username: "user", Secret: "secret", IntegrationCode: "code"}
	client, err := autotask.NewClient(t.Context(), auth, autotask.WithBaseURL(srv.URL))
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	_, err = autotask.Get[entities.Company](t.Context(), client, 1)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if got := gotHeaders.Get("ImpersonationResourceId"); got != "" {
		t.Errorf("ImpersonationResourceId = %q; want empty (no impersonation configured)", got)
	}
}

func TestAuthContentTypeAndUserAgent(t *testing.T) {
	t.Parallel()
	var gotHeaders http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeaders = r.Header.Clone()
		_ = json.NewEncoder(w).Encode(map[string]any{"item": map[string]any{"id": 1}})
	}))
	t.Cleanup(srv.Close)

	auth := autotask.AuthConfig{Username: "user", Secret: "secret", IntegrationCode: "code"}
	client, err := autotask.NewClient(t.Context(), auth, autotask.WithBaseURL(srv.URL))
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	_, err = autotask.Get[entities.Company](t.Context(), client, 1)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if got := gotHeaders.Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q; want %q", got, "application/json")
	}
	if got := gotHeaders.Get("User-Agent"); !strings.HasPrefix(got, "go-autotask/") {
		t.Errorf("User-Agent = %q; want prefix %q", got, "go-autotask/")
	}
}

func TestAuthSameOriginValidation(t *testing.T) {
	t.Parallel()

	// evilServer is a cross-origin server that should NOT receive auth credentials.
	var mu sync.Mutex
	var evilHeaders http.Header
	evilServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		evilHeaders = r.Header.Clone()
		mu.Unlock()
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items":       []any{},
			"pageDetails": map[string]any{"count": 0},
		})
	}))
	t.Cleanup(evilServer.Close)

	// legitimateServer returns a first page of results with a nextPageUrl pointing
	// to evilServer, simulating a spoofed pagination URL.
	legitimateServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items": []any{
				map[string]any{"id": 1, "companyName": "Acme"},
			},
			"pageDetails": map[string]any{
				"count":      1,
				"nextPageUrl": evilServer.URL + "/v1.0/Companies/query?page=2",
			},
		})
	}))
	t.Cleanup(legitimateServer.Close)

	auth := autotask.AuthConfig{Username: "secretuser", Secret: "secretpass", IntegrationCode: "secretcode"}
	client, err := autotask.NewClient(t.Context(), auth, autotask.WithBaseURL(legitimateServer.URL))
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	// List triggers a query that auto-follows pagination.
	q := autotask.NewQuery().Where("id", autotask.OpGt, 0)
	_, err = autotask.List[entities.Company](t.Context(), client, q)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	// Verify that the evil server received the request but NOT the auth credentials.
	mu.Lock()
	defer mu.Unlock()

	if evilHeaders == nil {
		t.Fatal("evilServer received no request; expected a cross-origin pagination request")
	}
	if got := evilHeaders.Get("UserName"); got != "" {
		t.Errorf("cross-origin UserName = %q; want empty (credentials should not leak)", got)
	}
	if got := evilHeaders.Get("Secret"); got != "" {
		t.Errorf("cross-origin Secret = %q; want empty (credentials should not leak)", got)
	}
	if got := evilHeaders.Get("ApiIntegrationCode"); got != "" {
		t.Errorf("cross-origin ApiIntegrationCode = %q; want empty (credentials should not leak)", got)
	}

	// Content-Type and User-Agent should still be set (they are not secrets).
	if got := evilHeaders.Get("Content-Type"); got != "application/json" {
		t.Errorf("cross-origin Content-Type = %q; want %q", got, "application/json")
	}
	if got := evilHeaders.Get("User-Agent"); !strings.HasPrefix(got, "go-autotask/") {
		t.Errorf("cross-origin User-Agent = %q; want prefix %q", got, "go-autotask/")
	}
}

// headerRecorder is a test server that records the headers of each request it
// receives and answers with a JSON item.
type headerRecorder struct {
	srv     *httptest.Server
	mu      sync.Mutex
	headers []http.Header
}

func newHeaderRecorder(t *testing.T, body any) *headerRecorder {
	t.Helper()
	rec := &headerRecorder{}
	rec.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.mu.Lock()
		rec.headers = append(rec.headers, r.Header.Clone())
		rec.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(body)
	}))
	t.Cleanup(rec.srv.Close)
	return rec
}

func (rec *headerRecorder) requests() []http.Header {
	rec.mu.Lock()
	defer rec.mu.Unlock()
	return slices.Clone(rec.headers)
}

// newRedirector returns a server that answers every request with a 307 to
// target joined with the request path.
func newRedirector(t *testing.T, target string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target+r.URL.RequestURI(), http.StatusTemporaryRedirect)
	}))
	t.Cleanup(srv.Close)
	return srv
}

var redirectCredentialHeaders = []string{"UserName", "Secret", "ApiIntegrationCode", "ImpersonationResourceId"}

func requireNoCredentials(t *testing.T, h http.Header) {
	t.Helper()
	for _, name := range redirectCredentialHeaders {
		if got := h.Get(name); got != "" {
			t.Errorf("redirect target got %s = %q; want empty", name, got)
		}
	}
}

func TestAuthCrossOriginRedirectDropsCredentials(t *testing.T) {
	t.Parallel()
	target := newHeaderRecorder(t, map[string]any{"item": map[string]any{"id": 1}})
	origin := newRedirector(t, target.srv.URL)

	auth := autotask.AuthConfig{Username: "secretuser", Secret: "secretpass", IntegrationCode: "secretcode"}
	client, err := autotask.NewClient(t.Context(), auth,
		autotask.WithBaseURL(origin.URL),
		autotask.WithImpersonation(12345),
	)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	if _, err := autotask.Get[entities.Company](t.Context(), client, 1); err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	reqs := target.requests()
	if len(reqs) != 1 {
		t.Fatalf("redirect target got %d requests; want 1", len(reqs))
	}
	requireNoCredentials(t, reqs[0])
	// Headers that are not credentials still follow the redirect.
	if got := reqs[0].Get("User-Agent"); !strings.HasPrefix(got, "go-autotask/") {
		t.Errorf("redirect target User-Agent = %q; want prefix %q", got, "go-autotask/")
	}
}

func TestAuthSameOriginRedirectKeepsCredentials(t *testing.T) {
	t.Parallel()
	var mu sync.Mutex
	var gotHeaders http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/moved/v1.0/Companies/1" {
			mu.Lock()
			gotHeaders = r.Header.Clone()
			mu.Unlock()
			_ = json.NewEncoder(w).Encode(map[string]any{"item": map[string]any{"id": 1}})
			return
		}
		http.Redirect(w, r, "/moved"+r.URL.Path, http.StatusTemporaryRedirect)
	}))
	t.Cleanup(srv.Close)

	auth := autotask.AuthConfig{Username: "user", Secret: "secret", IntegrationCode: "code"}
	client, err := autotask.NewClient(t.Context(), auth,
		autotask.WithBaseURL(srv.URL),
		autotask.WithImpersonation(12345),
	)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	if _, err := autotask.Get[entities.Company](t.Context(), client, 1); err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if gotHeaders == nil {
		t.Fatal("redirected request never reached the server")
	}
	want := map[string]string{
		"UserName":                "user",
		"Secret":                  "secret",
		"ApiIntegrationCode":      "code",
		"ImpersonationResourceId": "12345",
	}
	for name, v := range want {
		if got := gotHeaders.Get(name); got != v {
			t.Errorf("same-origin redirect %s = %q; want %q", name, got, v)
		}
	}
}

func TestAuthRedirectCallsCallerCheckRedirect(t *testing.T) {
	t.Parallel()
	target := newHeaderRecorder(t, map[string]any{"item": map[string]any{"id": 1}})
	origin := newRedirector(t, target.srv.URL)

	var mu sync.Mutex
	var seen []http.Header
	hc := &http.Client{CheckRedirect: func(req *http.Request, _ []*http.Request) error {
		mu.Lock()
		seen = append(seen, req.Header.Clone())
		mu.Unlock()
		return http.ErrUseLastResponse
	}}

	auth := autotask.AuthConfig{Username: "user", Secret: "secret", IntegrationCode: "code"}
	client, err := autotask.NewClient(t.Context(), auth,
		autotask.WithBaseURL(origin.URL),
		autotask.WithHTTPClient(hc),
	)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	// ErrUseLastResponse hands the 307 back to the client, which reports it as
	// an error; only the redirect handling matters here.
	_, _ = autotask.Get[entities.Company](t.Context(), client, 1)

	mu.Lock()
	defer mu.Unlock()
	if len(seen) != 1 {
		t.Fatalf("caller CheckRedirect called %d times; want 1", len(seen))
	}
	// The credentials are already gone when the caller's CheckRedirect runs.
	requireNoCredentials(t, seen[0])
	if n := len(target.requests()); n != 0 {
		t.Errorf("redirect target got %d requests; want 0 (caller stopped the redirect)", n)
	}
}

func TestAuthRedirectDefaultLimit(t *testing.T) {
	t.Parallel()
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, srv.URL+r.URL.Path, http.StatusTemporaryRedirect)
	}))
	t.Cleanup(srv.Close)

	hc := &http.Client{}
	auth := autotask.AuthConfig{Username: "user", Secret: "secret", IntegrationCode: "code"}
	client, err := autotask.NewClient(t.Context(), auth,
		autotask.WithBaseURL(srv.URL),
		autotask.WithHTTPClient(hc),
	)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	_, err = autotask.Get[entities.Company](t.Context(), client, 1)
	if err == nil || !strings.Contains(err.Error(), "stopped after 10 redirects") {
		t.Errorf("Get error = %v; want one containing %q", err, "stopped after 10 redirects")
	}
	if hc.CheckRedirect != nil {
		t.Error("NewClient set CheckRedirect on the caller's http.Client; want it left nil")
	}
}

func TestAuthThresholdMonitorRedirectDropsCredentials(t *testing.T) {
	t.Parallel()
	target := newHeaderRecorder(t, map[string]any{
		"currentTimeframeRequestCount": 95,
		"externalRequestThreshold":     100,
	})
	origin := newRedirector(t, target.srv.URL)

	fired := make(chan struct{}, 1)
	auth := autotask.AuthConfig{Username: "user", Secret: "secret", IntegrationCode: "code"}
	client, err := autotask.NewClient(t.Context(), auth,
		autotask.WithBaseURL(origin.URL),
		autotask.WithThresholdMonitor(
			middleware.WithCheckInterval(time.Hour),
			middleware.WithCriticalCallback(func(middleware.ThresholdInfo) {
				select {
				case fired <- struct{}{}:
				default:
				}
			}),
		),
	)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	select {
	case <-fired:
	case <-time.After(5 * time.Second):
		t.Fatal("threshold check did not reach the redirect target")
	}
	reqs := target.requests()
	if len(reqs) == 0 {
		t.Fatal("redirect target got no requests")
	}
	requireNoCredentials(t, reqs[0])
}
