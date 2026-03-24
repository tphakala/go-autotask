# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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

[1.0.0]: https://github.com/tphakala/go-autotask/releases/tag/v1.0.0
