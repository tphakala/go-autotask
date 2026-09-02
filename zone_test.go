package autotask

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestZoneCacheSetAndGet(t *testing.T) {
	cache := newZoneCache(1 * time.Hour)
	zone := &ZoneInfo{URL: "https://webservices5.autotask.net", ZoneName: "Zone 5"}
	cache.Set("user@example.com", zone)
	got, ok := cache.Get("user@example.com")
	if !ok {
		t.Fatal("expected cache hit")
	}
	if got.URL != zone.URL {
		t.Fatalf("URL = %q; want %q", got.URL, zone.URL)
	}
}

func TestZoneCacheExpiration(t *testing.T) {
	cache := newZoneCache(1 * time.Millisecond)
	zone := &ZoneInfo{URL: "https://example.com"}
	cache.Set("user@example.com", zone)
	time.Sleep(5 * time.Millisecond)
	_, ok := cache.Get("user@example.com")
	if ok {
		t.Fatal("expected cache miss after expiration")
	}
}

func TestZoneCacheMiss(t *testing.T) {
	cache := newZoneCache(1 * time.Hour)
	_, ok := cache.Get("nobody@example.com")
	if ok {
		t.Fatal("expected cache miss for unknown user")
	}
}

func TestZoneCacheReturnsCopy(t *testing.T) {
	cache := newZoneCache(1 * time.Hour)
	zone := &ZoneInfo{URL: "https://original.com", ZoneName: "Zone 1"}
	cache.Set("user@example.com", zone)
	got, _ := cache.Get("user@example.com")
	got.URL = "https://mutated.com"
	got2, _ := cache.Get("user@example.com")
	if got2.URL != "https://original.com" {
		t.Fatalf("cache was mutated: URL = %q", got2.URL)
	}
}

// zoneServer builds a mock that advertises apiVersions and serves the
// zoneInformation endpoint only for servedSegment. Every other advertised
// version gets a poison route that fails the test loudly, so a wrong version
// selection surfaces as a clear message instead of a generic 404.
func zoneServer(t *testing.T, advertised []string, servedSegment string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /atservicesrest/versioninformation", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"apiVersions": advertised})
	})
	if servedSegment != "" {
		mux.HandleFunc("GET /atservicesrest/"+servedSegment+"/zoneInformation", func(w http.ResponseWriter, r *http.Request) {
			user := r.URL.Query().Get("user")
			if user != "test@example.com" {
				http.Error(w, "bad user", http.StatusBadRequest)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"zoneName": "Zone 5",
				"url":      "https://webservices5.autotask.net/atservicesrest",
				"webUrl":   "https://ww5.autotask.net",
				"ci":       5,
			})
		})
	}
	for _, v := range advertised {
		// Mirror discoverZone's normalization: the client requests the trimmed
		// segment, so trim before comparing and before building the poison route
		// (a raw " V1.0 " would otherwise register an unmatchable spaced pattern).
		poison := strings.TrimSpace(v)
		if poison == servedSegment {
			continue
		}
		mux.HandleFunc("GET /atservicesrest/"+poison+"/zoneInformation", func(w http.ResponseWriter, _ *http.Request) {
			t.Errorf("zoneInformation for %q must not be requested; it is advertised but not served", poison)
			http.Error(w, "not found", http.StatusNotFound)
		})
	}
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestDiscoverZone(t *testing.T) {
	// Autotask advertises both V1.0 and V2.0 but no longer serves V2.0. Selecting
	// the supported V1.0 (not the last element) must drive discovery.
	srv := zoneServer(t, []string{"V1.0", "V2.0"}, "V1.0")

	zone, err := discoverZone(t.Context(), srv.Client(), srv.URL, "test@example.com")
	if err != nil {
		t.Fatalf("discoverZone: %v", err)
	}
	if zone.ZoneName != "Zone 5" {
		t.Fatalf("ZoneName = %q; want Zone 5", zone.ZoneName)
	}
	if zone.URL != "https://webservices5.autotask.net/atservicesrest" {
		t.Fatalf("URL = %q", zone.URL)
	}
}

func TestDiscoverZoneVersionInMiddle(t *testing.T) {
	// Supported version is neither the first nor the last advertised entry, so
	// neither a first-element nor a last-element selection would land on it.
	srv := zoneServer(t, []string{"V2.0", "V1.0", "V3.0"}, "V1.0")

	zone, err := discoverZone(t.Context(), srv.Client(), srv.URL, "test@example.com")
	if err != nil {
		t.Fatalf("discoverZone: %v", err)
	}
	if zone.ZoneName != "Zone 5" {
		t.Fatalf("ZoneName = %q; want Zone 5", zone.ZoneName)
	}
}

func TestDiscoverZoneNoSupportedVersion(t *testing.T) {
	// The server advertises only versions this client does not implement.
	srv := zoneServer(t, []string{"V2.0", "V3.0"}, "")

	_, err := discoverZone(t.Context(), srv.Client(), srv.URL, "test@example.com")
	if err == nil {
		t.Fatal("expected error when no supported version is advertised, got nil")
	}
	if !strings.Contains(err.Error(), supportedAPIVersion) {
		t.Fatalf("error %q should mention supported version %q", err.Error(), supportedAPIVersion)
	}
}

func TestDiscoverZoneVersionFormTolerance(t *testing.T) {
	cases := []struct {
		name       string
		advertised string
		served     string
	}{
		{"leading capital V", "V1.0", "V1.0"},
		{"lowercase v", "v1.0", "v1.0"},
		{"bare version", "1.0", "1.0"},
		{"surrounding whitespace", " V1.0 ", "V1.0"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := zoneServer(t, []string{tc.advertised}, tc.served)
			zone, err := discoverZone(t.Context(), srv.Client(), srv.URL, "test@example.com")
			if err != nil {
				t.Fatalf("discoverZone(%q): %v", tc.advertised, err)
			}
			if zone.ZoneName != "Zone 5" {
				t.Fatalf("ZoneName = %q; want Zone 5", zone.ZoneName)
			}
		})
	}
}

func TestJoinURL(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		path    string
		want    string
	}{
		{"no trailing slash", "https://api.example.com", "/v1.0/Companies/1", "https://api.example.com/v1.0/Companies/1"},
		{"trailing slash trimmed", "https://api.example.com/", "/v1.0/Companies/1", "https://api.example.com/v1.0/Companies/1"},
		{"absolute path returned verbatim", "https://api.example.com", "https://api.example.com/v1.0/Companies?page=2", "https://api.example.com/v1.0/Companies?page=2"},
		{"absolute path ignores trailing-slash base", "https://api.example.com/", "https://other.example.com/next", "https://other.example.com/next"},
		{"multiple trailing slashes trimmed", "https://api.example.com//", "/v1.0/Companies/1", "https://api.example.com/v1.0/Companies/1"},
		{"relative path with trailing-slash base", "https://api.example.com/", "v1.0/Companies/1", "https://api.example.com/v1.0/Companies/1"},
		{"relative path with no-slash base", "https://api.example.com", "v1.0/Companies/1", "https://api.example.com/v1.0/Companies/1"},
		{"uppercase scheme returned verbatim", "https://api.example.com", "HTTPS://other.example.com/next", "HTTPS://other.example.com/next"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := joinURL(tt.baseURL, tt.path); got != tt.want {
				t.Fatalf("joinURL(%q, %q) = %q; want %q", tt.baseURL, tt.path, got, tt.want)
			}
		})
	}
}
