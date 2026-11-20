package autotask

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
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

// TestZoneCacheSetNilIgnored covers the nil guard in Set: a nil zone must not
// create an entry (and must not panic dereferencing it). Deleting the guard
// makes this panic on the nil dereference.
func TestZoneCacheSetNilIgnored(t *testing.T) {
	cache := newZoneCache(1 * time.Hour)
	cache.Set("user@example.com", nil)
	if _, ok := cache.Get("user@example.com"); ok {
		t.Fatal("Set(nil) must not create a cache entry")
	}
}

// TestZoneCacheSetOverwrites pins that a second Set for the same user replaces
// the cached zone rather than keeping the first. A Set that skipped existing
// keys would return the stale entry and fail here.
func TestZoneCacheSetOverwrites(t *testing.T) {
	cache := newZoneCache(1 * time.Hour)
	cache.Set("user@example.com", &ZoneInfo{URL: "https://old.example", ZoneName: "old"})
	cache.Set("user@example.com", &ZoneInfo{URL: "https://new.example", ZoneName: "new"})
	got, ok := cache.Get("user@example.com")
	if !ok {
		t.Fatal("expected cache hit")
	}
	if got.URL != "https://new.example" || got.ZoneName != "new" {
		t.Fatalf("Set did not overwrite the existing entry: got %+v", *got)
	}
}

// TestZoneCacheSetStoresCopy is the write-side companion to
// TestZoneCacheReturnsCopy: mutating the caller's struct after Set must not
// reach into the cache. Storing the caller's pointer instead of a value copy
// turns this red.
func TestZoneCacheSetStoresCopy(t *testing.T) {
	cache := newZoneCache(1 * time.Hour)
	zone := &ZoneInfo{URL: "https://original.example", ZoneName: "Zone 1"}
	cache.Set("user@example.com", zone)
	zone.URL = "https://mutated.example"
	got, ok := cache.Get("user@example.com")
	if !ok {
		t.Fatal("expected cache hit")
	}
	if got.URL != "https://original.example" {
		t.Fatalf("cache stored the caller's pointer, not a copy: URL = %q", got.URL)
	}
}

// TestZoneCacheConcurrentAccess exercises the RWMutex under -race: concurrent
// Set and Get on the same key must not race. Dropping the locks in Get/Set makes
// the race detector fail this.
func TestZoneCacheConcurrentAccess(t *testing.T) {
	cache := newZoneCache(1 * time.Hour)
	zone := &ZoneInfo{URL: "https://webservices5.autotask.net", ZoneName: "Zone 5"}
	const goroutines = 8
	var wg sync.WaitGroup
	wg.Add(goroutines * 2)
	for range goroutines {
		go func() {
			defer wg.Done()
			for range 100 {
				cache.Set("user@example.com", zone)
			}
		}()
		go func() {
			defer wg.Done()
			for range 100 {
				cache.Get("user@example.com")
			}
		}()
	}
	wg.Wait()
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

	zone, err := discoverZone(t.Context(), srv.Client(), srv.URL, "test@example.com", defaultMaxResponseBytes)
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

	zone, err := discoverZone(t.Context(), srv.Client(), srv.URL, "test@example.com", defaultMaxResponseBytes)
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

	_, err := discoverZone(t.Context(), srv.Client(), srv.URL, "test@example.com", defaultMaxResponseBytes)
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
			zone, err := discoverZone(t.Context(), srv.Client(), srv.URL, "test@example.com", defaultMaxResponseBytes)
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

// zoneSecret is a password placed in zone URLs to check that NewClient's
// error never repeats it.
const zoneSecret = "s3cret-pw"

func TestWithZoneInfoValidation(t *testing.T) {
	t.Parallel()
	valid := ZoneInfo{ZoneName: "Zone 5", URL: "https://webservices5.autotask.net/atservicesrest/", WebURL: "https://ww5.autotask.net/", CI: 5}
	withURL := func(u string) ZoneInfo { z := valid; z.URL = u; return z }
	slashURL := "http://127.0.0.1:1/atservicesrest/"
	tests := []struct {
		name    string
		opts    []ClientOption
		wantErr bool
		want    ZoneInfo // the zone a success row passes in
	}{
		{"empty URL", []ClientOption{WithZoneInfo(withURL(""))}, true, ZoneInfo{}},
		{"no scheme", []ClientOption{WithZoneInfo(withURL("webservices5.autotask.net/atservicesrest"))}, true, ZoneInfo{}},
		{"other scheme", []ClientOption{WithZoneInfo(withURL("ftp://webservices5.autotask.net"))}, true, ZoneInfo{}},
		{"no host", []ClientOption{WithZoneInfo(withURL("https://"))}, true, ZoneInfo{}},
		{"port without host", []ClientOption{WithZoneInfo(withURL("http://:8080"))}, true, ZoneInfo{}},
		{"trailing newline", []ClientOption{WithZoneInfo(withURL("https://webservices5.autotask.net\n"))}, true, ZoneInfo{}},
		{"query", []ClientOption{WithZoneInfo(withURL("https://webservices5.autotask.net/atservicesrest?x=1"))}, true, ZoneInfo{}},
		{"empty query", []ClientOption{WithZoneInfo(withURL("https://webservices5.autotask.net/atservicesrest?"))}, true, ZoneInfo{}},
		{"fragment", []ClientOption{WithZoneInfo(withURL("https://webservices5.autotask.net/atservicesrest#x"))}, true, ZoneInfo{}},
		{"user info", []ClientOption{WithZoneInfo(withURL("https://apiuser:" + zoneSecret + "@webservices5.autotask.net/atservicesrest/"))}, true, ZoneInfo{}},
		{"user info, missing slash", []ClientOption{WithZoneInfo(withURL("https:/apiuser:" + zoneSecret + "@webservices5.autotask.net"))}, true, ZoneInfo{}},
		{"user info, parse failure", []ClientOption{WithZoneInfo(withURL("https://apiuser:" + zoneSecret + "@webservices5.autotask.net:abc"))}, true, ZoneInfo{}},
		{"password with slash", []ClientOption{WithZoneInfo(withURL("https://apiuser:" + zoneSecret + "/x@webservices5.autotask.net/"))}, true, ZoneInfo{}},
		{"password with hash", []ClientOption{WithZoneInfo(withURL("https://apiuser:" + zoneSecret + "#x@webservices5.autotask.net/"))}, true, ZoneInfo{}},
		{"digit password with slash", []ClientOption{WithZoneInfo(withURL("https://apiuser:1234/" + zoneSecret + "@webservices5.autotask.net/"))}, true, ZoneInfo{}},
		{"zone then base URL", []ClientOption{WithZoneInfo(valid), WithBaseURL("https://other.example")}, true, ZoneInfo{}},
		{"base URL then zone", []ClientOption{WithBaseURL("https://other.example"), WithZoneInfo(valid)}, true, ZoneInfo{}},
		{"empty base URL then zone", []ClientOption{WithBaseURL(""), WithZoneInfo(valid)}, false, valid},
		{"http with path and trailing slash", []ClientOption{WithZoneInfo(withURL(slashURL))}, false, withURL(slashURL)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// Every request fails the test: building a client from a given zone
			// must send none, valid or not.
			noNetwork := &http.Client{Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
				t.Errorf("unexpected request %s %s", r.Method, r.URL)
				return nil, http.ErrHandlerTimeout
			})}
			opts := append([]ClientOption{WithHTTPClient(noNetwork)}, tt.opts...)
			client, err := NewClient(t.Context(), AuthConfig{Username: "user"}, opts...)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("NewClient succeeded with zone URL %q; want an error", client.zone.URL)
				}
				if strings.Contains(err.Error(), zoneSecret) {
					t.Fatalf("error %q repeats the password from the zone URL", err)
				}
				if client != nil {
					t.Fatal("NewClient returned a client with an error")
				}
				return
			}
			if err != nil {
				t.Fatalf("NewClient: %v", err)
			}
			t.Cleanup(func() { _ = client.Close() })
			if got, ok := client.Zone(); !ok || got != tt.want {
				t.Fatalf("Zone() = %+v, %v; want the given zone %+v, true", got, ok, tt.want)
			}
			if client.baseURL != tt.want.URL {
				t.Fatalf("baseURL = %q; want the given zone URL %q verbatim", client.baseURL, tt.want.URL)
			}
		})
	}
}
