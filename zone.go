package autotask

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"
)

const (
	defaultZoneBaseURL  = "https://webservices2.autotask.net"
	defaultZoneCacheTTL = 24 * time.Hour
)

type ZoneInfo struct {
	ZoneName string `json:"zoneName"`
	URL      string `json:"url"`
	WebURL   string `json:"webUrl"`
	CI       int    `json:"ci"`
}

type ZoneCache struct {
	mu      sync.RWMutex
	entries map[string]cachedZone
	ttl     time.Duration
}

type cachedZone struct {
	zone      ZoneInfo
	expiresAt time.Time
}

func newZoneCache(ttl time.Duration) *ZoneCache {
	return &ZoneCache{
		entries: make(map[string]cachedZone),
		ttl:     ttl,
	}
}

func (c *ZoneCache) Get(username string) (*ZoneInfo, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, ok := c.entries[username]
	if !ok || time.Now().After(entry.expiresAt) {
		return nil, false
	}
	cp := entry.zone
	return &cp, true
}

func (c *ZoneCache) Set(username string, zone *ZoneInfo) {
	if zone == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[username] = cachedZone{
		zone:      *zone,
		expiresAt: time.Now().Add(c.ttl),
	}
}

func discoverZone(ctx context.Context, httpClient *http.Client, baseURL, username string) (*ZoneInfo, error) {
	versionsURL := baseURL + "/atservicesrest/versioninformation"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, versionsURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("autotask: creating version request: %w", err)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("autotask: zone discovery version request: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // error ignored in defer, nothing useful to do with it
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("autotask: version request returned %d", resp.StatusCode)
	}
	var versionResp struct {
		Versions []string `json:"versions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&versionResp); err != nil {
		return nil, fmt.Errorf("autotask: decoding version response: %w", err)
	}
	if len(versionResp.Versions) == 0 {
		return nil, fmt.Errorf("autotask: no API versions available")
	}
	// The Autotask API currently only has version "1.0". We select the last
	// element assuming ascending order. If multiple versions exist in the
	// future, implement explicit version comparison (e.g., semver parsing).
	version := versionResp.Versions[len(versionResp.Versions)-1]
	zoneURL := fmt.Sprintf("%s/atservicesrest/%s/zoneInformation?user=%s", baseURL, version, url.QueryEscape(username))
	req, err = http.NewRequestWithContext(ctx, http.MethodGet, zoneURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("autotask: creating zone request: %w", err)
	}
	resp, err = httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("autotask: zone discovery request: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // error ignored in defer, nothing useful to do with it
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("autotask: zone discovery returned %d", resp.StatusCode)
	}
	var zone ZoneInfo
	if err := json.NewDecoder(resp.Body).Decode(&zone); err != nil {
		return nil, fmt.Errorf("autotask: decoding zone response: %w", err)
	}
	if zone.URL == "" {
		return nil, fmt.Errorf("autotask: zone discovery returned empty URL")
	}
	return &zone, nil
}
