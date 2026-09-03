# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- **`ListRawIter`** — lazy, early-exit iterator over untyped top-level entities (the untyped counterpart of `ListIter`). Callers that need only the first few items can stop without buffering every page. `ListRaw` now consumes it, so its accumulate-all and `MaxRecords` behavior is unchanged.
- **`ListChildRawIter`** — lazy, early-exit iterator over untyped child entities (the untyped counterpart of `ListChildIter`). Callers that need only the first few items can stop without buffering every page. `ListChildRaw` now consumes it, so its accumulate-all behavior is unchanged.

### Changed

- **Pagination consolidation (internal)** — `List` and `ListChild` now delegate to `ListIter` and `ListChildIter` instead of duplicating the page-follow loop, so all four list-all functions share one shape. `ListRawIter` and `ListChildRawIter` now decode pages through a shared named `rawPageResponse` type. `List` still applies the `MaxRecords` cap on accumulation and, for successful responses, returns the same results and issues the same page-request sequence as before. Two edge cases on the `List` error path change: a malformed item positioned beyond the requested `MaxRecords` no longer fails the call (it is never decoded, matching `ListRaw`), and a decode failure is now reported as `autotask: decoding <Entity>: ...` instead of `autotask: decoding <Entity> item: ...` (matching `ListIter` and the other list functions).

### Fixed

- **Zone discovery API-version selection** — discovery now selects the advertised API version this client implements (case-insensitive, tolerant of a leading `V` and surrounding whitespace) instead of the last advertised entry. A version that Autotask advertises but does not serve no longer breaks `NewClient`. Request-URL joins are normalized to avoid a `//v1.0/` double slash.
- **`autotasktest` version endpoint** — the mock now returns the `apiVersions` key that zone discovery decodes (was `versions`).

## [1.4.0] - 2026-03-25

### Added

- **`WithErrorCallback`** — new `ThresholdMonitor` option to receive errors from background monitoring checks instead of silent failures

### Fixed

- **`Get` nil/null item guard** — returns a clear error when the API returns `null` or missing `item` instead of a confusing unmarshal error
- **Zone discovery response body leak** — first HTTP response body was never closed because the `resp` variable was reassigned before its defer ran; now uses separate variables
- **Nil entity guards in `Create`/`Update`** — return error instead of panicking on nil entity (consistent with `CreateChild`)
- **Timer leak in rate limiter** — replaced `time.After` with `time.NewTimer` + explicit `Stop()` to prevent timer/memory leaks on context cancellation
- **Retry-After parsing consistency** — middleware now uses `strconv.Atoi` matching the `error.go` implementation
- **Version constant** — updated User-Agent from `go-autotask/0.1.0` to `go-autotask/1.3.0`

### Changed

- **Query** — documented that the builder is mutable and should not be shared across goroutines

## [1.3.0] - 2026-03-25

### Added

- **`WithMaxConcurrency(n)`** — semaphore-based middleware limiting concurrent in-flight API requests. Autotask enforces a per-integration-code thread limit (default 3); this prevents the client from exceeding it. Blocks with context cancellation support, releases slot on response completion.

## [1.2.0] - 2026-03-25

### Added

- **16 new entity types:** ProjectNotes, CompanyNotes, TicketAttachments, Quotes, QuoteItems, Opportunities, Invoices, BillingItems, BillingItemApprovalLevels, BillingCodes, ExpenseReports, ExpenseItems, Products, Services, ServiceBundles, Departments
- **Generator: acronym normalization** — `goName()` recognizes 16 Go acronyms (ID, URL, API, SKU, etc.), producing idiomatic field names
- **Generator: conditional `time` import** — template only imports `"time"` when datetime fields are present, preventing build failures on entities without time fields
- **Generator: configurable entity list** — `-entities` flag accepts comma-separated names; defaults expanded from 9 to 25
- **Generator: idiomatic filenames** — `toSnakeCase()` produces `ticket_notes.go` instead of `ticketnotes.go`, with acronym handling (`HTTPServer` → `http_server`)
- **Generator: smarter pluralization** — `singular()` handles irregular plurals like `Statuses` → `Status`
- **`ListChild` and `ListChildIter`** — automatic pagination for child entities, replacing first-page-only `GetChild`
- **`ListChildRaw` and `CreateChildRaw`** — untyped child entity operations
- **`metadata.GetPickList`** — convenience function to fetch picklist values for a single field
- **`EntityWithID` interface** — `Create` and `CreateChild` parse `{"itemId": N}` from API responses and populate the entity's ID via optional `SetID()` method
- **Pagination safety guards** — all pagination functions (`List`, `ListRaw`, `ListChild`, `ListChildIter`, `ListIter`) enforce a `maxPages` (1000) limit with `MaxPagesExceededError`

### Fixed

- **Zone discovery:** API returns `apiVersions` not `versions` — fixed field tag in `zone.go`
- **Picklist values:** `PickListValue.Value` changed from `int` to `string` to match actual API responses; added `SortOrder`, `ParentValue`, `IsSystem` fields

### Deprecated

- `GetChild` — use `ListChild` which provides automatic pagination

## [1.1.0] - 2026-03-24

### Added

- Generator: configurable `-entities` flag, expanded default entity list (25 entities)
- Generator: idiomatic `toSnakeCase()` filenames, improved `singular()` pluralization
- `ListChild` and `ListChildIter` for child entity pagination
- `metadata.GetPickList` convenience function
- Pagination safety guards (`MaxPagesExceededError`) on all pagination functions
- `EntityWithID` interface for parsing `itemId` from `Create`/`CreateChild` responses
- `ListChildRaw` and `CreateChildRaw` for untyped child operations
- `toSnakeCase` acronym handling (`HTTPServer` → `http_server`)

### Deprecated

- `GetChild` — use `ListChild`

## [1.0.0] - 2026-03-24

### Added

- Type-safe generic CRUD operations (`Get`, `List`, `Create`, `Update`, `Delete`)
- Query builder with fluent API (`Where`, `Or`, `And`, field selection, limits)
- Iterator pagination via `ListIter` returning `iter.Seq2`
- Three-state `Optional[T]` type (unset / null / value) for correct API semantics
- Middleware: rate limiter, circuit breaker, API threshold monitor
- Raw operations (`GetRaw`, `ListRaw`, etc.) for undefined entities
- Child entity support (`GetChild`, `CreateChild`)
- Metadata introspection (field definitions, UDFs, entity capabilities)
- Code generation tool (`autotask-gen`) for entity structs from live API metadata
- Test support via `autotasktest.NewMockClient` with fixtures
- Automatic zone discovery for API endpoint resolution
- Structured error types mapped to HTTP status codes
- Entity types: Company, Contact, Ticket, Resource, Contract, Project, Task, ConfigurationItem, TicketNote, TimeEntry
- GitHub Actions: CI (test + lint), CodeQL, govulncheck, Dependabot, automated releases, stale issue cleanup

[1.4.0]: https://github.com/tphakala/go-autotask/compare/v1.3.0...v1.4.0
[1.3.0]: https://github.com/tphakala/go-autotask/compare/v1.2.0...v1.3.0
[1.2.0]: https://github.com/tphakala/go-autotask/compare/v1.1.0...v1.2.0
[1.1.0]: https://github.com/tphakala/go-autotask/compare/v1.0.0...v1.1.0
[1.0.0]: https://github.com/tphakala/go-autotask/releases/tag/v1.0.0
