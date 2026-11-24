package autotask_test

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	autotask "github.com/tphakala/go-autotask"
	"github.com/tphakala/go-autotask/entities"
	"github.com/tphakala/go-autotask/internal/logtest"
	"github.com/tphakala/go-autotask/middleware"
)

const (
	msgBreaker  = "autotask: circuit breaker state changed"
	msgBackoff  = "autotask: server requested backoff"
	msgCritical = "autotask: API usage reached critical threshold"
	msgZone     = "autotask: zone discovered"
)

// newLoggingServer serves zone discovery, the threshold endpoint at 95% usage
// and Companies/1 with the given status. A non-empty retryAfter is sent with it.
func newLoggingServer(t *testing.T, companyStatus int, retryAfter string) *httptest.Server {
	t.Helper()
	var srv *httptest.Server
	mux := http.NewServeMux()
	mux.HandleFunc("GET /atservicesrest/versioninformation", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"apiVersions": []string{"V1.0"}})
	})
	mux.HandleFunc("GET /atservicesrest/V1.0/zoneInformation", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"zoneName": "Test Zone", "url": srv.URL})
	})
	mux.HandleFunc("GET /v1.0/ThresholdInformation", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"currentTimeframeRequestCount": 95, "externalRequestThreshold": 100})
	})
	mux.HandleFunc("GET /v1.0/Companies/1", func(w http.ResponseWriter, _ *http.Request) {
		if retryAfter != "" {
			w.Header().Set("Retry-After", retryAfter)
		}
		w.WriteHeader(companyStatus)
	})
	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// signal returns a critical callback and a function that waits for it. The wait
// is bounded so a monitor that never reports fails the test instead of hanging.
func signal(t *testing.T) (opt middleware.ThresholdMonitorOption, wait func()) {
	t.Helper()
	fired := make(chan struct{}, 1)
	opt = middleware.WithCriticalCallback(func(middleware.ThresholdInfo) {
		select {
		case fired <- struct{}{}:
		default:
		}
	})
	return opt, func() {
		t.Helper()
		select {
		case <-fired:
		case <-time.After(5 * time.Second):
			t.Fatal("the threshold monitor did not report")
		}
	}
}

func messages(rec *logtest.Recorder) map[string]int {
	got := map[string]int{}
	for _, r := range rec.Records() {
		got[r.Message]++
	}
	return got
}

func TestClientLoggerReachesComponents(t *testing.T) {
	t.Parallel()
	srv := newLoggingServer(t, http.StatusTooManyRequests, "1")
	auth := autotask.AuthConfig{Username: "user-secret-name", Secret: "secret-value", IntegrationCode: "secret-code"}
	rec := logtest.New()
	critical, waitCritical := signal(t)
	// WithLogger comes last: where it is listed must not matter.
	client, err := autotask.NewClient(t.Context(), auth,
		autotask.WithZoneBaseURL(srv.URL),
		autotask.WithImpersonation(424242),
		autotask.WithRateLimiter(),
		autotask.WithCircuitBreaker(middleware.WithFailureThreshold(1)),
		autotask.WithThresholdMonitor(critical),
		autotask.WithLogger(rec.Logger()),
	)
	if err != nil {
		t.Fatal(err)
	}
	// The monitor's check goes through the breaker, so it must finish before
	// the 429 opens it.
	waitCritical()
	if _, err := autotask.Get[entities.Company](t.Context(), client, 1); err == nil {
		t.Fatal("want the 429 as an error")
	}
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}

	got := messages(rec)
	for _, msg := range []string{msgZone, msgBackoff, msgBreaker, msgCritical} {
		if got[msg] == 0 {
			t.Errorf("no %q record; got %v", msg, got)
		}
	}
	for _, r := range rec.Records() {
		text := r.Text
		for _, secret := range []string{auth.Username, auth.Secret, auth.IntegrationCode, "424242"} {
			if strings.Contains(text, secret) {
				t.Errorf("record contains %q: %s", secret, text)
			}
		}
	}
}

func TestMiddlewareLoggerOverridesClientLogger(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		status     int
		retryAfter string
		component  func(own *slog.Logger, critical middleware.ThresholdMonitorOption) autotask.ClientOption
		viaMonitor bool
		wantMsg    string
	}{
		{
			name: "circuit breaker", status: http.StatusServiceUnavailable,
			component: func(own *slog.Logger, _ middleware.ThresholdMonitorOption) autotask.ClientOption {
				return autotask.WithCircuitBreaker(middleware.WithFailureThreshold(1), middleware.WithCircuitBreakerLogger(own))
			},
			wantMsg: msgBreaker,
		},
		{
			name: "rate limiter", status: http.StatusTooManyRequests, retryAfter: "1",
			component: func(own *slog.Logger, _ middleware.ThresholdMonitorOption) autotask.ClientOption {
				return autotask.WithRateLimiter(middleware.WithRateLimitLogger(own))
			},
			wantMsg: msgBackoff,
		},
		{
			name: "threshold monitor", status: http.StatusOK, viaMonitor: true,
			component: func(own *slog.Logger, critical middleware.ThresholdMonitorOption) autotask.ClientOption {
				return autotask.WithThresholdMonitor(critical, middleware.WithThresholdMonitorLogger(own))
			},
			wantMsg: msgCritical,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			srv := newLoggingServer(t, tt.status, tt.retryAfter)
			clientRec, ownRec := logtest.New(), logtest.New()
			critical, waitCritical := signal(t)
			client, err := autotask.NewClient(t.Context(), autotask.AuthConfig{},
				autotask.WithLogger(clientRec.Logger()),
				autotask.WithBaseURL(srv.URL),
				tt.component(ownRec.Logger(), critical),
			)
			if err != nil {
				t.Fatal(err)
			}
			if tt.viaMonitor {
				waitCritical()
			} else if _, err := autotask.Get[entities.Company](t.Context(), client, 1); err == nil {
				t.Fatal("want the error status as an error")
			}
			if err := client.Close(); err != nil {
				t.Fatal(err)
			}
			if got := messages(ownRec)[tt.wantMsg]; got == 0 {
				t.Errorf("the component's own logger has no %q record; got %v", tt.wantMsg, messages(ownRec))
			}
			if got := messages(clientRec); len(got) != 0 {
				t.Errorf("the client logger got records %v; want none", got)
			}
		})
	}
}

func TestWithLoggerNilKeepsEarlierLogger(t *testing.T) {
	t.Parallel()
	srv := newLoggingServer(t, http.StatusServiceUnavailable, "")
	rec := logtest.New()
	client, err := autotask.NewClient(t.Context(), autotask.AuthConfig{},
		autotask.WithLogger(rec.Logger()),
		autotask.WithLogger(nil),
		autotask.WithBaseURL(srv.URL),
		autotask.WithCircuitBreaker(middleware.WithFailureThreshold(1)),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	if _, err := autotask.Get[entities.Company](t.Context(), client, 1); err == nil {
		t.Fatal("want the 503 as an error")
	}
	if got := messages(rec)[msgBreaker]; got != 1 {
		t.Errorf("got %d breaker records; want 1", got)
	}
}

func TestThresholdMonitorWithoutOptionsRuns(t *testing.T) {
	t.Parallel()
	srv := newLoggingServer(t, http.StatusOK, "")
	rec := logtest.New()
	client, err := autotask.NewClient(t.Context(), autotask.AuthConfig{Username: "u", Secret: "s", IntegrationCode: "c"},
		autotask.WithZoneBaseURL(srv.URL),
		autotask.WithThresholdMonitor(),
		autotask.WithLogger(rec.Logger()),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	// The monitor checks once when it starts; the server reports 95% usage.
	deadline := time.Now().Add(5 * time.Second)
	for messages(rec)[msgCritical] == 0 {
		if time.Now().After(deadline) {
			t.Fatal("a monitor enabled with no options wrote no critical record")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestThresholdMonitorKeepsOptionsOfEveryCall(t *testing.T) {
	t.Parallel()
	srv := newLoggingServer(t, http.StatusOK, "")
	critical, waitCritical := signal(t)
	client, err := autotask.NewClient(t.Context(), autotask.AuthConfig{Username: "u", Secret: "s", IntegrationCode: "c"},
		autotask.WithZoneBaseURL(srv.URL),
		autotask.WithThresholdMonitor(critical),
		autotask.WithThresholdMonitor(middleware.WithCheckInterval(time.Hour)),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	// A second call that replaced the first call's options would drop the
	// critical callback.
	waitCritical()
}
