package autotask_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	autotask "github.com/tphakala/go-autotask"
	"github.com/tphakala/go-autotask/entities"
)

// newZoneDiscoveryServer creates an httptest.Server that handles zone discovery
// endpoints and a single GET /v1.0/Companies/{id} API endpoint. The returned
// server's own URL is advertised as the zone URL so that subsequent API
// requests are routed back to the same server.
//
// If versionCalls is non-nil it is incremented each time the version endpoint
// is hit, which allows callers to assert caching behaviour.
func newZoneDiscoveryServer(t *testing.T, versionCalls *atomic.Int32) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()
	var srv *httptest.Server

	mux.HandleFunc("GET /atservicesrest/versioninformation", func(w http.ResponseWriter, r *http.Request) {
		if versionCalls != nil {
			versionCalls.Add(1)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"versions": []string{"1.0"},
		})
	})

	mux.HandleFunc("GET /atservicesrest/1.0/zoneInformation", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"zoneName": "Test Zone",
			"url":      srv.URL,
			"webUrl":   srv.URL,
			"ci":       1,
		})
	})

	mux.HandleFunc("GET /v1.0/Companies/{id}", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"item": map[string]any{
				"id":          1,
				"companyName": "Test Co",
			},
		})
	})

	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestZoneDiscovery(t *testing.T) {
	t.Parallel()

	srv := newZoneDiscoveryServer(t, nil)

	auth := autotask.AuthConfig{
		Username:        "test@example.com",
		Secret:          "secret",
		IntegrationCode: "code",
	}
	client, err := autotask.NewClient(t.Context(), auth, autotask.WithZoneBaseURL(srv.URL))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	company, err := autotask.Get[entities.Company](t.Context(), client, 1)
	if err != nil {
		t.Fatalf("Get Company: %v", err)
	}

	name, ok := company.CompanyName.Get()
	if !ok || name != "Test Co" {
		t.Fatalf("CompanyName = %q (set=%v), want %q", name, ok, "Test Co")
	}
}

func TestZoneCaching(t *testing.T) {
	t.Parallel()

	var versionCalls atomic.Int32
	srv := newZoneDiscoveryServer(t, &versionCalls)

	auth := autotask.AuthConfig{
		Username:        "test@example.com",
		Secret:          "secret",
		IntegrationCode: "code",
	}
	client, err := autotask.NewClient(t.Context(), auth, autotask.WithZoneBaseURL(srv.URL))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	// First API call — zone discovery should happen once during NewClient.
	if _, err := autotask.Get[entities.Company](t.Context(), client, 1); err != nil {
		t.Fatalf("first Get Company: %v", err)
	}

	// Second API call — zone should be cached, no additional discovery.
	if _, err := autotask.Get[entities.Company](t.Context(), client, 1); err != nil {
		t.Fatalf("second Get Company: %v", err)
	}

	if n := versionCalls.Load(); n != 1 {
		t.Fatalf("expected 1 zone lookup, got %d", n)
	}
}

func TestZoneDiscoveryFailure(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	auth := autotask.AuthConfig{
		Username:        "test@example.com",
		Secret:          "secret",
		IntegrationCode: "code",
	}
	_, err := autotask.NewClient(t.Context(), auth, autotask.WithZoneBaseURL(srv.URL))
	if err == nil {
		t.Fatal("expected error from zone discovery failure, got nil")
	}
}
