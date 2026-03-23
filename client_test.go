package autotask

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tphakala/go-autotask/middleware"
)

func TestNewClientWithBaseURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()
	auth := AuthConfig{Username: "user", Secret: "secret", IntegrationCode: "code"}
	client, err := NewClient(context.Background(), auth, WithBaseURL(srv.URL))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if client.baseURL != srv.URL {
		t.Fatalf("baseURL = %q; want %q", client.baseURL, srv.URL)
	}
}

func TestClientAuthHeaders(t *testing.T) {
	var gotHeaders http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeaders = r.Header
		json.NewEncoder(w).Encode(map[string]any{"item": map[string]any{"id": 1}})
	}))
	defer srv.Close()
	auth := AuthConfig{Username: "user@test.com", Secret: "s3cret", IntegrationCode: "INT123"}
	client, err := NewClient(context.Background(), auth, WithBaseURL(srv.URL))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if err := client.do(context.Background(), http.MethodGet, "/v1.0/Tickets/1", nil, nil); err != nil {
		t.Fatal(err)
	}
	if gotHeaders.Get("UserName") != "user@test.com" {
		t.Fatalf("UserName header = %q", gotHeaders.Get("UserName"))
	}
	if gotHeaders.Get("Secret") != "s3cret" {
		t.Fatalf("Secret header = %q", gotHeaders.Get("Secret"))
	}
	if gotHeaders.Get("ApiIntegrationcode") != "INT123" {
		t.Fatalf("ApiIntegrationcode header = %q", gotHeaders.Get("ApiIntegrationcode"))
	}
	if !strings.HasPrefix(gotHeaders.Get("User-Agent"), "go-autotask/") {
		t.Fatalf("User-Agent header = %q", gotHeaders.Get("User-Agent"))
	}
}

func TestClientImpersonation(t *testing.T) {
	var gotHeaders http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeaders = r.Header
		json.NewEncoder(w).Encode(map[string]any{"item": map[string]any{"id": 1}})
	}))
	defer srv.Close()
	auth := AuthConfig{Username: "user", Secret: "secret", IntegrationCode: "code"}
	client, err := NewClient(context.Background(), auth, WithBaseURL(srv.URL), WithImpersonation(12345))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if err := client.do(context.Background(), http.MethodGet, "/v1.0/Tickets/1", nil, nil); err != nil {
		t.Fatal(err)
	}
	if gotHeaders.Get("ImpersonationResourceId") != "12345" {
		t.Fatalf("ImpersonationResourceId = %q", gotHeaders.Get("ImpersonationResourceId"))
	}
}

func TestClientClose(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()
	auth := AuthConfig{Username: "user", Secret: "secret", IntegrationCode: "code"}
	client, err := NewClient(context.Background(), auth, WithBaseURL(srv.URL))
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestClientDoPost(t *testing.T) {
	var gotBody []byte
	var gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotBody, _ = io.ReadAll(r.Body)
		json.NewEncoder(w).Encode(map[string]any{"itemId": 1})
	}))
	defer srv.Close()
	auth := AuthConfig{Username: "user", Secret: "secret", IntegrationCode: "code"}
	client, err := NewClient(context.Background(), auth, WithBaseURL(srv.URL))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	payload := map[string]any{"title": "test"}
	var result map[string]any
	err = client.do(context.Background(), http.MethodPost, "/v1.0/Tickets", payload, &result)
	if err != nil {
		t.Fatal(err)
	}
	if gotMethod != "POST" {
		t.Fatalf("method = %s; want POST", gotMethod)
	}
	if !strings.Contains(string(gotBody), "title") {
		t.Fatalf("body = %s; missing title", gotBody)
	}
}

func TestClientWithMiddleware(t *testing.T) {
	var middlewareCalled bool
	customMiddleware := func(next http.RoundTripper) http.RoundTripper {
		return roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			middlewareCalled = true
			return next.RoundTrip(req)
		})
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"item": map[string]any{"id": 1}})
	}))
	defer srv.Close()
	auth := AuthConfig{Username: "u", Secret: "s", IntegrationCode: "c"}
	client, err := NewClient(context.Background(), auth, WithBaseURL(srv.URL), WithMiddleware(customMiddleware))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if err := client.do(context.Background(), http.MethodGet, "/v1.0/Test/1", nil, nil); err != nil {
		t.Fatal(err)
	}
	if !middlewareCalled {
		t.Fatal("middleware was not called")
	}
}

func TestClientWithRateLimiter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"item": map[string]any{"id": 1}})
	}))
	defer srv.Close()
	auth := AuthConfig{Username: "u", Secret: "s", IntegrationCode: "c"}
	client, err := NewClient(context.Background(), auth,
		WithBaseURL(srv.URL),
		WithRateLimiter(middleware.WithRequestsPerHour(1000)),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	err = client.do(context.Background(), http.MethodGet, "/v1.0/Test/1", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
