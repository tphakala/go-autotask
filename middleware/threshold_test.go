package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
