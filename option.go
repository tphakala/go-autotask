package autotask

import (
	"log/slog"
	"net/http"

	"github.com/tphakala/go-autotask/middleware"
)

type ClientOption func(*Client)

func WithHTTPClient(hc *http.Client) ClientOption {
	return func(c *Client) { c.httpClient = hc }
}

func WithLogger(l *slog.Logger) ClientOption {
	return func(c *Client) { c.logger = l }
}

func WithBaseURL(url string) ClientOption {
	return func(c *Client) { c.baseURL = url }
}

func WithUserAgent(ua string) ClientOption {
	return func(c *Client) { c.userAgent = ua }
}

func WithImpersonation(resourceID int64) ClientOption {
	return func(c *Client) { c.impersonationID = resourceID }
}

func WithMiddleware(m Middleware) ClientOption {
	return func(c *Client) {
		c.middlewares = append(c.middlewares, m)
	}
}

// WithRateLimiter enables rate limiting middleware.
func WithRateLimiter(opts ...middleware.RateLimitOption) ClientOption {
	return func(c *Client) {
		c.middlewares = append(c.middlewares, func(next http.RoundTripper) http.RoundTripper {
			return middleware.NewRateLimiter(next, opts...)
		})
	}
}

// WithCircuitBreaker enables circuit breaker middleware.
func WithCircuitBreaker(opts ...middleware.CircuitBreakerOption) ClientOption {
	return func(c *Client) {
		c.middlewares = append(c.middlewares, func(next http.RoundTripper) http.RoundTripper {
			return middleware.NewCircuitBreaker(next, opts...)
		})
	}
}

// WithThresholdMonitor enables background API usage monitoring.
func WithThresholdMonitor(opts ...middleware.ThresholdMonitorOption) ClientOption {
	return func(c *Client) {
		c.thresholdMonitorOpts = opts
	}
}

// WithMaxConcurrency limits the number of concurrent in-flight API requests.
// Autotask enforces a per-integration-code thread limit (default 3).
// If n <= 0, the default of 3 is used.
//
// Middleware ordering matters: options are applied in the order specified, with
// the last middleware wrapping closest to the transport. For best performance,
// place fast-fail middleware (circuit breaker) first and resource-consuming
// middleware (concurrency, rate limiter) last:
//
//	autotask.NewClient(ctx, auth,
//	    autotask.WithCircuitBreaker(),   // fail fast if circuit open
//	    autotask.WithRateLimiter(),      // then enforce rate limit
//	    autotask.WithMaxConcurrency(3),  // then limit concurrency
//	)
func WithMaxConcurrency(n int) ClientOption {
	return func(c *Client) {
		c.middlewares = append(c.middlewares, func(next http.RoundTripper) http.RoundTripper {
			return middleware.NewConcurrencyLimiter(next, n)
		})
	}
}

// WithZoneBaseURL overrides the base URL used for zone discovery.
// This is primarily useful for testing.
func WithZoneBaseURL(url string) ClientOption {
	return func(c *Client) { c.zoneBaseURL = url }
}
