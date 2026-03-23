package autotask

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/tphakala/go-autotask/middleware"
)

const version = "0.1.0"

var _ interface{ Close() error } = (*Client)(nil)

type Client struct {
	httpClient           *http.Client
	baseURL              string
	auth                 AuthConfig
	zoneCache            *ZoneCache
	middlewares           []Middleware
	thresholdMonitorOpts []middleware.ThresholdMonitorOption
	logger               *slog.Logger
	userAgent            string
	impersonationID      int64
	closers              []func() error
}

type AuthConfig struct {
	Username        string
	Secret          string
	IntegrationCode string
}

type Middleware func(next http.RoundTripper) http.RoundTripper

func NewClient(ctx context.Context, auth AuthConfig, opts ...ClientOption) (*Client, error) {
	c := &Client{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					MinVersion: tls.VersionTLS12,
				},
			},
		},
		auth:      auth,
		zoneCache: newZoneCache(defaultZoneCacheTTL),
		logger:    slog.New(discardHandler{}),
		userAgent: "go-autotask/" + version,
	}
	for _, opt := range opts {
		opt(c)
	}
	// Apply middlewares to the HTTP transport. Clone the http.Client
	// to avoid mutating a user-provided or shared client (e.g., http.DefaultClient).
	if len(c.middlewares) > 0 {
		cloned := *c.httpClient
		transport := cloned.Transport
		if transport == nil {
			transport = http.DefaultTransport
		}
		for i := len(c.middlewares) - 1; i >= 0; i-- {
			transport = c.middlewares[i](transport)
		}
		cloned.Transport = transport
		c.httpClient = &cloned
	}
	// If no base URL override, perform zone discovery.
	if c.baseURL == "" {
		zone, err := c.resolveZone(ctx)
		if err != nil {
			return nil, err
		}
		c.baseURL = zone.URL
	}
	// Start threshold monitor if configured.
	if len(c.thresholdMonitorOpts) > 0 {
		auth := middleware.AuthHeaders{
			Username:        c.auth.Username,
			Secret:          c.auth.Secret,
			IntegrationCode: c.auth.IntegrationCode,
		}
		monitor := middleware.NewThresholdMonitor(c.httpClient, c.baseURL, auth, c.thresholdMonitorOpts...)
		monitor.Start()
		c.closers = append(c.closers, monitor.Stop)
	}
	return c, nil
}

func (c *Client) Close() error {
	var errs []error
	for _, closer := range c.closers {
		if err := closer(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (c *Client) resolveZone(ctx context.Context) (*ZoneInfo, error) {
	if zone, ok := c.zoneCache.Get(c.auth.Username); ok {
		return zone, nil
	}
	zone, err := discoverZone(ctx, c.httpClient, defaultZoneBaseURL, c.auth.Username)
	if err != nil {
		return nil, err
	}
	c.zoneCache.Set(c.auth.Username, zone)
	return zone, nil
}

// do executes an HTTP request. path is appended to baseURL unless it starts
// with "http" (absolute URL from pagination nextPageUrl).
func (c *Client) do(ctx context.Context, method, path string, body any, result any) error {
	var bodyReader *bytes.Buffer
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("autotask: marshaling request body: %w", err)
		}
		bodyReader = bytes.NewBuffer(b)
	}
	requestURL := path
	if !strings.HasPrefix(path, "http") {
		requestURL = c.baseURL + path
	}
	var req *http.Request
	var err error
	if bodyReader != nil {
		req, err = http.NewRequestWithContext(ctx, method, requestURL, bodyReader)
	} else {
		req, err = http.NewRequestWithContext(ctx, method, requestURL, nil)
	}
	if err != nil {
		return fmt.Errorf("autotask: creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", c.userAgent)
	// Only attach credentials if the request targets our base URL to prevent
	// credential leaks to external hosts (e.g., via a spoofed nextPageUrl).
	if c.baseURL != "" && strings.HasPrefix(requestURL, c.baseURL) {
		req.Header.Set("UserName", c.auth.Username)
		req.Header.Set("Secret", c.auth.Secret)
		req.Header.Set("ApiIntegrationCode", c.auth.IntegrationCode)
		if c.impersonationID != 0 {
			req.Header.Set("ImpersonationResourceId", strconv.FormatInt(c.impersonationID, 10))
		}
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("autotask: request failed: %w", err)
	}
	defer resp.Body.Close()
	return parseResponse(resp, result)
}

// Do is the exported version of do for sub-packages (metadata, autotasktest).
func (c *Client) Do(ctx context.Context, method, path string, body any, result any) error {
	return c.do(ctx, method, path, body, result)
}

type discardHandler struct{}

func (discardHandler) Enabled(context.Context, slog.Level) bool  { return false }
func (discardHandler) Handle(context.Context, slog.Record) error { return nil }
func (d discardHandler) WithAttrs([]slog.Attr) slog.Handler      { return d }
func (d discardHandler) WithGroup(string) slog.Handler            { return d }
