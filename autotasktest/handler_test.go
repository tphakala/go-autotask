package autotasktest

import (
	"encoding/json"
	"net/http"
	"slices"
	"testing"
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
	// The mock validates auth on every request; use the defaults NewServer sets.
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
