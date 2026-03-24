package middleware

import (
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestCircuitBreakerClosedState(t *testing.T) {
	inner := &mockTransport{}
	cb := NewCircuitBreaker(inner)
	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://example.com/test", http.NoBody)
	resp, err := cb.RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d; want %d", resp.StatusCode, http.StatusOK)
	}
	if cb.State() != StateClosed {
		t.Fatalf("state = %s; want closed", cb.State())
	}
}

func TestCircuitBreakerOpensOnFailures(t *testing.T) {
	inner := &mockTransport{
		responses: make([]*http.Response, 10),
	}
	for i := range inner.responses {
		inner.responses[i] = &http.Response{StatusCode: http.StatusInternalServerError, Body: http.NoBody}
	}
	cb := NewCircuitBreaker(inner, WithFailureThreshold(3), WithFailureWindow(10*time.Second))
	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://example.com/test", http.NoBody)
	for range 3 {
		resp, _ := cb.RoundTrip(req)
		if resp != nil {
			_ = resp.Body.Close()
		}
	}
	if cb.State() != StateOpen {
		t.Fatalf("state = %s; want open", cb.State())
	}
	resp, err := cb.RoundTrip(req)
	if resp != nil {
		_ = resp.Body.Close()
	}
	if err == nil || !strings.Contains(err.Error(), "circuit breaker is open") {
		t.Fatalf("expected circuit breaker open error, got: %v", err)
	}
}

func TestCircuitBreakerTransitionsToHalfOpen(t *testing.T) {
	inner := &mockTransport{
		responses: []*http.Response{
			{StatusCode: 500, Body: http.NoBody},
			{StatusCode: 500, Body: http.NoBody},
			{StatusCode: 500, Body: http.NoBody},
		},
	}
	cb := NewCircuitBreaker(inner, WithFailureThreshold(3), WithOpenTimeout(10*time.Millisecond))
	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://example.com/test", http.NoBody)
	for range 3 {
		resp, _ := cb.RoundTrip(req)
		if resp != nil {
			_ = resp.Body.Close()
		}
	}
	time.Sleep(20 * time.Millisecond)
	if cb.State() != StateHalfOpen {
		t.Fatalf("state = %s; want half-open", cb.State())
	}
}

func TestCircuitBreakerIgnoresNon5xx(t *testing.T) {
	inner := &mockTransport{
		responses: []*http.Response{
			{StatusCode: 404, Body: http.NoBody},
			{StatusCode: 404, Body: http.NoBody},
			{StatusCode: 404, Body: http.NoBody},
		},
	}
	cb := NewCircuitBreaker(inner, WithFailureThreshold(3))
	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://example.com/test", http.NoBody)
	for range 3 {
		resp, _ := cb.RoundTrip(req)
		if resp != nil {
			_ = resp.Body.Close()
		}
	}
	if cb.State() != StateClosed {
		t.Fatalf("state = %s; want closed (4xx should not trigger)", cb.State())
	}
}

var _ error = (*CircuitBreakerOpenError)(nil)

func TestCircuitBreakerOpenError(t *testing.T) {
	err := &CircuitBreakerOpenError{}
	if _, ok := errors.AsType[*CircuitBreakerOpenError](err); !ok {
		t.Fatal("should match CircuitBreakerOpenError")
	}
}
