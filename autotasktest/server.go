package autotasktest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	autotask "github.com/tphakala/go-autotask"
)

// defaultPageSize is the default maximum number of items per page.
const defaultPageSize = 500

// RecordedRequest captures details of an HTTP request for test assertions.
type RecordedRequest struct {
	Method  string
	Path    string
	Headers http.Header
	Body    []byte
}

// TestServer is a validating mock Autotask REST API server for testing.
type TestServer struct {
	*httptest.Server
	mu       sync.Mutex
	entities map[string]*entityStore
	auth     authConfig
	requests []RecordedRequest
	opts     serverOptions
	t        testing.TB
	nextID   int64
}

type authConfig struct {
	username        string
	secret          string
	integrationCode string
}

type serverOptions struct {
	pageSize   int
	latency    time.Duration
	errorRules []errorRule
	metadata   map[string]*entityMetadata
	zoneInfo   *zoneConfig
}

type errorRule struct {
	method     string
	pathSuffix string
	status     int
	errors     []string
	headers    map[string]string // extra headers to set on error response (e.g., Retry-After)
}

type zoneConfig struct {
	name string
	url  string
}

type entityMetadata struct {
	info   *EntityInfoResponse
	fields []FieldInfoResponse
	udfs   []UDFInfoResponse
}

// EntityInfoResponse is exported for use with WithEntityMetadata.
type EntityInfoResponse struct {
	Name                 string `json:"name"`
	CanCreate            bool   `json:"canCreate"`
	CanUpdate            bool   `json:"canUpdate"`
	CanDelete            bool   `json:"canDelete"`
	CanQuery             bool   `json:"canQuery"`
	HasUserDefinedFields bool   `json:"hasUserDefinedFields"`
}

// FieldInfoResponse is exported for use with WithEntityMetadata.
type FieldInfoResponse struct {
	Name       string `json:"name"`
	Label      string `json:"label"`
	DataType   string `json:"dataType"`
	IsRequired bool   `json:"isRequired"`
	IsReadOnly bool   `json:"isReadOnly"`
	IsPickList bool   `json:"isPickList"`
}

// UDFInfoResponse is exported for use with WithEntityMetadata.
type UDFInfoResponse struct {
	Name       string `json:"name"`
	Label      string `json:"label"`
	DataType   string `json:"dataType"`
	IsRequired bool   `json:"isRequired"`
}

// NewServer creates a new TestServer and a Client pointed at it.
// The server is cleaned up automatically when the test finishes.
func NewServer(tb testing.TB, opts ...ServerOption) (*TestServer, *autotask.Client) {
	tb.Helper()
	ts := &TestServer{
		entities: make(map[string]*entityStore),
		auth: authConfig{
			username:        "test-user",
			secret:          "test-secret",
			integrationCode: "test-code",
		},
		opts: serverOptions{
			pageSize: defaultPageSize,
			metadata: make(map[string]*entityMetadata),
		},
		t:      tb,
		nextID: 1,
	}
	for _, opt := range opts {
		opt(ts)
	}
	ts.Server = httptest.NewServer(ts.handler())
	tb.Cleanup(ts.Close)

	auth := autotask.AuthConfig{
		Username:        ts.auth.username,
		Secret:          ts.auth.secret,
		IntegrationCode: ts.auth.integrationCode,
	}
	client, err := autotask.NewClient(context.Background(), auth, autotask.WithBaseURL(ts.URL))
	if err != nil {
		tb.Fatal(err)
	}
	tb.Cleanup(func() { _ = client.Close() })
	return ts, client
}

// Requests returns all recorded requests.
func (ts *TestServer) Requests() []RecordedRequest {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	cp := make([]RecordedRequest, len(ts.requests))
	copy(cp, ts.requests)
	return cp
}

// LastRequest returns the most recent recorded request.
func (ts *TestServer) LastRequest() RecordedRequest {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	if len(ts.requests) == 0 {
		ts.t.Fatal("no requests recorded")
	}
	return ts.requests[len(ts.requests)-1]
}

// RequestCount returns the number of recorded requests.
func (ts *TestServer) RequestCount() int {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	return len(ts.requests)
}

func (ts *TestServer) recordRequest(r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	// Replace body so handlers can still read it.
	r.Body = io.NopCloser(strings.NewReader(string(body)))
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.requests = append(ts.requests, RecordedRequest{
		Method:  r.Method,
		Path:    r.URL.Path,
		Headers: r.Header.Clone(),
		Body:    body,
	})
}

func (ts *TestServer) allocateID() int64 {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	id := ts.nextID
	ts.nextID++
	return id
}

func (ts *TestServer) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ts.recordRequest(r)

		if ts.opts.latency > 0 {
			time.Sleep(ts.opts.latency)
		}

		// Check for error injection rules.
		for _, rule := range ts.opts.errorRules {
			methodMatch := rule.method == "" || r.Method == rule.method
			if methodMatch && strings.HasSuffix(r.URL.Path, rule.pathSuffix) {
				for k, v := range rule.headers {
					w.Header().Set(k, v)
				}
				writeErrorResponse(w, rule.status, rule.errors)
				return
			}
		}

		// Validate auth headers.
		if err := ts.validateAuth(r); err != nil {
			writeErrorResponse(w, http.StatusUnauthorized, []string{err.Error()})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		ts.route(w, r)
	})
}

func (ts *TestServer) route(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// Zone discovery.
	if strings.HasSuffix(path, "/zoneInformation") {
		ts.handleZoneInfo(w, r)
		return
	}
	if strings.HasSuffix(path, "/versioninformation") {
		ts.handleVersionInfo(w, r)
		return
	}

	// Threshold info.
	if strings.HasSuffix(path, "/ThresholdInformation") {
		ts.handleThresholdInfo(w, r)
		return
	}

	// Entity metadata.
	if strings.HasSuffix(path, "/entityInformation/fields") {
		ts.handleEntityFields(w, r)
		return
	}
	if strings.HasSuffix(path, "/entityInformation/userDefinedFields") {
		ts.handleEntityUDFs(w, r)
		return
	}
	if strings.HasSuffix(path, "/entityInformation") {
		ts.handleEntityInfo(w, r)
		return
	}

	// CRUD operations — extract entity name from path.
	// Paths: /v1.0/{entity}/{id}, /v1.0/{entity}/query, /v1.0/{entity}/query/count,
	//        /v1.0/{entity}, /v1.0/{parent}/{parentID}/{child}/...
	switch r.Method {
	case http.MethodGet:
		ts.handleGet(w, r)
	case http.MethodPost:
		switch {
		case strings.HasSuffix(path, "/query/count"):
			ts.handleCount(w, r)
		case strings.HasSuffix(path, "/query"):
			ts.handleQuery(w, r)
		default:
			ts.handleCreate(w, r)
		}
	case http.MethodPatch:
		ts.handleUpdate(w, r)
	case http.MethodDelete:
		ts.handleDelete(w, r)
	default:
		writeErrorResponse(w, http.StatusMethodNotAllowed, []string{"Method not allowed"})
	}
}

func writeErrorResponse(w http.ResponseWriter, status int, errors []string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"errors": errors}) //nolint:errchkjson // test helper
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v) //nolint:errchkjson // test helper
}
