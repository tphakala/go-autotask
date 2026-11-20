package autotask

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// supportedAPIVersion is the Autotask REST API wire version this client
// implements. Every entity path is hardcoded to "/v1.0/", so zone discovery
// must resolve against this version rather than whatever the server happens to
// advertise last. Bump this (and the entity paths) when a new version is
// actually implemented here.
const supportedAPIVersion = "1.0"

const (
	defaultZoneBaseURL  = "https://webservices2.autotask.net"
	defaultZoneCacheTTL = 24 * time.Hour
)

// ZoneInfo describes the Autotask zone an API user belongs to, as zone
// discovery returns it. URL is the API base URL. The JSON names are those of
// the zone information response, so a ZoneInfo can be stored as JSON and given
// back to NewClient with WithZoneInfo.
type ZoneInfo struct {
	ZoneName string `json:"zoneName"`
	URL      string `json:"url"`
	WebURL   string `json:"webUrl"`
	CI       int    `json:"ci"`
}

// validateZoneURL checks the URL of a zone given with WithZoneInfo: it must be
// an absolute http or https URL with a host and no "@", query or fragment.
// joinURL appends API paths to it, which a query or fragment would swallow. An
// "@" anywhere is rejected, not only in parsed user info: net/url (go1.27.4)
// reads "https://user:1234/x@host/" as host "user", port 1234, no user info.
// The errors leave the URL out, so a password in it is not repeated.
func validateZoneURL(raw string) error {
	if raw == "" {
		return fmt.Errorf("autotask: WithZoneInfo: zone URL is empty")
	}
	u, err := url.Parse(raw)
	if err != nil {
		// Not wrapped: *url.Error prints the whole URL (go1.27.4
		// net/url/url.go:40), and its cause can quote part of the user info,
		// such as `invalid port ":pw" after host` (url.go:631).
		return fmt.Errorf("autotask: WithZoneInfo: zone URL does not parse")
	}
	if (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || strings.ContainsAny(raw, "@?#") {
		return fmt.Errorf("autotask: WithZoneInfo: zone URL is not an absolute http or https URL with a host and no @, query or fragment")
	}
	return nil
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

func discoverZone(ctx context.Context, httpClient *http.Client, baseURL, username string, maxBodyBytes int64) (*ZoneInfo, error) {
	baseURL = strings.TrimRight(baseURL, "/")
	versionsURL := baseURL + "/atservicesrest/versioninformation"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, versionsURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("autotask: creating version request: %w", err)
	}
	versionResp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("autotask: zone discovery version request: %w", err)
	}
	defer versionResp.Body.Close() //nolint:errcheck // error ignored in defer
	if versionResp.StatusCode != http.StatusOK {
		return nil, zoneStatusError("version request", versionResp)
	}
	var versions struct {
		Versions []string `json:"apiVersions"`
	}
	if err := decodeJSONBody(versionResp, &versions, maxBodyBytes); err != nil {
		return nil, fmt.Errorf("autotask: decoding version response: %w", err)
	}
	if len(versions.Versions) == 0 {
		return nil, fmt.Errorf("autotask: no API versions available")
	}
	// Select the advertised version that matches the one this client implements,
	// case-insensitively and tolerant of a leading "V" (Autotask advertises
	// entries like "V1.0"). Do not trust the server's ordering or newest entry:
	// Autotask has advertised a version it no longer serves (V2.0 returning 404
	// while still listed), which broke selecting the last element. The advertised
	// string (trimmed of surrounding whitespace) is used in the request path
	// because that is what the server routes on.
	apiVersion := ""
	for _, v := range versions.Versions {
		normalized := strings.TrimSpace(v)
		if strings.TrimPrefix(strings.ToLower(normalized), "v") == supportedAPIVersion {
			apiVersion = normalized
			break
		}
	}
	if apiVersion == "" {
		return nil, fmt.Errorf("autotask: server does not advertise supported API version %q (available: %v)", supportedAPIVersion, versions.Versions)
	}
	zoneURL := fmt.Sprintf("%s/atservicesrest/%s/zoneInformation?user=%s", baseURL, apiVersion, url.QueryEscape(username))
	req, err = http.NewRequestWithContext(ctx, http.MethodGet, zoneURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("autotask: creating zone request: %w", err)
	}
	zoneResp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("autotask: zone discovery request: %w", err)
	}
	defer zoneResp.Body.Close() //nolint:errcheck // error ignored in defer
	if zoneResp.StatusCode != http.StatusOK {
		return nil, zoneStatusError("zone discovery", zoneResp)
	}
	var zone ZoneInfo
	if err := decodeJSONBody(zoneResp, &zone, maxBodyBytes); err != nil {
		return nil, fmt.Errorf("autotask: decoding zone response: %w", err)
	}
	if zone.URL == "" {
		return nil, fmt.Errorf("autotask: zone discovery returned empty URL")
	}
	return &zone, nil
}

// zoneDiscoveryError keeps the message of a failed zone discovery step and
// unwraps to the status-typed error, so errors.As and IsTransient classify it
// by status.
type zoneDiscoveryError struct {
	msg string
	err error
}

func (e *zoneDiscoveryError) Error() string { return e.msg }
func (e *zoneDiscoveryError) Unwrap() error { return e.err }

func zoneStatusError(step string, resp *http.Response) error {
	base := Error{StatusCode: resp.StatusCode, Message: http.StatusText(resp.StatusCode)}
	return &zoneDiscoveryError{
		msg: fmt.Sprintf("autotask: %s returned %d", step, resp.StatusCode),
		err: statusToError(resp, base),
	}
}

// decodeJSONBody reads at most limit bytes of resp's body and decodes its first
// JSON value into v. A body that is not blank, not labelled as JSON and not
// valid JSON, such as the HTML maintenance page, returns an
// *UnexpectedContentTypeError; a blank body returns an *EmptyResponseError and
// a longer one a *ResponseTooLargeError.
func decodeJSONBody(resp *http.Response, v any, limit int64) error {
	body, tooLarge, err := readLimited(resp.Body, limit)
	if err != nil {
		return err
	}
	if tooLarge {
		return newResponseTooLargeError(resp.StatusCode, limit)
	}
	if err := checkJSONBody(resp, body); err != nil {
		return err
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return newEmptyResponseError(resp.StatusCode)
	}
	return json.NewDecoder(bytes.NewReader(body)).Decode(v)
}
