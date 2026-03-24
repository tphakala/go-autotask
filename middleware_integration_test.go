package autotask_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	autotask "github.com/tphakala/go-autotask"
	"github.com/tphakala/go-autotask/autotasktest"
	"github.com/tphakala/go-autotask/entities"
	"github.com/tphakala/go-autotask/middleware"
)

func TestMiddlewareCircuitBreakerOpensOnErrors(t *testing.T) {
	t.Parallel()

	company := autotasktest.CompanyFixture()
	srv, _ := autotasktest.NewServer(t,
		autotasktest.WithEntity(company),
		autotasktest.WithErrorOn("GET", "Companies/1", http.StatusInternalServerError, []string{"server error"}),
	)

	// Create a new client with circuit breaker middleware pointing at the test server.
	auth := autotask.AuthConfig{Username: "test-user", Secret: "test-secret", IntegrationCode: "test-code"}
	client, err := autotask.NewClient(t.Context(), auth,
		autotask.WithBaseURL(srv.URL),
		autotask.WithCircuitBreaker(
			middleware.WithFailureThreshold(2),
			middleware.WithOpenTimeout(100*time.Millisecond),
		),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })

	// First 2 requests hit the server and fail with 500.
	for range 2 {
		_, err := autotask.Get[entities.Company](t.Context(), client, 1)
		if err == nil {
			t.Fatal("expected error from server 500")
		}
	}

	initialCount := srv.RequestCount()

	// Third request should be rejected by the circuit breaker without hitting the server.
	_, err = autotask.Get[entities.Company](t.Context(), client, 1)
	if err == nil {
		t.Fatal("expected error from open circuit")
	}

	if srv.RequestCount() != initialCount {
		t.Fatalf("circuit breaker should have prevented request; server request count changed from %d to %d",
			initialCount, srv.RequestCount())
	}
}

func TestMiddlewareCircuitBreakerRecovers(t *testing.T) {
	t.Parallel()

	company := autotasktest.CompanyFixture()
	srv, _ := autotasktest.NewServer(t,
		autotasktest.WithEntity(company),
		autotasktest.WithErrorOn("GET", "Companies/1", http.StatusInternalServerError, []string{"server error"}),
	)

	auth := autotask.AuthConfig{Username: "test-user", Secret: "test-secret", IntegrationCode: "test-code"}
	client, err := autotask.NewClient(t.Context(), auth,
		autotask.WithBaseURL(srv.URL),
		autotask.WithCircuitBreaker(
			middleware.WithFailureThreshold(2),
			middleware.WithOpenTimeout(100*time.Millisecond),
		),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })

	// Trip the circuit breaker with 2 failures.
	for range 2 {
		_, err := autotask.Get[entities.Company](t.Context(), client, 1)
		if err == nil {
			t.Fatal("expected error from server 500")
		}
	}

	// Confirm the circuit is open.
	_, err = autotask.Get[entities.Company](t.Context(), client, 1)
	if err == nil {
		t.Fatal("expected error from open circuit")
	}

	countBeforeWait := srv.RequestCount()

	// Wait for the open timeout to pass so the circuit transitions to half-open.
	time.Sleep(150 * time.Millisecond)

	// The next request should be allowed through (half-open probe).
	// The error injection is still active, so it will fail, but the circuit
	// breaker should have allowed the request to reach the server.
	_, err = autotask.Get[entities.Company](t.Context(), client, 1)
	if err == nil {
		t.Fatal("expected error (error injection still active)")
	}

	if srv.RequestCount() <= countBeforeWait {
		t.Fatal("circuit breaker should have allowed probe request in half-open state, but server was not called")
	}
}

func TestMiddlewareThresholdMonitorCallback(t *testing.T) {
	t.Parallel()

	// Use a standalone httptest server that returns threshold data matching the
	// field names expected by the ThresholdMonitor: "currentTimeframeRequestCount"
	// and "externalRequestThreshold".
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"currentTimeframeRequestCount": 8000,
			"externalRequestThreshold":     10000,
		})
	}))
	t.Cleanup(srv.Close)

	var callbackFired atomic.Bool
	callbackCh := make(chan middleware.ThresholdInfo, 1)

	auth := autotask.AuthConfig{Username: "test-user", Secret: "test-secret", IntegrationCode: "test-code"}
	client, err := autotask.NewClient(t.Context(), auth,
		autotask.WithBaseURL(srv.URL),
		autotask.WithThresholdMonitor(
			middleware.WithCheckInterval(50*time.Millisecond),
			middleware.WithWarningCallback(func(info middleware.ThresholdInfo) {
				if callbackFired.CompareAndSwap(false, true) {
					callbackCh <- info
				}
			}),
		),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })

	// Wait for the callback to fire (the monitor performs an immediate check on Start).
	select {
	case info := <-callbackCh:
		if info.UsagePercent < 75 {
			t.Fatalf("expected usage >= 75%%, got %.1f%%", info.UsagePercent)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("threshold monitor warning callback was not called within timeout")
	}
}
