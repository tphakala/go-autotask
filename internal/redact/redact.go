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

// URLError returns a redacted copy of the first *url.Error in err's chain: its
// URL becomes URL(ue.URL), and every *url.Error in the tree below it is copied
// the same way. net/http wraps what a transport returns in a *url.Error, so a
// transport that returns one gives two. Errors in the tree that hold no
// *url.Error are kept as they are, so errors.Is still finds a joined
// context.DeadlineExceeded or io.EOF, and errors.As returns only redacted
// copies of a *url.Error. A join or wrapper that holds one is replaced by an
// error with the same branches, redacted; its own type is not kept. An err with
// no *url.Error is returned as it is. The wrappers above the first *url.Error
// are not kept, and err and the errors it wraps are not modified, so an error
// that other callers share is safe to pass.
//
// When the parsed URL contains "@" the cause of a parse error is replaced too,
// because url.Parse can quote part of the user info in it, such as
// `invalid port ":pw" after host`.
func URLError(err error) error {
	ue, ok := errors.AsType[*url.Error](err)
	if !ok {
		return err
	}
	return redactURLError(ue)
}

// redactURLError returns a copy of ue with its URL redacted and its cause
// redacted by redactTree, or replaced when it can quote user info.
func redactURLError(ue *url.Error) *url.Error {
	redacted := *ue
	redacted.URL = URL(ue.URL)
	if ue.Op == "parse" && strings.Contains(ue.URL, "@") {
		redacted.Err = errInvalidURL
	} else {
		redacted.Err = redactTree(ue.Err)
	}
	return &redacted
}

// redactTree returns err unchanged when no *url.Error is in its tree, and
// otherwise a copy built from redacted branches.
func redactTree(err error) error {
	found, ok := errors.AsType[*url.Error](err)
	if !ok {
		return err
	}
	if reflect.TypeOf(err) == reflect.TypeFor[*url.Error]() {
		return redactURLError(found)
	}
	var branches []error
	if multi, isMulti := err.(interface{ Unwrap() []error }); isMulti {
		branches = multi.Unwrap()
	} else if child := errors.Unwrap(err); child != nil {
		branches = []error{child}
	} else {
		// err reaches the *url.Error through an As method; keep only that.
		return redactURLError(found)
	}
	msg := err.Error()
	redacted := make([]error, len(branches))
	for i, branch := range branches {
		redacted[i] = redactTree(branch)
		msg = replaceBranchText(msg, branch, redacted[i])
	}
	return &redactedError{msg: msg, errs: redacted}
}

// replaceBranchText swaps the text of branch in msg for the text of its
// redacted copy. When msg does not hold the branch text as it is (a wrapper
// that quotes or reformats it), the wrapper's own text cannot be rewritten
// safely, so only the redacted text is kept.
func replaceBranchText(msg string, branch, redacted error) string {
	before, after := branch.Error(), redacted.Error()
	if before == after {
		return msg
	}
	if strings.Contains(msg, before) {
		return strings.ReplaceAll(msg, before, after)
	}
	return after
}

// redactedError stands in for a join or wrapper that held a *url.Error: it has
// that error's text with each branch redacted, and unwraps to the redacted
// branches.
type redactedError struct {
	msg  string
	errs []error
}

func (e *redactedError) Error() string   { return e.msg }
func (e *redactedError) Unwrap() []error { return e.errs }
