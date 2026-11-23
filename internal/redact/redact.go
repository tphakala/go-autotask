// Package redact removes the parts of a URL that can carry credentials, so the
// URL can be logged or returned in an error.
package redact

import (
	"errors"
	"net/url"
	"strings"
)

// URL returns raw as scheme://host plus its path, without user info, query or
// fragment. It returns "" when raw does not parse or contains "@": url.Parse
// reads "https://user:1234/x@host/" as host "user:1234", so dropping the user
// info alone would still print that text.
func URL(raw string) string {
	if strings.Contains(raw, "@") {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	u.User = nil
	u.RawQuery = ""
	u.ForceQuery = false
	u.Fragment = ""
	u.RawFragment = ""
	return u.String()
}

// URLError replaces the URL of the *url.Error in err's chain, if any, with
// URL(ue.URL), and returns err. It changes the *url.Error in place, so apply it
// only to an error built for the current call, as net/http and net/url do.
func URLError(err error) error {
	if ue, ok := errors.AsType[*url.Error](err); ok {
		ue.URL = URL(ue.URL)
	}
	return err
}
