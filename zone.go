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

	"github.com/tphakala/go-autotask/internal/baseurl"
	"github.com/tphakala/go-autotask/internal/redact"
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

// ValidateBaseURL returns an error unless rawURL can be an API base URL: an
// absolute http or https URL with a host and no "@", query or fragment. API
// paths are appended to the base URL, which a query or fragment would swallow.
//
// NewClient applies this rule to a non-empty [WithBaseURL], to the URL of
// [WithZoneInfo], to [WithZoneBaseURL] when zone discovery runs, and to the zone
// URL that discovery returns. middleware.NewThresholdMonitor applies it to its
// base URL. An empty rawURL is an error here, though WithBaseURL("") is accepted
// and leaves zone discovery on, so a program whose empty setting means
// "discover" should skip the call for it.
//
// The error leaves rawURL out, so a password in it is not repeated. NewClient's
// own errors name the option that was given and so differ in text.
func ValidateBaseURL(rawURL string) error {
	return baseurl.Check("base URL", rawURL)
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

func discoverZone(ctx context.Context, httpClient *http.Client, baseURL, username, userAgent string, maxBodyBytes int64) (*ZoneInfo, error) {
	if err := baseurl.Check("WithZoneBaseURL: zone base URL", baseURL); err != nil {
		return nil, err
	}
	baseURL = strings.TrimRight(baseURL, "/")
	versionsURL := baseURL + "/atservicesrest/versioninformation"
	versionResp, err := discoveryGet(ctx, httpClient, versionsURL, userAgent, "creating version request", "zone discovery version request")
	if err != nil {
		return nil, err
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
	zoneResp, err := discoveryGet(ctx, httpClient, zoneURL, userAgent, "creating zone request", "zone discovery request")
	if err != nil {
		return nil, err
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
	if err := baseurl.Check("zone discovery: zone URL", zone.URL); err != nil {
		return nil, err
	}
	return &zone, nil
}

// discoveryGet sends a GET for a zone discovery step with the client's
// User-Agent. createWhat and doWhat name the step in the error when the request
// cannot be built or sent.
func discoveryGet(ctx context.Context, httpClient *http.Client, rawURL, userAgent, createWhat, doWhat string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, http.NoBody)
	if err != nil {
		return nil, requestError(createWhat, err)
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, requestError(doWhat, err)
	}
	return resp, nil
}

// requestError wraps err, which can carry a *url.Error, for a failed zone
// discovery request. The request URL of the zone step has the username in its
// query, and the base URL may be caller-supplied, so the *url.Error is redacted
// before fmt.Errorf copies its text into the message.
func requestError(what string, err error) error {
	return fmt.Errorf("autotask: %s: %w", what, redact.URLError(err))
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
