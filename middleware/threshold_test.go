package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestThresholdMonitorCallsWarning(t *testing.T) {
	var warningCalled atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"currentTimeframeRequestCount": 8000,
			"externalRequestThreshold":     10000,
		})
	}))
	defer srv.Close()
	m := NewThresholdMonitor(srv.Client(), srv.URL, AuthHeaders{},
		WithCheckInterval(10*time.Millisecond),
		WithWarningCallback(func(info ThresholdInfo) { warningCalled.Store(true) }),
	)
	m.Start()
	defer m.Stop()
	time.Sleep(50 * time.Millisecond)
	if !warningCalled.Load() {
		t.Fatal("warning callback should have been called at 80% usage")
	}
}

func TestThresholdMonitorCallsCritical(t *testing.T) {
	var criticalCalled atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"currentTimeframeRequestCount": 9500,
			"externalRequestThreshold":     10000,
		})
	}))
	defer srv.Close()
	m := NewThresholdMonitor(srv.Client(), srv.URL, AuthHeaders{},
		WithCheckInterval(10*time.Millisecond),
		WithCriticalCallback(func(info ThresholdInfo) { criticalCalled.Store(true) }),
	)
	m.Start()
	defer m.Stop()
	time.Sleep(50 * time.Millisecond)
	if !criticalCalled.Load() {
		t.Fatal("critical callback should have been called at 95% usage")
	}
}

func TestThresholdMonitorStop(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"currentTimeframeRequestCount": 100,
			"externalRequestThreshold":     10000,
		})
	}))
	defer srv.Close()
	m := NewThresholdMonitor(srv.Client(), srv.URL, AuthHeaders{}, WithCheckInterval(10*time.Millisecond))
	m.Start()
	err := m.Stop()
	if err != nil {
		t.Fatal(err)
	}
}
