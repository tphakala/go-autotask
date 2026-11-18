package autotask

import (
	"net/http"
	"net/http/httptest"
	"runtime/debug"
	"testing"
)

func TestUserAgentFor(t *testing.T) {
	t.Parallel()
	dep := func(version string, replace *debug.Module) *debug.BuildInfo {
		return &debug.BuildInfo{
			Main: debug.Module{Path: "example.com/app", Version: "v9.9.9"},
			Deps: []*debug.Module{
				{Path: "golang.org/x/time", Version: "v0.1.0"},
				{Path: modulePath, Version: version, Replace: replace},
			},
		}
	}
	main := func(version string) *debug.BuildInfo {
		return &debug.BuildInfo{Main: debug.Module{Path: modulePath, Version: version}}
	}
	for _, tt := range []struct {
		name string
		info *debug.BuildInfo
		want string
	}{
		{"nil info", nil, "go-autotask"},
		{"dependency version, not the consumer's", dep("v1.5.1", nil), "go-autotask/1.5.1"},
		{"local replace has no version", dep("v1.5.1", &debug.Module{Path: "../go-autotask", Version: "(devel)"}), "go-autotask"},
		{"replace version decides", dep("v1.5.1", &debug.Module{Path: "example.com/fork", Version: "v1.7.0"}), "go-autotask/1.7.0"},
		{"workspace checkout", dep("(devel)", nil), "go-autotask"},
		{"other main module, no dependency entry", &debug.BuildInfo{Main: debug.Module{Path: "example.com/app", Version: "v1.0.0"}}, "go-autotask"},
		{"main module in a test binary", main("(devel)"), "go-autotask"},
		{"main module pseudo-version", main("v1.5.2-0.20261118121938-dc337645bbfd+dirty"), "go-autotask/1.5.2-0.20261118121938-dc337645bbfd+dirty"},
		{"main module with no version", main(""), "go-autotask"},
		{"version with characters outside a module version", dep("v1.5.1 (devel)", nil), "go-autotask"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := userAgentFor(tt.info); got != tt.want {
				t.Errorf("userAgentFor = %q; want %q", got, tt.want)
			}
		})
	}
}

// TestNewClientUserAgent pins that a default client sends defaultUserAgent() on
// an API request and that WithUserAgent replaces it. The content of the default
// is pinned by TestUserAgentFor.
func TestNewClientUserAgent(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name string
		opts []ClientOption
		want string
	}{
		{"default", nil, defaultUserAgent()},
		{"override", []ClientOption{WithUserAgent("custom/1")}, "custom/1"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var got string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				got = r.Header.Get("User-Agent")
				_, _ = w.Write([]byte(`{}`))
			}))
			t.Cleanup(srv.Close)
			opts := append([]ClientOption{WithBaseURL(srv.URL)}, tt.opts...)
			c, err := NewClient(t.Context(), AuthConfig{Username: "u", Secret: "s", IntegrationCode: "i"}, opts...)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = c.Close() })
			if err := c.do(t.Context(), http.MethodGet, "/v1.0/Tickets/1", nil, nil); err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("User-Agent = %q; want %q", got, tt.want)
			}
		})
	}
}
