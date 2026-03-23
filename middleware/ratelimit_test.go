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
	return &http.Response{StatusCode: 200, Body: http.NoBody}, nil
}

func TestRateLimiterAllowsRequests(t *testing.T) {
	inner := &mockTransport{}
	rl := NewRateLimiter(inner)
	req, err := http.NewRequest("GET", "https://example.com/test", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := rl.RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d; want 200", resp.StatusCode)
	}
}

func TestRateLimiterRespectsRetryAfter(t *testing.T) {
	inner := &mockTransport{
		responses: []*http.Response{
			{StatusCode: 429, Header: http.Header{"Retry-After": []string{"1"}}, Body: http.NoBody},
		},
	}
	rl := NewRateLimiter(inner)
	req, err := http.NewRequest("GET", "https://example.com/test", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := rl.RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 429 {
		t.Fatalf("status = %d; want 429", resp.StatusCode)
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
	req, err := http.NewRequest("GET", "https://example.com/test", nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = rl.RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	if rl.config.requestsPerHour != 1000 {
		t.Fatalf("requestsPerHour = %d; want 1000", rl.config.requestsPerHour)
	}
}
