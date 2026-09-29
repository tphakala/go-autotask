package middleware

import (
	"bytes"
	"context"
	"errors"
	"io"
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
// caller still receives the whole body. A round trip that returns an error the
// predicate rejects is neither a failure nor, in the half-open state, a
// success.
func WithFailurePredicate(fn func(resp *http.Response, err error) bool) CircuitBreakerOption {
	return func(c *circuitBreakerConfig) {
		if fn != nil {
			c.isFailure = fn
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
//   - a transport error, except the caller's context cancellation;
//   - status 429 and 5xx, except a 500 that reports a validation failure (for
//     example a field value over its maximum length), which a retry does not
//     fix and which says nothing about the health of the service;
//   - a 2xx response whose Content-Type is present and not JSON, such as the
//     HTML page Autotask serves with HTTP 200 during planned maintenance. Only
//     the header is checked. text/plain is not counted, because a server that
//     sets no Content-Type gets that label from net/http, and a 204 or an
//     empty body is not counted.
//
// A 500 body is read up to 64 KiB, so a caller that uses DefaultFailure
// outside the circuit breaker must restore resp.Body afterwards.
func DefaultFailure(resp *http.Response, err error) bool {
	if err != nil {
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
	if err != nil {
		return true
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
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	return &CircuitBreaker{
		next: next, config: cfg, state: StateClosed, lastStateChange: time.Now(),
	}
}

func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.RLock()
	state := cb.state
	shouldTransition := state == StateOpen && time.Since(cb.lastStateChange) >= cb.config.openTimeout
	cb.mu.RUnlock()

	if shouldTransition {
		cb.mu.Lock()
		// Double-check under write lock
		if cb.state == StateOpen && time.Since(cb.lastStateChange) >= cb.config.openTimeout {
			cb.state = StateHalfOpen
			cb.lastStateChange = time.Now()
		}
		state = cb.state
		cb.mu.Unlock()
	}
	return state
}

func (cb *CircuitBreaker) RoundTrip(req *http.Request) (*http.Response, error) {
	state := cb.State()
	switch state {
	case StateOpen:
		return nil, &CircuitBreakerOpenError{}
	case StateClosed, StateHalfOpen:
		// Circuit is closed or half-open; proceed with request
	}
	resp, err := cb.next.RoundTrip(req)
	if err != nil {
		if cb.config.isFailure(nil, err) {
			cb.recordFailure()
		}
		return nil, err
	}
	if cb.responseFailed(resp) {
		cb.recordFailure()
	} else if state == StateHalfOpen {
		cb.recordHalfOpenSuccess()
	}
	return resp, nil
}

// responseFailed asks the failure predicate about resp. The predicate reads a
// copy of the first peekLimit bytes of the body, and resp.Body is then rebuilt
// from what it read and the rest of the stream.
func (cb *CircuitBreaker) responseFailed(resp *http.Response) bool {
	if resp.Body == nil {
		return cb.config.isFailure(resp, nil)
	}
	var seen bytes.Buffer
	probe := *resp
	probe.Body = io.NopCloser(io.TeeReader(io.LimitReader(resp.Body, peekLimit), &seen))
	failed := cb.config.isFailure(&probe, nil)
	resp.Body = &replayBody{Reader: io.MultiReader(&seen, resp.Body), closer: resp.Body}
	return failed
}

type replayBody struct {
	io.Reader
	closer io.Closer
}

func (b *replayBody) Close() error { return b.closer.Close() }

func (cb *CircuitBreaker) recordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
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
		return
	}
	if len(recent) >= cb.config.failureThreshold {
		cb.state = StateOpen
		cb.lastStateChange = now
		cb.failures = nil
	}
}

func (cb *CircuitBreaker) recordHalfOpenSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	if cb.state != StateHalfOpen {
		return
	}
	cb.halfOpenSuccesses++
	if cb.halfOpenSuccesses >= cb.config.successThreshold {
		cb.state = StateClosed
		cb.lastStateChange = time.Now()
		cb.halfOpenSuccesses = 0
		cb.failures = nil
	}
}
