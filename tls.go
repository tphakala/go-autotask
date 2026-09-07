package autotask

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptrace"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// forced12TTL bounds how long a host is pinned to TLS 1.2 after a successful
// downgrade before the next request re-probes TLS 1.3.
const forced12TTL = time.Hour

// tlsFallbackTransport prefers TLS 1.3 but transparently downgrades to TLS 1.2 for
// hosts that refuse a TLS 1.3 handshake.
//
// Autotask's edge infrastructure does not complete a TLS 1.3 handshake: a TLS 1.3
// ClientHello (which Go offers by default from Go 1.24 on, carrying the larger
// X25519MLKEM768 post-quantum key share) is dropped or reset before any
// ServerHello, surfacing to callers as "EOF" or "connection reset by peer".
// Autotask serves TLS 1.2 (ECDHE + AES-GCM) cleanly. Verified against
// webservices2.autotask.net: TLS 1.3 (with or without the post-quantum curve) is
// refused, TLS 1.2 returns 200.
//
// Rather than pin every deployment to 1.2, the primary (1.3-capable) transport is
// tried first. The request is resent over a 1.2-capped transport ONLY when the
// primary obtained no usable connection (the httptrace GotConn hook never fired,
// meaning the 1.3 handshake or dial was refused and no request bytes were sent).
// Because nothing reached the server, the resend is safe even for a non-idempotent
// request (the body, if any, is restored from Request.GetBody). If the primary did
// obtain a connection, the failure is not a version problem and the request is not
// resent, so a POST or PATCH the server may already have processed is never
// duplicated. When the 1.2 attempt succeeds where 1.3 failed, the host is
// remembered so later requests go straight to 1.2 and do not pay a failed 1.3
// handshake each time. A host that genuinely supports 1.3 keeps using it.
//
// The downgrade is remembered only for forced12TTL. After it lapses the next
// request re-probes TLS 1.3, so a host pinned to 1.2 by a transient handshake
// blip self-heals once the blip clears, while a host that keeps refusing 1.3 is
// re-pinned on that probe. A pinned host otherwise pays no failed 1.3 handshake;
// only when the TTL lapses does a request re-probe, and requests arriving
// concurrently in that window can each re-probe once before the host is
// re-pinned.
type tlsFallbackTransport struct {
	primary  http.RoundTripper // TLS 1.3 capable
	fallback http.RoundTripper // TLS 1.2 capped

	// now returns the current time; nil defaults to time.Now. Overridable in
	// tests to exercise the forced12 TTL without sleeping.
	now func() time.Time
	// ttl is how long a host stays pinned to TLS 1.2 after a downgrade before a
	// request re-probes TLS 1.3. Zero defaults to forced12TTL.
	ttl time.Duration

	mu       sync.RWMutex
	forced12 map[string]time.Time // host -> time its TLS 1.2 downgrade was recorded
}

func newTLSFallbackTransport() *tlsFallbackTransport {
	return &tlsFallbackTransport{
		primary:  &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12}},
		fallback: &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, MaxVersion: tls.VersionTLS12}},
		now:      time.Now,
		ttl:      forced12TTL,
		forced12: make(map[string]time.Time),
	}
}

// clock returns the current time, honouring an injected now for tests.
func (t *tlsFallbackTransport) clock() time.Time {
	if t.now != nil {
		return t.now()
	}
	return time.Now()
}

// forcedTTL is the pin lifetime, defaulting to forced12TTL when unset.
func (t *tlsFallbackTransport) forcedTTL() time.Duration {
	if t.ttl > 0 {
		return t.ttl
	}
	return forced12TTL
}

func (t *tlsFallbackTransport) hostForced12(host string) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	downgradedAt, ok := t.forced12[host]
	if !ok {
		return false
	}
	// An entry older than the TTL is ignored so the next request re-probes TLS
	// 1.3 and a transient downgrade self-heals. The stale entry is harmless: it
	// is overwritten with a fresh timestamp if the downgrade recurs.
	return t.clock().Sub(downgradedAt) < t.forcedTTL()
}

func (t *tlsFallbackTransport) markHostForced12(host string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.forced12[host] = t.clock()
}

func (t *tlsFallbackTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.hostForced12(req.URL.Host) {
		return t.fallback.RoundTrip(req)
	}
	// Record whether the primary ever obtained a usable connection. GotConn fires
	// only after a connection (for a new conn, after the TLS handshake) is ready to
	// carry the request, so if it never fires the 1.3 connection could not be
	// established and no request bytes were sent. Resending is then safe for any
	// method. If it did fire, the 1.3 connection worked and the failure is not a
	// version problem, so we must not resend (a non-idempotent request the server
	// already processed would be duplicated).
	var gotConn atomic.Bool
	probe := req.WithContext(httptrace.WithClientTrace(req.Context(), &httptrace.ClientTrace{
		GotConn: func(httptrace.GotConnInfo) { gotConn.Store(true) },
	}))
	resp, err := t.primary.RoundTrip(probe)
	if err == nil || gotConn.Load() || !isTLSHandshakeFailure(err) {
		return resp, err
	}
	retryReq, ok := rewindForRetry(req)
	if !ok {
		return resp, err
	}
	fResp, fErr := t.fallback.RoundTrip(retryReq)
	if fErr != nil {
		// The 1.2 attempt did not help; surface the original error and do not
		// mark the host.
		return resp, err
	}
	t.markHostForced12(req.URL.Host)
	return fResp, nil
}

// rewindForRetry returns a copy of req whose body is reset to the start so it can
// be sent a second time. A request that carries a body but no GetBody cannot be
// rewound and is reported unretryable (ok=false); a bodyless request is always
// retryable.
func rewindForRetry(req *http.Request) (*http.Request, bool) {
	retry := req.Clone(req.Context())
	if req.Body == nil || req.Body == http.NoBody {
		return retry, true
	}
	if req.GetBody == nil {
		return nil, false
	}
	body, err := req.GetBody()
	if err != nil {
		return nil, false
	}
	retry.Body = body
	return retry, true
}

// isTLSHandshakeFailure reports whether err is a connection-level failure
// consistent with a peer that refused the connection during setup, of the kind
// Autotask returns when it refuses a TLS 1.3 handshake (verified: bare io.EOF and
// ECONNRESET while reading the ServerHello). It is used together with the GotConn
// guard in RoundTrip, which is what actually makes the resend safe. Timeouts and
// context cancellation are deliberately excluded, they are not version-negotiation
// failures and must not trigger a silent downgrade.
func isTLSHandshakeFailure(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	if netErr, ok := errors.AsType[net.Error](err); ok && netErr.Timeout() {
		return false
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, syscall.ECONNRESET) {
		return true
	}
	_, ok := errors.AsType[tls.RecordHeaderError](err)
	return ok
}
