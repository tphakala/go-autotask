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
	for _, tc := range []struct {
		name         string
		transportErr bool
	}{
		{"response failure", false},
		{"transport error", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testLateFailureIsNotAHalfOpenProbe(t, tc.transportErr)
		})
	}
}

func testLateFailureIsNotAHalfOpenProbe(t *testing.T, transportErr bool) {
	t.Helper()
	synctest.Test(t, func(t *testing.T) {
		const openTimeout = 10 * time.Second
		release := make(chan struct{})
		releaseHeld := sync.OnceFunc(func() { close(release) })
		defer releaseHeld() // an early Fatalf must not leave requests blocked in the bubble
		inner := roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.Header.Get("X-Hold") != "" {
				<-release
				if transportErr {
					return nil, errors.New("connection reset")
				}
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

func TestCircuitBreakerIgnoresProbeFromEarlierHalfOpenPeriod(t *testing.T) {
	for _, tc := range []struct {
		name       string
		lateStatus int
	}{
		{"late failure does not reopen", http.StatusServiceUnavailable},
		{"late success does not count", http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				const openTimeout = 10 * time.Second
				releaseA := make(chan struct{})
				releaseB := make(chan struct{})
				releaseProbeA := sync.OnceFunc(func() { close(releaseA) })
				releaseProbeB := sync.OnceFunc(func() { close(releaseB) })
				defer releaseProbeA() // an early Fatalf must not leave requests blocked in the bubble
				defer releaseProbeB()
				inner := roundTripFunc(func(req *http.Request) (*http.Response, error) {
					status := http.StatusOK
					switch req.Header.Get("X-Probe") {
					case "a":
						<-releaseA
						status = tc.lateStatus
					case "b":
						<-releaseB
						status = http.StatusServiceUnavailable
					}
					if req.Header.Get("X-Fail") != "" {
						status = http.StatusServiceUnavailable
					}
					return &http.Response{StatusCode: status, Body: http.NoBody}, nil
				})
				cb := NewCircuitBreaker(inner, WithFailureThreshold(1), WithOpenTimeout(openTimeout), WithSuccessThreshold(2))
				want := func(step string, state CircuitState) {
					t.Helper()
					if got := cb.State(); got != state {
						t.Fatalf("state %s = %s; want %s", step, got, state)
					}
				}

				failing := newReq(t)
				failing.Header.Set("X-Fail", "1")
				resp, err := cb.RoundTrip(failing)
				if err != nil {
					t.Fatal(err)
				}
				_ = resp.Body.Close()
				synctest.Sleep(openTimeout)
				want("after the first open period", StateHalfOpen)

				// Two probes are in flight in the first half-open period.
				var probes sync.WaitGroup
				for _, name := range []string{"a", "b"} {
					req := newReq(t)
					req.Header.Set("X-Probe", name)
					probes.Go(func() {
						resp, _ := cb.RoundTrip(req)
						if resp != nil {
							_ = resp.Body.Close()
						}
					})
				}
				synctest.Wait()

				// One fails and reopens the breaker; the next half-open period starts.
				releaseProbeB()
				synctest.Wait()
				want("after a probe failed", StateOpen)
				synctest.Sleep(openTimeout)
				want("after the second open period", StateHalfOpen)

				// The other probe ends in the new period and must not count in it.
				releaseProbeA()
				probes.Wait()
				want("after the stale probe ended", StateHalfOpen)

				// Two successes of the new period are still needed to close.
				if err := send(t, cb); err != nil {
					t.Fatal(err)
				}
				want("after one success of the new period", StateHalfOpen)
				if err := send(t, cb); err != nil {
					t.Fatal(err)
				}
				want("after two successes of the new period", StateClosed)
			})
		})
	}
}

func TestCircuitBreakerHalfOpenFailureRestartsOpenTimeout(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const openTimeout = 10 * time.Second
		inner := roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusServiceUnavailable, Body: http.NoBody}, nil
		})
		cb := NewCircuitBreaker(inner, WithFailureThreshold(1), WithOpenTimeout(openTimeout))
		_ = send(t, cb)
		synctest.Sleep(openTimeout)
		if got := cb.State(); got != StateHalfOpen {
			t.Fatalf("state after the first open period = %s; want half-open", got)
		}

		synctest.Sleep(openTimeout / 2)
		_ = send(t, cb) // the probe fails
		synctest.Sleep(openTimeout / 2)
		if got := cb.State(); got != StateOpen {
			t.Fatalf("state openTimeout/2 after the probe failed = %s; want open", got)
		}
		synctest.Sleep(openTimeout / 2)
		if got := cb.State(); got != StateHalfOpen {
			t.Errorf("state openTimeout after the probe failed = %s; want half-open", got)
		}
	})
}

func TestCircuitBreakerIgnoresProbeThatEndsAfterTheBreakerMoved(t *testing.T) {
	for _, tc := range []struct {
		name    string
		bStatus int
		// wait is how long after the other probe ended the held probe fails.
		wait      time.Duration
		afterA    CircuitState
		afterWait CircuitState
	}{
		// The other probe fails and reopens the breaker; the held probe fails
		// while it is open, and the open deadline stays where it was.
		{"fails while open", http.StatusServiceUnavailable, 5 * time.Second, StateOpen, StateHalfOpen},
		// The other probe succeeds and closes the breaker; the held probe fails
		// after that and is not counted against the closed breaker.
		{"fails after the breaker closed", http.StatusOK, 0, StateClosed, StateClosed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				const openTimeout = 10 * time.Second
				releaseA := make(chan struct{})
				releaseB := make(chan struct{})
				releaseProbeA := sync.OnceFunc(func() { close(releaseA) })
				releaseProbeB := sync.OnceFunc(func() { close(releaseB) })
				defer releaseProbeA() // an early Fatalf must not leave requests blocked in the bubble
				defer releaseProbeB()
				inner := roundTripFunc(func(req *http.Request) (*http.Response, error) {
					status := http.StatusServiceUnavailable
					switch req.Header.Get("X-Probe") {
					case "a":
						<-releaseA
					case "b":
						<-releaseB
						status = tc.bStatus
					}
					return &http.Response{StatusCode: status, Body: http.NoBody}, nil
				})
				cb := NewCircuitBreaker(inner, WithFailureThreshold(1), WithOpenTimeout(openTimeout), WithSuccessThreshold(1))
				want := func(step string, state CircuitState) {
					t.Helper()
					if got := cb.State(); got != state {
						t.Fatalf("state %s = %s; want %s", step, got, state)
					}
				}

				_ = send(t, cb) // the breaker opens
				synctest.Sleep(openTimeout)
				want("after the open period", StateHalfOpen)

				var probes sync.WaitGroup
				for _, name := range []string{"a", "b"} {
					req := newReq(t)
					req.Header.Set("X-Probe", name)
					probes.Go(func() {
						resp, _ := cb.RoundTrip(req)
						if resp != nil {
							_ = resp.Body.Close()
						}
					})
				}
				synctest.Wait()

				releaseProbeB()
				synctest.Wait()
				synctest.Sleep(tc.wait)
				releaseProbeA()
				probes.Wait()
				want("after the held probe failed", tc.afterA)

				synctest.Sleep(openTimeout - tc.wait)
				want("openTimeout after the other probe ended", tc.afterWait)
			})
		})
	}
}

func TestCircuitBreakerIgnoresFailureFromEarlierClosedPeriod(t *testing.T) {
	for _, tc := range []struct {
		name         string
		transportErr bool
	}{
		{"response failure", false},
		{"transport error", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				const openTimeout = 10 * time.Second
				release := make(chan struct{})
				releaseHeld := sync.OnceFunc(func() { close(release) })
				defer releaseHeld() // an early Fatalf must not leave requests blocked in the bubble
				inner := roundTripFunc(func(req *http.Request) (*http.Response, error) {
					status := http.StatusOK
					switch {
					case req.Header.Get("X-Hold") != "":
						<-release
						if tc.transportErr {
							return nil, errors.New("connection reset")
						}
						status = http.StatusServiceUnavailable
					case req.Header.Get("X-Fail") != "":
						status = http.StatusServiceUnavailable
					}
					return &http.Response{StatusCode: status, Body: http.NoBody}, nil
				})
				cb := NewCircuitBreaker(inner, WithFailureThreshold(1), WithOpenTimeout(openTimeout), WithSuccessThreshold(1))
				want := func(step string, state CircuitState) {
					t.Helper()
					if got := cb.State(); got != state {
						t.Fatalf("state %s = %s; want %s", step, got, state)
					}
				}

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

				// The breaker opens, goes half-open and closes again.
				failing := newReq(t)
				failing.Header.Set("X-Fail", "1")
				resp, err := cb.RoundTrip(failing)
				if err != nil {
					t.Fatal(err)
				}
				_ = resp.Body.Close()
				want("after the opening failure", StateOpen)
				synctest.Sleep(openTimeout)
				want("after openTimeout", StateHalfOpen)
				if err := send(t, cb); err != nil {
					t.Fatal(err)
				}
				want("after a successful probe", StateClosed)

				// The held request fails in the new closed period. It was admitted
				// in the earlier one, so it does not count against this one.
				releaseHeld()
				done.Wait()
				want("after a failure of a request admitted in the earlier closed period", StateClosed)
			})
		})
	}
}
