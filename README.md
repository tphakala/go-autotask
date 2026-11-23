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
- **Child entities**: `ListChild`, `CreateChild`, `UpdateChild` and `DeleteChild` for parent-child relationships
- **Metadata introspection**: query field definitions, picklists, UDFs, and entity capabilities at runtime
- **Code generation**: `autotask-gen` generates entity structs from live API metadata
- **Test support**: `autotasktest.NewMockClient` for in-memory testing with fixtures
- **Automatic zone discovery**: resolves the correct API endpoint for your account, and can reuse a saved zone

## Install

```
go get github.com/tphakala/go-autotask
```

Requires Go 1.27 or later.

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

// Create: the returned entity carries the server-assigned id in ID.
created, err := autotask.Create(ctx, client, &entities.Ticket{
    Title:     autotask.Set("Server down"),
    CompanyID: autotask.Set(int64(123)),
    Status:    autotask.Set(int64(1)),
    Priority:  autotask.Set(int64(2)),
})
if err != nil {
    return err
}
newID, ok := created.ID.Get() // ok is false if the id was not set

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

By default a round trip counts as a failure when it is a transport error (a caller's context cancellation excepted), a 429 or a 5xx, or a 2xx response with a non-JSON `Content-Type` such as the HTML maintenance page. A 500 whose error message matches the validation wording that `(*ServerError).IsValidation` uses does not count, so repeated validation failures cannot open the breaker for all traffic. A validation 500 worded differently still counts. `WithFailurePredicate(fn)` replaces that rule. `middleware.DefaultFailure` is the default and `middleware.StatusFailure` is the rule the breaker used before: transport errors, 429 and 5xx. The predicate can read the first 64 KiB of the response body and the caller still gets the whole body.

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

// UpdateRaw returns the response body, for example {"itemId": 123}
resp, err := autotask.UpdateRaw(ctx, client, "Companies",
    map[string]any{"id": 123, "companyName": "Renamed"},
)

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

// Update a note through its ticket: PATCH /v1.0/Tickets/{ticketID}/Notes
// The note's ID must be set; every set field is sent.
note.Title = autotask.Set("Update (edited)")
note, err = autotask.UpdateChild[entities.Ticket](ctx, client, ticketID, note)

// Delete an attachment through its note: DELETE /v1.0/TicketNotes/{noteID}/Attachments/{id}
err = autotask.DeleteChild[entities.TicketNote, entities.TicketNoteAttachment](ctx, client, noteID, attachmentID)
```

Some entities are updated or deleted only through the parent's URL; `Update` sends `/v1.0/{Entity}` and `Delete` sends `/v1.0/{Entity}/{id}`, URLs the API does not offer for them. `UpdateChildRaw` and `DeleteChildRaw` do the same with entity names instead of types.

## Problem, incident and change request tickets

An incident ticket points at its problem ticket through `Ticket.ProblemTicketID`. The JSON field is `problemTicketId`, with a lowercase d, and Autotask accepts it only on Incident tickets. List the incidents of a problem by filtering on it; the filter needs the field to be queryable in your instance, which `metadata.GetFields` reports as `IsQueryable`. Ticket type values come from `metadata.GetPickList(ctx, client, "Tickets", "ticketType")`.

```go
incidents, err := autotask.List[entities.Ticket](ctx, client,
    autotask.NewQuery().Where("problemTicketId", autotask.OpEq, problemID),
)
```

`ChangeRequestLink` links a Change Request ticket to a Problem or Incident ticket. Autotask has no update for this entity, so delete a link and create a new one instead of calling `Update`. `Create` sets the new link's `ID` on the returned entity.

```go
_, err := autotask.Create(ctx, client, &entities.ChangeRequestLink{
    ChangeRequestTicketID:     autotask.Set(changeRequestID),
    ProblemOrIncidentTicketID: autotask.Set(incidentID),
})

links, err := autotask.List[entities.ChangeRequestLink](ctx, client,
    autotask.NewQuery().
        Where("changeRequestTicketID", autotask.OpEq, changeRequestID).
        Where("problemOrIncidentTicketID", autotask.OpEq, incidentID),
)
if len(links) > 0 {
    id, _ := links[0].ID.Get()
    err = autotask.Delete[entities.ChangeRequestLink](ctx, client, id)
}
```

The `autotasktest` server accepts PATCH for every entity. To make a test fail the way the API would for an update, add `autotasktest.WithErrorOn(http.MethodPatch, "/ChangeRequestLinks", ...)`.

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

Run from a checkout of this repository, this regenerates the generated files in `./entities`; files generated earlier are overwritten, except one whose type is declared in another file: it is skipped and stays in place, so delete it when it is a leftover. An entity whose Go type is already declared in another Go source file of the output directory (test files and names starting with `.` or `_` are not read), such as the hand-written `Ticket` and `Company`, is skipped with a warning. A file that does not start with the `// Code generated by autotask-gen. DO NOT EDIT.` header is never overwritten. The command exits with an error when no entity was generated. Use `-entities Name1,Name2` to pick entities by their Autotask (plural) name and `-output` to pick the directory; generated files declare `package entities`.

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

`autotasktest.NewServer` is a validating in-memory server that keeps what tests create. It stores child items per parent and per entity type: a child URL such as `/v1.0/Tickets/{id}/Attachments` reaches the store of the `TicketAttachments` entity (the singular parent name plus the segment), so the same items answer `Get`, `List` and `Count` for that type. A type whose entity name is not the singular parent name plus the segment (`Task` under `Projects`) needs `WithChildEntity` for that, or a `WithEntity` seed when its entity name is the segment. A child list shows the items created under that parent, child `GET` by id, `PATCH` and `DELETE` check the parent, and a list pages by `?page=N`. `WithChildEntity[Parent](parentID, items...)` seeds children under a parent, and is also how to route a child type whose entity name is neither of those names. Items seeded with `WithEntity` have no parent and appear under every parent whose child URL resolves to their store. `DELETE` needs `WithDeleteSupport` with the child's entity name, for example `WithDeleteSupport("TicketNoteAttachments")`.

## Zone discovery

`NewClient` finds the API endpoint for the account with two requests (the API version list, then the zone information for the username) before it returns. `Client.Zone` returns what it found:

```go
zone, ok := client.Zone() // ok is false for a client built with a non-empty WithBaseURL
fmt.Println(zone.ZoneName, zone.URL, zone.WebURL)
```

A process that starts often, such as one run per event, can save the zone and skip both requests on later runs. `ZoneInfo` marshals to JSON as it is. This loads a saved zone when there is one, and saves the discovered zone when there is not:

```go
var opts []autotask.ClientOption
loaded := false
data, err := os.ReadFile(zonePath)
switch {
case err == nil:
    var zone autotask.ZoneInfo
    if err := json.Unmarshal(data, &zone); err != nil {
        return err
    }
    opts = append(opts, autotask.WithZoneInfo(zone))
    loaded = true
case !errors.Is(err, fs.ErrNotExist):
    return err
}

client, err := autotask.NewClient(ctx, auth, opts...)
if err != nil {
    return err
}

// First run: no saved zone, so NewClient discovered it. Save it.
if !loaded {
    if zone, ok := client.Zone(); ok {
        data, err := json.Marshal(zone)
        if err != nil {
            return err
        }
        if err := os.WriteFile(zonePath, data, 0o600); err != nil {
            return err
        }
    }
}
```

With `WithZoneInfo`, `NewClient` sends no zone discovery request, uses `zone.URL` as the base URL, and `client.Zone()` returns the same value, `WebURL` included. `NewClient` returns an error when `zone.URL` is not an absolute `http` or `https` URL with a host and no `@`, query or fragment, or when a non-empty `WithBaseURL` is also set. The client sends the API credentials to `zone.URL`, so protect the saved file like the credentials and keep one per API username. A saved zone does not expire: if calls start to fail in a way that points at the endpoint, delete it and let `NewClient` discover the zone again.

## Client options

| Option | Description |
|--------|-------------|
| `WithBaseURL(url)` | Override automatic zone discovery with a fixed API base URL; with a non-empty URL, `Client.Zone` reports no zone |
| `WithZoneInfo(z)` | Use a zone saved from `Client.Zone` on an earlier run; no zone discovery request is made (see [Zone discovery](#zone-discovery)) |
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
| `WithMaxResponseBytes(n)` | Largest API or zone discovery response body the client reads (default 128 MiB) |

## Available entities

The `entities` package ships type-safe structs, some generated with `autotask-gen` and some hand-written, for:

`BillingCode`, `BillingItem`, `BillingItemApprovalLevel`, `ChangeRequestLink`, `Company`, `CompanyNote`, `ConfigurationItem`, `Contact`, `Contract`, `Department`, `ExpenseItem`, `ExpenseReport`, `Invoice`, `Opportunity`, `Product`, `Project`, `ProjectNote`, `Quote`, `QuoteItem`, `Resource`, `Service`, `ServiceBundle`, `Task`, `Ticket`, `TicketAttachment`, `TicketNote`, `TicketNoteAttachment`, `TimeEntry`

All entities use `Optional[T]` fields, and all except `ChangeRequestLink` and `TicketNoteAttachment` (Autotask gives them none) support user-defined fields via `UserDefinedFields []autotask.UDF`. For an entity not listed here, use the raw (`map[string]any`) API or generate a struct with `autotask-gen`.

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

`Get` and `GetRaw` also return `NotFoundError` when a GET succeeds but its `item` is missing or null, which is how Autotask can answer for an id that does not exist. That error has `StatusCode` 200.

A 2xx response whose body is not blank (empty or only whitespace), not labelled as JSON and not valid JSON returns `UnexpectedContentTypeError`. Its `Err.StatusCode` holds the status, `ContentType` the header and `Snippet` up to the first 256 bytes of the body. It unwraps to `*Error` like the other typed errors, so check for it before a generic `*Error` branch. Autotask answers with HTTP 200 and an HTML page during planned maintenance, so `IsTransient` reports this error as transient. A create call, such as `Create` or `CreateChild`, that got it may already have taken effect, so sending it again can duplicate the record; see [Deciding whether to retry](#deciding-whether-to-retry). `NewClient` returns it (wrapped) when zone discovery hits the same page.

A 2xx response with an empty or whitespace-only body returns `EmptyResponseError` from any call that decodes a result, such as `Get`, `Count`, `Create`, `UpdateRaw` or a list page. It is not a `NotFoundError`: a blank body says nothing about whether the record exists. `Delete` and `DeleteRaw` still accept a blank body.

A 2xx list page whose JSON body has no `items` member, such as `{}`, returns `MissingItemsError` from `List`, `ListRaw`, `ListChild`, `ListChildRaw`, their iterators and `GetChild`. It is not an empty result, so do not read it as "no match" before a create. A page with `"items": []` or `"items": null` is an empty page. `MissingItemsError` unwraps to `*Error` with `StatusCode` 200, and `IsTransient` reports it as transient. An iterator can yield it after the items of earlier pages, and a new iteration starts again at the first page, so work done per item must be safe to repeat before a loop is retried.

The client reads at most 128 MiB of an API or zone discovery response body; change the limit with `WithMaxResponseBytes`. A longer 2xx body returns `ResponseTooLargeError`. For an API response with any other status the status-typed error is returned (a 503 is still a `ServerError`), with the `ResponseTooLargeError` reachable through `errors.As`. `EmptyResponseError` and `ResponseTooLargeError` unwrap to `*Error` and carry the status in `Err.StatusCode`, and `NewClient` returns them (wrapped) when a zone discovery response is blank or over the limit.

### Deciding whether to retry

```go
if err != nil && autotask.IsTransient(err) {
    // Back off and retry. Do not resend a create call (Create, CreateRaw, CreateChild, CreateChildRaw) on
    // this alone: it may already have taken effect, see below.
}
```

`IsTransient` is true for rate limiting, server errors, a blank or non-JSON 2xx response, a list page without `items`, an open circuit breaker, timeouts, refused or dropped connections and truncated bodies. It is false for 400, 401, 403, 404, 409 and 422, a cancelled context (unless it cut short the body of a 429 or 5xx response, which is classified by its status), `ResponseTooLargeError`, `MaxPagesExceededError`, an unrecognised error and nil. A zone discovery failure is classified by its status. Autotask answers HTTP 500 for some validation failures, and a retry returns the same error. `(*ServerError).IsValidation()` reports such a 500, and `IsTransient` treats it as permanent. It matches the error message against a short list of known wording, so a validation 500 with other wording still looks like a server failure; a transient result says the failure may pass, not that the request is safe to send again, because a create call, such as `Create` or `CreateChild`, that failed after it was sent may already have taken effect.

## License

Apache-2.0
