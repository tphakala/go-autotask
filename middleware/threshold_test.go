package middleware

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tphakala/go-autotask/internal/baseurl"
	"github.com/tphakala/go-autotask/internal/logtest"
	"github.com/tphakala/go-autotask/internal/useragent"
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

const (
	warningMsg  = "autotask: API usage reached warning threshold"
	criticalMsg = "autotask: API usage reached critical threshold"
	failedMsg   = "autotask: threshold check failed"
)

var testAuth = AuthHeaders{Username: "user-secret-name", Secret: "secret-value", IntegrationCode: "secret-code"}

// checkOnce runs one threshold check against handler with a recording logger and
// a warning and an error callback. It fails the test when the credential
// headers did not reach the server or appear in a record, so the leak check is
// not vacuous.
func checkOnce(t *testing.T, handler http.HandlerFunc) (recs []logtest.Record, warned bool, cbErr error) {
	t.Helper()
	rec := logtest.New()
	var gotUser atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser.Store(r.Header.Get("UserName"))
		handler(w, r)
	}))
	defer srv.Close()
	var warnedFlag atomic.Bool
	var callbackErr atomic.Pointer[error]
	m := NewThresholdMonitor(srv.Client(), srv.URL, testAuth,
		WithThresholdMonitorLogger(rec.Logger()),
		WithWarningCallback(func(ThresholdInfo) { warnedFlag.Store(true) }),
		WithErrorCallback(func(err error) { callbackErr.Store(&err) }),
	)
	m.check(t.Context())

	if gotUser.Load() != testAuth.Username {
		t.Fatal("the credential headers did not reach the server, so the leak check below proves nothing")
	}
	recs = rec.Records()
	for _, r := range recs {
		for _, secret := range []string{testAuth.Username, testAuth.Secret, testAuth.IntegrationCode, "secret-q"} {
			if strings.Contains(r.Text, secret) {
				t.Errorf("record contains %q: %s", secret, r.Text)
			}
		}
	}
	if ce := callbackErr.Load(); ce != nil {
		cbErr = *ce
	}
	return recs, warnedFlag.Load(), cbErr
}

func TestThresholdMonitorLogsUsageLevels(t *testing.T) {
	usage := func(count int) http.HandlerFunc {
		return func(w http.ResponseWriter, _ *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"currentTimeframeRequestCount": count,
				"externalRequestThreshold":     100,
			})
		}
	}
	tests := []struct {
		name       string
		count      int
		wantMsg    string // "" for no record
		wantWarned bool   // the warning callback ran
	}{
		{"warning level", 80, warningMsg, true},
		{"exactly the warning level", 75, warningMsg, true},
		{"critical level", 95, criticalMsg, true},
		{"exactly the critical level", 90, criticalMsg, true},
		{"just below warning", 74, "", false},
		{"below warning", 50, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recs, warned, _ := checkOnce(t, usage(tt.count))
			if warned != tt.wantWarned {
				t.Errorf("warning callback ran = %v; want %v", warned, tt.wantWarned)
			}
			if tt.wantMsg == "" {
				if len(recs) != 0 {
					t.Fatalf("got %d records; want none", len(recs))
				}
				return
			}
			if len(recs) != 1 {
				t.Fatalf("got %d records; want 1", len(recs))
			}
			if recs[0].Level != slog.LevelWarn || recs[0].Message != tt.wantMsg {
				t.Errorf("record = %s %q; want Warn %q", recs[0].Level, recs[0].Message, tt.wantMsg)
			}
			for _, key := range []string{"usage_percent", "current_usage", "threshold"} {
				if !recs[0].Has(key) {
					t.Errorf("record has no %s attribute", key)
				}
			}
		})
	}
}

// TestThresholdMonitorLogsFailedChecks also pins that the error callback still
// runs and that a redirect target's query string reaches neither the record nor
// the callback.
func TestThresholdMonitorLogsFailedChecks(t *testing.T) {
	tests := []struct {
		name    string
		handler http.HandlerFunc
	}{
		{"server error", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusInternalServerError) }},
		{"failed redirect", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "http://127.0.0.1:1/x?token=secret-q", http.StatusTemporaryRedirect)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recs, _, cbErr := checkOnce(t, tt.handler)
			if len(recs) != 1 {
				t.Fatalf("got %d records; want 1", len(recs))
			}
			if recs[0].Level != slog.LevelWarn || recs[0].Message != failedMsg || !recs[0].Has("error") {
				t.Errorf("record = %s %q (error attribute %v); want Warn %q with an error", recs[0].Level, recs[0].Message, recs[0].Has("error"), failedMsg)
			}
			if cbErr == nil {
				t.Fatal("the error callback must still run")
			}
			if strings.Contains(cbErr.Error(), "secret-q") {
				t.Errorf("the error callback received the query string: %v", cbErr)
			}
		})
	}
}

// TestThresholdMonitorCriticalFallsThroughToWarningCallback pins the behavior
// the monitor has always had: critical usage with no critical callback calls the
// warning callback, and the log record follows the usage, not the callbacks.
func TestThresholdMonitorCriticalFallsThroughToWarningCallback(t *testing.T) {
	recs, warned, _ := checkOnce(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"currentTimeframeRequestCount": 95, "externalRequestThreshold": 100})
	})
	if !warned {
		t.Error("the warning callback must run at critical usage when only it is set")
	}
	if len(recs) != 1 || recs[0].Message != criticalMsg {
		t.Errorf("records = %v; want one critical record", recs)
	}
}

func TestNilLoggerOptionsKeepEarlierLogger(t *testing.T) {
	t.Run("circuit breaker", func(t *testing.T) {
		rec := logtest.New()
		cb := NewCircuitBreaker(&mockTransport{responses: []*http.Response{{StatusCode: http.StatusServiceUnavailable, Body: http.NoBody}}},
			WithFailureThreshold(1), WithCircuitBreakerLogger(rec.Logger()), WithCircuitBreakerLogger(nil))
		_ = send(t, cb)
		if got := len(rec.Records()); got != 1 {
			t.Errorf("got %d records; want 1", got)
		}
	})
	t.Run("rate limiter", func(t *testing.T) {
		rec := logtest.New()
		inner := &mockTransport{responses: []*http.Response{
			{StatusCode: http.StatusTooManyRequests, Header: http.Header{"Retry-After": []string{"1"}}, Body: http.NoBody},
		}}
		rl := NewRateLimiter(inner, WithRateLimitLogger(rec.Logger()), WithRateLimitLogger(nil))
		if err := send(t, rl); err != nil {
			t.Fatal(err)
		}
		if got := len(rec.Records()); got != 1 {
			t.Errorf("got %d records; want 1", got)
		}
	})
	t.Run("threshold monitor", func(t *testing.T) {
		rec := logtest.New()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer srv.Close()
		m := NewThresholdMonitor(srv.Client(), srv.URL, AuthHeaders{},
			WithThresholdMonitorLogger(rec.Logger()), WithThresholdMonitorLogger(nil))
		m.check(t.Context())
		if got := len(rec.Records()); got != 1 {
			t.Errorf("got %d records; want 1", got)
		}
	})
}

// TestNewThresholdMonitorRejectsBaseURL pins that a base URL the client's rule
// refuses sends no request, and that every check reports the same error the
// shared rule gives, without the URL in it.
func TestNewThresholdMonitorRejectsBaseURL(t *testing.T) {
	const secret = "s3cret-pw"
	tests := []struct {
		name string
		url  string
	}{
		{"query", "https://host.example/?x=1"},
		{"fragment", "https://host.example/#f"},
		{"user info", "https://apiuser:" + secret + "@host.example/"},
		{"other scheme", "ftp://host.example"},
		{"empty", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			noNetwork := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				t.Errorf("unexpected request %s %s", r.Method, r.URL)
				return nil, http.ErrHandlerTimeout
			})}
			rec := logtest.New()
			var mu sync.Mutex
			var cbErrs []error
			m := NewThresholdMonitor(noNetwork, tt.url, AuthHeaders{},
				WithThresholdMonitorLogger(rec.Logger()),
				WithErrorCallback(func(err error) {
					mu.Lock()
					defer mu.Unlock()
					cbErrs = append(cbErrs, err)
				}))
			m.check(t.Context())
			m.check(t.Context())

			want := baseurl.Check("threshold monitor: base URL", tt.url)
			if want == nil {
				t.Fatalf("baseurl.Check accepted %q; the row needs a URL it rejects", tt.url)
			}
			recs := rec.Records()
			if len(recs) != 2 {
				t.Fatalf("got %d records; want 2, one per check", len(recs))
			}
			for _, r := range recs {
				if r.Message != failedMsg {
					t.Errorf("record message = %q; want %q", r.Message, failedMsg)
				}
				if strings.Contains(r.Text, secret) {
					t.Errorf("record %q repeats the password from the URL", r.Text)
				}
			}
			mu.Lock()
			defer mu.Unlock()
			if len(cbErrs) != 2 {
				t.Fatalf("got %d callback errors; want 2, one per check", len(cbErrs))
			}
			for _, err := range cbErrs {
				if err.Error() != want.Error() {
					t.Errorf("callback error = %q; want %q", err, want)
				}
				if strings.Contains(err.Error(), secret) {
					t.Errorf("error %q repeats the password from the URL", err)
				}
			}
		})
	}
}

// TestThresholdMonitorCriticalCallsOnlyTheCriticalCallback pins that critical
// usage with both callbacks set calls the critical one and not the warning one.
func TestThresholdMonitorCriticalCallsOnlyTheCriticalCallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"currentTimeframeRequestCount": 95, "externalRequestThreshold": 100})
	}))
	defer srv.Close()
	var critical, warning atomic.Int32
	m := NewThresholdMonitor(srv.Client(), srv.URL, AuthHeaders{},
		WithCriticalCallback(func(ThresholdInfo) { critical.Add(1) }),
		WithWarningCallback(func(ThresholdInfo) { warning.Add(1) }))
	m.check(t.Context())
	if critical.Load() != 1 || warning.Load() != 0 {
		t.Errorf("critical callback ran %d times and warning %d; want 1 and 0", critical.Load(), warning.Load())
	}
}

// TestThresholdMonitorStoppingCheckIsNotLogged pins that a check that fails
// because the monitor's context ended writes no record, and that the error
// callback still runs.
func TestThresholdMonitorStoppingCheckIsNotLogged(t *testing.T) {
	rec := logtest.New()
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer srv.Close()
	var cbErr atomic.Pointer[error]
	m := NewThresholdMonitor(srv.Client(), srv.URL, AuthHeaders{},
		WithThresholdMonitorLogger(rec.Logger()), WithErrorCallback(func(err error) { cbErr.Store(&err) }))
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	m.check(ctx)
	if got := rec.Records(); len(got) != 0 {
		t.Errorf("got %d records for a stopping monitor; want none", len(got))
	}
	if cbErr.Load() == nil {
		t.Error("the error callback must still run")
	}
}

// TestThresholdMonitorSendsUserAgent pins the User-Agent of the threshold
// check: the client's default without options, the value of
// WithThresholdMonitorUserAgent otherwise, and none for an empty value.
func TestThresholdMonitorSendsUserAgent(t *testing.T) {
	for _, tt := range []struct {
		name string
		opts []ThresholdMonitorOption
		want string
	}{
		{"default", nil, useragent.Default()},
		{"option", []ThresholdMonitorOption{WithThresholdMonitorUserAgent("monitor/1")}, "monitor/1"},
		{"empty option sends none", []ThresholdMonitorOption{WithThresholdMonitorUserAgent("")}, ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var got atomic.Pointer[string]
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				ua := r.Header.Get("User-Agent")
				got.Store(&ua)
				_ = json.NewEncoder(w).Encode(map[string]any{"currentTimeframeRequestCount": 1, "externalRequestThreshold": 100})
			}))
			defer srv.Close()
			m := NewThresholdMonitor(srv.Client(), srv.URL, AuthHeaders{}, tt.opts...)
			m.check(t.Context())
			ua := got.Load()
			if ua == nil {
				t.Fatal("the monitor sent no request")
			}
			if *ua != tt.want {
				t.Errorf("User-Agent = %q; want %q", *ua, tt.want)
			}
		})
	}
}
