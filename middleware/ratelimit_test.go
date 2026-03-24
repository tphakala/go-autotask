package middleware

import (
	"net/http"
	"testing"
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
