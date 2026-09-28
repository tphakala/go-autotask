// Package redirect keeps the Autotask credential headers on same-origin
// redirects only. It is shared by the client and the middleware package.
package redirect

import (
	"fmt"
	"net/http"
	"net/url"
)

// CredentialHeaders are the request headers that carry Autotask credentials.
var CredentialHeaders = []string{"UserName", "Secret", "ApiIntegrationCode", "ImpersonationResourceId"}

// maxRedirects matches the limit net/http applies when CheckRedirect is nil.
const maxRedirects = 10

// SameOrigin reports whether rawURL has the same scheme and host as baseURL.
// Hosts are compared ignoring ASCII case, as DNS names are; any other byte
// must match exactly.
func SameOrigin(rawURL, baseURL string) bool {
	reqParsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	baseParsed, err := url.Parse(baseURL)
	if err != nil {
		return false
	}
	return reqParsed.Scheme == baseParsed.Scheme && equalASCIIFold(reqParsed.Host, baseParsed.Host)
}

// equalASCIIFold reports whether a and b are equal when ASCII letters are
// compared without case. Unlike strings.EqualFold it does no Unicode folding,
// so a non-ASCII rune only matches itself.
func equalASCIIFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range len(a) {
		if lowerASCII(a[i]) != lowerASCII(b[i]) {
			return false
		}
	}
	return true
}

func lowerASCII(c byte) byte {
	if 'A' <= c && c <= 'Z' {
		return c + ('a' - 'A')
	}
	return c
}

// Guard returns a copy of hc whose CheckRedirect removes CredentialHeaders
// from a redirect that leaves baseURL's origin. net/http copies the original
// request's headers onto each redirect and handles only standard ones such as
// Authorization and Cookie itself. hc is not modified. After removing the
// headers it defers to hc.CheckRedirect, or to the net/http default of
// stopping after 10 redirects when that is nil.
func Guard(hc *http.Client, baseURL string) *http.Client {
	cloned := *hc
	next := hc.CheckRedirect
	cloned.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if !SameOrigin(req.URL.String(), baseURL) {
			for _, h := range CredentialHeaders {
				req.Header.Del(h)
			}
		}
		if next != nil {
			return next(req, via)
		}
		if len(via) >= maxRedirects {
			return fmt.Errorf("stopped after %d redirects", maxRedirects)
		}
		return nil
	}
	return &cloned
}
