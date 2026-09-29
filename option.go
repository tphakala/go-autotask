package autotask

import (
	"log/slog"
	"net/http"

	"github.com/tphakala/go-autotask/middleware"
)

type ClientOption func(*Client)

// WithHTTPClient sets the HTTP client used for all requests. NewClient uses a
// copy of hc, so setting hc's fields after NewClient returns has no effect.
// The copy's CheckRedirect removes the Autotask credential headers from any
// redirect to another scheme or host, then calls hc.CheckRedirect if it is set.
func WithHTTPClient(hc *http.Client) ClientOption {
	return func(c *Client) { c.httpClient = hc }
}

// WithMaxResponseBytes sets the largest response body the client reads, in
// bytes, including zone discovery responses. A longer body returns a
// *ResponseTooLargeError (for a non-2xx status, the status-typed error with
// the size error beside it). The default is 128 MiB. Values <= 0 are ignored.
func WithMaxResponseBytes(n int64) ClientOption {
	return func(c *Client) {
		if n > 0 {
			c.maxResponseBytes = n
		}
	}
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
