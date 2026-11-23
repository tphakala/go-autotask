package middleware

import "errors"

// The Limiter values of a LimiterWaitError.
const (
	limiterConcurrency = "concurrency"
	limiterRate        = "rate"
)

// LimiterWaitError reports that a request gave up waiting in ConcurrencyLimiter
// or RateLimiter before it was sent, because its context ended or its deadline
// was too close for the next rate token. Limiter is "concurrency" or "rate".
//
// The request never reached the network, so DefaultFailure does not count it as
// a circuit breaker failure. A custom failure predicate can recognise it with
// errors.AsType[*LimiterWaitError].
//
// Unwrap returns Err, so errors.Is still matches context.Canceled and
// context.DeadlineExceeded and IsTransient answers as it does for Err alone,
// and Timeout reports what Err reports, so (*url.Error).Timeout does too. When
// the rate limiter refuses a request because its deadline is too close for the
// next token, Err is the plain error golang.org/x/time/rate returns for that
// (rate/rate.go:276 at v0.16.0), which wraps no context error.
type LimiterWaitError struct {
	Limiter string
	Err     error
}

// Error returns the limiter name followed by the wrapped error's text.
func (e *LimiterWaitError) Error() string {
	return "autotask: " + e.Limiter + " limiter: " + e.Err.Error()
}

// Unwrap returns the error that ended the wait.
func (e *LimiterWaitError) Unwrap() error { return e.Err }

// Timeout reports whether the error that ended the wait is a timeout.
func (e *LimiterWaitError) Timeout() bool {
	t, ok := errors.AsType[interface {
		error
		Timeout() bool
	}](e.Err)
	return ok && t.Timeout()
}
