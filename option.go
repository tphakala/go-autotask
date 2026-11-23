package autotask

import (
	"log/slog"
	"net/http"

	"github.com/tphakala/go-autotask/middleware"
)

type ClientOption func(*Client)

// WithHTTPClient sets the HTTP client used for all requests. A nil hc keeps the
// default client. NewClient uses a copy of hc, so setting hc's fields after
// NewClient returns has no effect.
// The copy's CheckRedirect removes the Autotask credential headers from any
// redirect to another scheme or host, then calls hc.CheckRedirect if it is set.
func WithHTTPClient(hc *http.Client) ClientOption {
	return func(c *Client) { c.httpClient = hc }
}

// WithMaxResponseBytes sets the largest API or zone discovery response body
// the client reads, in bytes. A longer body returns a *ResponseTooLargeError;
// for an API response with a non-2xx status, the status-typed error is
// returned with the size error beside it. The default is 128 MiB. Values <= 0
// are ignored.
func WithMaxResponseBytes(n int64) ClientOption {
	return func(c *Client) {
		if n > 0 {
			c.maxResponseBytes = n
		}
	}
}

// WithLogger sets the logger the client writes to. A nil l is ignored. The
// default discards everything.
//
// The client logs these events, every message starting with "autotask: ":
//   - Warn: the circuit breaker changed state (closed, open, half-open).
//   - Warn: the server answered 429 with a Retry-After header.
//   - Info: a request waited for that Retry-After period.
//   - Debug: a request waited for a rate limit token, or for the adaptive delay.
//   - Warn: API usage reached the warning (75%) or critical (90%) level, on
//     every check at or above it, and a threshold check failed.
//   - Info: zone discovery found the zone. Warn: it failed.
//   - Warn: a host refused TLS 1.3 and is now pinned to TLS 1.2, once per pin.
//
// The logger reaches the rate limiter, circuit breaker and threshold monitor
// that WithRateLimiter, WithCircuitBreaker and WithThresholdMonitor build,
// wherever WithLogger is listed among the options. A logger passed to
// middleware.WithRateLimitLogger, middleware.WithCircuitBreakerLogger or
// middleware.WithThresholdMonitorLogger in those options replaces it for that
// component. Middleware added with WithMiddleware or WithMaxConcurrency is not
// logged. The TLS records need the default HTTP client, not one set with
// WithHTTPClient.
//
// A record carries method, URL path, host, counts, durations, state
// names and error values. It never carries credentials, request or
// response bodies, or a URL query string. Error values are logged as returned
// to the caller, so two cases are not redacted: a Location header that does not
// parse, which net/http copies into the error of a redirect, and the list of
// API versions a zone discovery response advertised.
func WithLogger(l *slog.Logger) ClientOption {
	return func(c *Client) {
		if l != nil {
			c.logger = l
		}
	}
}

// WithBaseURL sets a fixed API base URL and skips zone discovery, so
// Client.Zone reports no zone. An empty url leaves discovery on. A non-empty
// url cannot be combined with WithZoneInfo.
func WithBaseURL(url string) ClientOption {
	return func(c *Client) { c.baseURL = url }
}

// WithZoneInfo makes NewClient use a zone that is already known, such as one
// saved from Client.Zone on an earlier run, instead of discovering it.
// NewClient uses z.URL as the API base URL and sends no zone discovery
// request, and Client.Zone returns z unchanged. The client uses only z.URL;
// the other fields are kept for the caller.
//
// NewClient returns an error when z.URL is empty or is not an absolute http or
// https URL with a host and no "@", query or fragment, and when
// WithZoneInfo is combined with a non-empty WithBaseURL. WithZoneBaseURL has
// no effect together with WithZoneInfo.
//
// The client sends the API credentials to z.URL, so load z only from storage
// you trust as much as the credentials. Zone discovery is per username, so
// store one zone per API user.
func WithZoneInfo(z ZoneInfo) ClientOption {
	return func(c *Client) { c.zone, c.hasZone = z, true }
}

// WithUserAgent sets the User-Agent header. The default is go-autotask/<module
// version>, taken from the build info, or plain go-autotask when the build info
// has no version for this module (a local replace directive, a workspace, the
// module's own tests).
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
			return middleware.NewRateLimiter(next, withLogger(c.logger, middleware.WithRateLimitLogger, opts)...)
		})
	}
}

// WithCircuitBreaker enables circuit breaker middleware.
func WithCircuitBreaker(opts ...middleware.CircuitBreakerOption) ClientOption {
	return func(c *Client) {
		c.middlewares = append(c.middlewares, func(next http.RoundTripper) http.RoundTripper {
			return middleware.NewCircuitBreaker(next, withLogger(c.logger, middleware.WithCircuitBreakerLogger, opts)...)
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
//
// With the default failure predicate, a request that gives up waiting for a
// slot or a rate token is not a circuit breaker failure (see
// middleware.LimiterWaitError).
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

// withLogger returns opts with the option that sets the client's logger in
// front, so an option the caller passed for the same component comes later and
// wins. It reads the logger when the middleware is built, after every client
// option has run.
func withLogger[O any](l *slog.Logger, set func(*slog.Logger) O, opts []O) []O {
	return append([]O{set(l)}, opts...)
}
