package autotasktest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	autotask "github.com/tphakala/go-autotask"
)

type MockOption func(*mockConfig)

type mockConfig struct {
	fixtures []fixture
	latency  time.Duration
}

type fixture struct {
	method string
	path   string
	status int
	body   any
}

func WithFixture(method, path string, status int, body any) MockOption {
	return func(c *mockConfig) {
		c.fixtures = append(c.fixtures, fixture{method: method, path: path, status: status, body: body})
	}
}

func WithLatency(d time.Duration) MockOption {
	return func(c *mockConfig) { c.latency = d }
}

func NewMockClient(t *testing.T, opts ...MockOption) *autotask.Client {
	t.Helper()
	cfg := &mockConfig{}
	for _, opt := range opts {
		opt(cfg)
	}
	mux := http.NewServeMux()
	for _, f := range cfg.fixtures {
		pattern := f.method + " " + f.path
		mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
			if cfg.latency > 0 {
				time.Sleep(cfg.latency)
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(f.status)
			if f.body != nil {
				if err := json.NewEncoder(w).Encode(f.body); err != nil {
					http.Error(w, "mock encode error: "+err.Error(), http.StatusInternalServerError)
				}
			}
		})
	}
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	auth := autotask.AuthConfig{Username: "test", Secret: "test", IntegrationCode: "test"}
	client, err := autotask.NewClient(context.Background(), auth, autotask.WithBaseURL(srv.URL))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}
