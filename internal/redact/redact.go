// Package redact removes the parts of a URL that can carry credentials, so the
// URL can be logged or returned in an error.
package redact

import (
	"errors"
	"net/url"
	"reflect"
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
// replaced by URL(ue.URL). net/http wraps what a transport returns in a
// *url.Error, so a transport that returns one gives two: a *url.Error that is
// the copy's direct cause is redacted the same way. Any other cause that holds
// a *url.Error deeper in its tree, such as a joined or wrapped error, is kept
// whole behind redacted text, so errors.Is and errors.As still reach every
// branch; errors.As can then return a nested *url.Error with its original URL.
// An err with no *url.Error is returned as it is. The wrappers above the first
// *url.Error are not kept, and err and the errors it wraps are not modified, so
// an error that other callers share is safe to pass.
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
	switch {
	case hidesParseCause(ue):
		redacted.Err = errInvalidURL
	case reflect.TypeOf(ue.Err) == reflect.TypeFor[*url.Error]():
		redacted.Err = URLError(ue.Err)
	default:
		redacted.Err = redactTree(ue.Err)
	}
	return &redacted
}

// hidesParseCause reports whether the cause of ue must not be shown: a parse
// error of a URL with user info.
func hidesParseCause(ue *url.Error) bool {
	return ue.Op == "parse" && strings.Contains(ue.URL, "@")
}

// redactTree returns err unchanged when its tree holds no *url.Error, and
// otherwise err behind text with every nested URL, and every hidden parse
// cause, replaced.
func redactTree(err error) error {
	nested := urlErrors(err)
	if len(nested) == 0 {
		return err
	}
	msg := err.Error()
	for _, ue := range nested {
		if hidesParseCause(ue) && ue.Err != nil && ue.Err.Error() != "" {
			msg = strings.ReplaceAll(msg, ue.Err.Error(), errInvalidURL.Error())
		}
		if ue.URL != "" {
			msg = strings.ReplaceAll(msg, ue.URL, URL(ue.URL))
		}
	}
	return &redactedError{msg: msg, err: err}
}

// urlErrors returns every *url.Error in err's tree. Each node is visited
// through its Unwrap methods; errors.AsType on a node that is a *url.Error
// returns that node, so the set holds all of them.
func urlErrors(err error) []*url.Error {
	seen := map[*url.Error]bool{}
	var found []*url.Error
	var walk func(error)
	walk = func(node error) {
		if node == nil {
			return
		}
		if ue, ok := errors.AsType[*url.Error](node); ok && !seen[ue] {
			seen[ue] = true
			found = append(found, ue)
		}
		if multi, ok := node.(interface{ Unwrap() []error }); ok {
			for _, child := range multi.Unwrap() {
				walk(child)
			}
			return
		}
		walk(errors.Unwrap(node))
	}
	walk(err)
	return found
}

// redactedError is an error with redacted text that still unwraps to the error
// it was made from.
type redactedError struct {
	msg string
	err error
}

func (e *redactedError) Error() string { return e.msg }
func (e *redactedError) Unwrap() error { return e.err }
