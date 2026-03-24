# API Mock Test Suite Design

## Goal

Add comprehensive mock tests that validate the Go client library against the Autotask PSA REST API specification. Tests verify both request construction (headers, paths, methods, filter JSON) and response parsing (entities, pagination, errors, metadata).

## Constraints

- No live API access required — all tests use an in-process mock server
- Backward compatible with existing `autotasktest.NewMockClient` / `WithFixture` API
- Realistic fixtures with proper field types, dates, UDFs — not minimal stubs
- Tests must run fast (no network, no sleeps)

## Mock Server Architecture

### Core: `autotasktest/server.go`

A `TestServer` struct wrapping `httptest.Server`:

```go
type TestServer struct {
    *httptest.Server
    entities map[string]*entityStore  // in-memory entity storage
    auth     authConfig               // expected credentials
    requests []RecordedRequest        // request log for assertions
    options  serverOptions            // page size, latency, error injections
}

func NewServer(t *testing.T, opts ...ServerOption) (*TestServer, *autotask.Client)
```

`NewServer` returns both the server and a pre-configured `Client` pointed at it. The server:

- Routes requests by HTTP method + path pattern
- Validates auth headers on every request before dispatching
- Logs all requests for test assertions
- Cleans up via `t.Cleanup`

### Routing

Path patterns matched against the Autotask REST API URL structure:

| Pattern | Handler |
|---------|---------|
| `GET /v1.0/{entity}/{id}` | Get by ID |
| `POST /v1.0/{entity}/query` | Query with filter |
| `GET /v1.0/{entity}/query?search={json}` | Query via GET |
| `POST /v1.0/{entity}/query/count` | Count |
| `POST /v1.0/{entity}` | Create |
| `PATCH /v1.0/{entity}` | Update |
| `DELETE /v1.0/{entity}/{id}` | Delete |
| `GET /v1.0/{entity}/entityInformation` | Entity info |
| `GET /v1.0/{entity}/entityInformation/fields` | Field definitions |
| `GET /v1.0/{entity}/entityInformation/userDefinedFields` | UDF definitions |
| `GET /v1.0/ThresholdInformation` | API threshold info |
| `GET /v1.0/zoneInformation` | Zone discovery |

### Request Validation: `autotasktest/validate.go`

Every request is validated before the handler runs:

**Header validation:**
- `UserName` matches expected value
- `Secret` matches expected value
- `ApiIntegrationCode` matches expected value
- `Content-Type: application/json` present on POST/PATCH
- `ImpersonationResourceId` present only when configured

**Query filter validation** (on POST /query):
- JSON is well-formed
- Top-level `filter` array present
- Each condition has `op` field with valid operator (one of: eq, noteq, gt, gte, lt, lte, beginsWith, endsWith, contains, exist, notExist, in, notIn)
- Non-existence operators (`exist`, `notExist`) have no `value` field
- `in`/`notIn` operators have array `value`
- Nested `and`/`or` groups have `items` array
- UDF conditions have `"udf": true`
- `IncludeFields` is a valid comma-separated field list if present
- `MaxRecords` is 1-500 if present

**Create validation:**
- Body is well-formed JSON
- Required fields present per entity type (defined in entity store config)

### Entity Handlers: `autotasktest/handler.go`

Handlers operate on the in-memory entity store:

**Get by ID:**
- Parse numeric ID from path
- Look up in entity store
- Return entity JSON wrapped in `{"item": {...}}` or 404 with `{"errors": ["Entity not found"]}`

**Query:**
- Validate filter (see above)
- Apply filter to in-memory entities (basic operator matching for eq, noteq, contains, gt, lt, etc.)
- Paginate results based on `options.pageSize`
- Return `{"items": [...], "pageDetails": {"count": N, "requestCount": 500, "prevPageUrl": ..., "nextPageUrl": ...}}`

**Count:**
- Same filter validation
- Return `{"queryCount": N}`

**Create:**
- Validate required fields
- Assign auto-incremented ID
- Store entity
- Return `{"itemId": N}`

**Update (PATCH):**
- Validate `id` present in body
- Look up entity, merge fields
- Return `{"itemId": N}`

**Delete:**
- Check entity supports deletion (Contacts, Tasks, etc.)
- Remove from store
- Return `{"itemId": N}`

### Query Filter Engine: `autotasktest/handler_query.go`

In-memory filter evaluation for the mock server. Supports the full operator set against stored entity data:

- String operators: eq, noteq, beginsWith, endsWith, contains
- Numeric/date operators: gt, gte, lt, lte
- Null operators: exist, notExist
- Set operators: in, notIn
- Logical grouping: and, or (with nesting)

This does not need to be a perfect Autotask replica — it needs to be correct enough that tests exercising the library's query builder produce expected results from the mock.

### Configuration: `autotasktest/option.go`

Functional options for `NewServer`:

```go
func WithEntity[T autotask.Entity](items ...T) ServerOption
func WithAuth(username, secret, integrationCode string) ServerOption
func WithPageSize(n int) ServerOption
func WithErrorOn(method, pathSuffix string, status int, errors []string) ServerOption
func WithLatency(d time.Duration) ServerOption
func WithMetadata(entity string, info EntityMetadata) ServerOption
func WithZone(name, url string) ServerOption
```

Defaults:
- Auth: `test-user`, `test-secret`, `test-code`
- Page size: 500
- No latency
- No error injections

### Fixtures: `autotasktest/fixture.go`

Factory functions returning realistic entity data:

```go
func CompanyFixture(overrides ...func(*entities.Company)) entities.Company
func ContactFixture(overrides ...func(*entities.Contact)) entities.Contact
func TicketFixture(overrides ...func(*entities.Ticket)) entities.Ticket
// ... one per entity type
```

Each fixture includes:
- All required fields with valid values
- Common optional fields (phone, email, dates, status)
- At least one UDF per entity
- Proper date format: `2024-01-15T14:30:00.000Z`
- Valid picklist values where applicable
- Auto-incrementing IDs

Override functions allow tests to customize specific fields without rebuilding the whole entity.

### Request Log: `autotasktest/server.go`

```go
type RecordedRequest struct {
    Method  string
    Path    string
    Headers http.Header
    Body    []byte
}

func (s *TestServer) Requests() []RecordedRequest
func (s *TestServer) LastRequest() RecordedRequest
func (s *TestServer) RequestCount() int
```

Tests use these to assert the library sent correct requests.

## Test Organization

### New test files (at package root):

| File | Coverage |
|------|----------|
| `crud_entity_test.go` | CRUD operations across all 10 entity types |
| `query_validation_test.go` | Query filter construction and server-side validation |
| `pagination_test.go` | Multi-page iteration, boundaries, empty results |
| `error_handling_test.go` | HTTP error codes, validation errors, malformed responses |
| `auth_test.go` | Header injection, impersonation, credential safety |
| `child_entity_test.go` | Parent-child operations (Ticket->TicketNote, Project->Task) |
| `metadata_mock_test.go` | Field info, UDF info, entity info via mock server |
| `middleware_integration_test.go` | Rate limiter, circuit breaker, threshold with mock |
| `zone_discovery_test.go` | Zone lookup, caching, zone URL construction |
| `optional_fields_test.go` | Optional[T] serialization edge cases |

### Existing test files remain. New tests supplement, not replace.

## Entity CRUD Test Matrix

Each entity gets a table-driven test function exercising all supported operations.

| Entity | Get | List | Create | Update | Delete | Child | UDF |
|--------|-----|------|--------|--------|--------|-------|-----|
| Company | x | x | x | x | - | - | x |
| Contact | x | x | x | x | x | - | x |
| Ticket | x | x | x | x | - | TicketNote | x |
| TicketNote | x | x | x | x | - | - | x |
| Project | x | x | x | x | - | Task | x |
| Task | x | x | x | x | - | - | x |
| Resource | x | x | x | x | - | - | x |
| Contract | x | x | x | x | - | - | x |
| ConfigurationItem | x | x | x | x | - | - | x |
| TimeEntry | x | x | x | x | - | - | x |

### CRUD test structure (example for Company):

```go
func TestCompanyCRUD(t *testing.T) {
    company := autotasktest.CompanyFixture()
    srv, client := autotasktest.NewServer(t,
        autotasktest.WithEntity(company),
    )

    t.Run("Get", func(t *testing.T) {
        got, err := autotask.Get[entities.Company](ctx, client, company.ID)
        // assert no error, fields match fixture
    })

    t.Run("List", func(t *testing.T) {
        q := autotask.NewQuery().Where("companyName", autotask.OpEq, "Acme Corp")
        items, err := autotask.List[entities.Company](ctx, client, q)
        // assert filter sent correctly, results parsed
    })

    t.Run("Create", func(t *testing.T) {
        newCo := autotasktest.CompanyFixture(func(c *entities.Company) {
            c.ID = 0
            c.CompanyName = "New Corp"
        })
        id, err := autotask.Create(ctx, client, &newCo)
        // assert no error, ID returned
        // assert request body had required fields
    })

    t.Run("Update", func(t *testing.T) {
        company.Phone = autotask.Set("555-1234")
        id, err := autotask.Update(ctx, client, &company)
        // assert PATCH method used, id in body
    })

    t.Run("CreateWithUDF", func(t *testing.T) {
        co := autotasktest.CompanyFixture(func(c *entities.Company) {
            c.ID = 0
            c.UserDefinedFields = []autotask.UDF{
                {Name: "CustomerRanking", Value: "Gold"},
            }
        })
        id, err := autotask.Create(ctx, client, &co)
        // assert UDF serialized in request body
    })
}
```

## Query & Filter Validation Tests

```go
func TestQueryFilterOperators(t *testing.T) {
    // Table-driven: one subtest per operator
    tests := []struct{
        name     string
        op       autotask.Operator
        value    any
        wantJSON string // expected filter JSON fragment
    }{
        {"Eq", autotask.OpEq, "Acme", `{"op":"eq","field":"companyName","value":"Acme"}`},
        {"In", autotask.OpIn, []string{"A","B"}, `{"op":"in","field":"status","value":["A","B"]}`},
        {"Exist", autotask.OpExist, nil, `{"op":"exist","field":"phone"}`},
        // ... all 12 operators
    }
}

func TestQueryNestedGroups(t *testing.T) {
    // AND within OR, OR within AND, 3-level nesting
}

func TestQueryUDFFilter(t *testing.T) {
    // Verify "udf": true flag in filter JSON
}

func TestQueryIncludeFields(t *testing.T) {
    // Verify IncludeFields serialization
}

func TestQueryMaxRecords(t *testing.T) {
    // Verify MaxRecords value in query body
}
```

## Pagination Tests

```go
func TestPaginationMultiPage(t *testing.T) {
    // Seed 5 entities, page size 2 -> 3 pages
    // Verify ListIter fetches all 5
    // Verify nextPageUrl followed correctly
}

func TestPaginationSinglePage(t *testing.T) {
    // Seed 3 entities, page size 500 -> 1 page
    // Verify no nextPageUrl followed
}

func TestPaginationEmpty(t *testing.T) {
    // No matching entities -> empty items array
}

func TestPaginationContextCancel(t *testing.T) {
    // Cancel context mid-iteration -> iterator stops, no panic
}

func TestPaginationExactPageSize(t *testing.T) {
    // Seed exactly pageSize entities -> verify handles ambiguity
}
```

## Error Handling Tests

```go
func TestErrorAuthentication(t *testing.T) {
    // Bad credentials -> 401 -> AuthenticationError
}

func TestErrorNotFound(t *testing.T) {
    // GET nonexistent ID -> 404 -> NotFoundError
}

func TestErrorRateLimit(t *testing.T) {
    // 429 + Retry-After header -> RateLimitError with RetryAfter field
}

func TestErrorValidation(t *testing.T) {
    // Missing required field on create -> 400 -> ValidationError
}

func TestErrorServerError(t *testing.T) {
    // 500 -> ServerError
}

func TestErrorMultipleMessages(t *testing.T) {
    // {"errors": ["msg1", "msg2"]} -> error contains both messages
}

func TestErrorMalformedJSON(t *testing.T) {
    // Non-JSON response body -> appropriate error
}

func TestErrorEmptyBody(t *testing.T) {
    // Empty response body -> appropriate error
}

func TestErrorNetworkTimeout(t *testing.T) {
    // WithLatency exceeding context deadline -> context.DeadlineExceeded
}
```

## Auth & Security Tests

```go
func TestAuthHeadersPresent(t *testing.T) {
    // Every request includes UserName, Secret, ApiIntegrationCode
}

func TestAuthImpersonation(t *testing.T) {
    // ImpersonationResourceId header present when configured
}

func TestAuthImpersonationAbsent(t *testing.T) {
    // No impersonation header when not configured
}

func TestAuthSameOriginValidation(t *testing.T) {
    // Credentials not leaked on redirect to different origin
}
```

## Child Entity Tests

```go
func TestTicketNoteChild(t *testing.T) {
    // GetChild[Ticket, TicketNote] -> correct path /Tickets/{id}/TicketNotes
    // CreateChild[Ticket, TicketNote] -> correct path and body
}

func TestProjectTaskChild(t *testing.T) {
    // GetChild[Project, Task] -> correct path /Projects/{id}/Tasks
    // CreateChild[Project, Task] -> correct path and body
}
```

## Metadata Tests

```go
func TestGetEntityInfo(t *testing.T) {
    // Mock server returns entity info JSON
    // Verify parsing of canCreate, canDelete, canQuery, canUpdate flags
}

func TestGetFields(t *testing.T) {
    // Mock server returns field definitions
    // Verify dataType, isRequired, isPickList, picklistValues parsed
}

func TestGetUDFs(t *testing.T) {
    // Mock server returns UDF definitions
    // Verify name, dataType, isRequired parsed
}
```

## Middleware Integration Tests

```go
func TestRateLimiterWithMockServer(t *testing.T) {
    // Mock returns 429 + Retry-After -> rate limiter backs off
}

func TestCircuitBreakerWithMockServer(t *testing.T) {
    // Mock returns repeated 500s -> circuit opens
    // Mock recovers -> circuit half-opens -> closes
}

func TestThresholdMonitorWithMockServer(t *testing.T) {
    // Mock ThresholdInformation endpoint
    // Verify warning/critical callbacks fire at correct thresholds
}
```

## Zone Discovery Tests

```go
func TestZoneDiscovery(t *testing.T) {
    // Mock zoneInformation endpoint -> correct zone URL used
}

func TestZoneCaching(t *testing.T) {
    // Second request uses cached zone, no second lookup
}

func TestZoneDiscoveryFailure(t *testing.T) {
    // Zone endpoint returns error -> appropriate error propagation
}
```

## Optional Field Serialization Tests

```go
func TestOptionalSet(t *testing.T) {
    // Set("value") -> serialized as "value" in JSON
}

func TestOptionalNull(t *testing.T) {
    // Null[string]() -> serialized as null in JSON
}

func TestOptionalUnset(t *testing.T) {
    // Zero Optional -> omitted from JSON (omitzero)
}

func TestOptionalRoundTrip(t *testing.T) {
    // Create entity with optional fields -> Get same entity -> values match
}
```

## Implementation Notes

- All tests use `t.Parallel()` where safe (independent server instances)
- Test helpers use `t.Helper()` for clean failure messages
- No `time.Sleep` — use `WithLatency` + context deadlines for timing tests
- Mock server assigns auto-incrementing IDs starting from 1
- Pagination URLs are relative paths that the server can resolve
- The `RecordedRequest` log is goroutine-safe

## Test Count Estimate

- ~10 entity CRUD tests x 5 operations = ~50 subtests
- ~15 query/filter tests
- ~10 pagination tests
- ~10 error handling tests
- ~5 auth tests
- ~5 child entity tests
- ~5 metadata tests
- ~5 middleware integration tests
- ~5 zone discovery tests
- ~5 optional field tests

**Total: ~115 test cases**

## Files Changed

### New files:
- `autotasktest/server.go`
- `autotasktest/handler.go`
- `autotasktest/handler_query.go`
- `autotasktest/option.go`
- `autotasktest/validate.go`
- `autotasktest/fixture.go`
- `crud_entity_test.go`
- `query_validation_test.go`
- `pagination_test.go`
- `error_handling_test.go`
- `auth_test.go`
- `child_entity_test.go`
- `metadata_mock_test.go`
- `middleware_integration_test.go`
- `zone_discovery_test.go`
- `optional_fields_test.go`

### Existing files unchanged:
- `autotasktest/mock.go` — kept for backward compatibility
- All existing `*_test.go` files — not modified
