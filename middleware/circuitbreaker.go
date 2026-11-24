package middleware

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/tphakala/go-autotask/internal/apierr"
)

type CircuitState string

const (
	StateClosed   CircuitState = "closed"
	StateOpen     CircuitState = "open"
	StateHalfOpen CircuitState = "half-open"

	defaultFailureThreshold = 5
	defaultFailureWindow    = 10 * time.Second
	defaultOpenTimeout      = 30 * time.Second
	defaultSuccessThreshold = 2

	// peekLimit is how much of a response body a failure predicate can read.
	peekLimit = 64 << 10
)

type CircuitBreakerOption func(*circuitBreakerConfig)

type circuitBreakerConfig struct {
	failureThreshold int
	failureWindow    time.Duration
	openTimeout      time.Duration
	successThreshold int
	isFailure        func(*http.Response, error) bool
	logger           *slog.Logger
}

// WithFailureThreshold sets the number of failures before opening. Values <= 0 are ignored.
func WithFailureThreshold(n int) CircuitBreakerOption {
	return func(c *circuitBreakerConfig) {
		if n > 0 {
			c.failureThreshold = n
		}
	}
}

// WithFailureWindow sets the sliding window for counting failures. Values <= 0 are ignored.
func WithFailureWindow(d time.Duration) CircuitBreakerOption {
	return func(c *circuitBreakerConfig) {
		if d > 0 {
			c.failureWindow = d
		}
	}
}

// WithOpenTimeout sets how long the circuit stays open before half-open. Values <= 0 are ignored.
func WithOpenTimeout(d time.Duration) CircuitBreakerOption {
	return func(c *circuitBreakerConfig) {
		if d > 0 {
			c.openTimeout = d
		}
	}
}

// WithSuccessThreshold sets successes needed in half-open to close. Values <= 0 are ignored.
func WithSuccessThreshold(n int) CircuitBreakerOption {
	return func(c *circuitBreakerConfig) {
		if n > 0 {
			c.successThreshold = n
		}
	}
}

// WithFailurePredicate sets the function that decides whether a round trip
// counts as a failure. Exactly one of resp and err is non-nil. A nil fn is
// ignored. The default is DefaultFailure.
//
// The predicate may read resp.Body. It sees at most the first 64 KiB, and the
// caller still receives the whole body. The read runs inside RoundTrip, so a
// body that stalls holds the response back until the request context or the
// client timeout ends it. A round trip that returns an error the
// predicate rejects is neither a failure nor, in the half-open state, a
// success.
func WithFailurePredicate(fn func(resp *http.Response, err error) bool) CircuitBreakerOption {
	return func(c *circuitBreakerConfig) {
		if fn != nil {
			c.isFailure = fn
		}
	}
}

// WithCircuitBreakerLogger sets the logger the circuit breaker writes to. A nil
// l is ignored. The default discards everything.
//
// The breaker logs one Warn record, "autotask: circuit breaker state changed",
// each time its state changes (closed to open, open to half-open, half-open to
// open, half-open to closed). The record has the attributes "from" and "to",
// the state names, and nothing from the request. Requests rejected while the
// breaker is open are not logged. Records are written after the breaker
// releases its lock, so the records of changes made by concurrent requests can
// appear in a different order than the changes.
func WithCircuitBreakerLogger(l *slog.Logger) CircuitBreakerOption {
	return func(c *circuitBreakerConfig) {
		if l != nil {
			c.logger = l
		}
	}
}

// StatusFailure counts a transport error, a 5xx status and 429 as failures.
// It was the default before WithFailurePredicate existed, and it counts a
// caller's context cancellation and a validation 500 as failures.
func StatusFailure(resp *http.Response, err error) bool {
	if err != nil {
		return true
	}
	return resp.StatusCode >= http.StatusInternalServerError || resp.StatusCode == http.StatusTooManyRequests
}

// DefaultFailure is the default failure predicate. It counts:
//   - a transport error, except the caller's context cancellation and a
//     *LimiterWaitError, which means the request gave up waiting in a limiter
//     and was never sent;
//   - status 429 and 5xx, except a 500 whose error message matches the
//     validation wording that ServerError.IsValidation in the autotask package
//     uses, which a retry does not fix and which says nothing about the health
//     of the service;
//   - a 2xx response whose Content-Type is present and not JSON, such as the
//     HTML page Autotask serves with HTTP 200 during planned maintenance. Only
//     the header is checked. text/plain is not counted, because a server that
//     sets no Content-Type gets that label from net/http, and a 204 or a
//     response that declares a Content-Length of 0 is not counted.
//
// A 500 body is read up to 64 KiB, so a caller that uses DefaultFailure
// outside the circuit breaker must restore resp.Body afterwards.
func DefaultFailure(resp *http.Response, err error) bool {
	if err != nil {
		if _, ok := errors.AsType[*LimiterWaitError](err); ok {
			return false
		}
		return !errors.Is(err, context.Canceled)
	}
	switch {
	case resp.StatusCode == http.StatusInternalServerError:
		return !isValidationBody(resp.Body)
	case resp.StatusCode >= http.StatusInternalServerError, resp.StatusCode == http.StatusTooManyRequests:
		return true
	case resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices:
		return isNonJSON(resp)
	default:
		return false
	}
}

func isValidationBody(body io.Reader) bool {
	if body == nil {
		return false
	}
	data, err := io.ReadAll(io.LimitReader(body, peekLimit))
	if err != nil {
		return false
	}
	entries := apierr.Extract(data)
	messages := make([]string, len(entries))
	for i, e := range entries {
		messages[i] = e.Message
	}
	return apierr.IsValidation(messages...)
}

func isNonJSON(resp *http.Response) bool {
	if resp.StatusCode == http.StatusNoContent || resp.ContentLength == 0 {
		return false
	}
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		return false
	}
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil && mediaType == "" {
		// A malformed parameter returns the media type with an error, but
		// conflicting duplicate parameters return nothing. Take the type
		// before the first semicolon in that case.
		mediaType, _, _ = strings.Cut(contentType, ";")
		mediaType = strings.ToLower(strings.TrimSpace(mediaType))
	}
	return mediaType != "text/plain" && mediaType != "application/json" && !strings.HasSuffix(mediaType, "+json")
}

type CircuitBreakerOpenError struct{}

func (e *CircuitBreakerOpenError) Error() string {
	return "autotask: circuit breaker is open"
}

type CircuitBreaker struct {
	next              http.RoundTripper
	config            circuitBreakerConfig
	mu                sync.RWMutex
	state             CircuitState
	failures          []time.Time
	lastStateChange   time.Time
	halfOpenSuccesses int
}

func NewCircuitBreaker(next http.RoundTripper, opts ...CircuitBreakerOption) *CircuitBreaker {
	cfg := circuitBreakerConfig{
		failureThreshold: defaultFailureThreshold, failureWindow: defaultFailureWindow,
		openTimeout: defaultOpenTimeout, successThreshold: defaultSuccessThreshold,
		isFailure: DefaultFailure,
		logger:    slog.New(slog.DiscardHandler),
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	return &CircuitBreaker{
		next: next, config: cfg, state: StateClosed, lastStateChange: time.Now(),
	}
}

// State returns the breaker's state, moving an open breaker whose open timeout
// has passed to half-open first.
func (cb *CircuitBreaker) State() CircuitState {
	return cb.currentState(context.Background())
}

// currentState is State with the context its log record is written under.
func (cb *CircuitBreaker) currentState(ctx context.Context) CircuitState {
	cb.mu.RLock()
	state := cb.state
	shouldTransition := state == StateOpen && time.Since(cb.lastStateChange) >= cb.config.openTimeout
	cb.mu.RUnlock()

	if shouldTransition {
		moved := false
		cb.mu.Lock()
		// Double-check under write lock
		if cb.state == StateOpen && time.Since(cb.lastStateChange) >= cb.config.openTimeout {
			cb.state = StateHalfOpen
			cb.lastStateChange = time.Now()
			moved = true
		}
		state = cb.state
		cb.mu.Unlock()
		if moved {
			cb.logTransition(ctx, StateOpen, StateHalfOpen)
		}
	}
	return state
}

// logTransition writes the state change record. It is called after cb.mu is
// released, so a slow log handler never holds up other requests, and only by
// the goroutine that made the change, so each change is logged once.
func (cb *CircuitBreaker) logTransition(ctx context.Context, from, to CircuitState) {
	if from == to {
		return
	}
	cb.config.logger.LogAttrs(ctx, slog.LevelWarn, "autotask: circuit breaker state changed",
		slog.String("from", string(from)), slog.String("to", string(to)))
}

func (cb *CircuitBreaker) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx := req.Context()
	state := cb.currentState(ctx)
	switch state {
	case StateOpen:
		return nil, &CircuitBreakerOpenError{}
	case StateClosed, StateHalfOpen:
		// Circuit is closed or half-open; proceed with request
	}
	resp, err := cb.next.RoundTrip(req)
	if err != nil {
		if cb.config.isFailure(nil, err) {
			cb.recordFailure(ctx, state)
		}
		return nil, err
	}
	if cb.responseFailed(resp) {
		cb.recordFailure(ctx, state)
	} else if state == StateHalfOpen {
		cb.recordHalfOpenSuccess(ctx)
	}
	return resp, nil
}

// responseFailed asks the failure predicate about resp. The predicate reads a
// copy of the first peekLimit bytes of the body, and resp.Body is then rebuilt
// from what it read, a read error it met if any, and the rest of the stream.
// Closing the body the predicate sees does not close resp.Body.
func (cb *CircuitBreaker) responseFailed(resp *http.Response) bool {
	if resp.Body == nil {
		return cb.config.isFailure(resp, nil)
	}
	tap := &probeBody{src: io.LimitReader(resp.Body, peekLimit)}
	probe := *resp
	probe.Body = tap
	failed := cb.config.isFailure(&probe, nil)
	resp.Body = &replayBody{tap: tap, body: resp.Body}
	return failed
}

// probeBody is the body a failure predicate reads. It keeps the bytes it
// passes on, and a read error other than EOF, for replayBody.
type probeBody struct {
	src  io.Reader
	seen bytes.Buffer
	err  error
}

func (p *probeBody) Read(b []byte) (int, error) {
	n, err := p.src.Read(b)
	p.seen.Write(b[:n])
	if err != nil && !errors.Is(err, io.EOF) {
		p.err = err
	}
	return n, err
}

func (p *probeBody) Close() error { return nil }

// replayBody gives the caller what the predicate read. If the predicate met a
// read error, every later read returns it; otherwise the rest of the original
// body follows.
type replayBody struct {
	tap  *probeBody
	body io.ReadCloser
}

func (b *replayBody) Read(p []byte) (int, error) {
	if b.tap.seen.Len() > 0 {
		return b.tap.seen.Read(p)
	}
	if b.tap.err != nil {
		return 0, b.tap.err
	}
	return b.body.Read(p)
}

func (b *replayBody) Close() error { return b.body.Close() }

func (cb *CircuitBreaker) recordFailure(ctx context.Context, admitted CircuitState) {
	from, to := cb.applyFailure(admitted)
	cb.logTransition(ctx, from, to)
}

// applyFailure records a failure of a request that was admitted in state
// admitted, and returns the state before and after it. An in-flight request can
// fail after another one opened the breaker. That failure is ignored: it is not
// counted and it leaves the open deadline alone, so the breaker stays open for
// openTimeout after the failure that opened it. A failure in half-open is
// ignored the same way unless the request was admitted in half-open: only such
// a request is a probe.
func (cb *CircuitBreaker) applyFailure(admitted CircuitState) (from, to CircuitState) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	from = cb.state
	if from == StateOpen || (from == StateHalfOpen && admitted != StateHalfOpen) {
		return from, from
	}
	now := time.Now()
	cutoff := now.Add(-cb.config.failureWindow)
	var recent []time.Time
	for _, t := range cb.failures {
		if t.After(cutoff) {
			recent = append(recent, t)
		}
	}
	recent = append(recent, now)
	cb.failures = recent
	if cb.state == StateHalfOpen {
		cb.state = StateOpen
		cb.lastStateChange = now
		cb.halfOpenSuccesses = 0
		return from, cb.state
	}
	if len(recent) >= cb.config.failureThreshold {
		cb.state = StateOpen
		cb.lastStateChange = now
		cb.failures = nil
	}
	return from, cb.state
}

func (cb *CircuitBreaker) recordHalfOpenSuccess(ctx context.Context) {
	from, to := cb.applyHalfOpenSuccess()
	cb.logTransition(ctx, from, to)
}

func (cb *CircuitBreaker) applyHalfOpenSuccess() (from, to CircuitState) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	from = cb.state
	if cb.state != StateHalfOpen {
		return from, from
	}
	cb.halfOpenSuccesses++
	if cb.halfOpenSuccesses >= cb.config.successThreshold {
		cb.state = StateClosed
		cb.lastStateChange = time.Now()
		cb.halfOpenSuccesses = 0
		cb.failures = nil
	}
	return from, cb.state
}
