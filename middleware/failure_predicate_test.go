//nolint:bodyclose // the responses are fabricated and their bodies are in-memory readers
package middleware

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

const validation500 = `{"errors":["String value exceeds maximum length (field:description)"]}`

func respWith(status int, contentType, body string) *http.Response {
	h := http.Header{}
	if contentType != "" {
		h.Set("Content-Type", contentType)
	}
	return &http.Response{
		StatusCode: status, Header: h,
		Body: io.NopCloser(strings.NewReader(body)), ContentLength: int64(len(body)),
	}
}

func TestDefaultFailure(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		resp *http.Response
		err  error
		want bool
	}{
		{"transport error", nil, errors.New("boom"), true},
		{"deadline exceeded", nil, context.DeadlineExceeded, true},
		{"caller cancellation", nil, context.Canceled, false},
		{"wrapped cancellation", nil, errors.Join(errors.New("x"), context.Canceled), false},
		{"200 json", respWith(200, "application/json", `{}`), nil, false},
		{"200 problem json", respWith(200, "application/problem+json; charset=utf-8", `{}`), nil, false},
		{"200 html", respWith(200, "text/html; charset=utf-8", "<html>"), nil, true},
		{"200 text/plain is not counted", respWith(200, "text/plain; charset=utf-8", `{}`), nil, false},
		{"200 without content type", respWith(200, "", `{}`), nil, false},
		{"204 with html label", respWith(204, "text/html", ""), nil, false},
		{"200 empty body with html label", respWith(200, "text/html", ""), nil, false},
		{"unparsable content type", respWith(200, "///", "x"), nil, true},
		{"json with a malformed parameter", respWith(200, "application/json; charset", `{}`), nil, false},
		{"html with a malformed parameter", respWith(200, "text/html; charset", "<html>"), nil, true},
		{"json with conflicting duplicate parameters", respWith(200, "application/json; charset=utf-8; charset=latin1", `{}`), nil, false},
		{"text/plain with conflicting duplicate parameters", respWith(200, "Text/Plain; charset=utf-8; charset=latin1", `{}`), nil, false},
		{"html with conflicting duplicate parameters", respWith(200, "text/html; charset=utf-8; charset=latin1", "<html>"), nil, true},
		{"404", respWith(404, "application/json", `{}`), nil, false},
		{"429", respWith(429, "", ""), nil, true},
		{"503", respWith(503, "", validation500), nil, true},
		{"500 no body", respWith(500, "", ""), nil, true},
		{"500 unrelated message", respWith(500, "application/json", `{"errors":["database is down"]}`), nil, true},
		{"500 not json", respWith(500, "text/html", "<html>"), nil, true},
		{"500 validation", respWith(500, "application/json", validation500), nil, false},
		{"500 validation object form", respWith(500, "application/json", `{"errors":[{"message":"Exceeds Maximum Length"}]}`), nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := DefaultFailure(tt.resp, tt.err); got != tt.want {
				t.Fatalf("DefaultFailure = %v; want %v", got, tt.want)
			}
		})
	}
}

func TestStatusFailure(t *testing.T) {
	t.Parallel()
	if !StatusFailure(nil, context.Canceled) {
		t.Error("StatusFailure must count a transport error, cancellation included")
	}
	if !StatusFailure(respWith(500, "application/json", validation500), nil) {
		t.Error("StatusFailure must count a validation 500")
	}
	if StatusFailure(respWith(200, "text/html", "<html>"), nil) {
		t.Error("StatusFailure must not count a 200")
	}
	if !StatusFailure(respWith(429, "", ""), nil) || StatusFailure(respWith(404, "", ""), nil) {
		t.Error("StatusFailure: 429 counts, 404 does not")
	}
}

// scripted answers each round trip from a list, then 200.
type scripted struct {
	resps []*http.Response
	errs  []error
	calls int
}

func (s *scripted) RoundTrip(*http.Request) (*http.Response, error) {
	i := s.calls
	s.calls++
	if i < len(s.errs) && s.errs[i] != nil {
		return nil, s.errs[i]
	}
	if i < len(s.resps) {
		return s.resps[i], nil
	}
	return respWith(200, "application/json", `{}`), nil
}

func newReq(t *testing.T) *http.Request {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://example.com/x", http.NoBody)
	if err != nil {
		t.Fatal(err)
	}
	return req
}

func do(t *testing.T, cb *CircuitBreaker) (*http.Response, error) {
	t.Helper()
	return cb.RoundTrip(newReq(t))
}

func drain(t *testing.T, resp *http.Response) {
	t.Helper()
	if resp != nil {
		_ = resp.Body.Close()
	}
}

func TestBreakerIgnoresValidation500ByDefault(t *testing.T) {
	t.Parallel()
	inner := &scripted{}
	for range 6 {
		inner.resps = append(inner.resps, respWith(500, "application/json", validation500))
	}
	cb := NewCircuitBreaker(inner, WithFailureThreshold(3))
	for range 6 {
		resp, err := do(t, cb)
		if err != nil {
			t.Fatalf("round trip failed with the breaker %s: %v", cb.State(), err)
		}
		drain(t, resp)
	}
	if cb.State() != StateClosed {
		t.Fatalf("state = %s; want closed after validation 500s", cb.State())
	}
	resp, err := do(t, cb)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("following request = %v, %v; want 200", resp, err)
	}
	drain(t, resp)
}

func TestBreakerCountsUnmatched500(t *testing.T) {
	t.Parallel()
	inner := &scripted{}
	for range 3 {
		inner.resps = append(inner.resps, respWith(500, "application/json", `{"errors":["database is down"]}`))
	}
	cb := NewCircuitBreaker(inner, WithFailureThreshold(3))
	for range 3 {
		resp, _ := do(t, cb)
		drain(t, resp)
	}
	if cb.State() != StateOpen {
		t.Fatalf("state = %s; want open", cb.State())
	}
}

func TestBreakerOpensOnMaintenancePage(t *testing.T) {
	t.Parallel()
	inner := &scripted{}
	for range 3 {
		inner.resps = append(inner.resps, respWith(200, "text/html", "<html>maintenance</html>"))
	}
	cb := NewCircuitBreaker(inner, WithFailureThreshold(3))
	for range 3 {
		resp, _ := do(t, cb)
		drain(t, resp)
	}
	if cb.State() != StateOpen {
		t.Fatalf("state = %s; want open after 200 text/html responses", cb.State())
	}
}

func TestBreakerMaintenancePageIsNotHalfOpenSuccess(t *testing.T) {
	t.Parallel()
	inner := &scripted{resps: []*http.Response{
		respWith(503, "", ""), respWith(200, "text/html", "<html>"),
	}}
	cb := NewCircuitBreaker(inner, WithFailureThreshold(1), WithOpenTimeout(5*time.Millisecond), WithSuccessThreshold(1))
	resp, _ := do(t, cb)
	drain(t, resp)
	time.Sleep(15 * time.Millisecond)
	if cb.State() != StateHalfOpen {
		t.Fatalf("state = %s; want half-open", cb.State())
	}
	resp, _ = do(t, cb)
	drain(t, resp)
	if cb.State() != StateOpen {
		t.Fatalf("state = %s; want open, the HTML page is a failed probe", cb.State())
	}
}

func TestBreakerCancellationCountsNeitherWay(t *testing.T) {
	t.Parallel()
	inner := &scripted{errs: []error{context.Canceled, context.Canceled, context.Canceled}}
	cb := NewCircuitBreaker(inner, WithFailureThreshold(2))
	for range 3 {
		if _, err := do(t, cb); !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v; want context.Canceled", err)
		}
	}
	if cb.State() != StateClosed {
		t.Fatalf("state = %s; want closed, cancellation is not an upstream failure", cb.State())
	}
}

func TestBreakerCancelledProbeDoesNotCloseHalfOpen(t *testing.T) {
	t.Parallel()
	inner := &scripted{
		resps: []*http.Response{respWith(503, "", "")},
		errs:  []error{nil, context.Canceled},
	}
	cb := NewCircuitBreaker(inner, WithFailureThreshold(1), WithOpenTimeout(5*time.Millisecond), WithSuccessThreshold(1))
	resp, _ := do(t, cb)
	drain(t, resp)
	time.Sleep(15 * time.Millisecond)
	if _, err := do(t, cb); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}
	if cb.State() != StateHalfOpen {
		t.Fatalf("state = %s; want half-open, a cancelled probe proves nothing", cb.State())
	}
}

func TestBreakerTransportErrorStillCounts(t *testing.T) {
	t.Parallel()
	inner := &scripted{errs: []error{io.ErrUnexpectedEOF, io.ErrUnexpectedEOF}}
	cb := NewCircuitBreaker(inner, WithFailureThreshold(2))
	for range 2 {
		_, _ = do(t, cb)
	}
	if cb.State() != StateOpen {
		t.Fatalf("state = %s; want open", cb.State())
	}
}

func TestWithFailurePredicateReplacesDefault(t *testing.T) {
	t.Parallel()
	inner := &scripted{}
	for range 2 {
		inner.resps = append(inner.resps, respWith(418, "application/json", `{}`))
	}
	cb := NewCircuitBreaker(inner, WithFailureThreshold(2),
		WithFailurePredicate(func(resp *http.Response, _ error) bool { return resp != nil && resp.StatusCode == http.StatusTeapot }))
	for range 2 {
		resp, _ := do(t, cb)
		drain(t, resp)
	}
	if cb.State() != StateOpen {
		t.Fatalf("state = %s; want open from the custom predicate", cb.State())
	}
}

func TestWithFailurePredicateStatusOnlyKeepsOldBehaviour(t *testing.T) {
	t.Parallel()
	inner := &scripted{}
	for range 2 {
		inner.resps = append(inner.resps, respWith(500, "application/json", validation500))
	}
	cb := NewCircuitBreaker(inner, WithFailureThreshold(2), WithFailurePredicate(StatusFailure))
	for range 2 {
		resp, _ := do(t, cb)
		drain(t, resp)
	}
	if cb.State() != StateOpen {
		t.Fatalf("state = %s; want open with StatusFailure", cb.State())
	}
}

func TestWithFailurePredicateNilIgnored(t *testing.T) {
	t.Parallel()
	inner := &scripted{resps: []*http.Response{respWith(503, "", "")}}
	cb := NewCircuitBreaker(inner, WithFailureThreshold(1), WithFailurePredicate(nil))
	resp, _ := do(t, cb)
	drain(t, resp)
	if cb.State() != StateOpen {
		t.Fatalf("state = %s; want open, nil must leave the default in place", cb.State())
	}
}

func TestPredicateReadsBodyAndCallerStillGetsIt(t *testing.T) {
	t.Parallel()
	for _, size := range []int{0, 10, peekLimit, peekLimit + 1, 3*peekLimit + 7} {
		body := positional(size)
		var seen int
		cb := NewCircuitBreaker(&scripted{resps: []*http.Response{respWith(200, "text/plain", body)}},
			WithFailurePredicate(func(resp *http.Response, _ error) bool {
				b, err := io.ReadAll(resp.Body)
				if err != nil {
					t.Error(err)
				}
				seen = len(b)
				return false
			}))
		resp, err := do(t, cb)
		if err != nil {
			t.Fatal(err)
		}
		got, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		drain(t, resp)
		if string(got) != body {
			t.Fatalf("size %d: caller read %d bytes; want %d", size, len(got), size)
		}
		if want := min(size, peekLimit); seen != want {
			t.Fatalf("size %d: predicate saw %d bytes; want %d", size, seen, want)
		}
	}
}

// positional returns n bytes whose value depends on the position, so a body
// replayed out of order does not compare equal.
func positional(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte('a' + i%251%26 + i/251%3)
	}
	return string(b)
}

func TestPredicateStoppingEarlyLeavesRestOfBody(t *testing.T) {
	t.Parallel()
	for _, size := range []int{peekLimit + 1, 3*peekLimit + 7} {
		for _, n := range []int{1, 10, peekLimit - 1, peekLimit} {
			body := positional(size)
			var seen string
			cb := NewCircuitBreaker(&scripted{resps: []*http.Response{respWith(200, "text/plain", body)}},
				WithFailurePredicate(func(resp *http.Response, _ error) bool {
					b := make([]byte, n)
					got, err := io.ReadFull(resp.Body, b)
					if err != nil {
						t.Error(err)
					}
					seen = string(b[:got])
					return false
				}))
			resp, err := do(t, cb)
			if err != nil {
				t.Fatal(err)
			}
			got, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatal(err)
			}
			drain(t, resp)
			if string(got) != body {
				t.Fatalf("size %d, predicate read %d: caller got %d bytes in a different order or length; want the whole body", size, n, len(got))
			}
			if seen != body[:n] {
				t.Fatalf("size %d, predicate read %d: predicate saw the wrong bytes", size, n)
			}
		}
	}
}

func TestPredicateThatReadsNothingLeavesBodyIntact(t *testing.T) {
	t.Parallel()
	cb := NewCircuitBreaker(&scripted{resps: []*http.Response{respWith(200, "application/json", `{"a":1}`)}})
	resp, err := do(t, cb)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(resp.Body)
	drain(t, resp)
	if string(b) != `{"a":1}` {
		t.Fatalf("body = %q", b)
	}
}

func TestBreakerClosesOriginalBody(t *testing.T) {
	t.Parallel()
	closed := false
	resp := respWith(200, "application/json", `{}`)
	resp.Body = &closeSpy{Reader: strings.NewReader(`{}`), closed: &closed}
	cb := NewCircuitBreaker(&scripted{resps: []*http.Response{resp}})
	got, err := do(t, cb)
	if err != nil {
		t.Fatal(err)
	}
	if err := got.Body.Close(); err != nil {
		t.Fatal(err)
	}
	if !closed {
		t.Fatal("closing the returned body must close the original body")
	}
}

type closeSpy struct {
	io.Reader
	closed *bool
}

func (c *closeSpy) Close() error { *c.closed = true; return nil }

func TestBreakerNilBody(t *testing.T) {
	t.Parallel()
	cb := NewCircuitBreaker(&scripted{resps: []*http.Response{{StatusCode: 200}}})
	resp, err := do(t, cb)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("got %v, %v", resp, err)
	}
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("read failed") }

func TestDefaultFailureUnreadable500Body(t *testing.T) {
	t.Parallel()
	nilBody := &http.Response{StatusCode: http.StatusInternalServerError}
	if !DefaultFailure(nilBody, nil) {
		t.Error("a 500 without a body is a server failure")
	}
	broken := &http.Response{StatusCode: http.StatusInternalServerError, Body: io.NopCloser(failingReader{})}
	if !DefaultFailure(broken, nil) {
		t.Error("a 500 whose body cannot be read is a server failure")
	}
}

func TestBreakerClosesAfterGoodProbes(t *testing.T) {
	t.Parallel()
	inner := &scripted{resps: []*http.Response{respWith(503, "", "")}}
	cb := NewCircuitBreaker(inner, WithFailureThreshold(1), WithOpenTimeout(5*time.Millisecond), WithSuccessThreshold(2))
	resp, _ := do(t, cb)
	drain(t, resp)
	time.Sleep(15 * time.Millisecond)
	if cb.State() != StateHalfOpen {
		t.Fatalf("state = %s; want half-open", cb.State())
	}
	resp, err := do(t, cb)
	if err != nil {
		t.Fatal(err)
	}
	drain(t, resp)
	if cb.State() != StateHalfOpen {
		t.Fatalf("state = %s; want half-open after one good probe of two", cb.State())
	}
	resp, err = do(t, cb)
	if err != nil {
		t.Fatal(err)
	}
	drain(t, resp)
	if cb.State() != StateClosed {
		t.Fatalf("state = %s; want closed after two good probes", cb.State())
	}
}
