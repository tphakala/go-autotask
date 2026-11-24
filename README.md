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

Integrating directly with Autotask PSA from Go involves recurring protocol and API quirks: dynamic per-account zone discovery, partial updates that distinguish omitted fields from explicit `null` values, parent-child routing for nested resources, paginated queries over large datasets, hourly rate and concurrency limits, and failure modes such as HTML maintenance pages returned with HTTP 200 or validation errors returned as HTTP 500. `go-autotask` handles these details behind a type-safe, idiomatic Go API.

## Features

- **Type-safe CRUD and child operations**: generic functions for top-level and parent-child entities, plus untyped raw operations for entities without generated structs
- **Three-state `Optional[T]` fields**: represent unset, explicit `null`, and set values cleanly for partial updates
- **Query builder and lazy pagination**: fluent query construction with UDF support and `iter.Seq2` iterators that stream one page at a time
- **Automatic zone discovery**: resolves the account endpoint on startup and supports caching `ZoneInfo` across runs
- **Composable resilience middleware**: rate limiter, circuit breaker, concurrency limiter, API threshold monitor, and structured `slog` logging
- **Structured error classification**: typed HTTP and protocol errors with `IsTransient` retry guidance
- **Metadata introspection and code generation**: query field definitions, picklists, and UDFs at runtime, or generate structs with `autotask-gen`
- **In-memory test server**: `autotasktest` mock client and validating server for unit tests without live API credentials

## Install

```sh
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

	ticket, err := autotask.Get[entities.Ticket](ctx, client, 12345)
	if err != nil {
		panic(err)
	}
	if title, ok := ticket.Title.Get(); ok {
		fmt.Println(title)
	}
}
```

See [`examples/`](examples/) for complete runnable programs covering basic CRUD, queries with iterator pagination, and middleware configuration.

## Documentation

| Guide | Topics |
|---|---|
| [Configuration and zone discovery](docs/configuration.md) | Client options, authentication, and saving or reusing `ZoneInfo` |
| [CRUD, queries, and pagination](docs/operations.md) | Generic CRUD, `Optional[T]`, query builder, `iter.Seq2` iterators, raw operations, and child entities |
| [Entities, metadata, and codegen](docs/entities.md) | Built-in entity catalog, CMDB, users and contacts, ticket links, runtime metadata, and `autotask-gen` |
| [Middleware and logging](docs/middleware.md) | Rate limiter, circuit breaker, concurrency limiter, threshold monitor, and `slog` events |
| [Error handling and retries](docs/errors.md) | Typed HTTP and protocol errors, maintenance pages, `IsTransient`, and retry safety |
| [Testing](docs/testing.md) | `autotasktest.NewMockClient`, in-memory `NewServer`, fixtures, and write-restriction simulation |

Full Go API documentation is available on [pkg.go.dev](https://pkg.go.dev/github.com/tphakala/go-autotask).

## Sponsoring

If you find `go-autotask` useful, consider [sponsoring the author on GitHub](https://github.com/sponsors/tphakala).

## License

[Apache-2.0](LICENSE)
