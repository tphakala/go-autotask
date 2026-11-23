package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

const (
	defaultRequestsPerHour = 5000
	defaultBurstSize       = 20
	secondsPerHour         = 3600.0
	usageWarnThreshold     = 0.75
	usageLowThreshold      = 0.50
	rateLimitBackoffShort  = 500 * time.Millisecond
	rateLimitBackoffLong   = 60 * time.Second
)

// RateLimitOption configures a RateLimiter.
type RateLimitOption func(*rateLimitConfig)

type rateLimitConfig struct {
	requestsPerHour int
	burstSize       int
	adaptiveDelay   bool
	logger          *slog.Logger
}

// WithRequestsPerHour sets the sustained request rate. Values <= 0 are ignored.
func WithRequestsPerHour(n int) RateLimitOption {
	return func(c *rateLimitConfig) {
		if n > 0 {
			c.requestsPerHour = n
		}
	}
}

// WithBurstSize sets the maximum burst above the sustained rate. Values <= 0 are ignored.
func WithBurstSize(n int) RateLimitOption {
	return func(c *rateLimitConfig) {
		if n > 0 {
			c.burstSize = n
		}
	}
}

// WithAdaptiveDelay enables or disables adaptive delay based on window usage.
func WithAdaptiveDelay(enabled bool) RateLimitOption {
	return func(c *rateLimitConfig) { c.adaptiveDelay = enabled }
}

// WithRateLimitLogger sets the logger the rate limiter writes to. A nil l is
// ignored. The default discards everything.
//
// The limiter logs a Warn record when a 429 response sets a Retry-After
// deadline ("autotask: server requested backoff", attribute "retry_after"), an
// Info record when a request waits for that deadline ("autotask: waiting for
// server backoff", attribute "wait"), and Debug records when a request waits
// for a rate token ("autotask: waiting for rate limit token", attribute
// "waited") or for the adaptive delay ("autotask: adaptive rate limit delay",
// attribute "delay"). Every record also has "method" and the URL "path"; the
// query string and headers are never logged. The token wait is detected
// before it starts, so a Debug record can be missed or written for a wait that
// is close to zero when another request takes or refills a token meanwhile.
func WithRateLimitLogger(l *slog.Logger) RateLimitOption {
	return func(c *rateLimitConfig) {
		if l != nil {
			c.logger = l
		}
	}
}

// RateLimiter is an http.RoundTripper that enforces rate limits using a token
// bucket algorithm with optional adaptive delays. It also respects 429
// Retry-After headers from the server. It logs its waits to the logger set with
// WithRateLimitLogger.
type RateLimiter struct {
	next    http.RoundTripper
	limiter *rate.Limiter
	config  rateLimitConfig

	mu               sync.Mutex
	windowStart      time.Time
	requestsInWindow int
	retryAfterUntil  time.Time
}

// NewRateLimiter wraps next with rate-limiting behaviour.
// Default: 5000 requests/hour, burst of 20, adaptive delay enabled, nothing
// logged (see WithRateLimitLogger).
func NewRateLimiter(next http.RoundTripper, opts ...RateLimitOption) *RateLimiter {
	cfg := rateLimitConfig{
		requestsPerHour: defaultRequestsPerHour,
		burstSize:       defaultBurstSize,
		adaptiveDelay:   true,
		logger:          slog.New(slog.DiscardHandler),
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	rps := rate.Limit(float64(cfg.requestsPerHour) / secondsPerHour)
	return &RateLimiter{
		next:        next,
		limiter:     rate.NewLimiter(rps, cfg.burstSize),
		config:      cfg,
		windowStart: time.Now(),
	}
}

// RoundTrip implements http.RoundTripper with rate limiting. A request that
// gives up waiting for a Retry-After period, a token or the adaptive delay is
// not sent, and the error is a *LimiterWaitError.
func (rl *RateLimiter) RoundTrip(req *http.Request) (*http.Response, error) {
	if err := rl.waitForServerBackoff(req); err != nil {
		return nil, err
	}
	if err := rl.waitForToken(req); err != nil {
		return nil, err
	}
	if rl.config.adaptiveDelay {
		if err := rl.waitAdaptive(req); err != nil {
			return nil, err
		}
	}

	rl.recordRequest()

	resp, err := rl.next.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	rl.noteServerBackoff(req, resp)
	return resp, nil
}

// waitForServerBackoff honours any active Retry-After wait. It returns a
// *LimiterWaitError when the request's context ends first.
func (rl *RateLimiter) waitForServerBackoff(req *http.Request) error {
	rl.mu.Lock()
	retryUntil := rl.retryAfterUntil
	rl.mu.Unlock()

	wait := time.Until(retryUntil)
	if wait <= 0 {
		return nil
	}
	rl.logWait(req, slog.LevelInfo, "autotask: waiting for server backoff", "wait", wait)
	if err := sleepCtx(req.Context(), wait); err != nil {
		return &LimiterWaitError{Limiter: limiterRate, Err: err}
	}
	return nil
}

// waitForToken waits for a token from the bucket. It returns a
// *LimiterWaitError when the request's context ends or its deadline is too
// close for the next token.
func (rl *RateLimiter) waitForToken(req *http.Request) error {
	ctx := req.Context()
	// Tokens is read only to tell whether Wait will block, so the record costs
	// nothing when Debug is off. Another request can change the answer before
	// Wait runs, which only decides whether the record is written.
	willWait := rl.config.logger.Enabled(ctx, slog.LevelDebug) && rl.limiter.Tokens() < 1
	start := time.Now()
	if err := rl.limiter.Wait(ctx); err != nil {
		return &LimiterWaitError{Limiter: limiterRate, Err: err}
	}
	if willWait {
		rl.logWait(req, slog.LevelDebug, "autotask: waiting for rate limit token", "waited", time.Since(start))
	}
	return nil
}

// waitAdaptive applies the extra delay that high hourly window usage adds. It
// returns a *LimiterWaitError when the request's context ends first.
func (rl *RateLimiter) waitAdaptive(req *http.Request) error {
	delay := rl.adaptiveDelay()
	if delay <= 0 {
		return nil
	}
	rl.logWait(req, slog.LevelDebug, "autotask: adaptive rate limit delay", "delay", delay)
	if err := sleepCtx(req.Context(), delay); err != nil {
		return &LimiterWaitError{Limiter: limiterRate, Err: err}
	}
	return nil
}

// noteServerBackoff records the deadline a 429 response with a Retry-After
// header sets.
func (rl *RateLimiter) noteServerBackoff(req *http.Request, resp *http.Response) {
	if resp.StatusCode != http.StatusTooManyRequests {
		return
	}
	retryAfter := resp.Header.Get("Retry-After")
	if retryAfter == "" {
		return
	}
	d := parseRetryAfterHeader(retryAfter)
	if d <= 0 {
		return
	}
	rl.mu.Lock()
	rl.retryAfterUntil = time.Now().Add(d)
	rl.mu.Unlock()
	rl.logWait(req, slog.LevelWarn, "autotask: server requested backoff", "retry_after", d)
}

// logWait writes one limiter record with the request's method and URL path
// (never its query) and one duration attribute.
func (rl *RateLimiter) logWait(req *http.Request, level slog.Level, msg, key string, d time.Duration) {
	rl.config.logger.LogAttrs(req.Context(), level, msg,
		slog.Duration(key, d),
		slog.String("method", req.Method),
		slog.String("path", req.URL.Path))
}

// sleepCtx waits for d or until ctx ends, and returns ctx's error in that case.
func sleepCtx(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// recordRequest tracks requests in the current one-hour window.
func (rl *RateLimiter) recordRequest() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := time.Now()
	if now.Sub(rl.windowStart) > time.Hour {
		rl.windowStart = now
		rl.requestsInWindow = 0
	}
	rl.requestsInWindow++
}

// adaptiveDelay returns an extra delay when hourly usage is high.
func (rl *RateLimiter) adaptiveDelay() time.Duration {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	usage := float64(rl.requestsInWindow) / float64(rl.config.requestsPerHour)
	switch {
	case usage >= usageWarnThreshold:
		return 1 * time.Second
	case usage >= usageLowThreshold:
		return rateLimitBackoffShort
	default:
		return 0
	}
}

// parseRetryAfterHeader parses the Retry-After header as seconds or HTTP-date.
func parseRetryAfterHeader(header string) time.Duration {
	if header == "" {
		return rateLimitBackoffLong
	}
	if seconds, err := strconv.Atoi(header); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	if t, err := http.ParseTime(header); err == nil {
		if d := time.Until(t); d > 0 {
			return d
		}
	}
	return rateLimitBackoffLong
}
