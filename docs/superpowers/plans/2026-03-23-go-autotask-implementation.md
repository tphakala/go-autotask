# go-autotask Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a generic, open-source Go 1.26 client library for the Autotask PSA REST API with type-safe generics, composable middleware, and code-generated entity types.

**Architecture:** Core client in root `autotask` package handles auth/zone discovery/HTTP. Generic CRUD functions use Go generics with an `Entity` interface. Resilience (rate limiter, circuit breaker, threshold monitor) implemented as `http.RoundTripper` middleware in a `middleware/` package. Pre-generated entity types in `entities/`, runtime metadata in `metadata/`, code generator in `cmd/autotask-gen/`.

**Tech Stack:** Go 1.26, `golang.org/x/time/rate`, `encoding/json` with `omitzero`, `iter.Seq2`, `errors.AsType`, `slog`

**Spec:** `docs/superpowers/specs/2026-03-23-go-autotask-library-design.md`

**Reference implementation:** `/Users/e909385/src/vainu2/internal/autotask/` (existing production client to port patterns from)

---

## File Structure

```
go-autotask/
├── go.mod                             # github.com/tphakala/go-autotask
├── go.sum
├── LICENSE                            # Apache-2.0
├── optional.go                        # Optional[T] three-state type
├── optional_test.go                   # Optional JSON marshaling tests
├── entity.go                          # Entity interface, UDF type
├── error.go                           # Base Error, typed errors, response parsing
├── error_test.go                      # Error type tests, response parsing tests
├── zone.go                            # ZoneInfo, ZoneCache, zone discovery
├── zone_test.go                       # Zone cache + discovery tests
├── client.go                          # Client struct, NewClient, Close, do()
├── client_test.go                     # Client construction, auth headers, zone flow
├── option.go                          # All ClientOption functions
├── query.go                           # Query builder, Condition, serialization
├── query_test.go                      # Query builder + JSON serialization tests
├── crud.go                            # Get[T], List[T], Create[T], Update[T], Delete[T], Count[T]
├── crud_test.go                       # Generic CRUD tests with mock server
├── raw.go                             # GetRaw, ListRaw, CreateRaw, UpdateRaw, DeleteRaw
├── raw_test.go                        # Raw CRUD tests
├── iter.go                            # ListIter[T] iterator-based pagination
├── iter_test.go                       # Iterator tests
├── child.go                           # GetChild[P,C], CreateChild[P,C]
├── child_test.go                      # Child entity tests
├── middleware/
│   ├── ratelimit.go                   # Token bucket rate limiter RoundTripper
│   ├── ratelimit_test.go
│   ├── circuitbreaker.go              # Circuit breaker RoundTripper
│   ├── circuitbreaker_test.go
│   ├── threshold.go                   # Threshold monitor RoundTripper
│   └── threshold_test.go
├── entities/
│   ├── ticket.go                      # Ticket entity
│   ├── ticket_note.go                 # TicketNote child entity
│   ├── company.go                     # Company entity
│   ├── contact.go                     # Contact entity
│   ├── project.go                     # Project entity
│   ├── task.go                        # Task entity (Autotask project tasks)
│   ├── contract.go                    # Contract entity
│   ├── configuration_item.go          # ConfigurationItem entity
│   ├── resource.go                    # Resource entity
│   └── time_entry.go                  # TimeEntry entity
├── metadata/
│   ├── metadata.go                    # GetFields, GetUDFs, GetEntityInfo
│   └── metadata_test.go
├── autotasktest/
│   └── mock.go                        # Exported test helpers: NewMockClient
├── benchmark_test.go                  # Benchmarks for query, parsing, rate limiter
├── cmd/
│   └── autotask-gen/
│       ├── main.go                    # CLI entry point
│       └── generator.go               # Code generation logic
├── integration_test.go                # //go:build integration — live API tests
└── examples/
    ├── basic/main.go
    ├── query/main.go
    └── middleware/main.go
```

---

### Task 1: Project Scaffolding

**Files:**
- Create: `go.mod`
- Create: `LICENSE`

- [ ] **Step 1: Initialize Go module**

Run: `cd /Users/e909385/src/autotask && go mod init github.com/tphakala/go-autotask`

Edit `go.mod` to set Go version:
```
module github.com/tphakala/go-autotask

go 1.26
```

- [ ] **Step 2: Add Apache-2.0 LICENSE**

Create `LICENSE` with Apache-2.0 text.

- [ ] **Step 3: Add golang.org/x/time dependency**

Run: `go get golang.org/x/time/rate`

- [ ] **Step 4: Commit**

```bash
git add go.mod go.sum LICENSE
git commit -m "feat: initialize go-autotask module with Go 1.26"
```

---

### Task 2: Optional[T] Three-State Type

**Files:**
- Create: `optional.go`
- Create: `optional_test.go`

- [ ] **Step 1: Write failing tests for Optional[T]**

```go
// optional_test.go
package autotask

import (
	"encoding/json"
	"testing"
	"time"
)

func TestOptionalZeroValueIsUnset(t *testing.T) {
	var o Optional[string]
	if o.IsSet() {
		t.Fatal("zero value Optional should not be set")
	}
	if o.IsNull() {
		t.Fatal("zero value Optional should not be null")
	}
	if !o.IsZero() {
		t.Fatal("zero value Optional should be zero")
	}
}

func TestOptionalSet(t *testing.T) {
	o := Set("hello")
	if !o.IsSet() {
		t.Fatal("Set Optional should be set")
	}
	if o.IsNull() {
		t.Fatal("Set Optional should not be null")
	}
	if o.IsZero() {
		t.Fatal("Set Optional should not be zero")
	}
	v, ok := o.Get()
	if !ok || v != "hello" {
		t.Fatalf("Get() = %q, %v; want %q, true", v, ok, "hello")
	}
}

func TestOptionalNull(t *testing.T) {
	o := Null[string]()
	if !o.IsSet() {
		t.Fatal("Null Optional should be set (explicitly set to null)")
	}
	if !o.IsNull() {
		t.Fatal("Null Optional should be null")
	}
	if o.IsZero() {
		t.Fatal("Null Optional should not be zero (it's explicitly set)")
	}
}

func TestOptionalMarshalJSONSet(t *testing.T) {
	o := Set(42)
	b, err := json.Marshal(o)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "42" {
		t.Fatalf("Marshal Set(42) = %s; want 42", b)
	}
}

func TestOptionalMarshalJSONNull(t *testing.T) {
	o := Null[int]()
	b, err := json.Marshal(o)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "null" {
		t.Fatalf("Marshal Null = %s; want null", b)
	}
}

func TestOptionalOmitzeroInStruct(t *testing.T) {
	type S struct {
		Name  Optional[string] `json:"name,omitzero"`
		Value Optional[int]    `json:"value,omitzero"`
		Clear Optional[string] `json:"clear,omitzero"`
	}
	s := S{
		Name:  Set("test"),
		// Value is unset → should be omitted
		Clear: Null[string](), // explicitly null
	}
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	expected := `{"name":"test","clear":null}`
	if string(b) != expected {
		t.Fatalf("Marshal = %s; want %s", b, expected)
	}
}

func TestOptionalUnmarshalJSON(t *testing.T) {
	type S struct {
		Name  Optional[string] `json:"name,omitzero"`
		Value Optional[int]    `json:"value,omitzero"`
		Clear Optional[string] `json:"clear,omitzero"`
	}
	input := `{"name":"hello","clear":null}`
	var s S
	if err := json.Unmarshal([]byte(input), &s); err != nil {
		t.Fatal(err)
	}
	if v, ok := s.Name.Get(); !ok || v != "hello" {
		t.Fatalf("Name = %q, %v; want hello, true", v, ok)
	}
	if s.Value.IsSet() {
		t.Fatal("Value should not be set (missing from JSON)")
	}
	if !s.Clear.IsNull() {
		t.Fatal("Clear should be null")
	}
}

func TestOptionalTime(t *testing.T) {
	ts := time.Date(2026, 3, 23, 12, 0, 0, 0, time.UTC)
	o := Set(ts)
	b, err := json.Marshal(o)
	if err != nil {
		t.Fatal(err)
	}
	var o2 Optional[time.Time]
	if err := json.Unmarshal(b, &o2); err != nil {
		t.Fatal(err)
	}
	v, ok := o2.Get()
	if !ok || !v.Equal(ts) {
		t.Fatalf("round-trip time = %v, %v; want %v, true", v, ok, ts)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -run TestOptional -v ./...`
Expected: FAIL — `Optional` type not defined

- [ ] **Step 3: Implement Optional[T]**

```go
// optional.go
package autotask

import "encoding/json"

// Optional represents a three-state field: unset, null, or set to a value.
// Use with the `omitzero` struct tag so unset fields are omitted from JSON.
//   - Zero value: unset (omitted from JSON via omitzero + IsZero)
//   - Null[T](): explicitly null (serializes as JSON null, clears field in Autotask)
//   - Set(v): has a value (serializes as the JSON representation of v)
type Optional[T any] struct {
	value T
	set   bool
	null  bool
}

// Set creates an Optional with a value.
func Set[T any](v T) Optional[T] {
	return Optional[T]{value: v, set: true}
}

// Null creates an Optional that is explicitly null.
func Null[T any]() Optional[T] {
	return Optional[T]{set: true, null: true}
}

// Get returns the value and whether it is set (non-null).
func (o Optional[T]) Get() (T, bool) {
	if o.set && !o.null {
		return o.value, true
	}
	var zero T
	return zero, false
}

// IsSet returns true if the field was explicitly set (to a value or null).
func (o Optional[T]) IsSet() bool { return o.set }

// IsNull returns true if the field was explicitly set to null.
func (o Optional[T]) IsNull() bool { return o.null }

// IsZero returns true when the field is unset. Used by encoding/json with
// the omitzero struct tag to omit unset fields from JSON output.
func (o Optional[T]) IsZero() bool { return !o.set }

// MarshalJSON implements json.Marshaler.
func (o Optional[T]) MarshalJSON() ([]byte, error) {
	if o.null {
		return []byte("null"), nil
	}
	return json.Marshal(o.value)
}

// UnmarshalJSON implements json.Unmarshaler.
func (o *Optional[T]) UnmarshalJSON(data []byte) error {
	o.set = true
	if string(data) == "null" {
		o.null = true
		return nil
	}
	return json.Unmarshal(data, &o.value)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -run TestOptional -v ./...`
Expected: All PASS

- [ ] **Step 5: Commit**

```bash
git add optional.go optional_test.go
git commit -m "feat: add Optional[T] three-state type with omitzero JSON support"
```

---

### Task 3: Entity Interface and UDF Type

**Files:**
- Create: `entity.go`

- [ ] **Step 1: Implement Entity interface and UDF type**

```go
// entity.go
package autotask

// Entity is the interface all typed Autotask entities implement.
// EntityName() MUST use a value receiver to prevent double-pointer issues
// with generic CRUD functions (e.g., Get[*Ticket] would return **Ticket).
type Entity interface {
	EntityName() string
}

// UDF represents a user-defined field value.
type UDF struct {
	Name  string `json:"name"`
	Value any    `json:"value"`
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./...`
Expected: Success

- [ ] **Step 3: Commit**

```bash
git add entity.go
git commit -m "feat: add Entity interface and UDF type"
```

---

### Task 4: Error Types and Response Parsing

**Files:**
- Create: `error.go`
- Create: `error_test.go`

Reference: `/Users/e909385/src/vainu2/internal/autotask/errors.go` and `response_handler.go`

- [ ] **Step 1: Write failing tests for error types and response parsing**

```go
// error_test.go
package autotask

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestErrorImplementsError(t *testing.T) {
	err := &Error{StatusCode: 400, Message: "bad request"}
	if err.Error() == "" {
		t.Fatal("Error() should return non-empty string")
	}
}

func TestTypedErrorsAsType(t *testing.T) {
	base := Error{StatusCode: 404, Message: "not found"}
	err := &NotFoundError{Error: base}

	// Use Go 1.26 errors.AsType for type-safe error inspection.
	nf, ok := errors.AsType[*NotFoundError](err)
	if !ok {
		t.Fatal("errors.AsType should match NotFoundError")
	}
	if nf.StatusCode != 404 {
		t.Fatalf("StatusCode = %d; want 404", nf.StatusCode)
	}
}

func TestRateLimitErrorRetryAfter(t *testing.T) {
	err := &RateLimitError{
		Error:      Error{StatusCode: 429, Message: "too many requests"},
		RetryAfter: 60 * time.Second,
	}
	if err.RetryAfter != 60*time.Second {
		t.Fatalf("RetryAfter = %v; want 60s", err.RetryAfter)
	}
}

func TestParseResponse400(t *testing.T) {
	body := `{"errors":["Field Title is required"]}`
	resp := &http.Response{
		StatusCode: 400,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{},
	}
	err := parseResponse(resp, nil)
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected ValidationError, got %T: %v", err, err)
	}
}

func TestParseResponse401(t *testing.T) {
	resp := &http.Response{
		StatusCode: 401,
		Body:       io.NopCloser(strings.NewReader(`{"errors":["Invalid credentials"]}`)),
		Header:     http.Header{},
	}
	err := parseResponse(resp, nil)
	var ae *AuthenticationError
	if !errors.As(err, &ae) {
		t.Fatalf("expected AuthenticationError, got %T: %v", err, err)
	}
}

func TestParseResponse429WithRetryAfter(t *testing.T) {
	resp := &http.Response{
		StatusCode: 429,
		Body:       io.NopCloser(strings.NewReader(`{"errors":["Rate limit exceeded"]}`)),
		Header:     http.Header{"Retry-After": []string{"120"}},
	}
	err := parseResponse(resp, nil)
	rle, ok := errors.AsType[*RateLimitError](err)
	if !ok {
		t.Fatalf("expected RateLimitError, got %T: %v", err, err)
	}
	if rle.RetryAfter != 120*time.Second {
		t.Fatalf("RetryAfter = %v; want 120s", rle.RetryAfter)
	}
}

func TestParseResponse500(t *testing.T) {
	resp := &http.Response{
		StatusCode: 500,
		Body:       io.NopCloser(strings.NewReader(`{"errors":["Internal error"]}`)),
		Header:     http.Header{},
	}
	err := parseResponse(resp, nil)
	var se *ServerError
	if !errors.As(err, &se) {
		t.Fatalf("expected ServerError, got %T: %v", err, err)
	}
}

func TestParseResponse200Success(t *testing.T) {
	body := `{"item":{"id":123}}`
	resp := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{},
	}
	var result struct {
		Item struct {
			ID int `json:"id"`
		} `json:"item"`
	}
	err := parseResponse(resp, &result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Item.ID != 123 {
		t.Fatalf("ID = %d; want 123", result.Item.ID)
	}
}

func TestParseResponse200WithErrors(t *testing.T) {
	body := `{"errors":["Something went wrong"]}`
	resp := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{},
	}
	err := parseResponse(resp, nil)
	if err == nil {
		t.Fatal("expected error for 200 response with errors array")
	}
}

func TestParseRetryAfterSeconds(t *testing.T) {
	d := parseRetryAfter("30")
	if d != 30*time.Second {
		t.Fatalf("parseRetryAfter('30') = %v; want 30s", d)
	}
}

func TestParseRetryAfterDefault(t *testing.T) {
	d := parseRetryAfter("")
	if d != 60*time.Second {
		t.Fatalf("parseRetryAfter('') = %v; want 60s", d)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -run "TestError|TestParse|TestTyped|TestRateLimit" -v ./...`
Expected: FAIL — types not defined

- [ ] **Step 3: Implement error types and response parsing**

```go
// error.go
package autotask

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

// Error is the base error type for all Autotask API errors.
type Error struct {
	StatusCode int
	Message    string
	Errors     []APIError
}

// APIError represents a single error from the Autotask API response.
type APIError struct {
	Message string `json:"message"`
	Field   string `json:"field,omitempty"`
}

func (e *Error) Error() string {
	if len(e.Errors) > 0 {
		return fmt.Sprintf("autotask: %d %s: %s", e.StatusCode, e.Message, e.Errors[0].Message)
	}
	return fmt.Sprintf("autotask: %d %s", e.StatusCode, e.Message)
}

// Typed errors per HTTP status code.
type ValidationError struct{ Error }     // 400
type AuthenticationError struct{ Error } // 401
type AuthorizationError struct{ Error }  // 403
type NotFoundError struct{ Error }       // 404
type ConflictError struct{ Error }       // 409
type BusinessLogicError struct{ Error }  // 422

// RateLimitError is returned when the API returns 429. RetryAfter indicates
// how long to wait before retrying.
type RateLimitError struct {
	Error
	RetryAfter time.Duration
}

type ServerError struct{ Error } // 5xx

// parseResponse reads the HTTP response, checks for errors, and optionally
// unmarshals the body into result. Returns a typed error for non-2xx responses.
// Even 200 responses are checked for error payloads.
func parseResponse(resp *http.Response, result any) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("autotask: reading response body: %w", err)
	}

	// Check for errors in any response (even 200).
	apiErrors := extractErrors(body)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		if len(apiErrors) > 0 {
			return &Error{
				StatusCode: resp.StatusCode,
				Message:    "unexpected error in success response",
				Errors:     apiErrors,
			}
		}
		if result != nil && len(body) > 0 {
			if err := json.Unmarshal(body, result); err != nil {
				return fmt.Errorf("autotask: decoding response: %w", err)
			}
		}
		return nil
	}

	base := Error{
		StatusCode: resp.StatusCode,
		Message:    http.StatusText(resp.StatusCode),
		Errors:     apiErrors,
	}

	switch {
	case resp.StatusCode == 400:
		return &ValidationError{Error: base}
	case resp.StatusCode == 401:
		return &AuthenticationError{Error: base}
	case resp.StatusCode == 403:
		return &AuthorizationError{Error: base}
	case resp.StatusCode == 404:
		return &NotFoundError{Error: base}
	case resp.StatusCode == 409:
		return &ConflictError{Error: base}
	case resp.StatusCode == 422:
		return &BusinessLogicError{Error: base}
	case resp.StatusCode == 429:
		return &RateLimitError{
			Error:      base,
			RetryAfter: parseRetryAfter(resp.Header.Get("Retry-After")),
		}
	case resp.StatusCode >= 500:
		return &ServerError{Error: base}
	default:
		return &base
	}
}

// extractErrors parses the Autotask error array from a JSON response body.
func extractErrors(body []byte) []APIError {
	var envelope struct {
		Errors []json.RawMessage `json:"errors"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil || len(envelope.Errors) == 0 {
		return nil
	}
	var result []APIError
	for _, raw := range envelope.Errors {
		var ae APIError
		if err := json.Unmarshal(raw, &ae); err != nil {
			// Autotask sometimes returns errors as plain strings.
			var s string
			if err := json.Unmarshal(raw, &s); err == nil {
				ae = APIError{Message: s}
			}
		}
		result = append(result, ae)
	}
	return result
}

// parseRetryAfter parses the Retry-After header value as seconds or HTTP-date.
func parseRetryAfter(header string) time.Duration {
	if header == "" {
		return 60 * time.Second
	}
	if seconds, err := strconv.Atoi(header); err == nil {
		return time.Duration(seconds) * time.Second
	}
	if t, err := http.ParseTime(header); err == nil {
		d := time.Until(t)
		if d > 0 {
			return d
		}
	}
	return 60 * time.Second
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -run "TestError|TestParse|TestTyped|TestRateLimit" -v ./...`
Expected: All PASS

- [ ] **Step 5: Commit**

```bash
git add error.go error_test.go
git commit -m "feat: add typed errors and response parsing"
```

---

### Task 5: Zone Discovery and Cache

**Files:**
- Create: `zone.go`
- Create: `zone_test.go`

Reference: `/Users/e909385/src/vainu2/internal/autotask/zone_cache.go` and zone discovery in `client_impl.go`

- [ ] **Step 1: Write failing tests for zone cache and discovery**

```go
// zone_test.go
package autotask

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestZoneCacheSetAndGet(t *testing.T) {
	cache := newZoneCache(1 * time.Hour)
	zone := &ZoneInfo{URL: "https://webservices5.autotask.net", ZoneName: "Zone 5"}
	cache.Set("user@example.com", zone)

	got, ok := cache.Get("user@example.com")
	if !ok {
		t.Fatal("expected cache hit")
	}
	if got.URL != zone.URL {
		t.Fatalf("URL = %q; want %q", got.URL, zone.URL)
	}
}

func TestZoneCacheExpiration(t *testing.T) {
	cache := newZoneCache(1 * time.Millisecond)
	zone := &ZoneInfo{URL: "https://example.com"}
	cache.Set("user@example.com", zone)
	time.Sleep(5 * time.Millisecond)

	_, ok := cache.Get("user@example.com")
	if ok {
		t.Fatal("expected cache miss after expiration")
	}
}

func TestZoneCacheMiss(t *testing.T) {
	cache := newZoneCache(1 * time.Hour)
	_, ok := cache.Get("nobody@example.com")
	if ok {
		t.Fatal("expected cache miss for unknown user")
	}
}

func TestZoneCacheReturnsCopy(t *testing.T) {
	cache := newZoneCache(1 * time.Hour)
	zone := &ZoneInfo{URL: "https://original.com", ZoneName: "Zone 1"}
	cache.Set("user@example.com", zone)

	got, _ := cache.Get("user@example.com")
	got.URL = "https://mutated.com"

	got2, _ := cache.Get("user@example.com")
	if got2.URL != "https://original.com" {
		t.Fatalf("cache was mutated: URL = %q", got2.URL)
	}
}

func TestDiscoverZone(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /atservicesrest/versioninformation", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"versions": []string{"1.0"},
		})
	})
	mux.HandleFunc("GET /atservicesrest/1.0/zoneInformation", func(w http.ResponseWriter, r *http.Request) {
		user := r.URL.Query().Get("user")
		if user != "test@example.com" {
			http.Error(w, "bad user", 400)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"zoneName": "Zone 5",
			"url":      "https://webservices5.autotask.net/atservicesrest",
			"webUrl":   "https://ww5.autotask.net",
			"ci":       5,
		})
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	zone, err := discoverZone(context.Background(), srv.Client(), srv.URL, "test@example.com")
	if err != nil {
		t.Fatalf("discoverZone: %v", err)
	}
	if zone.ZoneName != "Zone 5" {
		t.Fatalf("ZoneName = %q; want Zone 5", zone.ZoneName)
	}
	if zone.URL != "https://webservices5.autotask.net/atservicesrest" {
		t.Fatalf("URL = %q", zone.URL)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -run TestZone -v ./...`
Expected: FAIL

- [ ] **Step 3: Implement zone cache and discovery**

```go
// zone.go
package autotask

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

const (
	defaultZoneBaseURL = "https://webservices2.autotask.net"
	defaultZoneCacheTTL = 24 * time.Hour
)

// ZoneInfo contains the Autotask zone endpoint information.
type ZoneInfo struct {
	ZoneName string `json:"zoneName"`
	URL      string `json:"url"`
	WebURL   string `json:"webUrl"`
	CI       int    `json:"ci"`
}

// ZoneCache is a thread-safe, TTL-based cache for zone information.
type ZoneCache struct {
	mu      sync.RWMutex
	entries map[string]cachedZone
	ttl     time.Duration
}

type cachedZone struct {
	zone      ZoneInfo
	expiresAt time.Time
}

func newZoneCache(ttl time.Duration) *ZoneCache {
	return &ZoneCache{
		entries: make(map[string]cachedZone),
		ttl:     ttl,
	}
}

// Get returns a copy of the cached zone and true if found and not expired.
func (c *ZoneCache) Get(username string) (*ZoneInfo, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, ok := c.entries[username]
	if !ok || time.Now().After(entry.expiresAt) {
		return nil, false
	}
	copy := entry.zone
	return &copy, true
}

// Set stores a copy of the zone with TTL-based expiration.
func (c *ZoneCache) Set(username string, zone *ZoneInfo) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[username] = cachedZone{
		zone:      *zone,
		expiresAt: time.Now().Add(c.ttl),
	}
}

// discoverZone performs Autotask zone discovery for the given username.
func discoverZone(ctx context.Context, httpClient *http.Client, baseURL, username string) (*ZoneInfo, error) {
	// Step 1: Get API versions.
	versionsURL := baseURL + "/atservicesrest/versioninformation"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, versionsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("autotask: creating version request: %w", err)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("autotask: zone discovery version request: %w", err)
	}
	defer resp.Body.Close()

	var versionResp struct {
		Versions []string `json:"versions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&versionResp); err != nil {
		return nil, fmt.Errorf("autotask: decoding version response: %w", err)
	}
	if len(versionResp.Versions) == 0 {
		return nil, fmt.Errorf("autotask: no API versions available")
	}
	version := versionResp.Versions[len(versionResp.Versions)-1]

	// Step 2: Discover zone for user.
	zoneURL := fmt.Sprintf("%s/atservicesrest/%s/zoneInformation?user=%s", baseURL, version, username)
	req, err = http.NewRequestWithContext(ctx, http.MethodGet, zoneURL, nil)
	if err != nil {
		return nil, fmt.Errorf("autotask: creating zone request: %w", err)
	}
	resp, err = httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("autotask: zone discovery request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("autotask: zone discovery returned %d", resp.StatusCode)
	}

	var zone ZoneInfo
	if err := json.NewDecoder(resp.Body).Decode(&zone); err != nil {
		return nil, fmt.Errorf("autotask: decoding zone response: %w", err)
	}
	if zone.URL == "" {
		return nil, fmt.Errorf("autotask: zone discovery returned empty URL")
	}
	return &zone, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -run TestZone -v ./...`
Expected: All PASS

- [ ] **Step 5: Commit**

```bash
git add zone.go zone_test.go
git commit -m "feat: add zone discovery and thread-safe zone cache"
```

---

### Task 6: Query Builder

**Files:**
- Create: `query.go`
- Create: `query_test.go`

- [ ] **Step 1: Write failing tests for query builder and JSON serialization**

```go
// query_test.go
package autotask

import (
	"encoding/json"
	"testing"
)

func TestQuerySimpleWhere(t *testing.T) {
	q := NewQuery().Where("status", OpEq, 1)
	b, err := json.Marshal(q)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	json.Unmarshal(b, &m)

	filter := m["filter"].([]any)
	if len(filter) != 1 {
		t.Fatalf("filter length = %d; want 1", len(filter))
	}
	f := filter[0].(map[string]any)
	if f["op"] != "eq" || f["field"] != "status" {
		t.Fatalf("filter = %v", f)
	}
}

func TestQueryMultipleWhere(t *testing.T) {
	q := NewQuery().
		Where("status", OpEq, 1).
		Where("queueID", OpEq, 8)
	b, _ := json.Marshal(q)
	var m map[string]any
	json.Unmarshal(b, &m)

	filter := m["filter"].([]any)
	if len(filter) != 2 {
		t.Fatalf("filter length = %d; want 2", len(filter))
	}
}

func TestQueryOr(t *testing.T) {
	q := NewQuery().Or(
		Field("priority", OpEq, 1),
		Field("priority", OpEq, 2),
	)
	b, _ := json.Marshal(q)
	var m map[string]any
	json.Unmarshal(b, &m)

	filter := m["filter"].([]any)
	orGroup := filter[0].(map[string]any)
	if orGroup["op"] != "or" {
		t.Fatalf("op = %v; want or", orGroup["op"])
	}
	items := orGroup["items"].([]any)
	if len(items) != 2 {
		t.Fatalf("items length = %d; want 2", len(items))
	}
}

func TestQueryNestedAndOr(t *testing.T) {
	q := NewQuery().Or(
		And(
			Field("status", OpEq, 1),
			Field("queueID", OpEq, 8),
		),
		And(
			Field("priority", OpEq, 1),
			Field("priority", OpEq, 2),
		),
	)
	b, _ := json.Marshal(q)
	var m map[string]any
	json.Unmarshal(b, &m)

	filter := m["filter"].([]any)
	orGroup := filter[0].(map[string]any)
	items := orGroup["items"].([]any)
	if len(items) != 2 {
		t.Fatalf("OR items = %d; want 2", len(items))
	}
	andGroup := items[0].(map[string]any)
	if andGroup["op"] != "and" {
		t.Fatalf("nested op = %v; want and", andGroup["op"])
	}
}

func TestQueryUDF(t *testing.T) {
	q := NewQuery().WhereUDF("CustomField", OpEq, "value")
	b, _ := json.Marshal(q)
	var m map[string]any
	json.Unmarshal(b, &m)

	filter := m["filter"].([]any)
	f := filter[0].(map[string]any)
	if f["udf"] != true {
		t.Fatalf("udf = %v; want true", f["udf"])
	}
}

func TestQueryFields(t *testing.T) {
	q := NewQuery().
		Where("status", OpEq, 1).
		Fields("id", "title", "status")
	b, _ := json.Marshal(q)
	var m map[string]any
	json.Unmarshal(b, &m)

	fields := m["IncludeFields"].([]any)
	if len(fields) != 3 {
		t.Fatalf("IncludeFields length = %d; want 3", len(fields))
	}
}

func TestQueryLimit(t *testing.T) {
	q := NewQuery().Where("status", OpEq, 1).Limit(100)
	b, _ := json.Marshal(q)
	var m map[string]any
	json.Unmarshal(b, &m)

	if m["MaxRecords"] != float64(100) {
		t.Fatalf("MaxRecords = %v; want 100", m["MaxRecords"])
	}
}

func TestQueryLimitClampedTo500(t *testing.T) {
	q := NewQuery().Limit(1000)
	b, _ := json.Marshal(q)
	var m map[string]any
	json.Unmarshal(b, &m)

	if m["MaxRecords"] != float64(500) {
		t.Fatalf("MaxRecords = %v; want 500 (clamped)", m["MaxRecords"])
	}
}

func TestFieldConstructor(t *testing.T) {
	f := Field("status", OpEq, 1)
	if f.Field != "status" || f.Op != OpEq {
		t.Fatalf("Field = %+v", f)
	}
}

func TestUDFieldConstructor(t *testing.T) {
	f := UDField("Custom", OpContains, "test")
	if !f.UDF {
		t.Fatal("UDField should set UDF=true")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -run TestQuery -v ./...`
Expected: FAIL

- [ ] **Step 3: Implement query builder**

```go
// query.go
package autotask

import "encoding/json"

// Operator is an Autotask query filter operator.
type Operator string

const (
	OpEq         Operator = "eq"
	OpNotEq      Operator = "noteq"
	OpGt         Operator = "gt"
	OpGte        Operator = "gte"
	OpLt         Operator = "lt"
	OpLte        Operator = "lte"
	OpBeginsWith Operator = "beginsWith"
	OpEndsWith   Operator = "endsWith"
	OpContains   Operator = "contains"
	OpExist      Operator = "exist"
	OpNotExist   Operator = "notExist"
	OpIn         Operator = "in"
	OpNotIn      Operator = "notIn"
)

// GroupOperator is used for AND/OR grouping.
type GroupOperator string

const (
	GroupAnd GroupOperator = "and"
	GroupOr  GroupOperator = "or"
)

// Condition is the interface for query filter expressions.
type Condition interface {
	conditionNode()
}

// FieldCondition is a simple field comparison filter.
type FieldCondition struct {
	Field string   `json:"field"`
	Op    Operator `json:"op"`
	Value any      `json:"value"`
	UDF   bool     `json:"udf,omitempty"`
}

func (FieldCondition) conditionNode() {}

// GroupCondition combines conditions with AND/OR.
type GroupCondition struct {
	Op    GroupOperator `json:"op"`
	Items []Condition   `json:"items"`
}

func (GroupCondition) conditionNode() {}

// Field creates a FieldCondition.
func Field(name string, op Operator, value any) FieldCondition {
	return FieldCondition{Field: name, Op: op, Value: value}
}

// UDField creates a FieldCondition for a user-defined field.
func UDField(name string, op Operator, value any) FieldCondition {
	return FieldCondition{Field: name, Op: op, Value: value, UDF: true}
}

// And creates a GroupCondition with AND operator.
func And(conditions ...Condition) GroupCondition {
	return GroupCondition{Op: GroupAnd, Items: conditions}
}

// Or creates a GroupCondition with OR operator.
func Or(conditions ...Condition) GroupCondition {
	return GroupCondition{Op: GroupOr, Items: conditions}
}

// Query represents an Autotask API query with filters, field selection, and limits.
type Query struct {
	conditions    []Condition
	includeFields []string
	maxRecords    int
}

// NewQuery creates an empty query.
func NewQuery() *Query {
	return &Query{}
}

// Where adds a simple field filter.
func (q *Query) Where(field string, op Operator, value any) *Query {
	q.conditions = append(q.conditions, FieldCondition{Field: field, Op: op, Value: value})
	return q
}

// WhereUDF adds a user-defined field filter.
func (q *Query) WhereUDF(field string, op Operator, value any) *Query {
	q.conditions = append(q.conditions, FieldCondition{Field: field, Op: op, Value: value, UDF: true})
	return q
}

// And adds a nested AND group.
func (q *Query) And(conditions ...Condition) *Query {
	q.conditions = append(q.conditions, GroupCondition{Op: GroupAnd, Items: conditions})
	return q
}

// Or adds a nested OR group.
func (q *Query) Or(conditions ...Condition) *Query {
	q.conditions = append(q.conditions, GroupCondition{Op: GroupOr, Items: conditions})
	return q
}

// Fields limits which fields are returned in the response.
func (q *Query) Fields(fields ...string) *Query {
	q.includeFields = fields
	return q
}

// Limit caps the total number of results across all pages.
// Clamped to 500 (Autotask per-page maximum) for the API request.
func (q *Query) Limit(n int) *Query {
	q.maxRecords = n
	return q
}

// MaxRecords returns the user-requested limit (unclamped, for pagination logic).
func (q *Query) MaxRecords() int {
	return q.maxRecords
}

// MarshalJSON serializes the query to the Autotask JSON filter format.
func (q *Query) MarshalJSON() ([]byte, error) {
	m := make(map[string]any)
	m["filter"] = q.conditions
	if len(q.includeFields) > 0 {
		m["IncludeFields"] = q.includeFields
	}
	if q.maxRecords > 0 {
		limit := q.maxRecords
		if limit > 500 {
			limit = 500
		}
		m["MaxRecords"] = limit
	}
	return json.Marshal(m)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -run TestQuery -v ./...`
Expected: All PASS

- [ ] **Step 5: Commit**

```bash
git add query.go query_test.go
git commit -m "feat: add query builder with nested AND/OR conditions"
```

---

### Task 7: Core Client

**Files:**
- Create: `client.go`
- Create: `option.go`
- Create: `client_test.go`

Reference: `/Users/e909385/src/vainu2/internal/autotask/client_impl.go`

- [ ] **Step 1: Write failing tests for client construction, auth headers, and HTTP requests**

```go
// client_test.go
package autotask

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	var srvURL string // captured by handler closures
	mux := http.NewServeMux()
	mux.HandleFunc("GET /atservicesrest/versioninformation", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"versions": []string{"1.0"}})
	})
	mux.HandleFunc("GET /atservicesrest/1.0/zoneInformation", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"zoneName": "TestZone",
			"url":      srvURL, // uses captured server URL
			"webUrl":   "https://test.autotask.net",
			"ci":       1,
		})
	})
	srv := httptest.NewServer(mux)
	srvURL = srv.URL // set after server starts
	t.Cleanup(srv.Close)
	return srv
}

func TestNewClientWithBaseURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()

	auth := AuthConfig{Username: "user", Secret: "secret", IntegrationCode: "code"}
	client, err := NewClient(context.Background(), auth, WithBaseURL(srv.URL))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if client.baseURL != srv.URL {
		t.Fatalf("baseURL = %q; want %q", client.baseURL, srv.URL)
	}
}

func TestClientAuthHeaders(t *testing.T) {
	var gotHeaders http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeaders = r.Header
		json.NewEncoder(w).Encode(map[string]any{"item": map[string]any{"id": 1}})
	}))
	defer srv.Close()

	auth := AuthConfig{Username: "user@test.com", Secret: "s3cret", IntegrationCode: "INT123"}
	client, err := NewClient(context.Background(), auth, WithBaseURL(srv.URL))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	// Make a request to trigger header injection
	client.do(context.Background(), http.MethodGet, "/v1.0/Tickets/1", nil, nil)

	if gotHeaders.Get("UserName") != "user@test.com" {
		t.Fatalf("UserName header = %q", gotHeaders.Get("UserName"))
	}
	if gotHeaders.Get("Secret") != "s3cret" {
		t.Fatalf("Secret header = %q", gotHeaders.Get("Secret"))
	}
	if gotHeaders.Get("ApiIntegrationcode") != "INT123" {
		t.Fatalf("ApiIntegrationcode header = %q", gotHeaders.Get("ApiIntegrationcode"))
	}
	if !strings.HasPrefix(gotHeaders.Get("User-Agent"), "go-autotask/") {
		t.Fatalf("User-Agent header = %q", gotHeaders.Get("User-Agent"))
	}
}

func TestClientImpersonation(t *testing.T) {
	var gotHeaders http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeaders = r.Header
		json.NewEncoder(w).Encode(map[string]any{"item": map[string]any{"id": 1}})
	}))
	defer srv.Close()

	auth := AuthConfig{Username: "user", Secret: "secret", IntegrationCode: "code"}
	client, _ := NewClient(context.Background(), auth,
		WithBaseURL(srv.URL),
		WithImpersonation(12345),
	)
	defer client.Close()

	client.do(context.Background(), http.MethodGet, "/v1.0/Tickets/1", nil, nil)

	if gotHeaders.Get("ImpersonationResourceId") != "12345" {
		t.Fatalf("ImpersonationResourceId = %q", gotHeaders.Get("ImpersonationResourceId"))
	}
}

func TestClientClose(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()

	auth := AuthConfig{Username: "user", Secret: "secret", IntegrationCode: "code"}
	client, _ := NewClient(context.Background(), auth, WithBaseURL(srv.URL))
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestClientDoPost(t *testing.T) {
	var gotBody []byte
	var gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotBody, _ = io.ReadAll(r.Body)
		json.NewEncoder(w).Encode(map[string]any{"itemId": 1})
	}))
	defer srv.Close()

	auth := AuthConfig{Username: "user", Secret: "secret", IntegrationCode: "code"}
	client, _ := NewClient(context.Background(), auth, WithBaseURL(srv.URL))
	defer client.Close()

	payload := map[string]any{"title": "test"}
	var result map[string]any
	err := client.do(context.Background(), http.MethodPost, "/v1.0/Tickets", payload, &result)
	if err != nil {
		t.Fatal(err)
	}
	if gotMethod != "POST" {
		t.Fatalf("method = %s; want POST", gotMethod)
	}
	if !strings.Contains(string(gotBody), "title") {
		t.Fatalf("body = %s; missing title", gotBody)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -run "TestNewClient|TestClient" -v ./...`
Expected: FAIL

- [ ] **Step 3: Implement client and options**

```go
// client.go
package autotask

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const version = "0.1.0"

// Compile-time assertion that Client implements io.Closer.
var _ interface{ Close() error } = (*Client)(nil)

// Client is the main entry point for the Autotask REST API.
type Client struct {
	httpClient      *http.Client
	baseURL         string
	auth            AuthConfig
	zoneCache       *ZoneCache
	middlewares     []Middleware
	logger          *slog.Logger
	userAgent       string
	impersonationID int64
	closers         []func() error
}

// AuthConfig holds the credentials for Autotask API authentication.
type AuthConfig struct {
	Username        string
	Secret          string
	IntegrationCode string
}

// Middleware wraps an HTTP RoundTripper for composable request/response handling.
type Middleware func(next http.RoundTripper) http.RoundTripper

// NewClient creates a new Autotask API client. If WithBaseURL is not set,
// it performs zone discovery to find the correct API endpoint.
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

	// Apply middlewares to the HTTP transport.
	if len(c.middlewares) > 0 {
		transport := c.httpClient.Transport
		if transport == nil {
			transport = http.DefaultTransport
		}
		for i := len(c.middlewares) - 1; i >= 0; i-- {
			transport = c.middlewares[i](transport)
		}
		c.httpClient.Transport = transport
	}

	// If no base URL override, perform zone discovery.
	if c.baseURL == "" {
		zone, err := c.resolveZone(ctx)
		if err != nil {
			return nil, err
		}
		c.baseURL = zone.URL
	}

	return c, nil
}

// Close releases resources held by the client.
func (c *Client) Close() error {
	for _, closer := range c.closers {
		if err := closer(); err != nil {
			return err
		}
	}
	return nil
}

// resolveZone performs zone discovery, checking the cache first.
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

// do executes an HTTP request with auth headers, optional JSON body, and
// response parsing. path is appended to baseURL unless it is already an
// absolute URL (starts with "http"), which happens during pagination when
// following nextPageUrl.
func (c *Client) do(ctx context.Context, method, path string, body any, result any) error {
	var bodyReader *bytes.Buffer
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("autotask: marshaling request body: %w", err)
		}
		bodyReader = bytes.NewBuffer(b)
	}

	url := path
	if !strings.HasPrefix(path, "http") {
		url = c.baseURL + path
	}
	var req *http.Request
	var err error
	if bodyReader != nil {
		req, err = http.NewRequestWithContext(ctx, method, url, bodyReader)
	} else {
		req, err = http.NewRequestWithContext(ctx, method, url, nil)
	}
	if err != nil {
		return fmt.Errorf("autotask: creating request: %w", err)
	}

	// Auth headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("UserName", c.auth.Username)
	req.Header.Set("Secret", c.auth.Secret)
	req.Header.Set("ApiIntegrationcode", c.auth.IntegrationCode)
	req.Header.Set("User-Agent", c.userAgent)

	if c.impersonationID != 0 {
		req.Header.Set("ImpersonationResourceId", strconv.FormatInt(c.impersonationID, 10))
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("autotask: request failed: %w", err)
	}
	defer resp.Body.Close()

	return parseResponse(resp, result)
}

// Do executes an HTTP request. Exported for use by sub-packages (metadata, autotasktest).
func (c *Client) Do(ctx context.Context, method, path string, body any, result any) error {
	return c.do(ctx, method, path, body, result)
}

// discardHandler is a slog.Handler that discards all log records.
// TODO: Replace with slog.DiscardHandler if available in Go 1.26 stdlib.
type discardHandler struct{}

func (discardHandler) Enabled(context.Context, slog.Level) bool  { return false }
func (discardHandler) Handle(context.Context, slog.Record) error { return nil }
func (d discardHandler) WithAttrs([]slog.Attr) slog.Handler      { return d }
func (d discardHandler) WithGroup(string) slog.Handler            { return d }
```

```go
// option.go
package autotask

import (
	"log/slog"
	"net/http"
)

// ClientOption configures a Client.
type ClientOption func(*Client)

// WithHTTPClient sets a custom http.Client for the Autotask client.
func WithHTTPClient(hc *http.Client) ClientOption {
	return func(c *Client) { c.httpClient = hc }
}

// WithLogger sets a structured logger for the client.
func WithLogger(l *slog.Logger) ClientOption {
	return func(c *Client) { c.logger = l }
}

// WithBaseURL overrides zone discovery and uses the given URL directly.
func WithBaseURL(url string) ClientOption {
	return func(c *Client) { c.baseURL = url }
}

// WithUserAgent overrides the default User-Agent header.
func WithUserAgent(ua string) ClientOption {
	return func(c *Client) { c.userAgent = ua }
}

// WithImpersonation sets the ImpersonationResourceId header on all requests.
func WithImpersonation(resourceID int64) ClientOption {
	return func(c *Client) { c.impersonationID = resourceID }
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -run "TestNewClient|TestClient" -v ./...`
Expected: All PASS

- [ ] **Step 5: Commit**

```bash
git add client.go option.go client_test.go
git commit -m "feat: add core Client with auth headers, zone discovery, and options"
```

---

### Task 8: Raw CRUD Operations

**Files:**
- Create: `raw.go`
- Create: `raw_test.go`

- [ ] **Step 1: Write failing tests for raw CRUD**

```go
// raw_test.go
package autotask

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newCRUDTestServer(t *testing.T) (*httptest.Server, *[]http.Request) {
	t.Helper()
	var requests []http.Request
	mux := http.NewServeMux()

	mux.HandleFunc("GET /v1.0/Tickets/{id}", func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, *r)
		json.NewEncoder(w).Encode(map[string]any{
			"item": map[string]any{"id": 123, "title": "Test Ticket"},
		})
	})
	mux.HandleFunc("POST /v1.0/Tickets/query", func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, *r)
		json.NewEncoder(w).Encode(map[string]any{
			"items":       []any{map[string]any{"id": 1}, map[string]any{"id": 2}},
			"pageDetails": map[string]any{"count": 2, "nextPageUrl": nil},
		})
	})
	mux.HandleFunc("POST /v1.0/Tickets", func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, *r)
		json.NewEncoder(w).Encode(map[string]any{"itemId": 456})
	})
	mux.HandleFunc("PATCH /v1.0/Tickets", func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, *r)
		json.NewEncoder(w).Encode(map[string]any{"item": map[string]any{"id": 123}})
	})
	mux.HandleFunc("DELETE /v1.0/Tickets/{id}", func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, *r)
		w.WriteHeader(200)
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, &requests
}

func testClient(t *testing.T, srv *httptest.Server) *Client {
	t.Helper()
	auth := AuthConfig{Username: "u", Secret: "s", IntegrationCode: "c"}
	client, err := NewClient(context.Background(), auth, WithBaseURL(srv.URL))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { client.Close() })
	return client
}

func TestGetRaw(t *testing.T) {
	srv, _ := newCRUDTestServer(t)
	client := testClient(t, srv)

	result, err := GetRaw(context.Background(), client, "Tickets", 123)
	if err != nil {
		t.Fatal(err)
	}
	if result["title"] != "Test Ticket" {
		t.Fatalf("title = %v", result["title"])
	}
}

func TestListRaw(t *testing.T) {
	srv, _ := newCRUDTestServer(t)
	client := testClient(t, srv)

	results, err := ListRaw(context.Background(), client, "Tickets", NewQuery().Where("status", OpEq, 1))
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("results = %d; want 2", len(results))
	}
}

func TestCreateRaw(t *testing.T) {
	srv, _ := newCRUDTestServer(t)
	client := testClient(t, srv)

	data := map[string]any{"title": "New Ticket"}
	result, err := CreateRaw(context.Background(), client, "Tickets", data)
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestUpdateRaw(t *testing.T) {
	srv, _ := newCRUDTestServer(t)
	client := testClient(t, srv)

	data := map[string]any{"id": 123, "title": "Updated"}
	result, err := UpdateRaw(context.Background(), client, "Tickets", data)
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestDeleteRaw(t *testing.T) {
	srv, _ := newCRUDTestServer(t)
	client := testClient(t, srv)

	err := DeleteRaw(context.Background(), client, "Tickets", 123)
	if err != nil {
		t.Fatal(err)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -run "TestGetRaw|TestListRaw|TestCreateRaw|TestUpdateRaw|TestDeleteRaw" -v ./...`
Expected: FAIL

- [ ] **Step 3: Implement raw CRUD operations**

```go
// raw.go
package autotask

import (
	"context"
	"fmt"
	"net/http"
)

// GetRaw fetches a single entity by ID using untyped access.
func GetRaw(ctx context.Context, c *Client, entityName string, id int64) (map[string]any, error) {
	path := fmt.Sprintf("/v1.0/%s/%d", entityName, id)
	var resp struct {
		Item map[string]any `json:"item"`
	}
	if err := c.do(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Item, nil
}

// ListRaw queries entities using untyped access with pagination.
func ListRaw(ctx context.Context, c *Client, entityName string, q *Query) ([]map[string]any, error) {
	path := fmt.Sprintf("/v1.0/%s/query", entityName)
	totalLimit := 0
	if q != nil {
		totalLimit = q.MaxRecords()
	}

	var allItems []map[string]any
	queryBody := q

	for {
		var resp struct {
			Items       []map[string]any `json:"items"`
			PageDetails struct {
				Count       int    `json:"count"`
				NextPageURL string `json:"nextPageUrl"`
			} `json:"pageDetails"`
		}
		if err := c.do(ctx, http.MethodPost, path, queryBody, &resp); err != nil {
			return nil, err
		}

		allItems = append(allItems, resp.Items...)

		// Check if we've reached the user's total limit.
		if totalLimit > 0 && len(allItems) >= totalLimit {
			allItems = allItems[:totalLimit]
			break
		}

		// Check for next page.
		if resp.PageDetails.NextPageURL == "" {
			break
		}

		// For subsequent pages, use the full URL directly.
		path = resp.PageDetails.NextPageURL
		queryBody = nil
	}

	return allItems, nil
}

// CreateRaw creates an entity using untyped access.
func CreateRaw(ctx context.Context, c *Client, entityName string, data map[string]any) (map[string]any, error) {
	path := fmt.Sprintf("/v1.0/%s", entityName)
	var resp map[string]any
	if err := c.do(ctx, http.MethodPost, path, data, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// UpdateRaw updates an entity using untyped access.
func UpdateRaw(ctx context.Context, c *Client, entityName string, data map[string]any) (map[string]any, error) {
	path := fmt.Sprintf("/v1.0/%s", entityName)
	var resp struct {
		Item map[string]any `json:"item"`
	}
	if err := c.do(ctx, http.MethodPatch, path, data, &resp); err != nil {
		return nil, err
	}
	return resp.Item, nil
}

// DeleteRaw deletes an entity by ID using untyped access.
func DeleteRaw(ctx context.Context, c *Client, entityName string, id int64) error {
	path := fmt.Sprintf("/v1.0/%s/%d", entityName, id)
	return c.do(ctx, http.MethodDelete, path, nil, nil)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -run "TestGetRaw|TestListRaw|TestCreateRaw|TestUpdateRaw|TestDeleteRaw" -v ./...`
Expected: All PASS

- [ ] **Step 5: Commit**

```bash
git add raw.go raw_test.go
git commit -m "feat: add raw CRUD operations with pagination"
```

---

### Task 9: Generic Typed CRUD Operations

**Files:**
- Create: `crud.go`
- Create: `crud_test.go`

- [ ] **Step 1: Write failing tests for generic CRUD**

```go
// crud_test.go
package autotask

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// testEntity is a simple entity for testing generics.
type testEntity struct {
	ID    Optional[int64]  `json:"id,omitzero"`
	Title Optional[string] `json:"title,omitzero"`
}

func (testEntity) EntityName() string { return "TestEntities" }

func newTypedTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	mux.HandleFunc("GET /v1.0/TestEntities/{id}", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"item": map[string]any{"id": 42, "title": "Hello"},
		})
	})
	mux.HandleFunc("POST /v1.0/TestEntities/query", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"items": []any{
				map[string]any{"id": 1, "title": "First"},
				map[string]any{"id": 2, "title": "Second"},
			},
			"pageDetails": map[string]any{"count": 2},
		})
	})
	mux.HandleFunc("POST /v1.0/TestEntities/query/count", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"queryCount": 42})
	})
	mux.HandleFunc("POST /v1.0/TestEntities", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"itemId": 99})
	})
	mux.HandleFunc("PATCH /v1.0/TestEntities", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"item": map[string]any{"id": 42, "title": "Updated"},
		})
	})
	mux.HandleFunc("DELETE /v1.0/TestEntities/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestGet(t *testing.T) {
	srv := newTypedTestServer(t)
	client := testClient(t, srv)

	entity, err := Get[testEntity](context.Background(), client, 42)
	if err != nil {
		t.Fatal(err)
	}
	if v, ok := entity.ID.Get(); !ok || v != 42 {
		t.Fatalf("ID = %v, %v; want 42", v, ok)
	}
	if v, ok := entity.Title.Get(); !ok || v != "Hello" {
		t.Fatalf("Title = %v, %v; want Hello", v, ok)
	}
}

func TestList(t *testing.T) {
	srv := newTypedTestServer(t)
	client := testClient(t, srv)

	entities, err := List[testEntity](context.Background(), client, NewQuery().Where("status", OpEq, 1))
	if err != nil {
		t.Fatal(err)
	}
	if len(entities) != 2 {
		t.Fatalf("len = %d; want 2", len(entities))
	}
}

func TestCount(t *testing.T) {
	srv := newTypedTestServer(t)
	client := testClient(t, srv)

	count, err := Count[testEntity](context.Background(), client, NewQuery())
	if err != nil {
		t.Fatal(err)
	}
	if count != 42 {
		t.Fatalf("count = %d; want 42", count)
	}
}

func TestCreate(t *testing.T) {
	srv := newTypedTestServer(t)
	client := testClient(t, srv)

	entity := &testEntity{Title: Set("New")}
	result, err := Create(context.Background(), client, entity)
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestUpdate(t *testing.T) {
	srv := newTypedTestServer(t)
	client := testClient(t, srv)

	entity := &testEntity{ID: Set(int64(42)), Title: Set("Updated")}
	result, err := Update(context.Background(), client, entity)
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestDelete(t *testing.T) {
	srv := newTypedTestServer(t)
	client := testClient(t, srv)

	err := Delete[testEntity](context.Background(), client, 42)
	if err != nil {
		t.Fatal(err)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -run "^Test(Get|List|Count|Create|Update|Delete)$" -v ./...`
Expected: FAIL

- [ ] **Step 3: Implement generic CRUD**

```go
// crud.go
package autotask

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// Get fetches a single entity by ID.
func Get[T Entity](ctx context.Context, c *Client, id int64) (*T, error) {
	var zero T
	path := fmt.Sprintf("/v1.0/%s/%d", zero.EntityName(), id)
	var resp struct {
		Item json.RawMessage `json:"item"`
	}
	if err := c.do(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return nil, err
	}
	var entity T
	if err := json.Unmarshal(resp.Item, &entity); err != nil {
		return nil, fmt.Errorf("autotask: decoding %s: %w", zero.EntityName(), err)
	}
	return &entity, nil
}

// List queries entities with pagination, returning all matching results.
func List[T Entity](ctx context.Context, c *Client, q *Query) ([]*T, error) {
	var zero T
	path := fmt.Sprintf("/v1.0/%s/query", zero.EntityName())
	totalLimit := 0
	if q != nil {
		totalLimit = q.MaxRecords()
	}

	var allItems []*T
	queryBody := q

	for {
		var resp struct {
			Items       []json.RawMessage `json:"items"`
			PageDetails struct {
				Count       int    `json:"count"`
				NextPageURL string `json:"nextPageUrl"`
			} `json:"pageDetails"`
		}
		if err := c.do(ctx, http.MethodPost, path, queryBody, &resp); err != nil {
			return nil, err
		}

		for _, raw := range resp.Items {
			var entity T
			if err := json.Unmarshal(raw, &entity); err != nil {
				return nil, fmt.Errorf("autotask: decoding %s item: %w", zero.EntityName(), err)
			}
			allItems = append(allItems, &entity)
		}

		if totalLimit > 0 && len(allItems) >= totalLimit {
			allItems = allItems[:totalLimit]
			break
		}

		if resp.PageDetails.NextPageURL == "" {
			break
		}

		path = resp.PageDetails.NextPageURL
		queryBody = nil
	}

	return allItems, nil
}

// Count returns the number of entities matching a query.
func Count[T Entity](ctx context.Context, c *Client, q *Query) (int64, error) {
	var zero T
	path := fmt.Sprintf("/v1.0/%s/query/count", zero.EntityName())
	var resp struct {
		QueryCount int64 `json:"queryCount"`
	}
	if err := c.do(ctx, http.MethodPost, path, q, &resp); err != nil {
		return 0, err
	}
	return resp.QueryCount, nil
}

// Create creates a new entity and returns the API response.
func Create[T Entity](ctx context.Context, c *Client, entity *T) (*T, error) {
	path := fmt.Sprintf("/v1.0/%s", (*entity).EntityName())
	var resp json.RawMessage
	if err := c.do(ctx, http.MethodPost, path, entity, &resp); err != nil {
		return nil, err
	}
	// Autotask returns itemId for creates; return the input entity as-is
	// since the full entity isn't always in the response.
	return entity, nil
}

// Update modifies an existing entity using PATCH.
func Update[T Entity](ctx context.Context, c *Client, entity *T) (*T, error) {
	path := fmt.Sprintf("/v1.0/%s", (*entity).EntityName())
	var resp struct {
		Item json.RawMessage `json:"item"`
	}
	if err := c.do(ctx, http.MethodPatch, path, entity, &resp); err != nil {
		return nil, err
	}
	if resp.Item != nil {
		var updated T
		if err := json.Unmarshal(resp.Item, &updated); err == nil {
			return &updated, nil
		}
	}
	return entity, nil
}

// Delete removes an entity by ID.
func Delete[T Entity](ctx context.Context, c *Client, id int64) error {
	var zero T
	path := fmt.Sprintf("/v1.0/%s/%d", zero.EntityName(), id)
	return c.do(ctx, http.MethodDelete, path, nil, nil)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -run "^Test(Get|List|Count|Create|Update|Delete)$" -v ./...`
Expected: All PASS

- [ ] **Step 5: Commit**

```bash
git add crud.go crud_test.go
git commit -m "feat: add generic typed CRUD operations"
```

---

### Task 10: Iterator-Based Pagination

**Files:**
- Create: `iter.go`
- Create: `iter_test.go`

- [ ] **Step 1: Write failing tests for ListIter**

```go
// iter_test.go
package autotask

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListIter(t *testing.T) {
	page := 0
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1.0/TestEntities/query", func(w http.ResponseWriter, r *http.Request) {
		page++
		if page == 1 {
			json.NewEncoder(w).Encode(map[string]any{
				"items":       []any{map[string]any{"id": 1}, map[string]any{"id": 2}},
				"pageDetails": map[string]any{"count": 2, "nextPageUrl": "/v1.0/TestEntities/query?page=2"},
			})
		} else {
			json.NewEncoder(w).Encode(map[string]any{
				"items":       []any{map[string]any{"id": 3}},
				"pageDetails": map[string]any{"count": 1},
			})
		}
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()
	client := testClient(t, srv)

	var ids []int64
	for entity, err := range ListIter[testEntity](context.Background(), client, NewQuery()) {
		if err != nil {
			t.Fatal(err)
		}
		v, _ := entity.ID.Get()
		ids = append(ids, v)
	}
	if len(ids) != 3 {
		t.Fatalf("got %d items; want 3", len(ids))
	}
}

func TestListIterBreakEarly(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1.0/TestEntities/query", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"items":       []any{map[string]any{"id": 1}, map[string]any{"id": 2}, map[string]any{"id": 3}},
			"pageDetails": map[string]any{"count": 3, "nextPageUrl": "/v1.0/TestEntities/query?page=2"},
		})
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()
	client := testClient(t, srv)

	count := 0
	for _, err := range ListIter[testEntity](context.Background(), client, NewQuery()) {
		if err != nil {
			t.Fatal(err)
		}
		count++
		if count == 2 {
			break // stop early
		}
	}
	if count != 2 {
		t.Fatalf("count = %d; want 2", count)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -run TestListIter -v ./...`
Expected: FAIL

- [ ] **Step 3: Implement ListIter**

```go
// iter.go
package autotask

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"net/http"
)

// ListIter returns an iterator that lazily paginates through query results.
// Each call to the iterator fetches the next entity. Pages are fetched on demand.
func ListIter[T Entity](ctx context.Context, c *Client, q *Query) iter.Seq2[*T, error] {
	return func(yield func(*T, error) bool) {
		var zero T
		path := fmt.Sprintf("/v1.0/%s/query", zero.EntityName())
		queryBody := q

		for {
			var resp struct {
				Items       []json.RawMessage `json:"items"`
				PageDetails struct {
					Count       int    `json:"count"`
					NextPageURL string `json:"nextPageUrl"`
				} `json:"pageDetails"`
			}
			if err := c.do(ctx, http.MethodPost, path, queryBody, &resp); err != nil {
				yield(nil, err)
				return
			}

			for _, raw := range resp.Items {
				var entity T
				if err := json.Unmarshal(raw, &entity); err != nil {
					if !yield(nil, fmt.Errorf("autotask: decoding %s: %w", zero.EntityName(), err)) {
						return
					}
					continue
				}
				if !yield(&entity, nil) {
					return
				}
			}

			if resp.PageDetails.NextPageURL == "" {
				return
			}

			path = resp.PageDetails.NextPageURL
			queryBody = nil
		}
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -run TestListIter -v ./...`
Expected: All PASS

- [ ] **Step 5: Commit**

```bash
git add iter.go iter_test.go
git commit -m "feat: add ListIter for lazy iterator-based pagination"
```

---

### Task 11: Child Entity Access

**Files:**
- Create: `child.go`
- Create: `child_test.go`

- [ ] **Step 1: Write failing tests**

```go
// child_test.go
package autotask

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

type testChildEntity struct {
	ID      Optional[int64]  `json:"id,omitzero"`
	Message Optional[string] `json:"message,omitzero"`
}

func (testChildEntity) EntityName() string { return "Notes" }

func TestGetChild(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1.0/TestEntities/{parentID}/Notes", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"items": []any{
				map[string]any{"id": 10, "message": "note 1"},
				map[string]any{"id": 11, "message": "note 2"},
			},
			"pageDetails": map[string]any{"count": 2},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	client := testClient(t, srv)

	children, err := GetChild[testEntity, testChildEntity](context.Background(), client, 42)
	if err != nil {
		t.Fatal(err)
	}
	if len(children) != 2 {
		t.Fatalf("len = %d; want 2", len(children))
	}
}

func TestCreateChild(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1.0/TestEntities/{parentID}/Notes", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"itemId": 12})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	client := testClient(t, srv)

	child := &testChildEntity{Message: Set("new note")}
	result, err := CreateChild[testEntity](context.Background(), client, 42, child)
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -run "TestGetChild|TestCreateChild" -v ./...`
Expected: FAIL

- [ ] **Step 3: Implement child entity access**

```go
// child.go
package autotask

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// GetChild fetches child entities for a parent entity (first page only — most
// parent-child relationships have fewer than 500 children).
// Example: GetChild[entities.Ticket, entities.TicketNote](ctx, client, ticketID)
func GetChild[P Entity, C Entity](ctx context.Context, c *Client, parentID int64) ([]*C, error) {
	var parent P
	var child C
	path := fmt.Sprintf("/v1.0/%s/%d/%s", parent.EntityName(), parentID, child.EntityName())

	var resp struct {
		Items       []json.RawMessage `json:"items"`
		PageDetails struct {
			NextPageURL string `json:"nextPageUrl"`
		} `json:"pageDetails"`
	}
	if err := c.do(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return nil, err
	}

	var result []*C
	for _, raw := range resp.Items {
		var entity C
		if err := json.Unmarshal(raw, &entity); err != nil {
			return nil, fmt.Errorf("autotask: decoding %s child: %w", child.EntityName(), err)
		}
		result = append(result, &entity)
	}
	return result, nil
}

// CreateChild creates a child entity under a parent.
// Example: CreateChild[entities.Ticket](ctx, client, ticketID, &note)
func CreateChild[P Entity, C Entity](ctx context.Context, c *Client, parentID int64, child *C) (*C, error) {
	var parent P
	path := fmt.Sprintf("/v1.0/%s/%d/%s", parent.EntityName(), parentID, (*child).EntityName())
	var resp json.RawMessage
	if err := c.do(ctx, http.MethodPost, path, child, &resp); err != nil {
		return nil, err
	}
	return child, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -run "TestGetChild|TestCreateChild" -v ./...`
Expected: All PASS

- [ ] **Step 5: Commit**

```bash
git add child.go child_test.go
git commit -m "feat: add child entity access (GetChild, CreateChild)"
```

---

### Task 12: Rate Limiter Middleware

**Files:**
- Create: `middleware/ratelimit.go`
- Create: `middleware/ratelimit_test.go`

Reference: `/Users/e909385/src/vainu2/internal/autotask/rate_limiter.go`

- [ ] **Step 1: Write failing tests**

```go
// middleware/ratelimit_test.go
package middleware

import (
	"net/http"
	"testing"
	"time"
)

type mockTransport struct {
	responses []*http.Response
	calls     int
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	idx := m.calls
	m.calls++
	if idx < len(m.responses) {
		return m.responses[idx], nil
	}
	return &http.Response{StatusCode: 200, Body: http.NoBody}, nil
}

func TestRateLimiterAllowsRequests(t *testing.T) {
	inner := &mockTransport{}
	rl := NewRateLimiter(inner)
	req, _ := http.NewRequest("GET", "https://example.com/test", nil)
	resp, err := rl.RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d; want 200", resp.StatusCode)
	}
}

func TestRateLimiterRespectsRetryAfter(t *testing.T) {
	inner := &mockTransport{
		responses: []*http.Response{
			{
				StatusCode: 429,
				Header:     http.Header{"Retry-After": []string{"1"}},
				Body:       http.NoBody,
			},
		},
	}
	rl := NewRateLimiter(inner)
	req, _ := http.NewRequest("GET", "https://example.com/test", nil)

	start := time.Now()
	resp, err := rl.RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	// Should return the 429 to the caller, not retry automatically.
	if resp.StatusCode != 429 {
		t.Fatalf("status = %d; want 429", resp.StatusCode)
	}
	_ = start // Rate limiter should record the 429 for future throttling
}

func TestRateLimiterCustomConfig(t *testing.T) {
	inner := &mockTransport{}
	rl := NewRateLimiter(inner,
		WithRequestsPerHour(1000),
		WithBurstSize(5),
	)
	req, _ := http.NewRequest("GET", "https://example.com/test", nil)
	_, err := rl.RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	if rl.config.requestsPerHour != 1000 {
		t.Fatalf("requestsPerHour = %d; want 1000", rl.config.requestsPerHour)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -run TestRateLimiter -v ./middleware/...`
Expected: FAIL

- [ ] **Step 3: Implement rate limiter middleware**

```go
// middleware/ratelimit.go
package middleware

import (
	"context"
	"net/http"
	"strconv"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// RateLimitOption configures the rate limiter.
type RateLimitOption func(*rateLimitConfig)

type rateLimitConfig struct {
	requestsPerHour int
	burstSize       int
	adaptiveDelay   bool
}

// WithRequestsPerHour sets the maximum requests per hour.
func WithRequestsPerHour(n int) RateLimitOption {
	return func(c *rateLimitConfig) { c.requestsPerHour = n }
}

// WithBurstSize sets the token bucket burst size.
func WithBurstSize(n int) RateLimitOption {
	return func(c *rateLimitConfig) { c.burstSize = n }
}

// WithAdaptiveDelay enables/disables adaptive delays matching Autotask thresholds.
func WithAdaptiveDelay(enabled bool) RateLimitOption {
	return func(c *rateLimitConfig) { c.adaptiveDelay = enabled }
}

// RateLimiter is an http.RoundTripper that enforces rate limits.
type RateLimiter struct {
	next    http.RoundTripper
	limiter *rate.Limiter
	config  rateLimitConfig

	mu               sync.Mutex
	windowStart      time.Time
	requestsInWindow int
	retryAfterUntil  time.Time // pause requests until this time (from 429 Retry-After)
}

// NewRateLimiter wraps a RoundTripper with token bucket rate limiting.
func NewRateLimiter(next http.RoundTripper, opts ...RateLimitOption) *RateLimiter {
	cfg := rateLimitConfig{
		requestsPerHour: 5000,
		burstSize:       20,
		adaptiveDelay:   true,
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	rps := rate.Limit(float64(cfg.requestsPerHour) / 3600.0)
	return &RateLimiter{
		next:        next,
		limiter:     rate.NewLimiter(rps, cfg.burstSize),
		config:      cfg,
		windowStart: time.Now(),
	}
}

// RoundTrip implements http.RoundTripper with rate limiting.
func (rl *RateLimiter) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx := req.Context()

	// Respect any active Retry-After pause from a previous 429 response.
	rl.mu.Lock()
	retryUntil := rl.retryAfterUntil
	rl.mu.Unlock()
	if wait := time.Until(retryUntil); wait > 0 {
		select {
		case <-time.After(wait):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	// Wait for token bucket.
	if err := rl.limiter.Wait(ctx); err != nil {
		return nil, err
	}

	// Apply adaptive delay based on usage.
	if rl.config.adaptiveDelay {
		delay := rl.adaptiveDelay()
		if delay > 0 {
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
	}

	rl.recordRequest()

	resp, err := rl.next.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	// If we get a 429, record the Retry-After to block future requests.
	if resp.StatusCode == 429 {
		if retryAfter := resp.Header.Get("Retry-After"); retryAfter != "" {
			if secs, err := strconv.Atoi(retryAfter); err == nil {
				rl.mu.Lock()
				rl.retryAfterUntil = time.Now().Add(time.Duration(secs) * time.Second)
				rl.mu.Unlock()
			}
		}
	}

	return resp, nil
}

func (rl *RateLimiter) recordRequest() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	if now.Sub(rl.windowStart) > time.Hour {
		rl.windowStart = now
		rl.requestsInWindow = 0
	}
	rl.requestsInWindow++
}

func (rl *RateLimiter) adaptiveDelay() time.Duration {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	usage := float64(rl.requestsInWindow) / float64(rl.config.requestsPerHour)
	switch {
	case usage >= 0.75:
		return 1 * time.Second
	case usage >= 0.50:
		return 500 * time.Millisecond
	default:
		return 0
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -run TestRateLimiter -v ./middleware/...`
Expected: All PASS

- [ ] **Step 5: Commit**

```bash
git add middleware/ratelimit.go middleware/ratelimit_test.go
git commit -m "feat: add rate limiter middleware with adaptive delays"
```

---

### Task 13: Circuit Breaker Middleware

**Files:**
- Create: `middleware/circuitbreaker.go`
- Create: `middleware/circuitbreaker_test.go`

Reference: `/Users/e909385/src/vainu2/internal/autotask/circuit_breaker.go`

- [ ] **Step 1: Write failing tests**

```go
// middleware/circuitbreaker_test.go
package middleware

import (
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestCircuitBreakerClosedState(t *testing.T) {
	inner := &mockTransport{}
	cb := NewCircuitBreaker(inner)

	req, _ := http.NewRequest("GET", "https://example.com/test", nil)
	resp, err := cb.RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d; want 200", resp.StatusCode)
	}
	if cb.State() != StateClosed {
		t.Fatalf("state = %s; want closed", cb.State())
	}
}

func TestCircuitBreakerOpensOnFailures(t *testing.T) {
	inner := &mockTransport{
		responses: make([]*http.Response, 10),
	}
	for i := range inner.responses {
		inner.responses[i] = &http.Response{
			StatusCode: 500,
			Body:       http.NoBody,
		}
	}
	cb := NewCircuitBreaker(inner,
		WithFailureThreshold(3),
		WithFailureWindow(10*time.Second),
	)

	req, _ := http.NewRequest("GET", "https://example.com/test", nil)
	// Trigger 3 failures
	for i := 0; i < 3; i++ {
		cb.RoundTrip(req)
	}

	if cb.State() != StateOpen {
		t.Fatalf("state = %s; want open", cb.State())
	}

	// Next request should be rejected immediately.
	_, err := cb.RoundTrip(req)
	if err == nil || !strings.Contains(err.Error(), "circuit breaker is open") {
		t.Fatalf("expected circuit breaker open error, got: %v", err)
	}
}

func TestCircuitBreakerTransitionsToHalfOpen(t *testing.T) {
	inner := &mockTransport{
		responses: []*http.Response{
			{StatusCode: 500, Body: http.NoBody},
			{StatusCode: 500, Body: http.NoBody},
			{StatusCode: 500, Body: http.NoBody},
		},
	}
	cb := NewCircuitBreaker(inner,
		WithFailureThreshold(3),
		WithOpenTimeout(10*time.Millisecond),
	)

	req, _ := http.NewRequest("GET", "https://example.com/test", nil)
	for i := 0; i < 3; i++ {
		cb.RoundTrip(req)
	}

	time.Sleep(20 * time.Millisecond)

	if cb.State() != StateHalfOpen {
		t.Fatalf("state = %s; want half-open", cb.State())
	}
}

func TestCircuitBreakerIgnoresNon5xx(t *testing.T) {
	inner := &mockTransport{
		responses: []*http.Response{
			{StatusCode: 404, Body: http.NoBody},
			{StatusCode: 404, Body: http.NoBody},
			{StatusCode: 404, Body: http.NoBody},
		},
	}
	cb := NewCircuitBreaker(inner, WithFailureThreshold(3))

	req, _ := http.NewRequest("GET", "https://example.com/test", nil)
	for i := 0; i < 3; i++ {
		cb.RoundTrip(req)
	}

	if cb.State() != StateClosed {
		t.Fatalf("state = %s; want closed (4xx should not trigger)", cb.State())
	}
}

var _ error = (*CircuitBreakerOpenError)(nil)

func TestCircuitBreakerOpenError(t *testing.T) {
	err := &CircuitBreakerOpenError{}
	var target *CircuitBreakerOpenError
	if !errors.As(err, &target) {
		t.Fatal("should match CircuitBreakerOpenError")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -run TestCircuitBreaker -v ./middleware/...`
Expected: FAIL

- [ ] **Step 3: Implement circuit breaker**

```go
// middleware/circuitbreaker.go
package middleware

import (
	"fmt"
	"net/http"
	"sync"
	"time"
)

// CircuitState represents the circuit breaker state.
type CircuitState string

const (
	StateClosed   CircuitState = "closed"
	StateOpen     CircuitState = "open"
	StateHalfOpen CircuitState = "half-open"
)

// CircuitBreakerOption configures the circuit breaker.
type CircuitBreakerOption func(*circuitBreakerConfig)

type circuitBreakerConfig struct {
	failureThreshold int
	failureWindow    time.Duration
	openTimeout      time.Duration
	successThreshold int
}

func WithFailureThreshold(n int) CircuitBreakerOption {
	return func(c *circuitBreakerConfig) { c.failureThreshold = n }
}

func WithFailureWindow(d time.Duration) CircuitBreakerOption {
	return func(c *circuitBreakerConfig) { c.failureWindow = d }
}

func WithOpenTimeout(d time.Duration) CircuitBreakerOption {
	return func(c *circuitBreakerConfig) { c.openTimeout = d }
}

func WithSuccessThreshold(n int) CircuitBreakerOption {
	return func(c *circuitBreakerConfig) { c.successThreshold = n }
}

// CircuitBreakerOpenError is returned when the circuit is open.
type CircuitBreakerOpenError struct{}

func (e *CircuitBreakerOpenError) Error() string {
	return "autotask: circuit breaker is open"
}

// CircuitBreaker is an http.RoundTripper that implements the circuit breaker pattern.
type CircuitBreaker struct {
	next   http.RoundTripper
	config circuitBreakerConfig

	mu              sync.RWMutex
	state           CircuitState
	failures        []time.Time
	lastStateChange time.Time
	halfOpenSuccesses int
}

// NewCircuitBreaker wraps a RoundTripper with circuit breaker protection.
func NewCircuitBreaker(next http.RoundTripper, opts ...CircuitBreakerOption) *CircuitBreaker {
	cfg := circuitBreakerConfig{
		failureThreshold: 5,
		failureWindow:    10 * time.Second,
		openTimeout:      30 * time.Second,
		successThreshold: 2,
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	return &CircuitBreaker{
		next:            next,
		config:          cfg,
		state:           StateClosed,
		lastStateChange: time.Now(),
	}
}

// State returns the current circuit state.
func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	if cb.state == StateOpen && time.Since(cb.lastStateChange) >= cb.config.openTimeout {
		return StateHalfOpen
	}
	return cb.state
}

// RoundTrip implements http.RoundTripper.
func (cb *CircuitBreaker) RoundTrip(req *http.Request) (*http.Response, error) {
	state := cb.State()

	switch state {
	case StateOpen:
		return nil, &CircuitBreakerOpenError{}
	case StateHalfOpen:
		cb.mu.Lock()
		cb.state = StateHalfOpen
		cb.mu.Unlock()
	}

	resp, err := cb.next.RoundTrip(req)
	if err != nil {
		cb.recordFailure()
		return nil, err
	}

	if cb.isFailure(resp) {
		cb.recordFailure()
	} else if state == StateHalfOpen {
		cb.recordHalfOpenSuccess()
	}

	return resp, nil
}

func (cb *CircuitBreaker) isFailure(resp *http.Response) bool {
	return resp.StatusCode >= 500 || resp.StatusCode == 429
}

func (cb *CircuitBreaker) recordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-cb.config.failureWindow)

	// Prune old failures.
	var recent []time.Time
	for _, t := range cb.failures {
		if t.After(cutoff) {
			recent = append(recent, t)
		}
	}
	recent = append(recent, now)
	cb.failures = recent

	if cb.state == StateHalfOpen {
		cb.state = StateOpen
		cb.lastStateChange = now
		cb.halfOpenSuccesses = 0
		return
	}

	if len(recent) >= cb.config.failureThreshold {
		cb.state = StateOpen
		cb.lastStateChange = now
		cb.failures = nil
	}
}

func (cb *CircuitBreaker) recordHalfOpenSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.halfOpenSuccesses++
	if cb.halfOpenSuccesses >= cb.config.successThreshold {
		cb.state = StateClosed
		cb.lastStateChange = time.Now()
		cb.halfOpenSuccesses = 0
		cb.failures = nil
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -run TestCircuitBreaker -v ./middleware/...`
Expected: All PASS

- [ ] **Step 5: Commit**

```bash
git add middleware/circuitbreaker.go middleware/circuitbreaker_test.go
git commit -m "feat: add circuit breaker middleware"
```

---

### Task 14: Threshold Monitor Middleware

**Files:**
- Create: `middleware/threshold.go`
- Create: `middleware/threshold_test.go`

Reference: `/Users/e909385/src/vainu2/internal/autotask/threshold_monitor.go`

- [ ] **Step 1: Write failing tests**

```go
// middleware/threshold_test.go
package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestThresholdMonitorCallsWarning(t *testing.T) {
	var warningCalled atomic.Bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"currentTimeframeRequestCount": 8000,
			"externalRequestThreshold":     10000,
		})
	}))
	defer srv.Close()

	m := NewThresholdMonitor(srv.Client(), srv.URL,
		WithCheckInterval(10*time.Millisecond),
		WithWarningCallback(func(info ThresholdInfo) {
			warningCalled.Store(true)
		}),
	)
	m.Start()
	defer m.Stop()

	time.Sleep(50 * time.Millisecond)

	if !warningCalled.Load() {
		t.Fatal("warning callback should have been called at 80% usage")
	}
}

func TestThresholdMonitorCallsCritical(t *testing.T) {
	var criticalCalled atomic.Bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"currentTimeframeRequestCount": 9500,
			"externalRequestThreshold":     10000,
		})
	}))
	defer srv.Close()

	m := NewThresholdMonitor(srv.Client(), srv.URL,
		WithCheckInterval(10*time.Millisecond),
		WithCriticalCallback(func(info ThresholdInfo) {
			criticalCalled.Store(true)
		}),
	)
	m.Start()
	defer m.Stop()

	time.Sleep(50 * time.Millisecond)

	if !criticalCalled.Load() {
		t.Fatal("critical callback should have been called at 95% usage")
	}
}

func TestThresholdMonitorStop(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"currentTimeframeRequestCount": 100,
			"externalRequestThreshold":     10000,
		})
	}))
	defer srv.Close()

	m := NewThresholdMonitor(srv.Client(), srv.URL, WithCheckInterval(10*time.Millisecond))
	m.Start()
	err := m.Stop()
	if err != nil {
		t.Fatal(err)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -run TestThresholdMonitor -v ./middleware/...`
Expected: FAIL

- [ ] **Step 3: Implement threshold monitor**

```go
// middleware/threshold.go
package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// ThresholdInfo contains Autotask API usage information.
type ThresholdInfo struct {
	CurrentUsage int
	Threshold    int
	UsagePercent float64
}

// ThresholdMonitorOption configures the threshold monitor.
type ThresholdMonitorOption func(*thresholdMonitorConfig)

type thresholdMonitorConfig struct {
	checkInterval    time.Duration
	warningCallback  func(ThresholdInfo)
	criticalCallback func(ThresholdInfo)
}

func WithCheckInterval(d time.Duration) ThresholdMonitorOption {
	return func(c *thresholdMonitorConfig) { c.checkInterval = d }
}

func WithWarningCallback(fn func(ThresholdInfo)) ThresholdMonitorOption {
	return func(c *thresholdMonitorConfig) { c.warningCallback = fn }
}

func WithCriticalCallback(fn func(ThresholdInfo)) ThresholdMonitorOption {
	return func(c *thresholdMonitorConfig) { c.criticalCallback = fn }
}

// ThresholdMonitor polls the Autotask ThresholdInformation endpoint and
// invokes callbacks when usage exceeds warning (75%) or critical (90%) levels.
type ThresholdMonitor struct {
	httpClient *http.Client
	baseURL    string
	config     thresholdMonitorConfig
	cancel     context.CancelFunc
	done       chan struct{}
}

// NewThresholdMonitor creates a new threshold monitor.
func NewThresholdMonitor(httpClient *http.Client, baseURL string, opts ...ThresholdMonitorOption) *ThresholdMonitor {
	cfg := thresholdMonitorConfig{
		checkInterval: 5 * time.Minute,
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	return &ThresholdMonitor{
		httpClient: httpClient,
		baseURL:    baseURL,
		config:     cfg,
		done:       make(chan struct{}),
	}
}

// Start begins the background monitoring goroutine.
func (m *ThresholdMonitor) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel

	go func() {
		defer close(m.done)
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

// Stop terminates the background monitor.
func (m *ThresholdMonitor) Stop() error {
	if m.cancel != nil {
		m.cancel()
		<-m.done
	}
	return nil
}

func (m *ThresholdMonitor) check(ctx context.Context) {
	url := m.baseURL + "/v1.0/ThresholdInformation"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return
	}

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	var data struct {
		CurrentCount int `json:"currentTimeframeRequestCount"`
		Threshold    int `json:"externalRequestThreshold"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return
	}

	if data.Threshold == 0 {
		return
	}

	info := ThresholdInfo{
		CurrentUsage: data.CurrentCount,
		Threshold:    data.Threshold,
		UsagePercent: float64(data.CurrentCount) / float64(data.Threshold) * 100,
	}

	if info.UsagePercent >= 90 && m.config.criticalCallback != nil {
		m.config.criticalCallback(info)
	} else if info.UsagePercent >= 75 && m.config.warningCallback != nil {
		m.config.warningCallback(info)
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -run TestThresholdMonitor -v ./middleware/...`
Expected: All PASS

- [ ] **Step 5: Commit**

```bash
git add middleware/threshold.go middleware/threshold_test.go
git commit -m "feat: add threshold monitor for API usage monitoring"
```

---

### Task 15: Wire Middleware Options to Client

**Files:**
- Modify: `option.go`
- Modify: `client.go`

- [ ] **Step 1: Write failing test for middleware wiring**

Add to `client_test.go`:

```go
func TestClientWithMiddleware(t *testing.T) {
	var middlewareCalled bool
	customMiddleware := func(next http.RoundTripper) http.RoundTripper {
		return roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			middlewareCalled = true
			return next.RoundTrip(req)
		})
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"item": map[string]any{"id": 1}})
	}))
	defer srv.Close()

	auth := AuthConfig{Username: "u", Secret: "s", IntegrationCode: "c"}
	client, _ := NewClient(context.Background(), auth,
		WithBaseURL(srv.URL),
		WithMiddleware(customMiddleware),
	)
	defer client.Close()

	client.do(context.Background(), http.MethodGet, "/v1.0/Test/1", nil, nil)

	if !middlewareCalled {
		t.Fatal("middleware was not called")
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -run TestClientWithMiddleware -v ./...`
Expected: FAIL

- [ ] **Step 3: Add middleware support to option.go and client.go**

Add to `option.go`:

```go
// WithMiddleware adds a custom RoundTripper middleware.
func WithMiddleware(m Middleware) ClientOption {
	return func(c *Client) {
		c.middlewares = append(c.middlewares, m)
	}
}
```

Update `client.go` — add `middlewares` field and apply them in `NewClient` after options:

Add to Client struct: `middlewares []Middleware`

After the options loop in `NewClient`, before zone discovery, apply middlewares to the httpClient's transport:

```go
// Apply middlewares to the HTTP transport.
if len(c.middlewares) > 0 {
	transport := c.httpClient.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	for i := len(c.middlewares) - 1; i >= 0; i-- {
		transport = c.middlewares[i](transport)
	}
	c.httpClient.Transport = transport
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -run TestClientWithMiddleware -v ./...`
Expected: PASS

- [ ] **Step 5: Add WithRateLimiter, WithCircuitBreaker, WithThresholdMonitor options**

Add to `option.go`:

```go
import "github.com/tphakala/go-autotask/middleware"

// WithRateLimiter enables rate limiting middleware.
func WithRateLimiter(opts ...middleware.RateLimitOption) ClientOption {
	return func(c *Client) {
		c.middlewares = append(c.middlewares, func(next http.RoundTripper) http.RoundTripper {
			return middleware.NewRateLimiter(next, opts...)
		})
	}
}

// WithCircuitBreaker enables circuit breaker middleware.
func WithCircuitBreaker(opts ...middleware.CircuitBreakerOption) ClientOption {
	return func(c *Client) {
		c.middlewares = append(c.middlewares, func(next http.RoundTripper) http.RoundTripper {
			return middleware.NewCircuitBreaker(next, opts...)
		})
	}
}

// WithThresholdMonitor enables background API usage monitoring.
func WithThresholdMonitor(opts ...middleware.ThresholdMonitorOption) ClientOption {
	return func(c *Client) {
		c.thresholdMonitorOpts = opts
	}
}
```

Add `thresholdMonitorOpts` field to Client struct in `client.go`:

```go
type Client struct {
	// ... existing fields ...
	thresholdMonitorOpts []middleware.ThresholdMonitorOption
}
```

Add threshold monitor startup at the end of `NewClient` in `client.go`, after zone discovery:

```go
	// Start threshold monitor if configured.
	if len(c.thresholdMonitorOpts) > 0 {
		monitor := middleware.NewThresholdMonitor(c.httpClient, c.baseURL, c.thresholdMonitorOpts...)
		monitor.Start()
		c.closers = append(c.closers, monitor.Stop)
	}
```

The `Close()` method already iterates `c.closers`, so `monitor.Stop` will be called automatically.

- [ ] **Step 6: Run all tests**

Run: `go test -v ./...`
Expected: All PASS

- [ ] **Step 7: Commit**

```bash
git add option.go client.go client_test.go
git commit -m "feat: wire middleware options to client"
```

---

### Task 16: Base Entity Types

**Files:**
- Create: `entities/ticket.go`
- Create: `entities/ticket_note.go`
- Create: `entities/company.go`
- Create: `entities/contact.go`
- Create: `entities/project.go`
- Create: `entities/task.go`
- Create: `entities/contract.go`
- Create: `entities/configuration_item.go`
- Create: `entities/resource.go`
- Create: `entities/time_entry.go`

Reference: Autotask API documentation for standard fields per entity. Use `/Users/e909385/src/vainu2/internal/autotask/types.go` for Ticket fields.

- [ ] **Step 1: Create Ticket entity with all standard fields**

```go
// entities/ticket.go
package entities

import (
	"time"

	autotask "github.com/tphakala/go-autotask"
)

// Ticket represents an Autotask Ticket entity.
type Ticket struct {
	ID                        autotask.Optional[int64]     `json:"id,omitzero"`
	Title                     autotask.Optional[string]    `json:"title,omitzero"`
	Description               autotask.Optional[string]    `json:"description,omitzero"`
	TicketNumber              autotask.Optional[string]    `json:"ticketNumber,omitzero"`
	Status                    autotask.Optional[int]       `json:"status,omitzero"`
	Priority                  autotask.Optional[int]       `json:"priority,omitzero"`
	QueueID                   autotask.Optional[int]       `json:"queueID,omitzero"`
	CompanyID                 autotask.Optional[int64]     `json:"companyID,omitzero"`
	CompanyLocationID         autotask.Optional[int64]     `json:"companyLocationID,omitzero"`
	ContactID                 autotask.Optional[int64]     `json:"contactID,omitzero"`
	ContractID                autotask.Optional[int64]     `json:"contractID,omitzero"`
	ConfigurationItemID       autotask.Optional[int64]     `json:"configurationItemID,omitzero"`
	AssignedResourceID        autotask.Optional[int64]     `json:"assignedResourceID,omitzero"`
	AssignedResourceRoleID    autotask.Optional[int64]     `json:"assignedResourceRoleID,omitzero"`
	DueDateTime               autotask.Optional[time.Time] `json:"dueDateTime,omitzero"`
	CreateDate                autotask.Optional[time.Time] `json:"createDate,omitzero"`
	LastActivityDate          autotask.Optional[time.Time] `json:"lastActivityDate,omitzero"`
	CompletedDate             autotask.Optional[time.Time] `json:"completedDate,omitzero"`
	TicketType                autotask.Optional[int]       `json:"ticketType,omitzero"`
	IssueType                 autotask.Optional[int]       `json:"issueType,omitzero"`
	SubIssueType              autotask.Optional[int]       `json:"subIssueType,omitzero"`
	TicketCategory            autotask.Optional[int]       `json:"ticketCategory,omitzero"`
	Source                    autotask.Optional[int]       `json:"source,omitzero"`
	BillingCodeID             autotask.Optional[int64]     `json:"billingCodeID,omitzero"`
	EstimatedHours            autotask.Optional[float64]   `json:"estimatedHours,omitzero"`
	ExternalID                autotask.Optional[string]    `json:"externalID,omitzero"`
	LastModifiedDate          autotask.Optional[time.Time] `json:"lastModifiedDate,omitzero"`
	UserDefinedFields         []autotask.UDF               `json:"userDefinedFields,omitempty"`
}

func (Ticket) EntityName() string { return "Tickets" }
```

- [ ] **Step 2: Create remaining entity files**

Follow the same pattern for TicketNote, Company, Contact, Project, Task, Contract, ConfigurationItem, Resource, TimeEntry. Each entity:
- Uses `autotask.Optional[T]` for all fields with `omitzero` tag
- Implements `EntityName()` with value receiver
- Includes `UserDefinedFields []autotask.UDF` field
- Includes standard fields from the Autotask API docs

- [ ] **Step 3: Verify compilation**

Run: `go build ./...`
Expected: Success

- [ ] **Step 4: Commit**

```bash
git add entities/
git commit -m "feat: add base entity types for common Autotask entities"
```

---

### Task 17: Metadata Discovery

**Files:**
- Create: `metadata/metadata.go`
- Create: `metadata/metadata_test.go`

- [ ] **Step 1: Write failing tests**

```go
// metadata/metadata_test.go
package metadata

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	autotask "github.com/tphakala/go-autotask"
)

func testClient(t *testing.T, handler http.Handler) *autotask.Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	auth := autotask.AuthConfig{Username: "u", Secret: "s", IntegrationCode: "c"}
	client, err := autotask.NewClient(context.Background(), auth, autotask.WithBaseURL(srv.URL))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { client.Close() })
	return client
}

func TestGetFields(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"fields": []any{
				map[string]any{
					"name": "status", "label": "Status", "dataType": "integer",
					"isRequired": true, "isReadOnly": false, "isPickList": true,
					"picklistValues": []any{
						map[string]any{"value": 1, "label": "New", "isActive": true},
						map[string]any{"value": 5, "label": "Complete", "isActive": true},
					},
				},
			},
		})
	})
	client := testClient(t, handler)

	fields, err := GetFields(context.Background(), client, "Tickets")
	if err != nil {
		t.Fatal(err)
	}
	if len(fields) != 1 {
		t.Fatalf("fields = %d; want 1", len(fields))
	}
	if fields[0].Name != "status" {
		t.Fatalf("name = %q; want status", fields[0].Name)
	}
	if len(fields[0].PickListValues) != 2 {
		t.Fatalf("picklist = %d; want 2", len(fields[0].PickListValues))
	}
}

func TestGetUDFs(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"fields": []any{
				map[string]any{
					"name": "CustomField1", "label": "Custom Field 1",
					"dataType": "string", "isRequired": false,
				},
			},
		})
	})
	client := testClient(t, handler)

	udfs, err := GetUDFs(context.Background(), client, "Tickets")
	if err != nil {
		t.Fatal(err)
	}
	if len(udfs) != 1 {
		t.Fatalf("udfs = %d; want 1", len(udfs))
	}
	if udfs[0].Name != "CustomField1" {
		t.Fatalf("name = %q; want CustomField1", udfs[0].Name)
	}
}

func TestGetEntityInfo(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"name":        "Tickets",
			"canCreate":   true,
			"canUpdate":   true,
			"canDelete":   false,
			"canQuery":    true,
			"hasUserDefinedFields": true,
		})
	})
	client := testClient(t, handler)

	info, err := GetEntityInfo(context.Background(), client, "Tickets")
	if err != nil {
		t.Fatal(err)
	}
	if info.Name != "Tickets" {
		t.Fatalf("name = %q", info.Name)
	}
	if !info.CanCreate {
		t.Fatal("expected CanCreate=true")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -run "TestGetFields|TestGetUDFs|TestGetEntityInfo" -v ./metadata/...`
Expected: FAIL

- [ ] **Step 3: Implement metadata discovery**

```go
// metadata/metadata.go
package metadata

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	autotask "github.com/tphakala/go-autotask"
)

// FieldInfo describes an entity field and its constraints.
type FieldInfo struct {
	Name           string          `json:"name"`
	Label          string          `json:"label"`
	Type           string          `json:"dataType"`
	IsRequired     bool            `json:"isRequired"`
	IsReadOnly     bool            `json:"isReadOnly"`
	IsPickList     bool            `json:"isPickList"`
	PickListValues []PickListValue `json:"picklistValues,omitempty"`
}

// PickListValue is a single pick list option.
type PickListValue struct {
	Value    int    `json:"value"`
	Label    string `json:"label"`
	IsActive bool   `json:"isActive"`
}

// UDFInfo describes a user-defined field.
type UDFInfo struct {
	Name       string `json:"name"`
	Label      string `json:"label"`
	Type       string `json:"dataType"`
	IsRequired bool   `json:"isRequired"`
}

// EntityInfo describes an entity's capabilities.
type EntityInfo struct {
	Name                 string `json:"name"`
	CanCreate            bool   `json:"canCreate"`
	CanUpdate            bool   `json:"canUpdate"`
	CanDelete            bool   `json:"canDelete"`
	CanQuery             bool   `json:"canQuery"`
	HasUserDefinedFields bool   `json:"hasUserDefinedFields"`
}

// GetFields returns field definitions for an entity, including picklist values.
func GetFields(ctx context.Context, c *autotask.Client, entityName string) ([]FieldInfo, error) {
	path := fmt.Sprintf("/v1.0/%s/entityInformation/fields", entityName)
	var resp struct {
		Fields []FieldInfo `json:"fields"`
	}
	if err := c.Do(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Fields, nil
}

// GetUDFs returns user-defined field definitions for an entity.
func GetUDFs(ctx context.Context, c *autotask.Client, entityName string) ([]UDFInfo, error) {
	path := fmt.Sprintf("/v1.0/%s/entityInformation/userDefinedFields", entityName)
	var resp struct {
		Fields []UDFInfo `json:"fields"`
	}
	if err := c.Do(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Fields, nil
}

// GetEntityInfo returns capability information for an entity.
func GetEntityInfo(ctx context.Context, c *autotask.Client, entityName string) (*EntityInfo, error) {
	path := fmt.Sprintf("/v1.0/%s/entityInformation", entityName)
	var info EntityInfo
	if err := c.Do(ctx, http.MethodGet, path, nil, &info); err != nil {
		return nil, err
	}
	return &info, nil
}
```

**Note:** This requires exporting `client.do` as `client.Do` for cross-package access. Add to `client.go`:

```go
// Do executes an HTTP request. Exported for use by sub-packages (metadata, autotasktest).
func (c *Client) Do(ctx context.Context, method, path string, body any, result any) error {
	return c.do(ctx, method, path, body, result)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -run "TestGetFields|TestGetUDFs|TestGetEntityInfo" -v ./metadata/...`
Expected: All PASS

- [ ] **Step 5: Commit**

```bash
git add metadata/ client.go
git commit -m "feat: add runtime metadata discovery for fields, UDFs, and entity info"
```

---

### Task 18: Test Helpers Package

**Files:**
- Create: `autotasktest/mock.go`

- [ ] **Step 1: Implement mock client helpers**

```go
// autotasktest/mock.go
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

// MockOption configures the mock server.
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

// WithFixture registers a response fixture for a method+path combination.
func WithFixture(method, path string, status int, body any) MockOption {
	return func(c *mockConfig) {
		c.fixtures = append(c.fixtures, fixture{method: method, path: path, status: status, body: body})
	}
}

// WithLatency adds artificial latency to all responses.
func WithLatency(d time.Duration) MockOption {
	return func(c *mockConfig) { c.latency = d }
}

// NewMockClient creates an autotask.Client backed by an httptest.Server
// with configurable response fixtures.
func NewMockClient(t *testing.T, opts ...MockOption) *autotask.Client {
	t.Helper()
	cfg := &mockConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	mux := http.NewServeMux()
	for _, f := range cfg.fixtures {
		f := f
		pattern := f.method + " " + f.path
		mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
			if cfg.latency > 0 {
				time.Sleep(cfg.latency)
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(f.status)
			if f.body != nil {
				json.NewEncoder(w).Encode(f.body)
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
	t.Cleanup(func() { client.Close() })
	return client
}
```

- [ ] **Step 2: Verify compilation**

Run: `go build ./...`
Expected: Success

- [ ] **Step 3: Commit**

```bash
git add autotasktest/
git commit -m "feat: add exported test helpers (NewMockClient, fixtures)"
```

---

### Task 19: Code Generator CLI (Scaffold)

**Files:**
- Create: `cmd/autotask-gen/main.go`
- Create: `cmd/autotask-gen/generator.go`

This task creates the scaffold and core generation logic. Full entity generation is iterative — the generator connects to a live instance and generates entity Go files.

- [ ] **Step 1: Implement generator scaffold**

```go
// cmd/autotask-gen/main.go
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	autotask "github.com/tphakala/go-autotask"
)

func main() {
	username := flag.String("username", "", "Autotask API username")
	secret := flag.String("secret", "", "Autotask API secret")
	integrationCode := flag.String("integration-code", "", "Autotask API integration code")
	output := flag.String("output", "./entities", "Output directory for generated files")
	flag.Parse()

	if *username == "" || *secret == "" || *integrationCode == "" {
		fmt.Fprintln(os.Stderr, "usage: autotask-gen -username USER -secret SECRET -integration-code CODE [-output DIR]")
		os.Exit(1)
	}

	ctx := context.Background()
	auth := autotask.AuthConfig{
		Username:        *username,
		Secret:          *secret,
		IntegrationCode: *integrationCode,
	}

	client, err := autotask.NewClient(ctx, auth)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	gen := &Generator{
		Client:    client,
		OutputDir: *output,
	}
	if err := gen.Generate(ctx); err != nil {
		log.Fatalf("Generation failed: %v", err)
	}
	fmt.Println("Generation complete.")
}
```

```go
// cmd/autotask-gen/generator.go
package main

import (
	"context"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	autotask "github.com/tphakala/go-autotask"
	"github.com/tphakala/go-autotask/metadata"
)

// Generator generates Go entity files from Autotask metadata.
type Generator struct {
	Client    *autotask.Client
	OutputDir string
}

// Generate discovers entities and generates Go source files.
func (g *Generator) Generate(ctx context.Context) error {
	if err := os.MkdirAll(g.OutputDir, 0o755); err != nil {
		return fmt.Errorf("creating output dir: %w", err)
	}

	// List of entities to generate. In a full implementation, this would
	// be discovered dynamically.
	entities := []string{
		"Tickets", "Companies", "Contacts", "Projects", "Tasks",
		"Contracts", "ConfigurationItems", "Resources", "TimeEntries",
	}

	for _, entityName := range entities {
		fields, err := metadata.GetFields(ctx, g.Client, entityName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: skipping %s: %v\n", entityName, err)
			continue
		}

		udfs, _ := metadata.GetUDFs(ctx, g.Client, entityName)

		if err := g.generateEntity(entityName, fields, udfs); err != nil {
			return fmt.Errorf("generating %s: %w", entityName, err)
		}
		fmt.Printf("Generated %s\n", entityName)
	}
	return nil
}

func (g *Generator) generateEntity(name string, fields []metadata.FieldInfo, udfs []metadata.UDFInfo) error {
	filename := strings.ToLower(name) + ".go"
	path := filepath.Join(g.OutputDir, filename)

	var buf strings.Builder
	if err := entityTemplate.Execute(&buf, entityData{
		Name:   name,
		Fields: fields,
		UDFs:   udfs,
	}); err != nil {
		return err
	}

	formatted, err := format.Source([]byte(buf.String()))
	if err != nil {
		// Write unformatted for debugging.
		return os.WriteFile(path, []byte(buf.String()), 0o644)
	}
	return os.WriteFile(path, formatted, 0o644)
}

type entityData struct {
	Name   string
	Fields []metadata.FieldInfo
	UDFs   []metadata.UDFInfo
}

var entityTemplate = template.Must(template.New("entity").Funcs(template.FuncMap{
	"goType":    goType,
	"goName":    goName,
	"singular":  singular,
	"jsonTag":   func(s string) string { return s },
}).Parse(`// Code generated by autotask-gen. DO NOT EDIT.
package entities

import (
	"time"

	autotask "github.com/tphakala/go-autotask"
)

// {{singular .Name}} represents an Autotask {{singular .Name}} entity.
type {{singular .Name}} struct {
{{- range .Fields}}
	{{goName .Name}} autotask.Optional[{{goType .Type}}] ` + "`" + `json:"{{jsonTag .Name}},omitzero"` + "`" + `
{{- end}}
	UserDefinedFields []autotask.UDF ` + "`" + `json:"userDefinedFields,omitempty"` + "`" + `
}

func ({{singular .Name}}) EntityName() string { return "{{.Name}}" }
`))

func goType(atType string) string {
	switch strings.ToLower(atType) {
	case "integer", "long":
		return "int64"
	case "short":
		return "int"
	case "double", "decimal":
		return "float64"
	case "boolean":
		return "bool"
	case "datetime":
		return "time.Time"
	default:
		return "string"
	}
}

func goName(s string) string {
	if len(s) == 0 {
		return s
	}
	// Capitalize first letter.
	return strings.ToUpper(s[:1]) + s[1:]
}

func singular(s string) string {
	if strings.HasSuffix(s, "ies") {
		return s[:len(s)-3] + "y"
	}
	return strings.TrimSuffix(s, "s")
}
```

- [ ] **Step 2: Verify compilation**

Run: `go build ./cmd/autotask-gen/`
Expected: Success

- [ ] **Step 3: Commit**

```bash
git add cmd/
git commit -m "feat: add autotask-gen code generator scaffold"
```

---

### Task 20: Examples

**Files:**
- Create: `examples/basic/main.go`
- Create: `examples/query/main.go`
- Create: `examples/middleware/main.go`

- [ ] **Step 1: Create basic example**

```go
// examples/basic/main.go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	autotask "github.com/tphakala/go-autotask"
	"github.com/tphakala/go-autotask/entities"
)

func main() {
	ctx := context.Background()

	client, err := autotask.NewClient(ctx, autotask.AuthConfig{
		Username:        os.Getenv("AUTOTASK_USERNAME"),
		Secret:          os.Getenv("AUTOTASK_SECRET"),
		IntegrationCode: os.Getenv("AUTOTASK_INTEGRATION_CODE"),
	})
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	// Get a ticket by ID.
	ticket, err := autotask.Get[entities.Ticket](ctx, client, 12345)
	if err != nil {
		log.Fatal(err)
	}

	if title, ok := ticket.Title.Get(); ok {
		fmt.Printf("Ticket: %s\n", title)
	}

	// Create a ticket.
	newTicket := &entities.Ticket{
		Title:     autotask.Set("Server unreachable"),
		CompanyID: autotask.Set(int64(0)),
		Status:    autotask.Set(1),
		Priority:  autotask.Set(2),
	}
	created, err := autotask.Create(ctx, client, newTicket)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Created ticket: %v\n", created)
}
```

- [ ] **Step 2: Create query example**

```go
// examples/query/main.go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	autotask "github.com/tphakala/go-autotask"
	"github.com/tphakala/go-autotask/entities"
)

func main() {
	ctx := context.Background()

	client, err := autotask.NewClient(ctx, autotask.AuthConfig{
		Username:        os.Getenv("AUTOTASK_USERNAME"),
		Secret:          os.Getenv("AUTOTASK_SECRET"),
		IntegrationCode: os.Getenv("AUTOTASK_INTEGRATION_CODE"),
	})
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	// Query open high-priority tickets.
	tickets, err := autotask.List[entities.Ticket](ctx, client,
		autotask.NewQuery().
			Where("status", autotask.OpEq, 1).
			Or(
				autotask.Field("priority", autotask.OpEq, 1),
				autotask.Field("priority", autotask.OpEq, 2),
			).
			Fields("id", "title", "status", "priority").
			Limit(50),
	)
	if err != nil {
		log.Fatal(err)
	}

	for _, t := range tickets {
		title, _ := t.Title.Get()
		fmt.Printf("  [%d] %s\n", func() int64 { v, _ := t.ID.Get(); return v }(), title)
	}

	// Iterator-based pagination for large result sets.
	fmt.Println("\nAll tickets (iterator):")
	for ticket, err := range autotask.ListIter[entities.Ticket](ctx, client, autotask.NewQuery()) {
		if err != nil {
			log.Fatal(err)
		}
		title, _ := ticket.Title.Get()
		fmt.Printf("  %s\n", title)
	}
}
```

- [ ] **Step 3: Create middleware example**

```go
// examples/middleware/main.go
package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"

	autotask "github.com/tphakala/go-autotask"
	"github.com/tphakala/go-autotask/entities"
	"github.com/tphakala/go-autotask/middleware"
)

func main() {
	ctx := context.Background()

	client, err := autotask.NewClient(ctx,
		autotask.AuthConfig{
			Username:        os.Getenv("AUTOTASK_USERNAME"),
			Secret:          os.Getenv("AUTOTASK_SECRET"),
			IntegrationCode: os.Getenv("AUTOTASK_INTEGRATION_CODE"),
		},
		autotask.WithLogger(slog.Default()),
		autotask.WithRateLimiter(
			middleware.WithRequestsPerHour(8000),
			middleware.WithBurstSize(10),
		),
		autotask.WithCircuitBreaker(
			middleware.WithFailureThreshold(5),
		),
		autotask.WithThresholdMonitor(
			middleware.WithCriticalCallback(func(info middleware.ThresholdInfo) {
				slog.Error("API usage critical",
					"percent", fmt.Sprintf("%.1f%%", info.UsagePercent),
					"current", info.CurrentUsage,
					"threshold", info.Threshold,
				)
			}),
		),
		autotask.WithImpersonation(12345),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	ticket, err := autotask.Get[entities.Ticket](ctx, client, 1)
	if err != nil {
		log.Fatal(err)
	}
	title, _ := ticket.Title.Get()
	fmt.Printf("Ticket: %s\n", title)
}
```

- [ ] **Step 4: Verify compilation**

Run: `go build ./examples/...`
Expected: Success (or expected build errors for missing env vars — examples aren't runnable without credentials)

- [ ] **Step 5: Commit**

```bash
git add examples/
git commit -m "feat: add usage examples for basic, query, and middleware patterns"
```

---

### Task 21: Benchmarks

**Files:**
- Create: `benchmark_test.go`

- [ ] **Step 1: Write benchmarks for query serialization, response parsing, rate limiter**

```go
// benchmark_test.go
package autotask

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func BenchmarkQueryMarshal(b *testing.B) {
	q := NewQuery().
		Where("status", OpEq, 1).
		Where("queueID", OpEq, 8).
		Or(
			Field("priority", OpEq, 1),
			Field("priority", OpEq, 2),
		).
		Fields("id", "title", "status").
		Limit(100)

	for b.Loop() {
		json.Marshal(q)
	}
}

func BenchmarkParseResponseSuccess(b *testing.B) {
	body := `{"item":{"id":123,"title":"Test Ticket","status":1}}`
	for b.Loop() {
		resp := &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{},
		}
		var result map[string]any
		parseResponse(resp, &result)
	}
}

func BenchmarkParseResponseError(b *testing.B) {
	body := `{"errors":["Not found"]}`
	for b.Loop() {
		resp := &http.Response{
			StatusCode: 404,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{},
		}
		parseResponse(resp, nil)
	}
}

func BenchmarkOptionalMarshal(b *testing.B) {
	type S struct {
		Name  Optional[string] `json:"name,omitzero"`
		Value Optional[int]    `json:"value,omitzero"`
		Clear Optional[string] `json:"clear,omitzero"`
	}
	s := S{Name: Set("test"), Clear: Null[string]()}
	for b.Loop() {
		json.Marshal(s)
	}
}
```

- [ ] **Step 2: Run benchmarks**

Run: `go test -bench=. -benchmem -v ./...`
Expected: Benchmarks complete with timing and allocation data

- [ ] **Step 3: Commit**

```bash
git add benchmark_test.go
git commit -m "feat: add benchmarks for query, parsing, and Optional marshaling"
```

---

### Task 22: Integration Test Scaffold

**Files:**
- Create: `integration_test.go`

- [ ] **Step 1: Create integration test file with build tag**

```go
//go:build integration

// integration_test.go
package autotask

import (
	"context"
	"os"
	"testing"

	"github.com/tphakala/go-autotask/entities"
	"github.com/tphakala/go-autotask/metadata"
)

func integrationClient(t *testing.T) *Client {
	t.Helper()
	username := os.Getenv("AUTOTASK_USERNAME")
	secret := os.Getenv("AUTOTASK_SECRET")
	code := os.Getenv("AUTOTASK_INTEGRATION_CODE")
	if username == "" || secret == "" || code == "" {
		t.Skip("AUTOTASK_USERNAME, AUTOTASK_SECRET, AUTOTASK_INTEGRATION_CODE required")
	}
	client, err := NewClient(context.Background(), AuthConfig{
		Username:        username,
		Secret:          secret,
		IntegrationCode: code,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { client.Close() })
	return client
}

func TestIntegrationZoneDiscovery(t *testing.T) {
	client := integrationClient(t)
	if client.baseURL == "" {
		t.Fatal("baseURL should be set after zone discovery")
	}
	t.Logf("Zone URL: %s", client.baseURL)
}

func TestIntegrationListTickets(t *testing.T) {
	client := integrationClient(t)
	tickets, err := List[entities.Ticket](context.Background(), client,
		NewQuery().Where("status", OpEq, 1).Limit(5),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Found %d open tickets", len(tickets))
}

func TestIntegrationGetFields(t *testing.T) {
	client := integrationClient(t)
	fields, err := metadata.GetFields(context.Background(), client, "Tickets")
	if err != nil {
		t.Fatal(err)
	}
	if len(fields) == 0 {
		t.Fatal("expected at least one field")
	}
	t.Logf("Found %d fields for Tickets", len(fields))
}

func TestIntegrationCountTickets(t *testing.T) {
	client := integrationClient(t)
	count, err := Count[entities.Ticket](context.Background(), client, NewQuery())
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Total tickets: %d", count)
}
```

- [ ] **Step 2: Verify build tag excludes from normal test runs**

Run: `go test -v ./...`
Expected: Integration tests NOT included (no "integration" build tag)

Run: `go test -tags integration -v -run TestIntegration ./...`
Expected: Only runs with real credentials (or skips with helpful message)

- [ ] **Step 3: Commit**

```bash
git add integration_test.go
git commit -m "feat: add integration test scaffold with build tag"
```

---

### Task 23: Final Verification and Cleanup

- [ ] **Step 1: Run full test suite**

Run: `go test -v ./...`
Expected: All PASS

- [ ] **Step 2: Run linter**

Run: `go vet ./...`
Expected: No issues

- [ ] **Step 3: Run benchmarks**

Run: `go test -bench=. -benchmem ./...`
Expected: All benchmarks complete

- [ ] **Step 4: Verify all packages build**

Run: `go build ./...`
Expected: Success

- [ ] **Step 5: Final commit**

```bash
git add -A
git commit -m "chore: final cleanup and verification"
```
