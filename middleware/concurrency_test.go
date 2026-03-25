package middleware

import (
	"context"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestConcurrencyLimiterAllowsRequests(t *testing.T) {
	inner := &mockTransport{}
	cl := NewConcurrencyLimiter(inner, 3)
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://example.com/test", http.NoBody)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := cl.RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d; want %d", resp.StatusCode, http.StatusOK)
	}
}

func TestConcurrencyLimiterEnforcesLimit(t *testing.T) {
	const maxConcurrency = 2
	var inflight atomic.Int32
	var maxSeen atomic.Int32

	inner := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		cur := inflight.Add(1)
		// Track max concurrency seen
		for {
			old := maxSeen.Load()
			if cur <= old || maxSeen.CompareAndSwap(old, cur) {
				break
			}
		}
		time.Sleep(50 * time.Millisecond)
		inflight.Add(-1)
		return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil
	})

	cl := NewConcurrencyLimiter(inner, maxConcurrency)

	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() {
			req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://example.com/test", http.NoBody)
			resp, err := cl.RoundTrip(req)
			if err != nil {
				t.Error(err)
				return
			}
			_ = resp.Body.Close()
		})
	}
	wg.Wait()

	if got := maxSeen.Load(); got > int32(maxConcurrency) {
		t.Fatalf("max concurrency = %d; want <= %d", got, maxConcurrency)
	}
}

func TestConcurrencyLimiterContextCancellation(t *testing.T) {
	// Fill all slots with a blocking transport
	inner := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		<-req.Context().Done()
		return nil, req.Context().Err()
	})
	cl := NewConcurrencyLimiter(inner, 1)

	// Fill the single slot
	ctx1, cancel1 := context.WithCancel(t.Context())
	defer cancel1()
	go func() {
		req, _ := http.NewRequestWithContext(ctx1, http.MethodGet, "https://example.com/test", http.NoBody)
		_, _ = cl.RoundTrip(req) //nolint:bodyclose // blocking transport returns nil response
	}()

	// Give the goroutine time to acquire the slot
	time.Sleep(10 * time.Millisecond)

	// Try to acquire with a cancelled context — should fail immediately
	ctx2, cancel2 := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel2()
	req, _ := http.NewRequestWithContext(ctx2, http.MethodGet, "https://example.com/test", http.NoBody)
	_, err := cl.RoundTrip(req) //nolint:bodyclose // error path returns nil response
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}

	cancel1() // cleanup
}

func TestConcurrencyLimiterDefaultsToThree(t *testing.T) {
	cl := NewConcurrencyLimiter(&mockTransport{}, 0)
	if cap(cl.sem) != 3 {
		t.Fatalf("sem cap = %d; want 3", cap(cl.sem))
	}

	cl2 := NewConcurrencyLimiter(&mockTransport{}, -1)
	if cap(cl2.sem) != 3 {
		t.Fatalf("sem cap = %d; want 3", cap(cl2.sem))
	}
}

// roundTripFunc adapts a function into an http.RoundTripper.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }
