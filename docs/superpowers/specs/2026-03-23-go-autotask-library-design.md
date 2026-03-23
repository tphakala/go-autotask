# go-autotask Library Design Spec

**Module:** `github.com/tphakala/go-autotask`
**Go version:** 1.26
**License:** Open source (MIT or Apache-2.0)
**Date:** 2026-03-23

## Overview

A generic, open-source Go client library for the Autotask PSA REST API. Provides type-safe CRUD operations for all Autotask entities using Go generics, with composable middleware for resilience (rate limiting, circuit breaking, threshold monitoring). Entity types are code-generated from Autotask metadata, with runtime discovery for tenant-specific picklists and UDFs.

## Architecture

### Core Client

```go
package autotask

type Client struct {
    httpClient  *http.Client
    baseURL     string          // discovered via zone info
    auth        AuthConfig
    zoneCache   *ZoneCache
    middlewares []Middleware
    logger      *slog.Logger
}

type AuthConfig struct {
    Username        string
    Secret          string
    IntegrationCode string
}

type Middleware func(next http.RoundTripper) http.RoundTripper

type ClientOption func(*Client)

func NewClient(ctx context.Context, auth AuthConfig, opts ...ClientOption) (*Client, error)
```

Key behaviors:
- `NewClient` accepts `context.Context` for zone discovery timeout/cancellation
- `NewClient` performs zone discovery automatically (cached for 24h)
- Auth headers (`Username`, `Secret`, `ApiIntegrationCode`) injected on every request
- Optional `ImpersonationResourceID` header via `WithImpersonation(resourceID int64)`
- Default `User-Agent: go-autotask/<version>` header, overridable via `WithUserAgent()`
- TLS 1.2+ enforced
- `*http.Client` customizable via `WithHTTPClient()`
- Middlewares compose as `RoundTripper` decorators
- `slog.Logger` for structured logging, defaults to no-op
- Implements `io.Closer` to clean up background goroutines (threshold monitor)

### ClientOption Functions

```go
func WithHTTPClient(c *http.Client) ClientOption
func WithLogger(l *slog.Logger) ClientOption
func WithBaseURL(url string) ClientOption           // override zone discovery
func WithRateLimiter(opts ...RateLimitOption) ClientOption
func WithCircuitBreaker(opts ...CircuitBreakerOption) ClientOption
func WithThresholdMonitor(opts ...ThresholdMonitorOption) ClientOption
func WithMiddleware(m Middleware) ClientOption       // custom middleware
func WithImpersonation(resourceID int64) ClientOption // ImpersonationResourceId header
func WithUserAgent(ua string) ClientOption            // override default User-Agent
```

## Generic CRUD Operations

```go
// Entity is the interface all typed entities implement.
// EntityName() MUST be implemented with a value receiver to prevent
// double-pointer issues with generic functions (e.g., Get[*Ticket] → **Ticket).
type Entity interface {
    EntityName() string  // e.g., "Tickets", "Companies"
}

// Typed CRUD
func Get[T Entity](ctx context.Context, c *Client, id int64) (*T, error)
func List[T Entity](ctx context.Context, c *Client, q *Query) ([]*T, error)
func Create[T Entity](ctx context.Context, c *Client, entity *T) (*T, error)
func Update[T Entity](ctx context.Context, c *Client, entity *T) (*T, error)
func Delete[T Entity](ctx context.Context, c *Client, id int64) error
func Count[T Entity](ctx context.Context, c *Client, q *Query) (int64, error)

// Iterator-based pagination for large result sets
func ListIter[T Entity](ctx context.Context, c *Client, q *Query) iter.Seq2[*T, error]

// Child entity access (e.g., ticket notes)
func GetChild[P Entity, C Entity](ctx context.Context, c *Client, parentID int64) ([]*C, error)
func CreateChild[P Entity, C Entity](ctx context.Context, c *Client, parentID int64, child *C) (*C, error)

// Untyped access for entities without generated types
func GetRaw(ctx context.Context, c *Client, entityName string, id int64) (map[string]any, error)
func ListRaw(ctx context.Context, c *Client, entityName string, q *Query) ([]map[string]any, error)
func CreateRaw(ctx context.Context, c *Client, entityName string, data map[string]any) (map[string]any, error)
func UpdateRaw(ctx context.Context, c *Client, entityName string, data map[string]any) (map[string]any, error)
func DeleteRaw(ctx context.Context, c *Client, entityName string, id int64) error
```

Key behaviors:
- `List`/`ListRaw` handle pagination internally — follow `nextPageUrl` until exhausted
- `Query.MaxRecords` caps total results across all pages (not per-page). Internal pagination stops once the cumulative count reaches the limit, slicing the final page if necessary
- `ListIter` returns results lazily via Go 1.26 iterators for memory efficiency
- Each page requests up to 500 records (Autotask maximum)

## Query Builder

```go
type Query struct {
    conditions    []Condition
    includeFields []string
    maxRecords    int
}

// Condition is the interface for filter expressions. Supports both simple
// field comparisons and recursive AND/OR grouping for complex queries like
// (A AND B) OR (C AND D).
type Condition interface {
    conditionNode()  // sealed interface
}

// FieldCondition is a simple field comparison.
type FieldCondition struct {
    Field string
    Op    Operator
    Value any
    UDF   bool     // true for user-defined field filters
}

// GroupCondition combines multiple conditions with AND/OR.
type GroupCondition struct {
    Op    GroupOperator  // "and" or "or"
    Items []Condition
}

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

type GroupOperator string

const (
    GroupAnd GroupOperator = "and"
    GroupOr  GroupOperator = "or"
)

// Fluent builder
func NewQuery() *Query
func (q *Query) Where(field string, op Operator, value any) *Query
func (q *Query) WhereUDF(field string, op Operator, value any) *Query
func (q *Query) And(conditions ...Condition) *Query    // nested AND group
func (q *Query) Or(conditions ...Condition) *Query     // nested OR group
func (q *Query) Fields(fields ...string) *Query
func (q *Query) Limit(n int) *Query                    // caps total results across pages

// Convenience constructors for conditions
func Field(name string, op Operator, value any) FieldCondition
func UDField(name string, op Operator, value any) FieldCondition
func And(conditions ...Condition) GroupCondition
func Or(conditions ...Condition) GroupCondition
```

Usage example:
```go
// Simple query
tickets, err := autotask.List[entities.Ticket](ctx, client,
    autotask.NewQuery().
        Where("status", autotask.OpEq, 1).
        Where("queueID", autotask.OpEq, 8).
        Fields("id", "title", "status", "priority").
        Limit(100),
)

// Complex nested query: (status=1 AND queueID=8) OR (priority=1 AND priority=2)
tickets, err := autotask.List[entities.Ticket](ctx, client,
    autotask.NewQuery().
        Or(
            autotask.And(
                autotask.Field("status", autotask.OpEq, 1),
                autotask.Field("queueID", autotask.OpEq, 8),
            ),
            autotask.And(
                autotask.Field("priority", autotask.OpEq, 1),
                autotask.Field("priority", autotask.OpEq, 2),
            ),
        ).
        Fields("id", "title", "status", "priority").
        Limit(100),
)
```

Serializes to Autotask JSON filter format. Always uses POST for query requests (simpler, avoids 2048 char URL limit).

## Error Handling

```go
type Error struct {
    StatusCode int
    Message    string
    Errors     []APIError
}

type APIError struct {
    Message string
    Field   string
}

func (e *Error) Error() string

// Typed errors per HTTP status
type ValidationError struct{ Error }       // 400
type AuthenticationError struct{ Error }   // 401
type AuthorizationError struct{ Error }    // 403
type NotFoundError struct{ Error }         // 404
type ConflictError struct{ Error }         // 409
type BusinessLogicError struct{ Error }    // 422
type RateLimitError struct {               // 429
    Error
    RetryAfter time.Duration
}
type ServerError struct{ Error }           // 5xx
```

Consumers use Go 1.26 `errors.AsType[E]()` for type-safe inspection:
```go
if rl, ok := errors.AsType[*autotask.RateLimitError](err); ok {
    time.Sleep(rl.RetryAfter)
}
```

Response parsing extracts Autotask error arrays from JSON bodies and maps HTTP status codes to typed errors. Even 200 responses are checked for error payloads.

## Middleware / Resilience

All resilience features are composable middleware wrapping `http.RoundTripper`, enabled via `ClientOption` functions.

### Rate Limiter

```go
func WithRateLimiter(opts ...RateLimitOption) ClientOption

type RateLimitOption func(*rateLimitConfig)
func WithRequestsPerHour(n int) RateLimitOption      // default: 5000
func WithBurstSize(n int) RateLimitOption             // default: 20
func WithAdaptiveDelay(enabled bool) RateLimitOption  // default: true
```

Token bucket implementation using `golang.org/x/time/rate`. Adaptive delays match Autotask thresholds: +0.5s at 50% usage, +1.0s at 75%+.

**Important:** The Autotask rate limit (10k req/hour) is per-database, shared across all integrations. The local token bucket is a best-effort client-side guard. The rate limiter middleware also parses and strictly respects HTTP `429 Retry-After` headers from Autotask to handle cases where other integrations consume quota or multiple replicas of the same service are running.

### Circuit Breaker

```go
func WithCircuitBreaker(opts ...CircuitBreakerOption) ClientOption

type CircuitBreakerOption func(*circuitBreakerConfig)
func WithFailureThreshold(n int) CircuitBreakerOption        // default: 5
func WithFailureWindow(d time.Duration) CircuitBreakerOption // default: 10s
func WithOpenTimeout(d time.Duration) CircuitBreakerOption   // default: 30s
```

Three-state circuit breaker (Closed, Open, Half-Open). Triggers on 5xx errors, rate limits, and timeouts.

### Threshold Monitor

```go
func WithThresholdMonitor(opts ...ThresholdMonitorOption) ClientOption

type ThresholdMonitorOption func(*thresholdMonitorConfig)
func WithCheckInterval(d time.Duration) ThresholdMonitorOption    // default: 5m
func WithWarningCallback(fn func(ThresholdInfo)) ThresholdMonitorOption   // 75%
func WithCriticalCallback(fn func(ThresholdInfo)) ThresholdMonitorOption  // 90%
```

Background goroutine polls `/v1.0/ThresholdInformation`. Stopped via `client.Close()`.

**Thread safety note:** Threshold callbacks are invoked asynchronously from the monitor goroutine. Consumer callback implementations must be goroutine-safe.

## Entity Types & Code Generation

### Pre-generated base types (`entities/` package)

```go
package entities

type Ticket struct {
    ID                Optional[int64]     `json:"id,omitzero"`
    Title             Optional[string]    `json:"title,omitzero"`
    Description       Optional[string]    `json:"description,omitzero"`
    Status            Optional[int]       `json:"status,omitzero"`
    Priority          Optional[int]       `json:"priority,omitzero"`
    QueueID           Optional[int]       `json:"queueID,omitzero"`
    CompanyID         Optional[int64]     `json:"companyID,omitzero"`
    DueDateTime       Optional[time.Time] `json:"dueDateTime,omitzero"`
    UserDefinedFields []UDF               `json:"userDefinedFields,omitempty"`
    // ... all standard fields
}

func (Ticket) EntityName() string { return "Tickets" }
```

#### Three-state Optional type

Standard Go pointer + `omitempty` cannot distinguish "unset" (omit from JSON) from
"clear" (send JSON `null`). The `Optional[T]` type solves this with Go 1.24+'s
`omitzero` tag and `IsZero()` method:

```go
package autotask

// Optional represents a three-state field: unset, null, or set to a value.
// - Zero value (unset): omitted from JSON via omitzero + IsZero()
// - Null(): serializes as JSON null (clears the field in Autotask)
// - Set(v): serializes as the JSON value
type Optional[T any] struct {
    value T
    set   bool  // true if explicitly set (to value or null)
    null  bool  // true if explicitly set to null
}

func Set[T any](v T) Optional[T]   // field has a value
func Null[T any]() Optional[T]      // explicitly clear the field
func (o Optional[T]) Get() (T, bool) // returns value and whether set
func (o Optional[T]) IsNull() bool
func (o Optional[T]) IsSet() bool

// IsZero returns true when the field is unset. Used by encoding/json with
// the `omitzero` struct tag to completely omit unset fields from JSON output.
func (o Optional[T]) IsZero() bool { return !o.set }

// MarshalJSON handles set vs null:
// - unset: not called (omitzero prevents it)
// - null: returns []byte("null")
// - set: returns json.Marshal(o.value)
func (o Optional[T]) MarshalJSON() ([]byte, error)
func (o *Optional[T]) UnmarshalJSON(data []byte) error
```

All entity struct fields use `omitzero` tags so unset `Optional[T]` fields are
completely omitted from JSON output (partial updates). Set fields serialize their
value, and `Null()` fields serialize as JSON `null` to explicitly clear fields.

Usage:
```go
// Create with values
t := &entities.Ticket{
    Title:     autotask.Set("Server down"),
    Priority:  autotask.Set(1),
    CompanyID: autotask.Set(int64(12345)),
}

// Update: clear the due date explicitly
update := &entities.Ticket{
    ID:          autotask.Set(int64(67890)),
    DueDateTime: autotask.Null[time.Time](), // sends null to Autotask
    // Title is unset → omitted from JSON, not modified
}
```

- `Optional[T]` with custom JSON marshaling handles all three states correctly
- All generated entity structs use `Optional[T]` for nullable fields
- UDFs represented as `[]UDF` slice on every entity
- All generated entity types use **value receivers** for `EntityName()` to prevent
  double-pointer issues with generic functions
- ~30 most common entities pre-generated in the library

### Runtime metadata discovery (`metadata/` package)

```go
package metadata

func GetFields(ctx context.Context, c *autotask.Client, entityName string) ([]FieldInfo, error)
func GetUDFs(ctx context.Context, c *autotask.Client, entityName string) ([]UDFInfo, error)
func GetEntityInfo(ctx context.Context, c *autotask.Client, entityName string) (*EntityInfo, error)

type FieldInfo struct {
    Name           string
    Label          string
    Type           string
    IsRequired     bool
    IsReadOnly     bool
    IsPickList     bool
    PickListValues []PickListValue
}

type PickListValue struct {
    Value    int
    Label    string
    IsActive bool
}
```

For tenant-specific picklists and UDFs that vary across Autotask instances.

### Code generator (`cmd/autotask-gen`)

```bash
autotask-gen -username USER -secret SECRET -integration-code CODE -output ./internal/autotask/entities/
```

Connects to a live Autotask instance, queries all entity metadata endpoints, and generates Go files with tenant-specific picklist constants and UDF-typed fields.

**Important:** The generator outputs into the *consumer's* repository, not the library itself. The open-source library ships with base entity types containing standard fields (consistent across all Autotask tenants). Consumers run `autotask-gen` against their own instance to get extended types with their tenant-specific UDFs and picklist values. Generated types embed the base types from the library so they remain compatible with the generic CRUD functions.

**UDF marshaling:** The Autotask API requires UDFs in the `"userDefinedFields": []` array, not as top-level JSON keys. Generated tenant-specific entity structs include custom `MarshalJSON()`/`UnmarshalJSON()` methods that:
- On marshal: pack typed UDF fields into the embedded base struct's `UserDefinedFields` array before serialization
- On unmarshal: unpack the `UserDefinedFields` array back into the typed UDF fields

This allows consumers to work with UDFs as typed struct fields while maintaining API compatibility.

## Project Structure

```text
go-autotask/
├── go.mod                          # github.com/tphakala/go-autotask, go 1.26
├── LICENSE
├── client.go                       # Client struct, NewClient, Close
├── option.go                       # ClientOption funcs
├── auth.go                         # AuthConfig, header injection
├── zone.go                         # Zone discovery + cache
├── crud.go                         # Get[T], List[T], Create[T], Update[T], Delete[T]
├── raw.go                          # GetRaw, ListRaw, CreateRaw, UpdateRaw, DeleteRaw
├── iter.go                         # ListIter[T], iterator-based pagination
├── child.go                        # GetChild[P,C], CreateChild[P,C]
├── query.go                        # Query builder, filter serialization
├── error.go                        # Typed errors, response parsing
├── entity.go                       # Entity interface, UDF type
├── middleware/
│   ├── ratelimit.go                # Token bucket rate limiter
│   ├── circuitbreaker.go           # Circuit breaker
│   └── threshold.go                # Threshold monitor
├── entities/
│   ├── ticket.go
│   ├── ticket_note.go
│   ├── company.go
│   ├── contact.go
│   ├── project.go
│   ├── task.go
│   ├── contract.go
│   ├── configuration_item.go
│   ├── resource.go
│   ├── time_entry.go
│   └── ...                         # ~30 most common entities
├── metadata/
│   └── metadata.go                 # Runtime field/UDF/picklist discovery
├── cmd/
│   └── autotask-gen/
│       └── main.go                 # Code generator CLI
└── examples/
    ├── basic/main.go
    ├── query/main.go
    └── middleware/main.go
```

## Dependencies

- `golang.org/x/time/rate` — rate limiter token bucket
- Standard library for everything else (`net/http`, `encoding/json`, `log/slog`, `iter`, etc.)

## Testing Strategy

### Unit tests
- Core client: mock HTTP via `httptest.Server` (Go 1.26 `example.com` redirect)
- Query builder: serialization to Autotask JSON filter format
- Error handling: HTTP status to typed error mapping
- Middleware: each tested in isolation wrapping a mock `RoundTripper`
- Zone cache: TTL expiration, thread safety

### Integration tests (build tag)
```go
//go:build integration
```
Require `AUTOTASK_USERNAME`, `AUTOTASK_SECRET`, `AUTOTASK_INTEGRATION_CODE` env vars.

### Exported test helpers
```go
package autotasktest

func NewMockClient(t *testing.T, opts ...MockOption) *autotask.Client

type MockOption func(*mockConfig)
func WithFixture(method, path string, status int, body any) MockOption
func WithLatency(d time.Duration) MockOption
```

### Benchmarks
- Query serialization, response parsing, rate limiter throughput
- Using `b.Loop()` (Go 1.26)
- Goroutine leak detection via `GOEXPERIMENT=goroutineleakprofile` in CI

## Autotask API Reference

### Authentication
- Header-based: `Username`, `Secret`, `ApiIntegrationCode` headers on every request
- No token refresh needed
- TLS 1.2 required

### Zone Discovery
- `GET https://webservices2.autotask.net/atservicesrest/versioninformation` for API versions
- `GET {baseUrl}/zoneInformation?user={username}` for zone-specific endpoint
- Zone URL cached, used as base for all subsequent calls
- Zone calls exempt from rate limits

### Endpoints Pattern
- `GET /v1.0/{Entity}/{id}` — get by ID
- `GET /v1.0/{Entity}/query?search={filter}` — query (GET)
- `POST /v1.0/{Entity}/query` — query (POST, for large filters)
- `GET /v1.0/{Entity}/query/count` — count
- `POST /v1.0/{Entity}` — create
- `PATCH /v1.0/{Entity}` — update
- `DELETE /v1.0/{Entity}/{id}` — delete
- `GET /v1.0/{Parent}/{id}/{Child}` — child entities
- `GET /v1.0/{Entity}/entityInformation/fields` — field metadata

### Pagination
- Max 500 records per response
- `pageDetails.nextPageUrl` for next page
- Follow until `nextPageUrl` is null

### Rate Limits
- 10,000 requests/hour per database (shared across all integrations)
- Progressive latency: 0-49.99% = 0s, 50-74.99% = +0.5s, 75%+ = +1.0s
- 429 response with `Retry-After` header when exceeded
- Thread limiting: 3 concurrent per endpoint per tracking ID

### Query Filter Format
```json
{
    "filter": [
        {"op": "eq", "field": "status", "value": 1},
        {
            "op": "or",
            "items": [
                {"op": "eq", "field": "priority", "value": 1},
                {"op": "eq", "field": "priority", "value": 2}
            ]
        }
    ],
    "MaxRecords": 100,
    "IncludeFields": ["id", "title", "status"]
}
```
