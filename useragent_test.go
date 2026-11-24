package autotask_test

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/tphakala/go-autotask"
	"github.com/tphakala/go-autotask/internal/useragent"
	"github.com/tphakala/go-autotask/middleware"
)

// TestNewClientUserAgent pins that the client sends one User-Agent on the
// version lookup, the zone lookup, an API request and the threshold check, and
// that middleware.WithThresholdMonitorUserAgent replaces it for the threshold
// check only. internal/useragent's TestFor pins how the version is chosen; that
// useragent.Default reads the real build info is not pinned, because a test
// binary carries no module version.
func TestNewClientUserAgent(t *testing.T) {
	t.Parallel()
	const (
		versionPath   = "/atservicesrest/versioninformation"
		zonePath      = "/atservicesrest/V1.0/zoneInformation"
		thresholdPath = "/v1.0/ThresholdInformation"
		apiPath       = "/v1.0/Tickets/1"
	)
	for _, tt := range []struct {
		name string
		opts []autotask.ClientOption
		mon  []middleware.ThresholdMonitorOption
		want map[string]string // User-Agent per request path
	}{
		{
			name: "default",
			want: map[string]string{
				versionPath: useragent.Default(), zonePath: useragent.Default(),
				thresholdPath: useragent.Default(), apiPath: useragent.Default(),
			},
		},
		{
			name: "WithUserAgent reaches every request",
			opts: []autotask.ClientOption{autotask.WithUserAgent("custom/1")},
			want: map[string]string{
				versionPath: "custom/1", zonePath: "custom/1", thresholdPath: "custom/1", apiPath: "custom/1",
			},
		},
		{
			name: "monitor option wins for the threshold check",
			opts: []autotask.ClientOption{autotask.WithUserAgent("custom/1")},
			mon:  []middleware.ThresholdMonitorOption{middleware.WithThresholdMonitorUserAgent("monitor/1")},
			want: map[string]string{
				versionPath: "custom/1", zonePath: "custom/1", thresholdPath: "monitor/1", apiPath: "custom/1",
			},
		},
		{
			name: "empty WithUserAgent sends none",
			opts: []autotask.ClientOption{autotask.WithUserAgent("")},
			want: map[string]string{versionPath: "", zonePath: "", thresholdPath: "", apiPath: ""},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var (
				mu  sync.Mutex
				got = map[string]string{}
			)
			var srv *httptest.Server
			mux := http.NewServeMux()
			// serve records the request's User-Agent under its path and answers body().
			serve := func(path string, body func() string) {
				mux.HandleFunc("GET "+path, func(w http.ResponseWriter, r *http.Request) {
					mu.Lock()
					got[r.URL.Path] = r.Header.Get("User-Agent")
					mu.Unlock()
					_, _ = w.Write([]byte(body()))
				})
			}
			serve(versionPath, func() string { return `{"apiVersions":["V1.0"]}` })
			serve(zonePath, func() string { return `{"zoneName":"Test Zone","url":"` + srv.URL + `"}` })
			serve(thresholdPath, func() string {
				return `{"currentTimeframeRequestCount":95,"externalRequestThreshold":100}`
			})
			serve(apiPath, func() string { return `{}` })
			srv = httptest.NewServer(mux)
			t.Cleanup(srv.Close)

			critical, waitCritical := signal(t)
			opts := append([]autotask.ClientOption{
				autotask.WithZoneBaseURL(srv.URL),
				autotask.WithThresholdMonitor(append([]middleware.ThresholdMonitorOption{critical}, tt.mon...)...),
			}, tt.opts...)
			c, err := autotask.NewClient(t.Context(), autotask.AuthConfig{Username: "u", Secret: "s", IntegrationCode: "i"}, opts...)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = c.Close() })
			if err := c.Do(t.Context(), http.MethodGet, apiPath, nil, nil); err != nil {
				t.Fatal(err)
			}
			waitCritical()
			mu.Lock()
			defer mu.Unlock()
			for path, want := range tt.want {
				seen, ok := got[path]
				if !ok {
					t.Errorf("no request reached %s", path)
					continue
				}
				if seen != want {
					t.Errorf("User-Agent on %s = %q; want %q", path, seen, want)
				}
			}
		})
	}
}
