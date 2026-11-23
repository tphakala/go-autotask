package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/tphakala/go-autotask/internal/redact"
	"github.com/tphakala/go-autotask/internal/redirect"
)

const (
	defaultCheckInterval = 5 * time.Minute
	percentMultiplier    = 100
	// maxThresholdBodyBytes caps the threshold response read; the real body is
	// a few dozen bytes.
	maxThresholdBodyBytes = 1 << 20

	// warningPercent and criticalPercent are the usage levels at which the
	// monitor calls its callbacks and logs.
	warningPercent  = 75
	criticalPercent = 90
)

// ThresholdInfo holds current API usage information relative to the threshold.
type ThresholdInfo struct {
	CurrentUsage int
	Threshold    int
	UsagePercent float64
}

// ThresholdMonitorOption configures a ThresholdMonitor.
type ThresholdMonitorOption func(*thresholdMonitorConfig)

type thresholdMonitorConfig struct {
	checkInterval    time.Duration
	warningCallback  func(ThresholdInfo)
	criticalCallback func(ThresholdInfo)
	errorCallback    func(error)
	logger           *slog.Logger
}

// WithCheckInterval sets how often the monitor polls the threshold endpoint.
// Values <= 0 are ignored.
func WithCheckInterval(d time.Duration) ThresholdMonitorOption {
	return func(c *thresholdMonitorConfig) {
		if d > 0 {
			c.checkInterval = d
		}
	}
}

// WithWarningCallback sets the function called when usage reaches the warning level (>=75%).
func WithWarningCallback(fn func(ThresholdInfo)) ThresholdMonitorOption {
	return func(c *thresholdMonitorConfig) { c.warningCallback = fn }
}

// WithCriticalCallback sets the function called when usage reaches the critical level (>=90%).
func WithCriticalCallback(fn func(ThresholdInfo)) ThresholdMonitorOption {
	return func(c *thresholdMonitorConfig) { c.criticalCallback = fn }
}

// WithErrorCallback sets the function called when the threshold check fails.
// The error's request URL has its query, user info and fragment removed.
func WithErrorCallback(fn func(error)) ThresholdMonitorOption {
	return func(c *thresholdMonitorConfig) { c.errorCallback = fn }
}

// WithThresholdMonitorLogger sets the logger the threshold monitor writes to. A
// nil l is ignored. The default discards everything.
//
// On every check the monitor logs a Warn record when usage is at or above 90%
// ("autotask: API usage reached critical threshold") or at or above 75%
// ("autotask: API usage reached warning threshold"), with the attributes
// "usage_percent", "current_usage" and "threshold", whether or not a callback
// is set. It logs a Warn record, "autotask: threshold check failed" with the
// attribute "error", for each failed check except one that fails because the
// monitor is stopping. The error has the query, user info and fragment of its
// request URL removed; the credentials are request headers and are never
// logged.
func WithThresholdMonitorLogger(l *slog.Logger) ThresholdMonitorOption {
	return func(c *thresholdMonitorConfig) {
		if l != nil {
			c.logger = l
		}
	}
}

// AuthHeaders holds the Autotask API credentials needed for authenticated requests.
type AuthHeaders struct {
	Username        string
	Secret          string
	IntegrationCode string
}

// ThresholdMonitor polls the Autotask ThresholdInformation endpoint in the
// background and invokes callbacks, and logs to the logger set with
// WithThresholdMonitorLogger, when API usage crosses warning or critical
// thresholds.
type ThresholdMonitor struct {
	httpClient *http.Client
	baseURL    string
	auth       AuthHeaders
	config     thresholdMonitorConfig
	cancel     context.CancelFunc
	done       chan struct{}

	mu      sync.Mutex
	running bool
}

// NewThresholdMonitor creates a new ThresholdMonitor. Call Start to begin polling.
// The monitor uses a copy of httpClient whose CheckRedirect removes the
// credential headers from any redirect to another scheme or host, then calls
// httpClient.CheckRedirect if it is set. httpClient is not modified.
func NewThresholdMonitor(httpClient *http.Client, baseURL string, auth AuthHeaders, opts ...ThresholdMonitorOption) *ThresholdMonitor {
	cfg := thresholdMonitorConfig{checkInterval: defaultCheckInterval, logger: slog.New(slog.DiscardHandler)}
	for _, opt := range opts {
		opt(&cfg)
	}
	return &ThresholdMonitor{
		httpClient: redirect.Guard(httpClient, baseURL), baseURL: baseURL, auth: auth, config: cfg, done: make(chan struct{}),
	}
}

// Start begins the background polling loop.
func (m *ThresholdMonitor) Start(ctx context.Context) {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return
	}
	m.running = true
	m.done = make(chan struct{})
	ctx, cancel := context.WithCancel(ctx)
	m.cancel = cancel
	m.mu.Unlock()
	go func() {
		defer close(m.done)
		// Perform an immediate check on startup before entering the ticker loop.
		m.check(ctx)
		ticker := time.NewTicker(m.config.checkInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.check(ctx)
			}
		}
	}()
}

// Stop cancels the background polling and waits for it to finish.
func (m *ThresholdMonitor) Stop() error {
	m.mu.Lock()
	if !m.running {
		m.mu.Unlock()
		return nil
	}
	m.running = false
	m.mu.Unlock()

	if m.cancel != nil {
		m.cancel()
		<-m.done
	}
	return nil
}

// reportError logs a failed check and calls the error callback. A check that
// fails because the monitor is stopping is not logged, but the callback still
// runs, as it always has.
func (m *ThresholdMonitor) reportError(ctx context.Context, err error) {
	if ctx.Err() == nil {
		m.config.logger.LogAttrs(ctx, slog.LevelWarn, "autotask: threshold check failed", slog.Any("error", err))
	}
	if m.config.errorCallback != nil {
		m.config.errorCallback(err)
	}
}

func (m *ThresholdMonitor) check(ctx context.Context) {
	checkURL := strings.TrimRight(m.baseURL, "/") + "/v1.0/ThresholdInformation"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, checkURL, http.NoBody)
	if err != nil {
		m.reportError(ctx, fmt.Errorf("threshold monitor: creating request: %w", redact.URLError(err)))
		return
	}
	// Inject auth headers — threshold endpoint requires authentication.
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("UserName", m.auth.Username)
	req.Header.Set("Secret", m.auth.Secret)
	req.Header.Set("ApiIntegrationCode", m.auth.IntegrationCode)

	resp, err := m.httpClient.Do(req)
	if err != nil {
		m.reportError(ctx, fmt.Errorf("threshold monitor: request failed: %w", redact.URLError(err)))
		return
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		m.reportError(ctx, fmt.Errorf("threshold monitor: unexpected status %d", resp.StatusCode))
		return
	}
	var data struct {
		CurrentCount int `json:"currentTimeframeRequestCount"`
		Threshold    int `json:"externalRequestThreshold"`
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxThresholdBodyBytes+1))
	if err != nil {
		m.reportError(ctx, fmt.Errorf("threshold monitor: reading response: %w", err))
		return
	}
	if len(body) > maxThresholdBodyBytes {
		m.reportError(ctx, fmt.Errorf("threshold monitor: response body exceeds %d bytes", maxThresholdBodyBytes))
		return
	}
	if err := json.Unmarshal(body, &data); err != nil {
		m.reportError(ctx, fmt.Errorf("threshold monitor: decoding response: %w", err))
		return
	}
	if data.Threshold == 0 {
		return
	}
	info := ThresholdInfo{
		CurrentUsage: data.CurrentCount,
		Threshold:    data.Threshold,
		UsagePercent: float64(data.CurrentCount) / float64(data.Threshold) * percentMultiplier,
	}
	m.notify(ctx, info)
}

// notify logs the usage level and then calls the callback for it. A critical
// usage with no critical callback falls through to the warning callback.
func (m *ThresholdMonitor) notify(ctx context.Context, info ThresholdInfo) {
	attrs := []slog.Attr{
		slog.Float64("usage_percent", info.UsagePercent),
		slog.Int("current_usage", info.CurrentUsage),
		slog.Int("threshold", info.Threshold),
	}
	switch {
	case info.UsagePercent >= criticalPercent:
		m.config.logger.LogAttrs(ctx, slog.LevelWarn, "autotask: API usage reached critical threshold", attrs...)
	case info.UsagePercent >= warningPercent:
		m.config.logger.LogAttrs(ctx, slog.LevelWarn, "autotask: API usage reached warning threshold", attrs...)
	}
	if info.UsagePercent >= criticalPercent && m.config.criticalCallback != nil {
		m.config.criticalCallback(info)
	} else if info.UsagePercent >= warningPercent && m.config.warningCallback != nil {
		m.config.warningCallback(info)
	}
}
