package autotask

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptrace"
	"strings"
	"syscall"
	"testing"
)

// errUnexpectedCall marks a RoundTripper branch that must never run.
var errUnexpectedCall = errors.New("unexpected round tripper call")

// fakeRT is a scriptable http.RoundTripper that records how often it was called.
type fakeRT struct {
	calls int
	fn    func(*http.Request) (*http.Response, error)
}

func (f *fakeRT) RoundTrip(r *http.Request) (*http.Response, error) {
	f.calls++
	return f.fn(r)
}

// fireGotConn simulates a transport obtaining a usable connection by invoking the
// request's httptrace GotConn hook, the same signal RoundTrip watches to decide
// whether a resend is safe.
func fireGotConn(r *http.Request) {
	if tr := httptrace.ContextClientTrace(r.Context()); tr != nil && tr.GotConn != nil {
		tr.GotConn(httptrace.GotConnInfo{})
	}
}

func okResponse() *http.Response {
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("ok"))}
}

// timeoutError is a net.Error whose Timeout reports true, used to prove a timeout
// does not trigger the TLS-version downgrade.
type timeoutError struct{}

func (timeoutError) Error() string   { return "i/o timeout" }
func (timeoutError) Timeout() bool   { return true }
func (timeoutError) Temporary() bool { return true }

func newFallbackFor(t *testing.T, primary, fallback http.RoundTripper) *tlsFallbackTransport {
	t.Helper()
	return &tlsFallbackTransport{primary: primary, fallback: fallback, forced12: make(map[string]bool)}
}

// mustGet issues a GET through rt, closes the body, and returns the status code
// (0 on error). Returning the code rather than the response keeps callers free of
// body-close bookkeeping.
func mustGet(t *testing.T, rt http.RoundTripper, url string) (int, error) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, http.NoBody)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := rt.RoundTrip(req)
	if resp != nil && resp.Body != nil {
		if cerr := resp.Body.Close(); cerr != nil {
			t.Errorf("close body: %v", cerr)
		}
	}
	if err != nil {
		return 0, err
	}
	return resp.StatusCode, nil
}

// TestNewTLSFallbackTransportVersions pins the version split that the whole fix
// rests on: the primary transport may offer TLS 1.3 (MaxVersion unset), the
// fallback is capped at TLS 1.2. Removing the fallback cap turns this red.
func TestNewTLSFallbackTransportVersions(t *testing.T) {
	ft := newTLSFallbackTransport()

	primary, ok := ft.primary.(*http.Transport)
	if !ok {
		t.Fatalf("primary is %T, want *http.Transport", ft.primary)
	}
	if primary.TLSClientConfig.MinVersion != tls.VersionTLS12 {
		t.Errorf("primary MinVersion = %#x, want TLS 1.2", primary.TLSClientConfig.MinVersion)
	}
	if primary.TLSClientConfig.MaxVersion != 0 {
		t.Errorf("primary MaxVersion = %#x, want unset so TLS 1.3 stays available", primary.TLSClientConfig.MaxVersion)
	}

	fallback, ok := ft.fallback.(*http.Transport)
	if !ok {
		t.Fatalf("fallback is %T, want *http.Transport", ft.fallback)
	}
	if fallback.TLSClientConfig.MaxVersion != tls.VersionTLS12 {
		t.Errorf("fallback MaxVersion = %#x, want TLS 1.2 (Autotask refuses TLS 1.3)", fallback.TLSClientConfig.MaxVersion)
	}
}

func TestTLSFallbackUsesPrimaryOnSuccess(t *testing.T) {
	primary := &fakeRT{fn: func(*http.Request) (*http.Response, error) { return okResponse(), nil }}
	fallback := &fakeRT{fn: func(*http.Request) (*http.Response, error) {
		t.Error("fallback must not run when primary succeeds")
		return nil, errUnexpectedCall
	}}
	ft := newFallbackFor(t, primary, fallback)

	status, err := mustGet(t, ft, "https://host.example/x")
	if err != nil || status != http.StatusOK {
		t.Fatalf("got status=%d err=%v", status, err)
	}
	if fallback.calls != 0 {
		t.Errorf("fallback called %d times, want 0", fallback.calls)
	}
	if ft.hostForced12("host.example") {
		t.Error("host must not be marked for TLS 1.2 after a 1.3 success")
	}
}

// TestTLSFallbackDowngradesOnHandshakeFailure exercises the safe GET downgrade: the
// primary refuses the 1.3 handshake (io.EOF, no GotConn), so the request is resent
// over the 1.2 transport and the host is remembered.
func TestTLSFallbackDowngradesOnHandshakeFailure(t *testing.T) {
	primary := &fakeRT{fn: func(*http.Request) (*http.Response, error) { return nil, io.EOF }}
	fallback := &fakeRT{fn: func(*http.Request) (*http.Response, error) { return okResponse(), nil }}
	ft := newFallbackFor(t, primary, fallback)

	status, err := mustGet(t, ft, "https://host.example/x")
	if err != nil {
		t.Fatalf("expected fallback to succeed, got err=%v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	if primary.calls != 1 || fallback.calls != 1 {
		t.Errorf("primary.calls=%d fallback.calls=%d, want 1 and 1", primary.calls, fallback.calls)
	}
	if !ft.hostForced12("host.example") {
		t.Error("host should be cached as TLS 1.2 after a successful downgrade")
	}
}

func TestTLSFallbackCachesHost(t *testing.T) {
	primary := &fakeRT{fn: func(*http.Request) (*http.Response, error) { return nil, io.EOF }}
	fallback := &fakeRT{fn: func(*http.Request) (*http.Response, error) { return okResponse(), nil }}
	ft := newFallbackFor(t, primary, fallback)

	if _, err := mustGet(t, ft, "https://host.example/a"); err != nil {
		t.Fatalf("first request: %v", err)
	}
	if _, err := mustGet(t, ft, "https://host.example/b"); err != nil {
		t.Fatalf("second request: %v", err)
	}
	// After caching, the primary (1.3) is not retried for that host.
	if primary.calls != 1 {
		t.Errorf("primary.calls=%d, want 1 (host cached after first downgrade)", primary.calls)
	}
	if fallback.calls != 2 {
		t.Errorf("fallback.calls=%d, want 2", fallback.calls)
	}
}

func TestTLSFallbackReturnsOriginalErrorWhenFallbackAlsoFails(t *testing.T) {
	primary := &fakeRT{fn: func(*http.Request) (*http.Response, error) { return nil, io.EOF }}
	fallbackErr := &net.DNSError{Err: "no such host", Name: "host.example"}
	fallback := &fakeRT{fn: func(*http.Request) (*http.Response, error) { return nil, fallbackErr }}
	ft := newFallbackFor(t, primary, fallback)

	_, err := mustGet(t, ft, "https://host.example/x")
	if !errors.Is(err, io.EOF) {
		t.Errorf("err = %v, want the original io.EOF (fallback did not help)", err)
	}
	if ft.hostForced12("host.example") {
		t.Error("host must not be cached when the fallback also fails")
	}
}

func TestTLSFallbackDoesNotDowngradeOnTimeout(t *testing.T) {
	primary := &fakeRT{fn: func(*http.Request) (*http.Response, error) { return nil, timeoutError{} }}
	fallback := &fakeRT{fn: func(*http.Request) (*http.Response, error) {
		t.Error("timeout must not trigger a TLS-version downgrade")
		return nil, errUnexpectedCall
	}}
	ft := newFallbackFor(t, primary, fallback)

	if _, err := mustGet(t, ft, "https://host.example/x"); err == nil {
		t.Fatal("expected the timeout error to be returned")
	}
	if fallback.calls != 0 {
		t.Errorf("fallback called %d times on timeout, want 0", fallback.calls)
	}
}

// TestTLSFallbackResendsWhenNoConnObtained is the safe handshake-refusal case: the
// primary never obtains a connection (GotConn does not fire), so nothing reached
// the server and resending the POST body over the 1.2 transport is safe. The
// fallback must see the full, rewound payload.
func TestTLSFallbackResendsWhenNoConnObtained(t *testing.T) {
	const payload = "the-request-body"
	primary := &fakeRT{fn: func(r *http.Request) (*http.Response, error) {
		_, _ = io.ReadAll(r.Body) // drain as the real transport would before failing
		return nil, io.EOF        // no GotConn: the 1.3 handshake was refused
	}}
	var seen string
	fallback := &fakeRT{fn: func(r *http.Request) (*http.Response, error) {
		b, _ := io.ReadAll(r.Body)
		seen = string(b)
		return okResponse(), nil
	}}
	ft := newFallbackFor(t, primary, fallback)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, "https://host.example/x", strings.NewReader(payload))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := ft.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip: %v", err)
	}
	if resp != nil && resp.Body != nil {
		if cerr := resp.Body.Close(); cerr != nil {
			t.Errorf("close body: %v", cerr)
		}
	}
	if fallback.calls != 1 {
		t.Errorf("fallback.calls=%d, want 1 (resend is safe after a handshake refusal)", fallback.calls)
	}
	if resp == nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("resp = %v, want status 200", resp)
	}
	if seen != payload {
		t.Errorf("fallback received body %q, want %q (body must be rewound for the resend)", seen, payload)
	}
}

// TestTLSFallbackDoesNotResendAfterConnObtained is the double-POST guard: the
// primary obtained a usable connection (GotConn fired) and the server may already
// have processed the body before the connection dropped, so the request must NOT be
// resent. The original error surfaces and the host is not downgraded.
func TestTLSFallbackDoesNotResendAfterConnObtained(t *testing.T) {
	const payload = "the-request-body"
	primary := &fakeRT{fn: func(r *http.Request) (*http.Response, error) {
		fireGotConn(r)            // a usable 1.3 connection was obtained
		_, _ = io.ReadAll(r.Body) // the server may already have processed the body
		return nil, io.EOF        // then the connection dropped after the response headers were expected
	}}
	fallback := &fakeRT{fn: func(*http.Request) (*http.Response, error) {
		t.Error("fallback must not run once a connection was obtained (would duplicate the POST)")
		return nil, errUnexpectedCall
	}}
	ft := newFallbackFor(t, primary, fallback)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, "https://host.example/x", strings.NewReader(payload))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := ft.RoundTrip(req)
	if resp != nil && resp.Body != nil {
		if cerr := resp.Body.Close(); cerr != nil {
			t.Errorf("close body: %v", cerr)
		}
	}
	if !errors.Is(err, io.EOF) {
		t.Errorf("err = %v, want the original io.EOF (no resend after a connection was obtained)", err)
	}
	if fallback.calls != 0 {
		t.Errorf("fallback.calls=%d, want 0 (a POST the server may have processed must not be resent)", fallback.calls)
	}
	if ft.hostForced12("host.example") {
		t.Error("host must not be cached when no downgrade occurred")
	}
}

// TestRewindForRetryUnretryableBody covers the safety path for a body that cannot
// be rewound: a request carrying a body but no GetBody (a NopCloser body) fails the
// 1.3 attempt without a connection, and the request must NOT be resent because the
// body cannot be replayed. The original error surfaces.
func TestRewindForRetryUnretryableBody(t *testing.T) {
	primary := &fakeRT{fn: func(r *http.Request) (*http.Response, error) {
		_, _ = io.ReadAll(r.Body) // drain as the real transport would before failing
		return nil, io.EOF        // no GotConn: a handshake refusal
	}}
	fallback := &fakeRT{fn: func(*http.Request) (*http.Response, error) {
		t.Error("fallback must not run for a body that cannot be rewound")
		return nil, errUnexpectedCall
	}}
	ft := newFallbackFor(t, primary, fallback)

	// A NopCloser body is not one of the types http.NewRequest can rewind, so
	// GetBody is left nil and the request cannot be safely resent.
	body := io.NopCloser(strings.NewReader("cannot-rewind"))
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, "https://host.example/x", body)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if req.GetBody != nil {
		t.Fatalf("test setup: GetBody should be nil for a NopCloser body")
	}
	resp, err := ft.RoundTrip(req)
	if resp != nil && resp.Body != nil {
		if cerr := resp.Body.Close(); cerr != nil {
			t.Errorf("close body: %v", cerr)
		}
	}
	if !errors.Is(err, io.EOF) {
		t.Errorf("err = %v, want the original io.EOF (unretryable body path)", err)
	}
	if fallback.calls != 0 {
		t.Errorf("fallback.calls=%d, want 0 (body cannot be rewound)", fallback.calls)
	}
}

func TestIsTLSHandshakeFailure(t *testing.T) {
	checks := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"eof", io.EOF, true},
		{"unexpected eof", io.ErrUnexpectedEOF, true},
		{"conn reset", syscall.ECONNRESET, true},
		{"timeout", timeoutError{}, false},
		{"context canceled", context.Canceled, false},
		{"deadline", context.DeadlineExceeded, false},
		{"op error wrapping eof", &net.OpError{Op: "read", Err: io.EOF}, true},
		{"op error non-sentinel", &net.OpError{Op: "dial", Err: stringError("refused")}, false},
		{"record header", tls.RecordHeaderError{Msg: "bad"}, true},
		{"plain error", stringError("nope"), false},
	}
	for _, c := range checks {
		if got := isTLSHandshakeFailure(c.err); got != c.want {
			t.Errorf("isTLSHandshakeFailure(%s) = %v, want %v", c.name, got, c.want)
		}
	}
}

type stringError string

func (e stringError) Error() string { return string(e) }

// TestNewClientUsesFallbackTransport verifies NewClient wires the fallback
// transport onto the client used for real requests (a baseURL override skips zone
// discovery so the constructor needs no network).
func TestNewClientUsesFallbackTransport(t *testing.T) {
	c, err := NewClient(t.Context(), AuthConfig{}, WithBaseURL("https://webservices2.autotask.net/ATServicesRest"))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(func() {
		if cerr := c.Close(); cerr != nil {
			t.Errorf("close client: %v", cerr)
		}
	})

	if _, ok := c.httpClient.Transport.(*tlsFallbackTransport); !ok {
		t.Errorf("client transport is %T, want *tlsFallbackTransport", c.httpClient.Transport)
	}
}
