package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// A threshold body longer than the cap is reported as an error and fires no
// callback, whether the valid JSON comes before or after the padding.
func TestThresholdMonitorBoundsBodyRead(t *testing.T) {
	const payload = `{"currentTimeframeRequestCount":9500,"externalRequestThreshold":10000}`
	padding := strings.Repeat(" ", maxThresholdBodyBytes)
	for name, body := range map[string]string{
		"padding first": padding + payload,
		"payload first": payload + padding,
	} {
		t.Run(name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(body))
			}))
			defer srv.Close()
			var gotErr error
			critical := false
			m := NewThresholdMonitor(srv.Client(), srv.URL, AuthHeaders{},
				WithErrorCallback(func(err error) { gotErr = err }),
				WithCriticalCallback(func(ThresholdInfo) { critical = true }),
			)
			m.check(t.Context())
			if gotErr == nil || !strings.Contains(gotErr.Error(), "exceeds") {
				t.Fatalf("error = %v; want a size error", gotErr)
			}
			if critical {
				t.Fatal("critical callback ran on a body past the cap")
			}
		})
	}
}

// A body of exactly the cap still decodes and fires the callback.
func TestThresholdMonitorDecodesBodyWithinCap(t *testing.T) {
	const payload = `{"currentTimeframeRequestCount":9500,"externalRequestThreshold":10000}`
	body := payload + strings.Repeat(" ", maxThresholdBodyBytes-len(payload))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()
	var gotErr error
	critical := false
	m := NewThresholdMonitor(srv.Client(), srv.URL, AuthHeaders{},
		WithErrorCallback(func(err error) { gotErr = err }),
		WithCriticalCallback(func(ThresholdInfo) { critical = true }),
	)
	m.check(t.Context())
	if gotErr != nil || !critical {
		t.Fatalf("err = %v, critical = %v; want nil, true", gotErr, critical)
	}
}

// A body that fails mid-read is reported as a read error and fires no
// callback.
func TestThresholdMonitorReportsReadError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "100")
		_, _ = w.Write([]byte(`{"currentTimeframeRequestCount":9500,`))
	}))
	defer srv.Close()
	var gotErr error
	critical := false
	m := NewThresholdMonitor(srv.Client(), srv.URL, AuthHeaders{},
		WithErrorCallback(func(err error) { gotErr = err }),
		WithCriticalCallback(func(ThresholdInfo) { critical = true }),
	)
	m.check(t.Context())
	if gotErr == nil || !strings.Contains(gotErr.Error(), "reading response") {
		t.Fatalf("error = %v; want a read error", gotErr)
	}
	if critical {
		t.Fatal("critical callback ran on a truncated body")
	}
}
