package autotask

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

func TestDiscoverZone(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /atservicesrest/versioninformation", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"versions": []string{"1.0"},
		})
	})
	mux.HandleFunc("GET /atservicesrest/1.0/zoneInformation", func(w http.ResponseWriter, r *http.Request) {
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
	srv := httptest.NewServer(mux)
	defer srv.Close()
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
