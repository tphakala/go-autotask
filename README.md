# go-autotask

A Go client library for the [Autotask PSA](https://www.autotask.net/) REST API.

```go
client, err := autotask.NewClient(ctx, autotask.AuthConfig{
    Username:        os.Getenv("AUTOTASK_USERNAME"),
    Secret:          os.Getenv("AUTOTASK_SECRET"),
    IntegrationCode: os.Getenv("AUTOTASK_INTEGRATION_CODE"),
})
```

## Features

- **Type-safe CRUD** — generic `Get`, `List`, `Create`, `Update`, `Delete` functions for any entity
- **Query builder** — fluent API with `Where`, `Or`, `And`, field selection, and limits
- **Iterator pagination** — `ListIter` returns `iter.Seq2` for memory-efficient large result sets
- **Optional fields** — three-state `Optional[T]` type (unset / null / value) for correct API semantics
- **Middleware** — composable rate limiter, circuit breaker, and API threshold monitor
- **Raw operations** — `GetRaw`, `ListRaw`, etc. for entities not defined in the library
- **Child entities** — `GetChild` and `CreateChild` for parent-child relationships
- **Metadata introspection** — query field definitions, UDFs, and entity capabilities at runtime
- **Code generation** — `autotask-gen` generates entity structs from live API metadata
- **Test support** — `autotasktest.NewMockClient` for in-memory testing with fixtures
- **Automatic zone discovery** — resolves the correct API endpoint for your account

## Install

```
go get github.com/tphakala/go-autotask
```

Requires Go 1.23 or later.

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
    Status:    autotask.Set(1),
    Priority:  autotask.Set(2),
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

## Iterator pagination

For large result sets, `ListIter` returns a Go iterator that fetches pages on demand:

```go
for ticket, err := range autotask.ListIter[entities.Ticket](ctx, client, autotask.NewQuery()) {
    if err != nil {
        return err
    }
    title, _ := ticket.Title.Get()
    fmt.Println(title)
}
```

## Optional fields

Autotask fields can be unset, explicitly null, or have a value. `Optional[T]` handles all three states:

```go
ticket := &entities.Ticket{
    Title:    autotask.Set("My ticket"),    // set to a value
    Priority: autotask.Null[int](),         // explicitly null
    // Status is omitted — unset, not sent in the request
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

Three-state circuit breaker (closed → open → half-open) that stops sending requests after repeated failures.

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
// Get all notes for a ticket
notes, err := autotask.GetChild[entities.Ticket, entities.TicketNote](ctx, client, ticketID)

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
    // use client in tests — server and client are cleaned up automatically
}
```

## Client options

| Option | Description |
|--------|-------------|
| `WithBaseURL(url)` | Override automatic zone discovery |
| `WithHTTPClient(hc)` | Use a custom `*http.Client` |
| `WithLogger(l)` | Structured logging via `*slog.Logger` |
| `WithUserAgent(ua)` | Custom User-Agent header |
| `WithImpersonation(id)` | Perform API calls as another resource |
| `WithMiddleware(m)` | Add custom `http.RoundTripper` middleware |
| `WithRateLimiter(opts...)` | Enable rate limiting |
| `WithCircuitBreaker(opts...)` | Enable circuit breaker |
| `WithThresholdMonitor(opts...)` | Enable API usage monitoring |

## Available entities

`Company`, `Contact`, `Ticket`, `Resource`, `Contract`, `Project`, `Task`, `ConfigurationItem`, `TicketNote`, `TimeEntry`

All entities use `Optional[T]` fields and support user-defined fields via `UserDefinedFields []autotask.UDF`.

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

MIT
