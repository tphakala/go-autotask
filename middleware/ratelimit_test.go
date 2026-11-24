package middleware

import (
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/tphakala/go-autotask/internal/logtest"
)

type mockTransport struct {
	responses []*http.Response
	calls     int
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	idx := m.calls
	m.calls++
	if idx < len(m.responses) {
		return m.responses[idx], nil
	}
	return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil
}

func TestRateLimiterAllowsRequests(t *testing.T) {
	inner := &mockTransport{}
	rl := NewRateLimiter(inner)
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://example.com/test", http.NoBody)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := rl.RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d; want %d", resp.StatusCode, http.StatusOK)
	}
}

func TestRateLimiterRespectsRetryAfter(t *testing.T) {
	inner := &mockTransport{
		responses: []*http.Response{
			{StatusCode: http.StatusTooManyRequests, Header: http.Header{"Retry-After": []string{"1"}}, Body: http.NoBody},
		},
	}
	rl := NewRateLimiter(inner)
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://example.com/test", http.NoBody)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := rl.RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("status = %d; want %d", resp.StatusCode, http.StatusTooManyRequests)
	}
	// Verify the retry-after was recorded
	rl.mu.Lock()
	retryUntil := rl.retryAfterUntil
	rl.mu.Unlock()
	if retryUntil.IsZero() {
		t.Fatal("retryAfterUntil should be set after 429")
	}
}

func TestRateLimiterCustomConfig(t *testing.T) {
	inner := &mockTransport{}
	rl := NewRateLimiter(inner, WithRequestsPerHour(1000), WithBurstSize(5))
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://example.com/test", http.NoBody)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := rl.RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if rl.config.requestsPerHour != 1000 {
		t.Fatalf("requestsPerHour = %d; want 1000", rl.config.requestsPerHour)
	}
	if rl.config.burstSize != 5 {
		t.Fatalf("burstSize = %d; want 5", rl.config.burstSize)
	}
}

func TestRateLimiterLogsBackoffAndWaits(t *testing.T) {
	const target = "https://example.com/v1.0/Tickets/query?search=secret-q"
	type want struct {
		level slog.Level
		msg   string
		attr  string
		value time.Duration
	}
	tests := []struct {
		name      string
		opts      []RateLimitOption
		responses []*http.Response
		requests  int
		want      []want
	}{
		{
			name:      "server backoff",
			responses: []*http.Response{{StatusCode: http.StatusTooManyRequests, Header: http.Header{"Retry-After": []string{"2"}}, Body: http.NoBody}},
			requests:  2,
			want: []want{
				{slog.LevelWarn, "autotask: server requested backoff", "retry_after", 2 * time.Second},
				{slog.LevelInfo, "autotask: waiting for server backoff", "wait", 2 * time.Second},
			},
		},
		{
			name:     "token bucket",
			opts:     []RateLimitOption{WithRequestsPerHour(3600), WithBurstSize(1), WithAdaptiveDelay(false)},
			requests: 2,
			want:     []want{{slog.LevelDebug, "autotask: waiting for rate limit token", "waited", time.Second}},
		},
		{
			name:     "adaptive delay",
			opts:     []RateLimitOption{WithRequestsPerHour(4)},
			requests: 3,
			want:     []want{{slog.LevelDebug, "autotask: adaptive rate limit delay", "delay", 500 * time.Millisecond}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				rec := logtest.New()
				rl := NewRateLimiter(&mockTransport{responses: tt.responses},
					append([]RateLimitOption{WithRateLimitLogger(rec.Logger())}, tt.opts...)...)
				for range tt.requests {
					req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, target, http.NoBody)
					if err != nil {
						t.Fatal(err)
					}
					resp, err := rl.RoundTrip(req)
					if err != nil {
						t.Fatal(err)
					}
					_ = resp.Body.Close()
				}
				recs := rec.Records()
				if len(recs) != len(tt.want) {
					t.Fatalf("got %d records; want %d", len(recs), len(tt.want))
				}
				for i, w := range tt.want {
					r := recs[i]
					if r.Level != w.level || r.Message != w.msg {
						t.Errorf("record %d = %s %q; want %s %q", i, r.Level, r.Message, w.level, w.msg)
					}
					if d, ok := r.Duration(w.attr); !ok || d != w.value {
						t.Errorf("record %d %s = %v (present %v); want %v", i, w.attr, d, ok, w.value)
					}
					if got := r.Str("path"); got != "/v1.0/Tickets/query" {
						t.Errorf("record %d path = %q; want the URL path", i, got)
					}
					if got := r.Str("method"); got != http.MethodGet {
						t.Errorf("record %d method = %q; want GET", i, got)
					}
				}
				for _, r := range recs {
					if strings.Contains(r.Text, "secret-q") {
						t.Errorf("record leaks the query string: %s", r.Text)
					}
				}
			})
		})
	}
}
