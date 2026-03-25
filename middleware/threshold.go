package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

const (
	defaultCheckInterval = 5 * time.Minute
	percentMultiplier    = 100
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
func WithErrorCallback(fn func(error)) ThresholdMonitorOption {
	return func(c *thresholdMonitorConfig) { c.errorCallback = fn }
}

// AuthHeaders holds the Autotask API credentials needed for authenticated requests.
type AuthHeaders struct {
	Username        string
	Secret          string
	IntegrationCode string
}

// ThresholdMonitor polls the Autotask ThresholdInformation endpoint in the
// background and invokes callbacks when API usage crosses warning or critical
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
func NewThresholdMonitor(httpClient *http.Client, baseURL string, auth AuthHeaders, opts ...ThresholdMonitorOption) *ThresholdMonitor {
	cfg := thresholdMonitorConfig{checkInterval: defaultCheckInterval}
	for _, opt := range opts {
		opt(&cfg)
	}
	return &ThresholdMonitor{
		httpClient: httpClient, baseURL: baseURL, auth: auth, config: cfg, done: make(chan struct{}),
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

func (m *ThresholdMonitor) reportError(err error) {
	if m.config.errorCallback != nil {
		m.config.errorCallback(err)
	}
}

func (m *ThresholdMonitor) check(ctx context.Context) {
	checkURL := m.baseURL + "/v1.0/ThresholdInformation"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, checkURL, http.NoBody)
	if err != nil {
		m.reportError(fmt.Errorf("threshold monitor: creating request: %w", err))
		return
	}
	// Inject auth headers — threshold endpoint requires authentication.
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("UserName", m.auth.Username)
	req.Header.Set("Secret", m.auth.Secret)
	req.Header.Set("ApiIntegrationCode", m.auth.IntegrationCode)

	resp, err := m.httpClient.Do(req)
	if err != nil {
		m.reportError(fmt.Errorf("threshold monitor: request failed: %w", err))
		return
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		m.reportError(fmt.Errorf("threshold monitor: unexpected status %d", resp.StatusCode))
		return
	}
	var data struct {
		CurrentCount int `json:"currentTimeframeRequestCount"`
		Threshold    int `json:"externalRequestThreshold"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		m.reportError(fmt.Errorf("threshold monitor: decoding response: %w", err))
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
	if info.UsagePercent >= 90 && m.config.criticalCallback != nil {
		m.config.criticalCallback(info)
	} else if info.UsagePercent >= 75 && m.config.warningCallback != nil {
		m.config.warningCallback(info)
	}
}
