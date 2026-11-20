package autotasktest

import (
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"testing"

	autotask "github.com/tphakala/go-autotask"
	"github.com/tphakala/go-autotask/entities"
)

// TestHandleVersionInfoUsesAPIVersionsKey pins the wire key of the mock version
// endpoint. discoverZone decodes the response under the "apiVersions" key, so
// returning any other key (the old "versions") yields an empty version list.
func TestHandleVersionInfoUsesAPIVersionsKey(t *testing.T) {
	ts, _ := NewServer(t)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, ts.URL+"/atservicesrest/versioninformation", http.NoBody)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	// No credentials: zone discovery is answered without the auth check.
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	// Decode with exactly the shape discoverZone uses.
	var decoded struct {
		Versions []string `json:"apiVersions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(decoded.Versions) == 0 {
		t.Fatal("apiVersions is empty; the mock must use the \"apiVersions\" key that discoverZone decodes")
	}
	if !slices.Contains(decoded.Versions, "V1.0") {
		t.Fatalf("apiVersions = %v; want it to advertise the supported V1.0", decoded.Versions)
	}
}

// TestHandleQueryNoMatchSendsEmptyItemsArray pins the wire shape of a query that
// matches no stored item: "items": [] like the Autotask API, not null.
func TestHandleQueryNoMatchSendsEmptyItemsArray(t *testing.T) {
	ts, _ := NewServer(t, WithEntity(CompanyFixture()))

	body := strings.NewReader(`{"filter":[{"op":"eq","field":"id","value":-1}]}`)
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, ts.URL+"/v1.0/Companies/query", body)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("UserName", "test-user")
	req.Header.Set("Secret", "test-secret")
	req.Header.Set("ApiIntegrationCode", "test-code")

	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var decoded struct {
		Items json.RawMessage `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got := string(decoded.Items); got != "[]" {
		t.Errorf("items = %s, want []", got)
	}
}

// TestNewClientDiscoversZoneFromServer runs real zone discovery against the
// test server. Discovery requests carry no credentials, so the server must
// answer them without the auth check; every other path stays auth-checked.
func TestNewClientDiscoversZoneFromServer(t *testing.T) {
	ts, _ := NewServer(t, WithEntity(CompanyFixture(func(c *entities.Company) { c.ID = autotask.Set(int64(7)) })))
	auth := autotask.AuthConfig{Username: "test-user", Secret: "test-secret", IntegrationCode: "test-code"}

	client, err := autotask.NewClient(t.Context(), auth, autotask.WithZoneBaseURL(ts.URL))
	if err != nil {
		t.Fatalf("NewClient with zone discovery: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	zone, ok := client.Zone()
	if !ok || zone.URL != ts.URL {
		t.Fatalf("Zone() = %+v, %v; want URL %q, true", zone, ok, ts.URL)
	}
	if _, err := autotask.Get[entities.Company](t.Context(), client, 7); err != nil {
		t.Fatalf("Get through the discovered zone: %v", err)
	}

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, ts.URL+"/v1.0/Companies/7", http.NoBody)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("entity GET without credentials: status = %d, want 401", resp.StatusCode)
	}
}

// TestDiscoveryRulesRunBeforeDiscovery pins that a rule on a discovery path
// still answers first, so a test can make zone discovery fail.
func TestDiscoveryRulesRunBeforeDiscovery(t *testing.T) {
	ts, _ := NewServer(t, WithErrorOn(http.MethodGet, "/versioninformation", http.StatusServiceUnavailable, []string{"maintenance"}))
	auth := autotask.AuthConfig{Username: "test-user", Secret: "test-secret", IntegrationCode: "test-code"}

	client, err := autotask.NewClient(t.Context(), auth, autotask.WithZoneBaseURL(ts.URL))
	if err == nil {
		_ = client.Close()
		t.Fatal("NewClient succeeded; want the 503 the rule serves for the version request")
	}
	if !strings.Contains(err.Error(), "503") {
		t.Fatalf("NewClient error = %v; want the rule's 503", err)
	}
}

// TestDiscoveryBypassIsLimitedToDiscoveryGETs pins that only the two discovery
// GETs at the server root skip the auth check: another method, the same suffix
// on another path, or an extra path segment still needs credentials.
func TestDiscoveryBypassIsLimitedToDiscoveryGETs(t *testing.T) {
	ts, _ := NewServer(t)
	for _, tc := range []struct{ method, path string }{
		{http.MethodPost, "/atservicesrest/versioninformation"},
		{http.MethodGet, "/v1.0/Companies/7/zoneInformation"},
		{http.MethodGet, "/v1.0/Companies/7/atservicesrest/versioninformation"},
		{http.MethodGet, "/atservicesrest/V1.0/x/zoneInformation"},
	} {
		req, err := http.NewRequestWithContext(t.Context(), tc.method, ts.URL+tc.path, http.NoBody)
		if err != nil {
			t.Fatalf("new request: %v", err)
		}
		resp, err := ts.Client().Do(req)
		if err != nil {
			t.Fatalf("do: %v", err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("%s %s without credentials: status = %d, want 401", tc.method, tc.path, resp.StatusCode)
		}
	}
}
