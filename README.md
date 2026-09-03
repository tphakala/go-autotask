# go-autotask

[![CI](https://github.com/tphakala/go-autotask/actions/workflows/ci.yml/badge.svg)](https://github.com/tphakala/go-autotask/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/tphakala/go-autotask.svg)](https://pkg.go.dev/github.com/tphakala/go-autotask)
[![codecov](https://codecov.io/gh/tphakala/go-autotask/branch/main/graph/badge.svg)](https://codecov.io/gh/tphakala/go-autotask)
[![Go Version](https://img.shields.io/github/go-mod/go-version/tphakala/go-autotask)](go.mod)
[![Latest release](https://img.shields.io/github/v/release/tphakala/go-autotask?sort=semver&label=release)](https://github.com/tphakala/go-autotask/releases/latest)
[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/tphakala/go-autotask/badge)](https://scorecard.dev/viewer/?uri=github.com/tphakala/go-autotask)
[![License: Apache-2.0](https://img.shields.io/badge/License-Apache--2.0-blue.svg)](LICENSE)
[![Sponsor](https://img.shields.io/github/sponsors/tphakala?logo=githubsponsors&color=ea4aaa&label=Sponsor)](https://github.com/sponsors/tphakala)

A Go client library for the [Autotask PSA](https://www.autotask.net/) REST API.

```go
client, err := autotask.NewClient(ctx, autotask.AuthConfig{
    Username:        os.Getenv("AUTOTASK_USERNAME"),
    Secret:          os.Getenv("AUTOTASK_SECRET"),
    IntegrationCode: os.Getenv("AUTOTASK_INTEGRATION_CODE"),
})
```

## Features

- **Type-safe CRUD**: generic `Get`, `List`, `Create`, `Update`, `Delete` functions for any entity
- **Query builder**: fluent API with `Where`, `Or`, `And`, field selection, and limits
- **Iterator pagination**: `ListIter` returns `iter.Seq2` for memory-efficient large result sets
- **Optional fields**: three-state `Optional[T]` type (unset / null / value) for correct API semantics
- **Middleware**: composable rate limiter, circuit breaker, concurrency limiter, and API threshold monitor
- **Raw operations**: `GetRaw`, `ListRaw`, and friends for entities not defined in the library
- **Child entities**: `ListChild` and `CreateChild` for parent-child relationships
- **Metadata introspection**: query field definitions, picklists, UDFs, and entity capabilities at runtime
- **Code generation**: `autotask-gen` generates entity structs from live API metadata
- **Test support**: `autotasktest.NewMockClient` for in-memory testing with fixtures
- **Automatic zone discovery**: resolves the correct API endpoint for your account

## Install

```
go get github.com/tphakala/go-autotask
```

Requires Go 1.26 or later.

## Quick start

```go
package main

import (
    "context"
    "fmt"
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
        panic(err)
    }
    defer func() { _ = client.Close() }()

    // Get a ticket by ID.
    ticket, err := autotask.Get[entities.Ticket](ctx, client, 12345)
    if err != nil {
        panic(err)
    }
    if title, ok := ticket.Title.Get(); ok {
        fmt.Println(title)
    }
}
```

## CRUD operations

All CRUD functions are generic over any type implementing the `Entity` interface:

```go
// Get by ID
ticket, err := autotask.Get[entities.Ticket](ctx, client, 42)

// List with query
tickets, err := autotask.List[entities.Ticket](ctx, client,
    autotask.NewQuery().Where("status", autotask.OpEq, 1),
)

// Count
n, err := autotask.Count[entities.Ticket](ctx, client, autotask.NewQuery())

// Create
created, err := autotask.Create(ctx, client, &entities.Ticket{
    Title:     autotask.Set("Server down"),
    CompanyID: autotask.Set(int64(123)),
    Status:    autotask.Set(int64(1)),
    Priority:  autotask.Set(int64(2)),
})

// Update
updated, err := autotask.Update(ctx, client, ticket)

// Delete
err = autotask.Delete[entities.Ticket](ctx, client, 42)
```

## Query builder

```go
q := autotask.NewQuery().
    Where("status", autotask.OpEq, 1).
    Or(
        autotask.Field("priority", autotask.OpEq, 1),
        autotask.Field("priority", autotask.OpEq, 2),
    ).
    Fields("id", "title", "status", "priority").
    Limit(50)

tickets, err := autotask.List[entities.Ticket](ctx, client, q)
```

Available operators: `OpEq`, `OpNotEq`, `OpGt`, `OpGte`, `OpLt`, `OpLte`, `OpBeginsWith`, `OpEndsWith`, `OpContains`, `OpExist`, `OpNotExist`, `OpIn`, `OpNotIn`.

Use `WhereUDF` to filter on a user-defined field instead of a standard one:

```go
q := autotask.NewQuery().WhereUDF("MyCustomField", autotask.OpEq, "value")
```

## Iterator pagination

For large result sets, the `*Iter` functions return Go iterators (`iter.Seq2`) that fetch pages on demand and stop fetching as soon as you break out of the range, so you never buffer more than one page at a time. Every list-all function has a matching iterator:

| List-all | Iterator | Entities |
| --- | --- | --- |
| `List` | `ListIter` | typed, top-level |
| `ListChild` | `ListChildIter` | typed, child of a parent |
| `ListRaw` | `ListRawIter` | untyped (`map[string]any`), top-level |
| `ListChildRaw` | `ListChildRawIter` | untyped (`map[string]any`), child of a parent |

```go
for ticket, err := range autotask.ListIter[entities.Ticket](ctx, client, autotask.NewQuery()) {
    if err != nil {
        return err
    }
    title, _ := ticket.Title.Get()
    fmt.Println(title)
}
```

The untyped iterators take entity names as strings and yield `map[string]any`, which is handy for entities without a generated Go type:

```go
for item, err := range autotask.ListRawIter(ctx, client, "Tickets", autotask.NewQuery()) {
    if err != nil {
        return err
    }
    fmt.Println(item["id"])
}
```

All four paginating loops are bounded by a `maxPages` safety limit; if it is ever hit (for example an API response that cycles `nextPageUrl`), the iterator yields a `*MaxPagesExceededError` rather than looping forever.

The top-level iterators (`ListIter`, `ListRawIter`) do not apply the query's `MaxRecords` cap client-side, unlike `List` and `ListRaw`; break out of the range once you have enough items.

## Optional fields

Autotask fields can be unset, explicitly null, or have a value. `Optional[T]` handles all three states:

```go
ticket := &entities.Ticket{
    Title:    autotask.Set("My ticket"),    // set to a value
    Priority: autotask.Null[int64](),       // explicitly null
    // Status is omitted (unset, not sent in the request)
}

if title, ok := ticket.Title.Get(); ok {
    fmt.Println(title)
}
```

## Middleware

### Rate limiter

```go
client, err := autotask.NewClient(ctx, auth,
    autotask.WithRateLimiter(
        middleware.WithRequestsPerHour(8000),
        middleware.WithBurstSize(10),
        middleware.WithAdaptiveDelay(true),
    ),
)
```

Token-bucket rate limiting with adaptive delays. Automatically respects `Retry-After` headers on 429 responses.

### Circuit breaker

```go
client, err := autotask.NewClient(ctx, auth,
    autotask.WithCircuitBreaker(
        middleware.WithFailureThreshold(5),
        middleware.WithOpenTimeout(30 * time.Second),
    ),
)
```

Three-state circuit breaker (closed, open, half-open) that stops sending requests after repeated failures. `WithFailureWindow` and `WithSuccessThreshold` tune the trip and recovery behavior.

### Concurrency limiter

```go
client, err := autotask.NewClient(ctx, auth,
    autotask.WithRateLimiter(),      // enforce the hourly rate first
    autotask.WithMaxConcurrency(3),  // then cap in-flight requests
)
```

Caps the number of requests in flight at once. Compose it after the rate limiter so the rate limit is enforced before the concurrency gate.

### Threshold monitor

```go
client, err := autotask.NewClient(ctx, auth,
    autotask.WithThresholdMonitor(
        middleware.WithCheckInterval(5 * time.Minute),
        middleware.WithWarningCallback(func(info middleware.ThresholdInfo) {
            log.Printf("API usage at %.0f%%", info.UsagePercent)
        }),
        middleware.WithCriticalCallback(func(info middleware.ThresholdInfo) {
            log.Printf("CRITICAL: API usage at %.0f%%", info.UsagePercent)
        }),
    ),
)
```

Polls the Autotask ThresholdInformation endpoint in the background and invokes callbacks when usage crosses 75% (warning) or 90% (critical).

## Raw operations

For entities not defined in the library, use the untyped API:

```go
result, err := autotask.GetRaw(ctx, client, "Companies", 123)
fmt.Println(result["companyName"])

results, err := autotask.ListRaw(ctx, client, "Companies",
    autotask.NewQuery().Where("isActive", autotask.OpEq, true),
)
```

## Child entities

```go
// List all notes for a ticket (paginated)
notes, err := autotask.ListChild[entities.Ticket, entities.TicketNote](ctx, client, ticketID)

// Create a note on a ticket
note, err := autotask.CreateChild[entities.Ticket](ctx, client, ticketID, &entities.TicketNote{
    Title:       autotask.Set("Update"),
    Description: autotask.Set("Fixed the issue."),
})
```

## Metadata

Query entity structure at runtime:

```go
import "github.com/tphakala/go-autotask/metadata"

fields, err := metadata.GetFields(ctx, client, "Tickets")
for _, f := range fields {
    fmt.Printf("%s (%s) required=%v\n", f.Name, f.Type, f.IsRequired)
}

// Picklist values for a specific field
values, err := metadata.GetPickList(ctx, client, "Tickets", "status")
for _, v := range values {
    fmt.Printf("%s = %s\n", v.Value, v.Label)
}

udfs, err := metadata.GetUDFs(ctx, client, "Tickets")

info, err := metadata.GetEntityInfo(ctx, client, "Tickets")
fmt.Printf("canCreate=%v canQuery=%v\n", info.CanCreate, info.CanQuery)
```

## Code generation

Generate entity structs from live API metadata:

```
go run ./cmd/autotask-gen \
    -username user@example.com \
    -secret s3cret \
    -integration-code INT123 \
    -output ./entities
```

## Testing

Use `autotasktest.NewMockClient` to create an in-memory client for tests:

```go
import "github.com/tphakala/go-autotask/autotasktest"

func TestMyCode(t *testing.T) {
    client := autotasktest.NewMockClient(t,
        autotasktest.WithFixture("GET", "/v1.0/Tickets/42", 200, map[string]any{
            "item": map[string]any{"id": 42, "title": "Test"},
        }),
    )
    // use client in tests, server and client are cleaned up automatically
}
```

## Client options

| Option | Description |
|--------|-------------|
| `WithBaseURL(url)` | Override automatic zone discovery with a fixed API base URL |
| `WithZoneBaseURL(url)` | Override the base URL used for zone discovery (mainly for testing) |
| `WithHTTPClient(hc)` | Use a custom `*http.Client` |
| `WithLogger(l)` | Structured logging via `*slog.Logger` |
| `WithUserAgent(ua)` | Custom User-Agent header |
| `WithImpersonation(id)` | Perform API calls as another resource |
| `WithMiddleware(m)` | Add custom `http.RoundTripper` middleware |
| `WithRateLimiter(opts...)` | Enable rate limiting |
| `WithCircuitBreaker(opts...)` | Enable circuit breaker |
| `WithMaxConcurrency(n)` | Cap the number of concurrent in-flight requests |
| `WithThresholdMonitor(opts...)` | Enable API usage monitoring |

## Available entities

The `entities` package ships generated, type-safe structs for:

`BillingCode`, `BillingItem`, `BillingItemApprovalLevel`, `Company`, `CompanyNote`, `ConfigurationItem`, `Contact`, `Contract`, `Department`, `ExpenseItem`, `ExpenseReport`, `Invoice`, `Opportunity`, `Product`, `Project`, `ProjectNote`, `Quote`, `QuoteItem`, `Resource`, `Service`, `ServiceBundle`, `Task`, `Ticket`, `TicketAttachment`, `TicketNote`, `TimeEntry`

All entities use `Optional[T]` fields and support user-defined fields via `UserDefinedFields []autotask.UDF`. For any entity without a generated struct, use the raw (`map[string]any`) API or generate one with `autotask-gen`.

## Error handling

API errors are returned as typed errors for easy matching:

```go
ticket, err := autotask.Get[entities.Ticket](ctx, client, 999)
if nf, ok := errors.AsType[*autotask.NotFoundError](err); ok {
    fmt.Println("not found:", nf.Err.Message)
}
```

| Type | HTTP Status |
|------|-------------|
| `ValidationError` | 400 |
| `AuthenticationError` | 401 |
| `AuthorizationError` | 403 |
| `NotFoundError` | 404 |
| `ConflictError` | 409 |
| `BusinessLogicError` | 422 |
| `RateLimitError` | 429 |
| `ServerError` | 5xx |

`RateLimitError` includes a `RetryAfter` duration parsed from the response header.

## License

Apache-2.0
