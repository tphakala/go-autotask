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

	// Advertise both V1.0 and V2.0 (as Autotask does) but only serve V1.0. If the
	// client ever selects the last advertised entry (V2.0) again, the V2.0 route
	// below fails the test loudly instead of a generic discovery failure.
	mux.HandleFunc("GET /atservicesrest/versioninformation", func(w http.ResponseWriter, r *http.Request) {
		if versionCalls != nil {
			versionCalls.Add(1)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"apiVersions": []string{"V1.0", "V2.0"},
		})
	})

	mux.HandleFunc("GET /atservicesrest/V1.0/zoneInformation", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"zoneName": "Test Zone",
			"url":      srv.URL,
			"webUrl":   "https://ww.example.test",
			"ci":       1,
		})
	})

	mux.HandleFunc("GET /atservicesrest/V2.0/zoneInformation", func(w http.ResponseWriter, _ *http.Request) {
		t.Errorf("V2.0 zoneInformation must not be requested; it is advertised but not served")
		http.Error(w, "not found", http.StatusNotFound)
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

func TestWithZoneInfoMakesNoDiscoveryRequest(t *testing.T) {
	t.Parallel()

	var requests atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1.0/Companies/{id}", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"item": map[string]any{"id": 1, "companyName": "Test Co"},
		})
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		http.Error(w, "not found", http.StatusNotFound)
	})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		mux.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)

	z := autotask.ZoneInfo{ZoneName: "Zone 5", URL: srv.URL, WebURL: "https://ww5.example.test", CI: 5}
	auth := autotask.AuthConfig{Username: "test@example.com", Secret: "secret", IntegrationCode: "code"}
	// WithZoneBaseURL points discovery at the same server, so a regression that
	// still discovers hits the catch-all handler instead of the real service.
	client, err := autotask.NewClient(t.Context(), auth,
		autotask.WithZoneInfo(z), autotask.WithZoneBaseURL(srv.URL))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	if n := requests.Load(); n != 0 {
		t.Fatalf("NewClient sent %d requests; want 0", n)
	}
	got, ok := client.Zone()
	if !ok || got != z {
		t.Fatalf("Zone() = %+v, %v; want %+v, true", got, ok, z)
	}
	if _, err := autotask.Get[entities.Company](t.Context(), client, 1); err != nil {
		t.Fatalf("Get: %v", err)
	}
	if n := requests.Load(); n != 1 {
		t.Fatalf("requests after Get = %d; want 1", n)
	}
}

func TestZoneRoundTripThroughJSON(t *testing.T) {
	t.Parallel()

	var versionCalls atomic.Int32
	srv := newZoneDiscoveryServer(t, &versionCalls)
	auth := autotask.AuthConfig{Username: "test@example.com", Secret: "secret", IntegrationCode: "code"}

	first, err := autotask.NewClient(t.Context(), auth, autotask.WithZoneBaseURL(srv.URL))
	if err != nil {
		t.Fatalf("NewClient (discovery): %v", err)
	}
	t.Cleanup(func() { _ = first.Close() })
	discovered, ok := first.Zone()
	want := autotask.ZoneInfo{ZoneName: "Test Zone", URL: srv.URL, WebURL: "https://ww.example.test", CI: 1}
	if !ok || discovered != want {
		t.Fatalf("Zone() after discovery = %+v, %v; want %+v, true", discovered, ok, want)
	}

	data, err := json.Marshal(discovered)
	if err != nil {
		t.Fatal(err)
	}
	var restored autotask.ZoneInfo
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatal(err)
	}

	second, err := autotask.NewClient(t.Context(), auth,
		autotask.WithZoneInfo(restored), autotask.WithZoneBaseURL(srv.URL))
	if err != nil {
		t.Fatalf("NewClient (saved zone): %v", err)
	}
	t.Cleanup(func() { _ = second.Close() })
	if got, ok := second.Zone(); !ok || got != discovered {
		t.Fatalf("Zone() from saved zone = %+v, %v; want %+v, true", got, ok, discovered)
	}
	if n := versionCalls.Load(); n != 1 {
		t.Fatalf("version endpoint hit %d times; want 1", n)
	}
	if _, err := autotask.Get[entities.Company](t.Context(), second, 1); err != nil {
		t.Fatalf("Get through saved zone: %v", err)
	}
}
