package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func testThresholdMonitor(t *testing.T, requestCount int, opts ...ThresholdMonitorOption) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"currentTimeframeRequestCount": requestCount,
			"externalRequestThreshold":     10000,
		})
	}))
	defer srv.Close()
	m := NewThresholdMonitor(srv.Client(), srv.URL, AuthHeaders{},
		append([]ThresholdMonitorOption{WithCheckInterval(10 * time.Millisecond)}, opts...)...,
	)
	m.Start(t.Context())
	defer func() { _ = m.Stop() }()
	time.Sleep(50 * time.Millisecond)
}

func TestThresholdMonitorCallsWarning(t *testing.T) {
	var warningCalled atomic.Bool
	testThresholdMonitor(t, 8000,
		WithWarningCallback(func(info ThresholdInfo) { warningCalled.Store(true) }),
	)
	if !warningCalled.Load() {
		t.Fatal("warning callback should have been called at 80% usage")
	}
}

func TestThresholdMonitorCallsCritical(t *testing.T) {
	var criticalCalled atomic.Bool
	testThresholdMonitor(t, 9500,
		WithCriticalCallback(func(info ThresholdInfo) { criticalCalled.Store(true) }),
	)
	if !criticalCalled.Load() {
		t.Fatal("critical callback should have been called at 95% usage")
	}
}

func TestThresholdMonitorStop(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"currentTimeframeRequestCount": 100,
			"externalRequestThreshold":     10000,
		})
	}))
	defer srv.Close()
	m := NewThresholdMonitor(srv.Client(), srv.URL, AuthHeaders{}, WithCheckInterval(10*time.Millisecond))
	m.Start(t.Context())
	err := m.Stop()
	if err != nil {
		t.Fatal(err)
	}
}

func TestThresholdMonitorRedirectDropsCredentials(t *testing.T) {
	var mu sync.Mutex
	var gotHeaders http.Header
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		gotHeaders = r.Header.Clone()
		mu.Unlock()
		_ = json.NewEncoder(w).Encode(map[string]any{
			"currentTimeframeRequestCount": 95,
			"externalRequestThreshold":     100,
		})
	}))
	defer target.Close()
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+r.URL.RequestURI(), http.StatusTemporaryRedirect)
	}))
	defer origin.Close()

	hc := &http.Client{}
	fired := make(chan struct{}, 1)
	m := NewThresholdMonitor(hc, origin.URL,
		AuthHeaders{Username: "user", Secret: "secret", IntegrationCode: "code"},
		WithCheckInterval(time.Hour),
		WithCriticalCallback(func(ThresholdInfo) {
			select {
			case fired <- struct{}{}:
			default:
			}
		}),
	)
	m.Start(t.Context())
	defer func() { _ = m.Stop() }()

	select {
	case <-fired:
	case <-time.After(5 * time.Second):
		t.Fatal("threshold check did not reach the redirect target")
	}
	mu.Lock()
	defer mu.Unlock()
	for _, name := range []string{"UserName", "Secret", "ApiIntegrationCode"} {
		if got := gotHeaders.Get(name); got != "" {
			t.Errorf("redirect target got %s = %q; want empty", name, got)
		}
	}
	if hc.CheckRedirect != nil {
		t.Error("NewThresholdMonitor set CheckRedirect on the caller's http.Client; want it left nil")
	}
}
