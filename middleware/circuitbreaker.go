package middleware

import (
	"net/http"
	"sync"
	"time"
)

type CircuitState string

const (
	StateClosed   CircuitState = "closed"
	StateOpen     CircuitState = "open"
	StateHalfOpen CircuitState = "half-open"
)

type CircuitBreakerOption func(*circuitBreakerConfig)

type circuitBreakerConfig struct {
	failureThreshold int
	failureWindow    time.Duration
	openTimeout      time.Duration
	successThreshold int
}

func WithFailureThreshold(n int) CircuitBreakerOption {
	return func(c *circuitBreakerConfig) { c.failureThreshold = n }
}

func WithFailureWindow(d time.Duration) CircuitBreakerOption {
	return func(c *circuitBreakerConfig) { c.failureWindow = d }
}

func WithOpenTimeout(d time.Duration) CircuitBreakerOption {
	return func(c *circuitBreakerConfig) { c.openTimeout = d }
}

func WithSuccessThreshold(n int) CircuitBreakerOption {
	return func(c *circuitBreakerConfig) { c.successThreshold = n }
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
		failureThreshold: 5, failureWindow: 10 * time.Second,
		openTimeout: 30 * time.Second, successThreshold: 2,
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
	}
	resp, err := cb.next.RoundTrip(req)
	if err != nil {
		cb.recordFailure()
		return nil, err
	}
	if cb.isFailure(resp) {
		cb.recordFailure()
	} else if state == StateHalfOpen {
		cb.recordHalfOpenSuccess()
	}
	return resp, nil
}

func (cb *CircuitBreaker) isFailure(resp *http.Response) bool {
	return resp.StatusCode >= 500 || resp.StatusCode == 429
}

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
	cb.halfOpenSuccesses++
	if cb.halfOpenSuccesses >= cb.config.successThreshold {
		cb.state = StateClosed
		cb.lastStateChange = time.Now()
		cb.halfOpenSuccesses = 0
		cb.failures = nil
	}
}
