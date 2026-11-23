// Package redact removes the parts of a URL that can carry credentials, so the
// URL can be logged or returned in an error.
package redact

import (
	"errors"
	"net/url"
	"reflect"
	"strconv"
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
// error with the same branches, redacted; its own type is not kept. Nil branches
// and typed-nil *url.Error values are skipped. An err with no *url.Error is
// returned as it is. The wrappers above the first *url.Error are not kept, and
// err and the errors it wraps are not modified, so an error that other callers
// share is safe to pass.
//
// When the parsed URL contains "@" the cause of a parse error is replaced too,
// because url.Parse can quote part of the user info in it, such as
// `invalid port ":pw" after host`.
func URLError(err error) error {
	ue, ok := errors.AsType[*url.Error](err)
	if !ok {
		return err
	}
	if ue == nil {
		// A typed-nil *url.Error came first; redact the rest of the tree.
		return redactTree(err)
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

// redactTree returns err unchanged when no *url.Error is in its tree, or when
// the only one is a typed nil, and otherwise a copy built from redacted
// branches, skipping nil ones. The copy keeps err's own text
// with each branch's text swapped for its redacted text, unless that text
// would still show a nested URL, query or user info; then it is the redacted
// branch texts joined by newlines.
func redactTree(err error) error {
	found, ok := errors.AsType[*url.Error](err)
	if !ok {
		return err
	}
	if reflect.TypeOf(err) == reflect.TypeFor[*url.Error]() {
		if found == nil {
			return err
		}
		return redactURLError(found)
	}
	var branches []error
	if multi, isMulti := err.(interface{ Unwrap() []error }); isMulti {
		branches = multi.Unwrap()
	} else if child := errors.Unwrap(err); child != nil {
		branches = []error{child}
	} else {
		// err reaches the *url.Error through an As method; keep only that.
		if found == nil {
			return err
		}
		return redactURLError(found)
	}
	msg := err.Error()
	redacted := make([]error, len(branches))
	texts := make([]string, len(branches))
	for i, branch := range branches {
		if isNil(branch) {
			continue
		}
		redacted[i] = redactTree(branch)
		texts[i] = redacted[i].Error()
		msg = replaceBranchText(msg, branch, redacted[i])
	}
	if showsSecret(msg, err) {
		// A wrapper printed URL text of its own; keep only the redacted
		// branches.
		msg = strings.Join(texts, "\n")
	}
	return &redactedError{msg: msg, errs: redacted}
}

// showsSecret reports whether msg still holds, as it is or quoted, a URL of a
// *url.Error in err's tree that URL would change, or that URL's query or user
// info.
func showsSecret(msg string, err error) bool {
	for _, ue := range urlErrors(err) {
		if ue.URL == "" || URL(ue.URL) == ue.URL {
			continue
		}
		quoted := strconv.Quote(ue.URL)
		if strings.Contains(msg, ue.URL) || strings.Contains(msg, quoted[1:len(quoted)-1]) {
			return true
		}
		if u, perr := url.Parse(ue.URL); perr == nil {
			if u.RawQuery != "" && strings.Contains(msg, u.RawQuery) {
				return true
			}
			if u.User != nil && strings.Contains(msg, u.User.String()) {
				return true
			}
		}
	}
	return false
}

// urlErrors returns every non-nil *url.Error in err's tree. Each node is
// visited through its Unwrap methods, and errors.AsType on a node that is a
// *url.Error returns that node, so the result holds all of them. Nil nodes are
// not visited, because calling their methods can panic.
func urlErrors(err error) []*url.Error {
	seen := map[*url.Error]bool{}
	var found []*url.Error
	var walk func(error)
	walk = func(node error) {
		if isNil(node) {
			return
		}
		if ue, ok := errors.AsType[*url.Error](node); ok && ue != nil && !seen[ue] {
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

// isNil reports whether err is nil or a nil pointer, whose Error method can
// panic.
func isNil(err error) bool {
	if err == nil {
		return true
	}
	v := reflect.ValueOf(err)
	return v.Kind() == reflect.Pointer && v.IsNil()
}

// redactedError stands in for a join or wrapper that held a *url.Error: its
// text is built by redactTree, and it unwraps to the redacted branches.
type redactedError struct {
	msg  string
	errs []error
}

func (e *redactedError) Error() string   { return e.msg }
func (e *redactedError) Unwrap() []error { return e.errs }
