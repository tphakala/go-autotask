package middleware

import "net/http"

const defaultMaxConcurrency = 3

// ConcurrencyLimiter is an http.RoundTripper that limits the number of
// concurrent in-flight requests using a semaphore. Autotask enforces a
// per-integration-code thread limit (default 3); this middleware prevents
// the client from exceeding it.
type ConcurrencyLimiter struct {
	next http.RoundTripper
	sem  chan struct{}
}

// NewConcurrencyLimiter wraps next with a concurrency limit of n.
// If n <= 0, defaultMaxConcurrency (3) is used.
func NewConcurrencyLimiter(next http.RoundTripper, n int) *ConcurrencyLimiter {
	if n <= 0 {
		n = defaultMaxConcurrency
	}
	return &ConcurrencyLimiter{
		next: next,
		sem:  make(chan struct{}, n),
	}
}

// RoundTrip acquires a concurrency slot, executes the request, and releases
// the slot. A request whose context has ended, before or while it waits for a
// slot, is not sent and gets a *LimiterWaitError wrapping the context's error.
func (cl *ConcurrencyLimiter) RoundTrip(req *http.Request) (*http.Response, error) {
	// A select over a free slot and a done context picks either at random, so
	// check the context first: a request that has already ended is never sent.
	if err := req.Context().Err(); err != nil {
		return nil, &LimiterWaitError{Limiter: limiterConcurrency, Err: err}
	}
	select {
	case cl.sem <- struct{}{}:
	case <-req.Context().Done():
		return nil, &LimiterWaitError{Limiter: limiterConcurrency, Err: req.Context().Err()}
	}
	defer func() { <-cl.sem }()
	return cl.next.RoundTrip(req)
}
