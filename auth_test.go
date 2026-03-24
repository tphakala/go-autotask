package autotask_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	autotask "github.com/tphakala/go-autotask"
	"github.com/tphakala/go-autotask/autotasktest"
	"github.com/tphakala/go-autotask/entities"
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
