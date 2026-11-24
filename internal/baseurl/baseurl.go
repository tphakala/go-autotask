// Package baseurl checks an Autotask API base URL. It is shared by the client
// and the middleware package.
package baseurl

import (
	"fmt"
	"net/url"
	"strings"
)

// Check checks a URL that API paths are appended to. Its callers are
// WithBaseURL, WithZoneInfo and WithZoneBaseURL, the zone URL that zone
// discovery returns, autotask.ValidateBaseURL and middleware.NewThresholdMonitor.
// The URL must be an absolute http or https URL with a host and no "@", query
// or fragment. A query or fragment would swallow the API paths appended to it.
// An "@" anywhere is rejected, not only in parsed user info: net/url ends the
// authority at the first "/" (go1.27.4 net/url/url.go:489), so it reads
// "https://user:1234/x@host/" as host "user", port 1234, no user info. name
// says which URL it is and starts the error after "autotask: ". The errors
// leave the URL out, so a password in it is not repeated.
func Check(name, raw string) error {
	if raw == "" {
		return fmt.Errorf("autotask: %s is empty", name)
	}
	u, err := url.Parse(raw)
	if err != nil {
		// Not wrapped: *url.Error prints the whole URL (go1.27.4
		// net/url/url.go:39), and its cause can quote part of the user info,
		// such as `invalid port ":pw" after host` (url.go:631).
		return fmt.Errorf("autotask: %s does not parse", name)
	}
	if (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || strings.ContainsAny(raw, "@?#") {
		return fmt.Errorf("autotask: %s is not an absolute http or https URL with a host and no @, query or fragment", name)
	}
	return nil
}
