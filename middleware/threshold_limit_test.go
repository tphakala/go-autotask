package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// A body padded past the cap must not decode, even when valid JSON follows the
// padding.
func TestThresholdMonitorBoundsBodyRead(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat(" ", maxThresholdBodyBytes)))
		_, _ = w.Write([]byte(`{"currentTimeframeRequestCount":9500,"externalRequestThreshold":10000}`))
	}))
	defer srv.Close()
	var gotErr error
	critical := false
	m := NewThresholdMonitor(srv.Client(), srv.URL, AuthHeaders{},
		WithErrorCallback(func(err error) { gotErr = err }),
		WithCriticalCallback(func(ThresholdInfo) { critical = true }),
	)
	m.check(t.Context())
	if gotErr == nil || !strings.Contains(gotErr.Error(), "decoding response") {
		t.Fatalf("error = %v; want a decoding error", gotErr)
	}
	if critical {
		t.Fatal("critical callback ran on a body past the cap")
	}
}
