package middleware

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

const (
	defaultRequestsPerHour  = 5000
	defaultBurstSize        = 20
	secondsPerHour          = 3600.0
	usageWarnThreshold      = 0.75
	usageLowThreshold       = 0.50
	rateLimitBackoffShort   = 500 * time.Millisecond
	rateLimitBackoffLong    = 60 * time.Second
)

// RateLimitOption configures a RateLimiter.
type RateLimitOption func(*rateLimitConfig)

type rateLimitConfig struct {
	requestsPerHour int
	burstSize       int
	adaptiveDelay   bool
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

// RateLimiter is an http.RoundTripper that enforces rate limits using a token
// bucket algorithm with optional adaptive delays. It also respects 429
// Retry-After headers from the server.
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
// Default: 5000 requests/hour, burst of 20, adaptive delay enabled.
func NewRateLimiter(next http.RoundTripper, opts ...RateLimitOption) *RateLimiter {
	cfg := rateLimitConfig{
		requestsPerHour: defaultRequestsPerHour,
		burstSize:       defaultBurstSize,
		adaptiveDelay:   true,
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

// RoundTrip implements http.RoundTripper with rate limiting.
func (rl *RateLimiter) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx := req.Context()

	// Honour any active Retry-After wait.
	rl.mu.Lock()
	retryUntil := rl.retryAfterUntil
	rl.mu.Unlock()

	if wait := time.Until(retryUntil); wait > 0 {
		select {
		case <-time.After(wait):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	// Token bucket wait.
	if err := rl.limiter.Wait(ctx); err != nil {
		return nil, err
	}

	// Optional adaptive delay based on hourly window usage.
	if rl.config.adaptiveDelay {
		delay := rl.adaptiveDelay()
		if delay > 0 {
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
	}

	rl.recordRequest()

	resp, err := rl.next.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	// If the server tells us to back off, record the deadline.
	if resp.StatusCode == http.StatusTooManyRequests {
		if retryAfter := resp.Header.Get("Retry-After"); retryAfter != "" {
			if d := parseRetryAfterHeader(retryAfter); d > 0 {
				rl.mu.Lock()
				rl.retryAfterUntil = time.Now().Add(d)
				rl.mu.Unlock()
			}
		}
	}

	return resp, nil
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
	// Try as seconds first.
	var seconds int
	if _, err := fmt.Sscanf(header, "%d", &seconds); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	// Try as HTTP-date (RFC 7231).
	if t, err := http.ParseTime(header); err == nil {
		if d := time.Until(t); d > 0 {
			return d
		}
	}
	return rateLimitBackoffLong
}
