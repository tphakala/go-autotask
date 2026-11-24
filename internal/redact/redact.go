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
// error with the same branches, redacted, whose text is the redacted branches'
// texts joined by newlines: the wrapper's own text is dropped, because it can
// show URL text in any form. Its own type is not kept, and its Timeout answer
// is. Nil branches and typed-nil *url.Error values are skipped. An err with no
// *url.Error is returned as it is. The wrappers above the first *url.Error are
// not kept, and err and the errors it wraps are not modified, so an error that
// other callers share is safe to pass.
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

// redactTree returns err unchanged when no non-nil *url.Error is in its tree,
// and otherwise a copy built from its redacted branches, skipping nil ones,
// whose text is the branches' redacted texts joined by newlines.
func redactTree(err error) error {
	found := urlErrors(err)
	if len(found) == 0 {
		return err
	}
	if reflect.TypeOf(err) == reflect.TypeFor[*url.Error]() {
		// err is the non-nil *url.Error urlErrors visited first.
		return redactURLError(found[0])
	}
	var branches []error
	if multi, isMulti := err.(interface{ Unwrap() []error }); isMulti {
		branches = multi.Unwrap()
	} else if child := errors.Unwrap(err); child != nil {
		branches = []error{child}
	} else {
		// err reaches the *url.Error only through an As method; keep only that.
		return redactURLError(found[0])
	}
	var redacted []error
	var texts []string
	for _, branch := range branches {
		if isNil(branch) {
			continue
		}
		r := redactTree(branch)
		redacted = append(redacted, r)
		texts = append(texts, r.Error())
	}
	timeout, isTimeout := err.(interface{ Timeout() bool })
	return &redactedError{msg: strings.Join(texts, "\n"), errs: redacted, timeout: isTimeout && timeout.Timeout()}
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
// text is its redacted branches' texts, it unwraps to those branches, and its
// Timeout reports what the replaced error's Timeout did, because
// (*url.Error).Timeout asks only its direct cause.
type redactedError struct {
	msg     string
	errs    []error
	timeout bool
}

func (e *redactedError) Error() string   { return e.msg }
func (e *redactedError) Unwrap() []error { return e.errs }
func (e *redactedError) Timeout() bool   { return e.timeout }
