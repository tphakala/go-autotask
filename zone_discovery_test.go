package autotask_test

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	autotask "github.com/tphakala/go-autotask"
	"github.com/tphakala/go-autotask/entities"
	"github.com/tphakala/go-autotask/internal/logtest"
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

// zoneMux serves the version endpoint and answers zoneInformation with zone.
func zoneMux(zone http.HandlerFunc) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /atservicesrest/versioninformation", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"apiVersions": []string{"V1.0"}})
	})
	mux.HandleFunc("GET /atservicesrest/V1.0/zoneInformation", zone)
	return mux
}

func TestZoneDiscoveryLogsAndRedactsUsername(t *testing.T) {
	t.Parallel()
	const username = "svc-secret@example.com"
	serverError := func(*httptest.Server) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusInternalServerError) })
	}
	zoneAt := func(suffix string) func(*httptest.Server) http.Handler {
		return func(srv *httptest.Server) http.Handler {
			return zoneMux(func(w http.ResponseWriter, _ *http.Request) {
				_ = json.NewEncoder(w).Encode(map[string]any{"zoneName": "Test Zone", "url": srv.URL + suffix})
			})
		}
	}
	tests := []struct {
		name      string
		handler   func(srv *httptest.Server) http.Handler
		baseQuery string // appended to the zone base URL
		wantLevel slog.Level
		wantMsg   string
		wantErr   bool
		errText   string // exact NewClient error; empty skips the comparison
	}{
		{name: "success", handler: zoneAt(""), wantLevel: slog.LevelInfo, wantMsg: "autotask: zone discovered"},
		{
			name: "zone URL with a query is rejected", handler: zoneAt("/?token=SECRET"),
			wantLevel: slog.LevelWarn, wantMsg: "autotask: zone discovery failed", wantErr: true,
			errText: "autotask: zone discovery: zone URL is not an absolute http or https URL with a host and no @, query or fragment",
		},
		{
			name: "base URL with a query", handler: serverError, baseQuery: "?token=SECRET",
			wantLevel: slog.LevelWarn, wantMsg: "autotask: zone discovery failed", wantErr: true,
		},
		{
			name: "version endpoint fails", handler: serverError,
			wantLevel: slog.LevelWarn, wantMsg: "autotask: zone discovery failed", wantErr: true,
		},
		{
			name: "zone request connection dropped",
			handler: func(*httptest.Server) http.Handler {
				return zoneMux(func(w http.ResponseWriter, _ *http.Request) {
					hj, ok := w.(http.Hijacker)
					if !ok {
						return
					}
					if conn, _, err := hj.Hijack(); err == nil {
						_ = conn.Close()
					}
				})
			},
			wantLevel: slog.LevelWarn, wantMsg: "autotask: zone discovery failed", wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var srv *httptest.Server
			srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				tt.handler(srv).ServeHTTP(w, r)
			}))
			t.Cleanup(srv.Close)
			rec := logtest.New()
			client, err := autotask.NewClient(t.Context(), autotask.AuthConfig{Username: username},
				autotask.WithZoneBaseURL(srv.URL+tt.baseQuery), autotask.WithLogger(rec.Logger()))
			if (err != nil) != tt.wantErr {
				t.Fatalf("NewClient error = %v; want error %v", err, tt.wantErr)
			}
			if client != nil {
				t.Cleanup(func() { _ = client.Close() })
			}
			if tt.errText != "" && err.Error() != tt.errText {
				t.Errorf("error = %q; want %q", err, tt.errText)
			}
			recs := rec.Records()
			if len(recs) != 1 {
				t.Fatalf("got %d records; want 1", len(recs))
			}
			if recs[0].Level != tt.wantLevel || recs[0].Message != tt.wantMsg {
				t.Errorf("record = %s %q; want %s %q", recs[0].Level, recs[0].Message, tt.wantLevel, tt.wantMsg)
			}
			if !tt.wantErr && recs[0].Str("zone_name") != "Test Zone" {
				t.Errorf("zone_name = %q; want Test Zone", recs[0].Str("zone_name"))
			}
			assertNoLeak(t, recs[0].Text, err)
		})
	}
}

// assertNoLeak fails when the record text or the error holds the username or a
// query string of the zone requests.
func assertNoLeak(t *testing.T, recordText string, err error) {
	t.Helper()
	texts := []string{recordText}
	if err != nil {
		texts = append(texts, err.Error())
	}
	for _, text := range texts {
		if strings.Contains(text, "svc-secret") || strings.Contains(text, "user=") || strings.Contains(text, "SECRET") {
			t.Errorf("the username or a query reached %q", text)
		}
	}
}

// TestZoneBaseURLWithUserInfoIsNotRepeated pins that the rejection of a zone
// base URL with user info leaves the user info out of the error and the log.
// url.Parse reads "user:pw" as the host of this URL and would quote ":pw".
func TestZoneBaseURLWithUserInfoIsNotRepeated(t *testing.T) {
	t.Parallel()
	rec := logtest.New()
	_, err := autotask.NewClient(t.Context(), autotask.AuthConfig{Username: "svc-secret@example.com"},
		autotask.WithZoneBaseURL("http://user:pw/x@host"), autotask.WithLogger(rec.Logger()))
	if err == nil {
		t.Fatal("want an error for the zone base URL with user info")
	}
	recs := rec.Records()
	if len(recs) != 1 {
		t.Fatalf("got %d records; want 1", len(recs))
	}
	for _, text := range []string{err.Error(), recs[0].Text} {
		if strings.Contains(text, "pw") || strings.Contains(text, "svc-secret") {
			t.Errorf("user info or username reached %q", text)
		}
	}
}
