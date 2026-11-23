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

// errInvalidURL replaces the cause of a parse error of a URL with user info.
var errInvalidURL = errors.New("invalid URL")

// URLError returns a copy of the first *url.Error in err's chain with its URL
// replaced by URL(ue.URL), and with the same done to every *url.Error nested in
// its Err chain: net/http wraps what a transport returns in a *url.Error, so a
// transport that returns one gives two. An err with no *url.Error is returned
// as it is. The wrappers above the *url.Error are not kept, and err and the
// errors it wraps are not modified, so an error that other callers share is safe
// to pass.
//
// When the parsed URL contains "@" the cause of a parse error is replaced too,
// because url.Parse can quote part of the user info in it, such as
// `invalid port ":pw" after host`.
func URLError(err error) error {
	ue, ok := errors.AsType[*url.Error](err)
	if !ok {
		return err
	}
	redacted := *ue
	redacted.URL = URL(ue.URL)
	if ue.Op == "parse" && strings.Contains(ue.URL, "@") {
		redacted.Err = errInvalidURL
	} else {
		redacted.Err = URLError(ue.Err)
	}
	return &redacted
}
