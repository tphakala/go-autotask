package middleware

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/tphakala/go-autotask/internal/logtest"
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

// transitions returns the (from, to) pairs of the circuit breaker records in
// rec and fails the test on a record that is not a Warn state change.
func transitions(t *testing.T, rec *logtest.Recorder) [][2]string {
	t.Helper()
	var got [][2]string
	for _, r := range rec.Records() {
		if r.Level != slog.LevelWarn || r.Message != "autotask: circuit breaker state changed" {
			t.Errorf("record = %s %q; want Warn state change", r.Level, r.Message)
			continue
		}
		got = append(got, [2]string{r.Str("from"), r.Str("to")})
	}
	return got
}

// send makes one request through rt and closes the response body.
func send(t *testing.T, rt http.RoundTripper) error {
	t.Helper()
	resp, err := rt.RoundTrip(newReq(t))
	if resp != nil {
		_ = resp.Body.Close()
	}
	return err
}

func TestCircuitBreakerLogsStateChanges(t *testing.T) {
	t.Run("every transition once and no rejected request", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			rec := logtest.New()
			inner := &mockTransport{responses: []*http.Response{
				{StatusCode: http.StatusServiceUnavailable, Body: http.NoBody}, // closed to open
				{StatusCode: http.StatusOK, Body: http.NoBody},                 // open to half-open, half-open to closed
				{StatusCode: http.StatusServiceUnavailable, Body: http.NoBody}, // closed to open
				{StatusCode: http.StatusServiceUnavailable, Body: http.NoBody}, // open to half-open, half-open to open
			}}
			cb := NewCircuitBreaker(inner, WithFailureThreshold(1), WithOpenTimeout(time.Second),
				WithSuccessThreshold(1), WithCircuitBreakerLogger(rec.Logger()))

			_ = send(t, cb)
			if err := send(t, cb); err == nil {
				t.Fatal("request while open: want a CircuitBreakerOpenError")
			}
			synctest.Sleep(time.Second)
			if err := send(t, cb); err != nil {
				t.Fatal(err)
			}
			_ = send(t, cb)
			synctest.Sleep(time.Second)
			_ = send(t, cb)

			want := [][2]string{
				{"closed", "open"},
				{"open", "half-open"}, {"half-open", "closed"},
				{"closed", "open"},
				{"open", "half-open"}, {"half-open", "open"},
			}
			if got := transitions(t, rec); !slices.Equal(got, want) {
				t.Errorf("transitions = %v; want %v", got, want)
			}
		})
	})

	t.Run("State reports the open to half-open change once", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			rec := logtest.New()
			cb := NewCircuitBreaker(&mockTransport{responses: []*http.Response{{StatusCode: http.StatusServiceUnavailable, Body: http.NoBody}}},
				WithFailureThreshold(1), WithOpenTimeout(time.Second), WithCircuitBreakerLogger(rec.Logger()))
			_ = send(t, cb)
			synctest.Sleep(time.Second)
			for range 2 {
				if got := cb.State(); got != StateHalfOpen {
					t.Fatalf("state = %s; want half-open", got)
				}
			}
			want := [][2]string{{"closed", "open"}, {"open", "half-open"}}
			if got := transitions(t, rec); !slices.Equal(got, want) {
				t.Errorf("transitions = %v; want %v", got, want)
			}
		})
	})

	t.Run("two in-flight failures open the breaker once", func(t *testing.T) {
		rec := logtest.New()
		var inFlight sync.WaitGroup
		inFlight.Add(2)
		both := roundTripFunc(func(*http.Request) (*http.Response, error) {
			inFlight.Done()
			inFlight.Wait()
			return &http.Response{StatusCode: http.StatusServiceUnavailable, Body: http.NoBody}, nil
		})
		cb := NewCircuitBreaker(both, WithFailureThreshold(1), WithCircuitBreakerLogger(rec.Logger()))
		var done sync.WaitGroup
		for range 2 {
			done.Go(func() { _ = send(t, cb) })
		}
		done.Wait()
		want := [][2]string{{"closed", "open"}}
		if got := transitions(t, rec); !slices.Equal(got, want) {
			t.Errorf("transitions = %v; want %v", got, want)
		}
	})
}

func TestCircuitBreakerLateFailuresKeepOpenDeadline(t *testing.T) {
	for _, threshold := range []int{1, 2} {
		t.Run(fmt.Sprintf("threshold %d", threshold), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				const openTimeout = 10 * time.Second
				release := make(chan struct{})
				releaseHeld := sync.OnceFunc(func() { close(release) })
				defer releaseHeld() // an early Fatalf must not leave requests blocked in the bubble
				inner := roundTripFunc(func(req *http.Request) (*http.Response, error) {
					if req.Header.Get("X-Hold") != "" {
						<-release
					}
					return &http.Response{StatusCode: http.StatusServiceUnavailable, Body: http.NoBody}, nil
				})
				cb := NewCircuitBreaker(inner, WithFailureThreshold(threshold), WithOpenTimeout(openTimeout))

				// Admit as many requests as the threshold while the breaker is closed,
				// and hold them in the transport.
				var held sync.WaitGroup
				for range threshold {
					req := newReq(t)
					req.Header.Set("X-Hold", "1")
					held.Go(func() {
						resp, _ := cb.RoundTrip(req)
						if resp != nil {
							_ = resp.Body.Close()
						}
					})
				}
				synctest.Wait()

				for range threshold {
					_ = send(t, cb)
				}
				if got := cb.State(); got != StateOpen {
					t.Fatalf("state after %d failures = %s; want open", threshold, got)
				}

				// The held requests fail halfway through the open period.
				synctest.Sleep(openTimeout / 2)
				releaseHeld()
				held.Wait()

				synctest.Sleep(openTimeout / 2)
				if got := cb.State(); got != StateHalfOpen {
					t.Errorf("state openTimeout after opening = %s; want half-open (late failures must not restart the open timeout)", got)
				}
			})
		})
	}
}

func TestCircuitBreakerLateFailureIsNotAHalfOpenProbe(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const openTimeout = 10 * time.Second
		release := make(chan struct{})
		releaseHeld := sync.OnceFunc(func() { close(release) })
		defer releaseHeld() // an early Fatalf must not leave requests blocked in the bubble
		inner := roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.Header.Get("X-Hold") != "" {
				<-release
			}
			status := http.StatusOK
			if req.Header.Get("X-Hold") != "" || req.Header.Get("X-Fail") != "" {
				status = http.StatusServiceUnavailable
			}
			return &http.Response{StatusCode: status, Body: http.NoBody}, nil
		})
		cb := NewCircuitBreaker(inner, WithFailureThreshold(1), WithOpenTimeout(openTimeout), WithSuccessThreshold(1))

		// A request admitted while the breaker is closed stays in flight.
		held := newReq(t)
		held.Header.Set("X-Hold", "1")
		var done sync.WaitGroup
		done.Go(func() {
			resp, _ := cb.RoundTrip(held)
			if resp != nil {
				_ = resp.Body.Close()
			}
		})
		synctest.Wait()

		failing := newReq(t)
		failing.Header.Set("X-Fail", "1")
		resp, err := cb.RoundTrip(failing)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		if got := cb.State(); got != StateOpen {
			t.Fatalf("state after the opening failure = %s; want open", got)
		}
		synctest.Sleep(openTimeout)
		if got := cb.State(); got != StateHalfOpen {
			t.Fatalf("state after openTimeout = %s; want half-open", got)
		}

		// It fails now, in half-open. It was not admitted as a probe.
		releaseHeld()
		done.Wait()
		if got := cb.State(); got != StateHalfOpen {
			t.Errorf("state after a failure of a request admitted while closed = %s; want half-open", got)
		}

		// A probe still decides the half-open state.
		if err := send(t, cb); err != nil {
			t.Fatal(err)
		}
		if got := cb.State(); got != StateClosed {
			t.Errorf("state after a successful probe = %s; want closed", got)
		}
	})
}
