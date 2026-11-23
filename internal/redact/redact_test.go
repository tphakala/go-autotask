package redact

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"
	"testing"
)

func TestURL(t *testing.T) {
	t.Parallel()
	tests := []struct{ name, raw, want string }{
		{"query dropped", "https://h.example/a/b?user=secret&x=1", "https://h.example/a/b"},
		{"user info dropped", "https://name:pw@h.example/a", ""},
		{"fragment dropped", "https://h.example/a#frag", "https://h.example/a"},
		{"at sign in the path", "https://user:1234/x@host/", ""},
		{"unparsable", "https://h.example/%zz", ""},
		{"empty query marker dropped", "https://h.example/a?", "https://h.example/a"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := URL(tt.raw); got != tt.want {
				t.Errorf("URL(%q) = %q; want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestURLErrorReturnsARedactedCopy(t *testing.T) {
	t.Parallel()
	orig := &url.Error{Op: "Get", URL: "https://h.example/z?user=secret", Err: errors.New("EOF")}
	got := URLError(orig)
	if strings.Contains(got.Error(), "secret") {
		t.Errorf("error text = %q; want no query", got.Error())
	}
	if !strings.Contains(orig.Error(), "secret") {
		t.Error("the argument must not be modified")
	}
	if plain := errors.New("plain"); !errors.Is(URLError(plain), plain) {
		t.Error("an error without a *url.Error must be returned unchanged")
	}
}

func TestURLErrorRedactsNestedURLErrors(t *testing.T) {
	t.Parallel()
	inner := &url.Error{Op: "Get", URL: "https://other.example/x?user=bob&tok=SECRET", Err: errors.New("boom")}
	outer := &url.Error{Op: "Get", URL: "https://h.example/a?user=alice", Err: inner}
	got := URLError(outer).Error()
	for _, leak := range []string{"user=", "SECRET", "alice", "bob"} {
		if strings.Contains(got, leak) {
			t.Errorf("error text %q still contains %q", got, leak)
		}
	}
	if !strings.Contains(inner.Error(), "SECRET") {
		t.Error("the nested error must not be modified")
	}
}

func TestURLErrorHidesTheCauseOfAParseErrorWithUserInfo(t *testing.T) {
	t.Parallel()
	// With an unescaped "/" in the password, url.Parse reads "user:pw" as the
	// host and quotes ":pw" in the cause of its error.
	err := &url.Error{Op: "parse", URL: "https://user:pw/x@host/a", Err: errors.New(`invalid port ":pw" after host`)}
	if got := URLError(err).Error(); strings.Contains(got, "pw") || strings.Contains(got, "user") {
		t.Errorf("error text %q still quotes the user info", got)
	}
}

func TestURLErrorKeepsTheCauseOfAParseErrorWithoutUserInfo(t *testing.T) {
	t.Parallel()
	err := &url.Error{Op: "parse", URL: "https://h.example/%zz", Err: errors.New("invalid URL escape")}
	if got := URLError(err).Error(); !strings.Contains(got, "invalid URL escape") {
		t.Errorf("error text %q lost the cause", got)
	}
}

func TestURLErrorKeepsTheCauseOfAnOtherErrorWithUserInfo(t *testing.T) {
	t.Parallel()
	// Only a parse error quotes the user info in its cause; a transport error
	// for the same URL keeps its cause so callers can still read it.
	err := &url.Error{Op: "Get", URL: "https://u:p@h.example/a", Err: errors.New("dial tcp: lookup h.example: no such host")}
	if got := URLError(err).Error(); !strings.Contains(got, "no such host") {
		t.Errorf("error text %q lost the cause", got)
	}
}

type timeoutError struct{}

func (timeoutError) Error() string { return "i/o timeout" }
func (timeoutError) Timeout() bool { return true }

func TestURLErrorKeepsTheErrorChain(t *testing.T) {
	t.Parallel()
	got := URLError(&url.Error{Op: "Get", URL: "https://h.example/z?user=secret", Err: io.EOF})
	if !errors.Is(got, io.EOF) {
		t.Errorf("errors.Is(%v, io.EOF) = false; want the cause reachable through the copy", got)
	}
	got = URLError(&url.Error{Op: "Post", URL: "https://h.example/z?user=secret", Err: context.DeadlineExceeded})
	if !errors.Is(got, context.DeadlineExceeded) {
		t.Errorf("errors.Is(%v, context.DeadlineExceeded) = false", got)
	}
	got = URLError(&url.Error{Op: "Get", URL: "https://h.example/z", Err: timeoutError{}})
	te, ok := errors.AsType[interface {
		error
		Timeout() bool
	}](got)
	if !ok || !te.Timeout() {
		t.Errorf("Timeout() of %v = false; want the cause's timeout kept", got)
	}
}

func TestURLErrorKeepsTheBranchesOfAJoinedCause(t *testing.T) {
	t.Parallel()
	// A transport can return a joined error that holds a *url.Error next to a
	// context error. The redacted copy must keep the whole tree reachable, so
	// errors.Is still finds the context error, while its text drops the query.
	joined := errors.Join(context.DeadlineExceeded,
		&url.Error{Op: "Get", URL: "https://inner.example/x?token=SECRET", Err: io.EOF})
	got := URLError(&url.Error{Op: "Get", URL: "https://h.example/a?user=alice", Err: joined})
	if !errors.Is(got, context.DeadlineExceeded) {
		t.Errorf("errors.Is(%v, context.DeadlineExceeded) = false; want the joined branch kept", got)
	}
	if !errors.Is(got, io.EOF) {
		t.Errorf("errors.Is(%v, io.EOF) = false; want the nested cause kept", got)
	}
	for _, leak := range []string{"SECRET", "alice", "token="} {
		if strings.Contains(got.Error(), leak) {
			t.Errorf("error text %q still contains %q", got.Error(), leak)
		}
	}
}

func TestURLErrorHidesAParseCauseWithUserInfoUnderAJoin(t *testing.T) {
	t.Parallel()
	parse := &url.Error{Op: "parse", URL: "https://user:pw/x@host/a", Err: errors.New(`invalid port ":pw" after host`)}
	joined := errors.Join(context.Canceled, parse)
	got := URLError(&url.Error{Op: "Get", URL: "https://h.example/a", Err: joined})
	if text := got.Error(); strings.Contains(text, "pw") || strings.Contains(text, "user") {
		t.Errorf("error text %q still quotes the user info", text)
	}
	if !errors.Is(got, context.Canceled) {
		t.Errorf("errors.Is(%v, context.Canceled) = false; want the joined branch kept", got)
	}
}

func TestURLErrorRedactsAQuotedNestedURLAndNeverExposesIt(t *testing.T) {
	t.Parallel()
	// url.Error quotes its URL with %q, so a quote in the URL is escaped in the
	// text; the redacted copy must not depend on finding the raw URL there.
	inner := &url.Error{Op: "Get", URL: `https://in.example/x?token=a"SECRET`, Err: io.EOF}
	prefix := &url.Error{Op: "Get", URL: "https://in.example/x", Err: io.ErrUnexpectedEOF}
	wrapped := fmt.Errorf("transport: %w", errors.Join(prefix, inner))
	got := URLError(&url.Error{Op: "Get", URL: "https://h.example/a", Err: wrapped})
	if strings.Contains(got.Error(), "SECRET") || strings.Contains(got.Error(), "token") {
		t.Errorf("error text %q still contains the nested query", got.Error())
	}
	if !errors.Is(got, io.EOF) || !errors.Is(got, io.ErrUnexpectedEOF) {
		t.Errorf("errors.Is lost a branch of %v", got)
	}
	var walk func(error)
	walk = func(e error) {
		if ue, ok := errors.AsType[*url.Error](e); ok && strings.Contains(ue.URL, "SECRET") {
			t.Errorf("a *url.Error with the original URL %q is reachable", ue.URL)
		}
		if multi, ok := e.(interface{ Unwrap() []error }); ok {
			for _, c := range multi.Unwrap() {
				walk(c)
			}
		} else if c := errors.Unwrap(e); c != nil {
			walk(c)
		}
	}
	walk(got)
}

func TestURLErrorCopiesADirectlyNestedURLError(t *testing.T) {
	t.Parallel()
	// net/http wraps a transport's *url.Error in its own; errors.As on the
	// result must reach a redacted copy of the inner one, never the original.
	inner := &url.Error{Op: "Get", URL: "https://other.example/x?tok=SECRET", Err: io.EOF}
	got := URLError(&url.Error{Op: "Get", URL: "https://h.example/a", Err: inner})
	next, ok := errors.AsType[*url.Error](errors.Unwrap(got))
	if !ok || next.URL != "https://other.example/x" {
		t.Errorf("unwrapped cause = %v; want a *url.Error copy with URL https://other.example/x", errors.Unwrap(got))
	}
	if inner.URL != "https://other.example/x?tok=SECRET" {
		t.Error("the nested error must not be modified")
	}
}

// quotingWrapError wraps an error but quotes its text, so the wrapped text does
// not appear in its own as it is.
type quotingWrapError struct{ err error }

func (w quotingWrapError) Error() string { return "wrapped " + strconv.Quote(w.err.Error()) }
func (w quotingWrapError) Unwrap() error { return w.err }

// asOnlyError reaches a *url.Error only through its As method.
type asOnlyError struct{ ue *url.Error }

func (a asOnlyError) Error() string { return "as only: " + a.ue.Error() }
func (a asOnlyError) As(target any) bool {
	if p, ok := target.(**url.Error); ok {
		*p = a.ue
		return true
	}
	return false
}

func TestURLErrorRedactsCausesItCannotRewriteInPlace(t *testing.T) {
	t.Parallel()
	inner := &url.Error{Op: "Get", URL: "https://in.example/x?token=SECRET", Err: io.EOF}
	for name, cause := range map[string]error{
		"wrapper that quotes its cause": quotingWrapError{inner},
		"cause reached through As":      asOnlyError{inner},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := URLError(&url.Error{Op: "Get", URL: "https://h.example/a", Err: cause})
			if strings.Contains(got.Error(), "SECRET") || strings.Contains(got.Error(), "token") {
				t.Errorf("error text %q still contains the nested query", got.Error())
			}
			if !errors.Is(got, io.EOF) {
				t.Errorf("errors.Is(%v, io.EOF) = false", got)
			}
		})
	}
}

func TestURLErrorFailsClosedWhenAWrapperPrintsTheURLItself(t *testing.T) {
	t.Parallel()
	raw := "https://in.example/x?token=SECRET"
	inner := &url.Error{Op: "Get", URL: raw, Err: io.EOF}
	for name, cause := range map[string]error{
		"raw URL in wrapper text":    fmt.Errorf("fetch %s: %w", raw, inner),
		"quoted URL in wrapper text": fmt.Errorf("fetch %q: %w", raw+`"x`, &url.Error{Op: "Get", URL: raw + `"x`, Err: io.EOF}),
		"query in wrapper text":      fmt.Errorf("query token=SECRET: %w", inner),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := URLError(&url.Error{Op: "Get", URL: "https://h.example/a", Err: cause})
			if strings.Contains(got.Error(), "SECRET") {
				t.Errorf("error text %q still contains the nested query", got.Error())
			}
			if !errors.Is(got, io.EOF) {
				t.Errorf("errors.Is(%v, io.EOF) = false", got)
			}
		})
	}
}

func TestURLErrorFailsClosedWhenAWrapperPrintsUserInfo(t *testing.T) {
	t.Parallel()
	inner := &url.Error{Op: "Get", URL: "https://bob:pw1@in.example/x", Err: io.EOF}
	got := URLError(&url.Error{Op: "Get", URL: "https://h.example/a", Err: fmt.Errorf("as bob:pw1: %w", inner)})
	if strings.Contains(got.Error(), "pw1") {
		t.Errorf("error text %q still contains the nested user info", got.Error())
	}
}

// nilBranchError holds a nil branch next to a real one.
type nilBranchError struct{ errs []error }

func (e nilBranchError) Error() string   { return "two branches" }
func (e nilBranchError) Unwrap() []error { return e.errs }

func TestURLErrorToleratesNilBranches(t *testing.T) {
	t.Parallel()
	inner := &url.Error{Op: "Get", URL: "https://in.example/x?token=SECRET", Err: io.EOF}
	got := URLError(&url.Error{Op: "Get", URL: "https://h.example/a", Err: nilBranchError{errs: []error{nil, inner}}})
	if strings.Contains(got.Error(), "SECRET") || !errors.Is(got, io.EOF) {
		t.Errorf("got %q; want the nested query dropped and io.EOF reachable", got.Error())
	}
	var nilURL *url.Error
	if got := URLError(nilURL); !errors.Is(got, nilURL) {
		t.Errorf("URLError(typed nil) = %v; want it returned as it is", got)
	}
	wrapped := fmt.Errorf("w: %w", nilURL)
	if got := URLError(&url.Error{Op: "Get", URL: "https://h.example/a", Err: wrapped}); !strings.Contains(got.Error(), "w: ") {
		t.Errorf("URLError with a typed-nil nested *url.Error = %q", got.Error())
	}
}

// asNilError answers As for *url.Error with a typed nil.
type asNilError struct{}

func (asNilError) Error() string { return "as nil" }
func (asNilError) As(target any) bool {
	if p, ok := target.(**url.Error); ok {
		*p = nil
		return true
	}
	return false
}

func TestURLErrorToleratesAnAsMethodThatYieldsNil(t *testing.T) {
	t.Parallel()
	got := URLError(&url.Error{Op: "Get", URL: "https://h.example/a?user=alice", Err: asNilError{}})
	if text := got.Error(); strings.Contains(text, "alice") || !strings.Contains(text, "as nil") {
		t.Errorf("error text = %q; want the cause kept and the query dropped", text)
	}
}

func TestURLErrorFailsClosedOnQuotedQueryAndFragmentText(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		raw, shown string
	}{
		"quoted query": {`https://in.example/x?token=a"SECRET`, `token=a"SECRET`},
		"fragment":     {"https://in.example/x#SECRET", "#SECRET"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			inner := &url.Error{Op: "Get", URL: tc.raw, Err: io.EOF}
			got := URLError(&url.Error{Op: "Get", URL: "https://h.example/a", Err: fmt.Errorf("saw %q: %w", tc.shown, inner)})
			if strings.Contains(got.Error(), "SECRET") {
				t.Errorf("error text %q still contains the nested secret", got.Error())
			}
		})
	}
}

// timeoutCauseError is a transport error that reports a timeout and wraps a
// *url.Error, like a *net.OpError around a redirect target's failure.
type timeoutCauseError struct{ err error }

func (e timeoutCauseError) Error() string { return "dial: " + e.err.Error() }
func (e timeoutCauseError) Unwrap() error { return e.err }
func (e timeoutCauseError) Timeout() bool { return true }

func TestURLErrorKeepsTheTimeoutOfARebuiltCause(t *testing.T) {
	t.Parallel()
	inner := &url.Error{Op: "Get", URL: "https://in.example/x?token=SECRET", Err: io.EOF}
	got := URLError(&url.Error{Op: "Get", URL: "https://h.example/a", Err: timeoutCauseError{inner}})
	ue, ok := errors.AsType[*url.Error](got)
	if !ok || !ue.Timeout() {
		t.Errorf("Timeout() of %v = false; want the cause's timeout kept", got)
	}
	if strings.Contains(got.Error(), "SECRET") {
		t.Errorf("error text %q still contains the nested query", got.Error())
	}
}

// extraTextError wraps an error and adds text of its own after the wrapped
// text.
type extraTextError struct {
	err   error
	extra string
}

func (e extraTextError) Error() string { return e.err.Error() + e.extra }
func (e extraTextError) Unwrap() error { return e.err }

func TestURLErrorDropsTheOwnTextOfAWrapperThatHoldsAURLError(t *testing.T) {
	t.Parallel()
	// A wrapper can show any part of a nested URL in any form: a fragment, a
	// password, a reordered or decoded query value. None of it may survive.
	inner := &url.Error{Op: "Get", URL: "https://bob:PWSECRET@in.example/x?token=a%20QSECRET&b=2#FRAGSECRET", Err: io.EOF}
	for name, extra := range map[string]string{
		"fragment":        " frag=FRAGSECRET",
		"password":        " pw=PWSECRET",
		"reordered query": " b=2&token=a%20QSECRET",
		"decoded query":   " token=a QSECRET",
		"empty":           "",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := URLError(&url.Error{Op: "Get", URL: "https://h.example/a", Err: extraTextError{inner, extra}})
			if strings.Contains(got.Error(), "SECRET") {
				t.Errorf("error text %q still contains a nested secret", got.Error())
			}
			if !errors.Is(got, io.EOF) {
				t.Errorf("errors.Is(%v, io.EOF) = false", got)
			}
		})
	}
}

func TestURLErrorKeepsTheTextOfAnEmptyWrapperSane(t *testing.T) {
	t.Parallel()
	inner := &url.Error{Op: "Get", URL: "https://in.example/x?token=SECRET", Err: io.EOF}
	got := URLError(&url.Error{Op: "Get", URL: "https://h.example/a", Err: extraTextError{extraTextError{inner, ""}, "outer"}})
	want := `Get "https://h.example/a": Get "https://in.example/x": EOF`
	if got.Error() != want {
		t.Errorf("error text = %q; want %q", got.Error(), want)
	}
}
