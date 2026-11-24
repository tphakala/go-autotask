# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed

- **`autotasktest.NewServer` answered a query or count on a child URL as if it were the root URL of the segment** (#73). `POST /v1.0/Tickets/{id}/Notes/query` and its `/query/count` looked `Notes` up as an entity name and found nothing, though `GET /v1.0/Tickets/{id}/Notes` listed the notes, and `/v1.0/Companies/9/Contacts/query` returned contacts created under company 7. A query or count on a child URL now resolves its store as the child list does and sees the items that list shows; the filter, paging and count apply to those. A child URL that resolves to no store still answers an empty page and a count of 0, and query and count on root URLs are unchanged.

## [1.11.0] - 2026-11-24

### Added

- **`middleware.WithThresholdMonitorUserAgent`** (#42): sets the `User-Agent` header of the threshold check. A monitor from `NewThresholdMonitor` sends `go-autotask/<version>` (or `go-autotask`) by default, the client's default; one started by `WithThresholdMonitor` sends the client's `User-Agent`, and this option passed to `WithThresholdMonitor` replaces it.
- **`MissingMemberError`** (#41): a 2xx response whose JSON body has no member holding the result, such as `{}` from `metadata.GetFields`. `Member` names the missing member (`fields`, `info` or `queryCount`). The error unwraps to `*Error` with `StatusCode` 200, and `IsTransient` reports it as transient.

### Changed

- **`NewClient` returns an error for a `WithBaseURL` or `WithZoneBaseURL` URL that is not an absolute `http` or `https` URL with a host and no `@`, query or fragment** (#44), the rule `WithZoneInfo` already applies. API paths are appended to the base URL, so such a URL produced request URLs with the path inside the query or fragment. `WithBaseURL("")` still leaves zone discovery on, and `WithZoneBaseURL` is checked only when zone discovery runs. A zone URL that zone discovery returns is checked the same way, so a zone from `Client.Zone` is one `WithZoneInfo` accepts.
- **The error an API call returns when its request cannot be built or fails no longer includes the request URL's query or user info** (#44), as for zone discovery and the threshold monitor in 1.10.0. Error types are unchanged; a caller that matches the full error text sees the shorter URL, and the cause of a parse error of a URL with user info is replaced by `invalid URL`.

### Fixed

- **Zone discovery and the threshold monitor sent Go's default `User-Agent`** (#42). The version and zone information requests now send the client's `User-Agent`, including a `WithUserAgent` value, and the threshold check sends it too (see `middleware.WithThresholdMonitorUserAgent`).
- **`WithThresholdMonitor()` with no options started no monitor** (#63). It now starts one with the default check interval, reporting through the client's logger. Repeated `WithThresholdMonitor` calls now start one monitor with the options of every call, in order; before, each call replaced the options of the one before it.
- **A circuit breaker counted outcomes of requests that were admitted before its last state change** (#64). A request that was in flight when the breaker opened could fail afterwards and restart the open timeout (every such failure did with a threshold of 1), and a request admitted in an earlier half-open period could reopen the breaker or count toward closing it in a later one. The breaker now ignores the outcome of a request admitted before its last state change: it is not counted and it does not move the open deadline, so the breaker goes half-open `WithOpenTimeout` after the failure that opened it. Failures of requests admitted in the current state count as before.
- **`autotask-gen` no longer leaves a broken or partly written file in the output directory** (#43). When the generated source fails to format, it returns the error and writes nothing; before, it wrote the unformatted source to the target file. Each file is now written to a temporary file in the same directory and renamed into place, so an interrupted run leaves the earlier file or the new one, never a file without the generated header that later runs would skip as hand-written. A target that is a symbolic link stays one: the file it points to is replaced, and a link to a file that does not exist stops the run with an error. The type scan of the output directory skips entries that are not regular files, such as a symbolic link to a directory with a name ending in `.go` or a dangling symbolic link, instead of stopping the run; a symbolic link to a regular file is still read, as the go tool compiles it.
- **`autotasktest.NewServer` matched child items by parent id alone** (#65). An item created at `/v1.0/Companies/9/Contacts` was also listed, read, updated and deleted at `/v1.0/Opportunities/9/Contacts`, since both URLs reach the `Contacts` store, and `WithChildEntity` seeds of one type under two parent types with the same id were listed under both. An item now belongs to the parent collection and id of the URL it was created at, or of the route `WithChildEntity` registers (parent id 0 means no parent), and a child list, `GET` by id, `PATCH` and `DELETE` check both. Items seeded with `WithEntity` or created at the root URL still appear under every parent, and root reads still see every item.
- **`autotasktest.NewServer` query filters compared numbers as float64** (#51). `id eq 9007199254740993` also matched 9007199254740992, and `gt`, `gte`, `lt` and `lte` could not tell such ids apart. `eq`, `noteq`, `in`, `notIn`, `gt`, `gte`, `lt` and `lte` now compare numbers by exact value, so `1` and `1.0` are equal. `contains`, `beginsWith` and `endsWith` see a number as its JSON text: `1000000` instead of `1e+06`.
- **`autotasktest.NewServer` answered metadata requests on a child URL for the parent** (#51). `GET /v1.0/Tickets/{id}/AdditionalConfigurationItems/entityInformation` and its `/fields` and `/userDefinedFields` answered with the `Tickets` metadata. They now answer for the entity the child URL resolves to (`TicketAdditionalConfigurationItems`), and `WithEntityMetadata` with that name sets the answer. The request creates no store.
- **The `metadata` calls and `Count` read a response without its result member as an empty result** (#41). `metadata.GetFields`, `metadata.GetUDFs` and `metadata.GetEntityInfo` now return `*MissingMemberError` for a 2xx JSON body without `fields` (or `info`, or with `"info": null`), such as `{}`; before they returned an empty list or an `EntityInfo` with every flag false and a nil error. `metadata.GetPickList` returns it too (it returned `field ... not found`). `Count` returns it for a body without `queryCount` (or with `"queryCount": null`); before it returned 0 with a nil error. `"fields": []`, `"fields": null`, `"info": {}` and `"queryCount": 0` are still results. `autotask-gen` skips an entity whose fields answer lacks the member, so it no longer rewrites that entity's file as a struct with no fields, and warns when the user-defined fields answer lacks it. An `autotasktest.NewMockClient` fixture for one of these calls whose JSON body has no such member now returns the error too, because the mock sends fixtures as given.

## [1.10.0] - 2026-11-24

### Added

- **`ListChildRawIterOf[P, C]`**: an untyped child iterator that builds its URL from the entity types, as `ListChildIter` does (parent segment from `ChildParentName` or the parent's `EntityName`, child segment from `ChildEntityName` or `EntityName`), so a caller that needs the raw maps no longer passes segment strings that can name a route the API does not serve.
- **`autotasktest.WithChildOnlyWrites[P, C]()`** (#58): opt-in. The mock accepts writes of `C` only on the child URL that `CreateChild[P]`, `UpdateChild[P]` and `DeleteChild[P, C]` send. A root write, a child write that resolves to the type's store under another parent or segment, and a create under a segment the type uses that matches no store and no `entities` type, answer 400, and a rejected write creates no store or item.
- **`autotasktest.WithoutUpdateSupport` and `autotasktest.WithoutCreateSupport`** (#35): opt-in. Each takes an entity name and answers PATCH (or POST) for it with a 400 at the root URL and at every child URL that resolves to its store.
- **`middleware.WithRateLimitLogger`, `middleware.WithCircuitBreakerLogger` and `middleware.WithThresholdMonitorLogger`** (#57): set the `*slog.Logger` a rate limiter, circuit breaker or threshold monitor writes to. The client passes its own logger to the ones `WithRateLimiter`, `WithCircuitBreaker` and `WithThresholdMonitor` build, and an option passed to those replaces it for that component.
- **`ChildParentEntity`** (#52): an optional interface for a child entity whose child URLs name the parent collection with a segment other than the parent's `EntityName`. The typed child calls use its `ChildParentName() string` in place of the parent type's `EntityName`. `entities.ExpenseItem` implements it with `Expenses`.
- **`middleware.LimiterWaitError`**: the error `ConcurrencyLimiter` and `RateLimiter` return when a request gives up waiting before it is sent, because its context ended or its deadline was too close for the next rate token. `Limiter` is `"concurrency"` or `"rate"`. It wraps the underlying error, so `errors.Is` for `context.Canceled` and `context.DeadlineExceeded`, `IsTransient` and `(*url.Error).Timeout` answer as before.

### Changed

- **The default `entityInformation` answer of `autotasktest.NewServer`** reports `canDelete` true for an entity with `WithDeleteSupport` (it was always false), and `canCreate` or `canUpdate` false for an entity with `WithoutCreateSupport` or `WithoutUpdateSupport`.
- **The error `NewClient` returns when a zone discovery request fails no longer includes the request URL's query or user info** (#57). The query carried the API username (`?user=...`). The same redaction applies to the error a threshold monitor passes to `WithErrorCallback` when its request fails. Error types are unchanged; a caller that matches the full error text sees the shorter URL, and the cause of a parse error of a URL with user info is replaced by `invalid URL`.
- **`WithHTTPClient(nil)` keeps the default HTTP client** instead of panicking in `NewClient` (#57).
- **`autotasktest.WithChildEntity` registers its route with the child's `ChildParentName`** when the child type has one, so the route matches the URL the typed child calls send.

### Fixed

- **`WithLogger` had no effect** (#57). The client now logs circuit breaker state changes (Warn), 429 `Retry-After` backoff (Warn) and the wait for it (Info), rate token and adaptive delay waits (Debug), threshold crossings at 75% and 90% on every check and failed threshold checks (Warn), zone discovery success (Info) and failure (Warn), and the TLS 1.3 to 1.2 downgrade of a host (Warn, once per pin). Records never carry credentials; apart from error values, which are logged as returned (see the `WithLogger` doc), they carry no bodies or query strings. `WithLogger(nil)` is ignored.
- **A request that timed out waiting in a limiter counted as a circuit breaker failure.** With the circuit breaker composed before the rate or concurrency limiter, as the `WithMaxConcurrency` doc recommends, local queueing could open the breaker. `DefaultFailure` now returns false for a `*middleware.LimiterWaitError`; a request that reached the transport and timed out still counts. `ConcurrencyLimiter` also refuses a request whose context has already ended before it takes a free slot, instead of choosing at random between the slot and the context. The error text of a limiter wait now starts with `autotask: concurrency limiter: ` or `autotask: rate limiter: `.
- **Typed child calls for `entities.QuoteItem` and `entities.ExpenseItem` addressed URLs the REST swagger does not list** (#52). `CreateChild`, `UpdateChild`, `DeleteChild`, `GetChild`, `ListChild` and `ListChildIter` now use the collection `/v1.0/Quotes/{id}/Items` for `QuoteItem` (`DeleteChild` appends the item id) (was `/v1.0/Quotes/{id}/QuoteItems`) and `/v1.0/Expenses/{id}/Items` for `ExpenseItem` (was `/v1.0/ExpenseReports/{id}/ExpenseItems`). This changes the URL for any caller of those calls with these two types. Root `Get`, `List`, `Count`, `Create`, `Update` and `Delete` are unchanged.

## [1.9.0] - 2026-11-23

### Added

- **`UpdateChild`, `UpdateChildRaw`, `DeleteChild` and `DeleteChildRaw`** (#46): update and delete a child entity through its parent, for entities the API serves only on the parent's URL. `UpdateChild[P, C]` and `UpdateChildRaw` send `PATCH /v1.0/{parent}/{parentID}/{child}` with the child's id in the body; `DeleteChild[P, C]` and `DeleteChildRaw` send `DELETE /v1.0/{parent}/{parentID}/{child}/{childID}`. `Update` and `Delete` still send the root URL.
- **`autotasktest.WithChildEntity[P, C](parentID, items...)`**: seeds child entities under a parent, and routes a child URL to a child type whose entity name is neither `singular(parent)+segment` nor the segment.
- **Eleven configuration item entity types**: `ConfigurationItemType` and `ConfigurationItemCategory` (root entities; only types can be deleted), and the child entities `ConfigurationItemCategoryUdfAssociation` (`UdfAssociations` under `ConfigurationItemCategories`; list, create, update, delete), `ConfigurationItemNote` (`Notes`; list, create, update), `ConfigurationItemAttachment` (`Attachments`; list, create, delete), `ConfigurationItemNoteAttachment` (`Attachments` under `ConfigurationItemNotes`; list, create, delete), `ConfigurationItemRelatedItem` (`RelatedItems`; list, create, delete), `ConfigurationItemBillingProductAssociation` (`BillingProductAssociations`; list, create, update, delete), `ConfigurationItemDnsRecord` and `ConfigurationItemSslSubjectAlternativeName` (list, delete) and `TicketAdditionalConfigurationItem` (`AdditionalConfigurationItems` under `Tickets`; list, create, delete). The operations follow the REST swagger. Child-only operations go through `CreateChild`, `UpdateChild` and `DeleteChild`, because the root URLs `Create`, `Update` and `Delete` send are not offered for them; each type's doc comment names the calls. None of them has `UserDefinedFields`. `Get`, `List` and `Count` work for all of them.
- **Eleven user management entity types**: the internal user assignments `ResourceRoleDepartment` (`RoleDepartments` under `Resources`; list, create, update), `ResourceRoleQueue` (`RoleQueues`; list, create, update), `ResourceServiceDeskRole` (`ServiceDeskRoles`; list, create, update), `ResourceSkill` (`Skills`; list, update), `ResourceAttachment` (`Attachments`; list, create, delete) and `ResourceRole` (`Roles`; list), the root entities `Role` (create, update), `Skill` (read-only) and `ClientPortalUser` (create, update), and `ContactGroup` (create, update, delete) with its members `ContactGroupContact` (`Contacts` under `ContactGroups`; list, create, delete). The operations follow the REST swagger. Child-only operations go through `CreateChild`, `UpdateChild` and `DeleteChild`, because the root URLs `Create`, `Update` and `Delete` send are not offered for them; each type's doc comment names the calls. None of them has `UserDefinedFields`. `Get`, `List` and `Count` work for all of them.

### Changed

- **`entities.ConfigurationItem` has every field of the entity metadata**, including `ProductID`, which the API requires on create, and fields are now in metadata order, so an unkeyed struct literal needs updating. Most of the Datto, RMM and SSL fields are read-only in the metadata, so do not send them back in an update. `LastActivityDate` is deprecated: the entity has no such field, so the API never returns it. `autotasktest.ConfigurationItemFixture` now sets `ProductID`.
- **`autotasktest.NewServer` keeps child items per parent and per entity type** (#48). A child URL `/v1.0/{Parent}/{id}/{Segment}` resolves to the store `WithChildEntity` registered for it, else the store named `singular(Parent)+Segment` (`Tickets` and `Attachments` give `TicketAttachments`). For an entity type in the `entities` package the store is then chosen by entity name alone: a create makes `singular(Parent)+Segment` when that is an entity name, else the store named `Segment` when that is one (`Contact` under `Companies`, `Task` under `Projects`), and a store named `Segment` is never used in place of a missing `singular(Parent)+Segment` entity. The mock sees only the URL, so a type from outside this module on a URL that names an `entities` type this way goes to that type's store, unless a store named `singular(Parent)+Segment` already exists, and needs `WithChildEntity`. On any other URL, the store named `Segment` is used when it exists, else a create makes `singular(Parent)+Segment`. A child list shows the items created under its parent plus items with no parent, and child `GET` by id, `PATCH` and `DELETE` check the parent (a mismatch answers 404 for `PATCH` and `DELETE`, and a null item for `GET`, as for a missing id at the root). Items seeded with `WithEntity` have no parent and still appear under every parent whose child URL resolves to their store. Items created through a child URL are now in the store of their entity type, so `List[T]`, `Count[T]` and `Get[T]` see them, for every entity type in the `entities` package. On other URLs, a type from outside this module whose entity name is neither `singular(Parent)+Segment` nor the segment needs `WithChildEntity` to read back. Ids come from one sequence per entity type, so two types that share a child segment (`TicketAttachment` and `TicketNoteAttachment`) no longer share ids.
- **A child list for a type with no store answers an empty page** instead of 404.
- **A `WithPageSize` of 0 or less puts every item on one page** in list and query responses.
- **`WithEntity` no longer registers the type's child segment as a store name.** Before, the last type seeded for a segment answered every child URL with that segment, whichever type the URL belonged to. A test that relied on this for a type whose entity name is neither `singular(parent)+segment` nor the segment needs `WithChildEntity`.
- **`entities.Contact` has every field of the entity metadata**, in metadata order, so an unkeyed struct literal needs updating. Contact writes go through the company: `CreateChild[Company]`, `UpdateChild[Company]` and `DeleteChild[Company, Contact]`; `Create`, `Update` and `Delete` send root URLs the REST swagger does not list for Contacts.
- **`entities.Resource` has every field of the entity metadata**, in metadata order, so an unkeyed struct literal needs updating. The API has no create or delete for resources, so only `Update` applies. `DateFormat`, `HireDate`, `InternalCost`, `LicenseType`, `LocationID`, `SurveyResourceRating`, `TimeFormat` and `UserType` are read-only.

### Fixed

- **`autotasktest.NewServer` child `DELETE` and child `GET` by id.** A child `DELETE` went to the parent's store, and a child `GET` by id returned the parent's list.
- **Child lists in `autotasktest.NewServer` ignored `?page=N`**, so a list longer than `WithPageSize` repeated its first page.
- **`autotasktest.NewServer` gave a created item the id of a seeded one.** Ids for creates started at 1 whatever ids `WithEntity` seeded, so a later read or update of that id could reach the seeded item. Seeding now moves the id sequence past each seeded id.
- **`autotasktest.NewServer` changed ids above 2^53**: it matched and stored ids through float64, so such an id could miss on `Get`, `Update` or `Delete`, or change after an update. Create, update and the lookup by id now keep numbers exact. A create body of `null` answers 400 instead of failing the request.
- **A large `?page=N` in `autotasktest.NewServer` list and query responses could panic or return the wrong page**, because the page offset overflowed. A page past the last item now answers an empty page.

## [1.8.0] - 2026-11-23

### Added

- **`entities.TicketNoteAttachment`** (entity `TicketNoteAttachments`): a file attached to a ticket note. Its child URL segment is `Attachments`, so `CreateChild[entities.TicketNote]` posts it to `/v1.0/TicketNotes/{noteID}/Attachments` and sets the new id on it, and `ListChild[entities.TicketNote, entities.TicketNoteAttachment]` lists a note's attachments from the same path. The file content goes base64-encoded in `Data`. Autotask has no update for it, and it has no `UserDefinedFields`. Before, attaching a file to a note with a typed call meant passing an `entities.TicketAttachment`, the type for a different entity, to `CreateChild[entities.TicketNote]`.

## [1.7.0] - 2026-11-20

### Added

- **`Client.Zone` and `WithZoneInfo`**: `Zone()` returns the `ZoneInfo` (`ZoneName`, `URL`, `WebURL`, `CI`) the client resolved in `NewClient`, and false for a client built with a non-empty `WithBaseURL`. `WithZoneInfo(z)` builds a client on `z.URL` without the two zone discovery requests, and `Zone()` then returns `z`. `ZoneInfo` marshals to JSON, so a short-lived process can discover the zone once, store it and reuse it on later runs. `NewClient` returns an error when `z.URL` is not an absolute `http` or `https` URL with a host and no `@`, query or fragment, or when `WithZoneInfo` is combined with a non-empty `WithBaseURL`; `WithZoneBaseURL` has no effect together with it. A `NewClient` call without `WithZoneInfo` behaves as before.
- **`entities.Ticket.Resolution` and `entities.Company.TerritoryID`**: `Resolution` (JSON `resolution`, a string of up to 32000 characters) and `TerritoryID` (JSON `territoryID`) are `Optional` fields like the others, so they are sent only when set. `Resolution` sits after `Description` and `TerritoryID` after `OwnerResourceID`, so an unkeyed struct literal of either type needs updating.

### Fixed

- **`autotasktest.NewServer` answers zone discovery without checking credentials.** The client sends no credentials for the `versioninformation` and `zoneInformation` requests, and the test server checked them on every path, so a client that ran zone discovery against it (no `WithBaseURL` or `WithZoneInfo`) failed with `version request returned 401`. The two discovery GETs, `/atservicesrest/versioninformation` and `/atservicesrest/<version>/zoneInformation` at the server root, are now answered before the auth check; every other request still requires the credentials. Rules added with `WithErrorOn` or `WithResponseOn` still run first.

## [1.6.0] - 2026-11-18

### Added

- **`MissingItemsError`** (#37): a 2xx list page whose JSON body has no `items` member, such as `{}`, now returns this error from `List`, `ListIter`, `ListRaw`, `ListRawIter`, `ListChild`, `ListChildIter`, `ListChildRaw`, `ListChildRawIter` and `GetChild`. In v1.5.1 those calls returned no items with a nil error for such a body, so a caller could read it as "no match". A page with `"items": []` or `"items": null` is still an empty page. The error unwraps to `*Error` with `StatusCode` 200, and `IsTransient` reports it as transient. An `autotasktest.NewMockClient` list fixture whose body has no `items` now returns it too, because the mock sends fixtures as given. `Count` and the `metadata` calls are unchanged.
- **`entities.Ticket.ProblemTicketID` and `entities.ChangeRequestLink`**: `ProblemTicketID` (JSON `problemTicketId`, with a lowercase d) links an incident ticket to its problem ticket, so `autotask.NewQuery().Where("problemTicketId", autotask.OpEq, problemID)` lists a problem's incidents. `ChangeRequestLink` (entity `ChangeRequestLinks`) links a Change Request ticket to a Problem or Incident ticket and works with `Create`, `Get`, `List`, `Count` and `Delete`. Autotask has no update for it, and it has no `UserDefinedFields`.
- **`autotasktest.WithResponseOn` and `autotasktest.Times`** (#13): `WithResponseOn(method, pathSuffix, status, contentType, body)` serves a fixed raw response, such as the HTML maintenance page with HTTP 200, an empty 500 or a proxy 502. An empty `contentType` sends no `Content-Type` header. `Times(n)` is a `RuleOption` that limits a rule to its first n matching requests; later requests fall through to the next matching rule or to the normal handler, so a test can fail twice and then recover. `n <= 0` leaves the rule unlimited. `WithErrorOn` and `WithRetryAfterError` accept the same options. Rules are tried in registration order, the first one that matches and has requests left answers, and they run before the auth check. Both existing functions gained a trailing variadic parameter, so a call site compiles unchanged, but a value of their old function type no longer matches.
- **`metadata.FieldInfo` and `autotasktest.FieldInfoResponse` report `Length`, `IsQueryable`, `IsReference` and `ReferenceEntityType`** (#16). `Length` is the maximum string length Autotask accepts for the field, or 0 when it reports none, so a caller can check a value before a write that Autotask would reject with an HTTP 500. `IsQueryable` and `IsReference` have no `omitempty`, so a marshalled `FieldInfoResponse` now always carries them, and a mock fixture that leaves `IsQueryable` unset reports the field as not queryable. `Length` sits after `Type` in both structs, so an unkeyed struct literal needs updating. `metadata.UDFInfo` is unchanged.
- **`IsTransient(err)` and `(*ServerError).IsValidation()`** (#11): `IsTransient` tells a caller whether retrying a failed call can succeed. Rate limiting, server errors, a blank or non-JSON 2xx response, a list page without `items`, an open circuit breaker, timeouts, refused or dropped connections and truncated bodies are transient. 400, 401, 403, 404, 409, 422, a cancelled context (unless it cut short the body of a 429 or 5xx response, which is classified by its status), `ResponseTooLargeError` and `MaxPagesExceededError` are not. A transient result says the failure may pass, not that the request is safe to send again: a create call, such as `Create` or `CreateChild`, that failed after it was sent may already have taken effect. `IsValidation` reports an HTTP 500 whose error message matches known validation wording (currently "exceeds maximum length"); `IsTransient` treats it as permanent. A failed zone discovery step now unwraps to its status-typed error, so `errors.As` and `IsTransient` classify it by status. The messages are unchanged.
- **`middleware.WithFailurePredicate`, `middleware.DefaultFailure` and `middleware.StatusFailure`** (#12): the circuit breaker's failure rule is configurable. A predicate can read the first 64 KiB of the response body, and the caller still receives the whole body.
- **`WithMaxResponseBytes` and `ResponseTooLargeError`** (#20): the client reads at most 128 MiB of a response body, including both zone discovery responses, and stops reading at the limit. `WithMaxResponseBytes(n)` changes the limit; values <= 0 keep the default. A longer 2xx body returns `*ResponseTooLargeError` with the `Limit` and the status in `Err.StatusCode`. A longer non-2xx API response body keeps the status-typed error (`*RateLimitError` with `RetryAfter`, `*ServerError` and so on), and the `*ResponseTooLargeError` is reachable beside it through `errors.As`. Before, every body was read in full with no cap. The threshold monitor started by `WithThresholdMonitor` now reads at most 1 MiB of its response and reports a longer one to its error callback.
- **`EmptyResponseError`**: a 2xx response with an empty or whitespace-only body now returns this error from any call that decodes a result: `Get`, `GetRaw`, `Count`, `Create`, `CreateRaw`, `CreateChild`, `CreateChildRaw`, `GetChild`, `Update`, `UpdateRaw`, the list and iterator functions (including `ListChild`, `ListChildRaw`, `ListChildIter` and `ListChildRawIter`), the `metadata` calls, `Client.Do` with a non-nil result, and zone discovery (wrapped). It carries the status in `Err.StatusCode` and unwraps to `*Error`. Calls that decode no result (`Delete`, `DeleteRaw`, `Client.Do` with a nil result) still accept a blank body. See Changed for what these calls returned before.
- **`UnexpectedContentTypeError`** (#10): a 2xx response whose body is neither labelled as JSON nor valid JSON, such as the HTML page Autotask serves with HTTP 200 during planned maintenance, now returns this typed error instead of an untyped `autotask: decoding response: invalid character '<' ...`. It holds the status in `Err.StatusCode` and unwraps to `*Error` like the other typed errors, and it carries the `ContentType` and a `Snippet` of up to 256 bytes of the body. Zone discovery returns it too, so a `NewClient` failure during maintenance can be matched with `errors.As`. A JSON body under another `Content-Type` still decodes, and a blank (empty or only whitespace) body does not return this error.

### Changed

- **`autotasktest.NewServer` answers a query that matches no stored item with `"items": []`** instead of `"items": null`, the shape the Autotask API sends. Client calls return the same results; only a test that reads the raw body sees the difference.
- **The circuit breaker's default failure rule changed** (#12). It no longer counts a caller's context cancellation or an HTTP 500 whose error message matches known validation wording (currently "exceeds maximum length"), and it now counts a 2xx response with a non-JSON `Content-Type`, such as the HTML maintenance page (`text/plain` is not counted). A request the caller cancels no longer counts as a failure either, so it no longer reopens a half-open breaker. Pass `middleware.WithFailurePredicate(middleware.StatusFailure)` for the previous rule.
- **Minimum Go version is now 1.27** (previously 1.26). The `go` directive in `go.mod` requires Go 1.27 or later, so consumers must build with Go 1.27+.
- Upgraded the linter to golangci-lint v2.15.0 and extended the ruleguard rule set with Go 1.27 modernization matchers. No public API changed.
- **`autotasktest.NewServer` answers a GET for a missing id with 200 and `{"item": null}`** instead of a 404, for an entity type the server has a store for (seeded with `WithEntity`, for example), so tests go through the same client path as a real missing record. `Get` and `GetRaw` still return `*NotFoundError` for it, now with `StatusCode` 200 and no `Errors`. Tests that inspect the raw HTTP status of that response will see 200. An entity type the server has no store for still answers 404, and `NewMockClient` is unchanged: it serves only the fixtures it is given.
- **`NewClient` always works on a copy of the `*http.Client`**, the default one or the one passed with `WithHTTPClient`. Before, it copied the client only when middleware was configured. Setting fields on the client passed to `WithHTTPClient` after `NewClient` returns no longer affects the client in use, and `NewClient` does not modify the caller's client. `middleware.NewThresholdMonitor` now also works on a copy of the client passed to it.
- **A 2xx body that is not blank, not labelled as JSON and not valid JSON now returns `*UnexpectedContentTypeError`** (#10). In v1.5.1, calls that decode a result returned an error wrapping `*json.SyntaxError` for it (`autotask: decoding response: ...`), and `NewClient` returned `autotask: decoding version response: ...` (or `decoding zone response`). The new error unwraps to `*Error` with the 2xx status in `StatusCode`, so an existing `errors.As` check for `*autotask.Error` now matches it and one for `*json.SyntaxError` no longer does.
- **A blank 2xx body is no longer read as a result.** In v1.5.1, `Get` returned an untyped `returned no item` error for an empty body. `GetRaw`, `CreateRaw` and `UpdateRaw` returned a nil map, `Count` returned 0, `Create` and `Update` returned the entity unchanged and the list functions returned no items, all with a nil error. The child calls did the same: `CreateChildRaw` returned a nil map, `CreateChild` returned the entity unchanged, and `GetChild` and the child list functions returned no items. A whitespace-only body returned `autotask: decoding response: unexpected end of JSON input`. Both now return `*EmptyResponseError`, including for an `autotasktest.NewMockClient` fixture registered with a nil body. A GET whose JSON body has a missing or null `item` returns `*NotFoundError` (see #14 under Fixed).

### Fixed

- **`autotask-gen` no longer breaks the output package.** It skips an entity whose Go type is already declared in another Go source file of the output directory (test files and names starting with `.` or `_` are not read), and it never overwrites a file that lacks the generated header; each skip prints a warning on stderr. Before, the README example wrote `tickets.go` beside the hand-written `ticket.go`, and the package stopped building. It now exits with status 1 when no entity was generated.
- **`Optional.UnmarshalJSON` decodes into a new value**, so a failed decode leaves the Optional unchanged, a decoded value clears null, and a value decoded after null or after another value holds only the new data. Before, a decode error left the Optional set with a zero value, and decoding a value into an Optional that held null left it null, so `Get` reported no value.
- **`Create` and `CreateChild` set the new id on every entity in `entities`**. Both read `{"itemId": N}` from the response but stored it only on types that implement `EntityWithID`, and no shipped type does, so `ID` stayed unset after a successful create. For a type without `SetID` they now decode `{"id": N}` into the entity, which sets the member with the JSON name `id` (`ID autotask.Optional[int64]` on every shipped and generated entity) and, for a type without its own `UnmarshalJSON`, changes no other member. A type that implements `EntityWithID` is handled as before. A custom entity whose id member has another JSON name still needs `SetID`. When the response has no `itemId`, or the id cannot be stored in the entity, the create still returns a nil error with `ID` unset.
- **A failed body read on a non-2xx response keeps the status-typed error** (#18). A 429 or 5xx whose body could not be read, for example after a connection reset mid-body, returned an untyped `autotask: reading response body: ...` error. It now returns the `*RateLimitError` (with `RetryAfter`), `*ServerError` or other typed error for the status, with no `Errors`, and the read error stays reachable through `errors.Is` and `errors.As`. A 2xx whose body cannot be read still returns the read error alone.
- **Redirect handling**: a redirect to a different scheme or host no longer carries the Autotask API headers. This applies to a `Client` and to a monitor built with `middleware.NewThresholdMonitor`. A `CheckRedirect` set on the `*http.Client` passed to either still runs, after this check.
- **`Delete` and `DeleteRaw` no longer report success for a 2xx body that is not blank, not labelled as JSON and not valid JSON** (#10). They decode no result, so the maintenance page read as a successful delete. They now return `UnexpectedContentTypeError`. The same applies to `Client.Do` called with a nil result.
- **`Get` and `GetRaw` return `*NotFoundError` for a missing or null `item`** (#14). `Get` returned an untyped error that `errors.As` could not match, and `GetRaw` returned `(nil, nil)`, so a missing record looked like success. The error has `StatusCode` 200, which tells it apart from a 404.
- **`UpdateRaw` returns the PATCH response body** (#15). It decoded an `item` field that the Autotask PATCH response does not carry, so a successful update returned a nil map. It now returns the decoded body, for example `{"itemId": 123}`, the same way `CreateRaw` does. It no longer unwraps an `item` field, so a PATCH response that carries one comes back as `{"item": {...}}`.
- **The User-Agent version comes from the build info** (#38). It was a constant that still said `go-autotask/1.3.0`. It is now `go-autotask/<module version>`, for example `go-autotask/1.6.0` in a program that depends on that release, and plain `go-autotask` when the build info has no version for the module (a local `replace`, a workspace, the module's own tests). `WithUserAgent` still overrides it.

## [1.5.1] - 2026-09-07

### Fixed

- **TLS 1.3 to TLS 1.2 fallback**: the HTTP client now falls back from TLS 1.3 to TLS 1.2 for hosts that refuse a TLS 1.3 handshake (Autotask's edge), fixing requests that failed with EOF or connection reset on Go 1.24+. A host downgraded to TLS 1.2 is re-probed for TLS 1.3 after an hour, so a downgrade triggered by a transient handshake blip self-heals instead of pinning the host to TLS 1.2 for the client's lifetime.

## [1.5.0] - 2026-09-03

### Added

- **`ListRawIter`**: lazy, early-exit iterator over untyped top-level entities (the untyped counterpart of `ListIter`). Callers that need only the first few items can stop without buffering every page. `ListRaw` now consumes it, so its accumulate-all and `MaxRecords` behavior is unchanged.
- **`ListChildRawIter`**: lazy, early-exit iterator over untyped child entities (the untyped counterpart of `ListChildIter`). Callers that need only the first few items can stop without buffering every page. `ListChildRaw` now consumes it, so its accumulate-all behavior is unchanged.

### Changed

- **Pagination consolidation (internal)**: `List` and `ListChild` now delegate to `ListIter` and `ListChildIter` instead of duplicating the page-follow loop, so all four list-all functions share one shape. The typed and untyped iterators decode pages through two shared helpers over a single generic page-response type. `List` still applies the `MaxRecords` cap on accumulation and, for successful responses, returns the same results and issues the same page-request sequence as before. Two edge cases on the `List` error path change: a malformed item positioned beyond the requested `MaxRecords` no longer fails the call (it is never decoded, matching `ListRaw`), and a decode failure is now reported as `autotask: decoding <Entity>: ...` instead of `autotask: decoding <Entity> item: ...` (matching `ListIter` and the other list functions).

### Fixed

- **Zone discovery API-version selection**: discovery now selects the advertised API version this client implements (case-insensitive, tolerant of a leading `V` and surrounding whitespace) instead of the last advertised entry. A version that Autotask advertises but does not serve no longer breaks `NewClient`. Request-URL joins are normalized to avoid a `//v1.0/` double slash.
- **`autotasktest` version endpoint**: the mock now returns the `apiVersions` key that zone discovery decodes (was `versions`).

## [1.4.4] - 2026-03-26

### Fixed

- **Child entity URL segments**: notes and attachments accessed as children now use the correct URL segment.

## [1.4.3] - 2026-03-25

### Fixed

- All picklist and integer fields in the hand-written entities changed from `Optional[int]` to `Optional[int64]` to match the API and the generated entities. Affected: Ticket (Status, Priority, Source, IssueType, SubIssueType, TicketType, TicketCategory), Company (CompanyType, Classification), Project (Status, Type), Task (Status, Priority), Contract (Status, ContractType), ConfigurationItem (ConfigurationItemType), TicketNote (NoteType, Publish).

### Changed

- **Breaking**: code that used `autotask.Set(1)` for any of the fields above must now use `autotask.Set(int64(1))`.

## [1.4.2] - 2026-03-25

### Fixed

- **`Resource.ResourceType`**: changed from `Optional[int64]` to `Optional[string]`; the API returns string values such as `"Employee"` and `"Contractor"`, not numeric IDs.

## [1.4.1] - 2026-03-25

### Fixed

- **`Resource.ResourceType`**: changed from `Optional[int]` to `Optional[int64]`; the API returns numeric picklist IDs.
- **`Contact.IsActive`**: changed from `Optional[bool]` to `Optional[int64]`; the API returns `1`/`0`, not `true`/`false` (unlike `Resource.IsActive`, which is boolean).
- **`GetEntityInfo`**: unwrap the `{"info": {...}}` response envelope so `CanCreate`, `CanQuery`, and the other fields are populated instead of left as zero values.

## [1.4.0] - 2026-03-25

### Added

- **`WithErrorCallback`**: new `ThresholdMonitor` option to receive errors from background monitoring checks instead of silent failures

### Fixed

- **`Get` nil/null item guard**: returns a clear error when the API returns `null` or a missing `item` instead of a confusing unmarshal error
- **Zone discovery response body leak**: the first HTTP response body was never closed because the `resp` variable was reassigned before its defer ran; now uses separate variables
- **Nil entity guards in `Create`/`Update`**: return an error instead of panicking on a nil entity (consistent with `CreateChild`)
- **Timer leak in rate limiter**: replaced `time.After` with `time.NewTimer` plus an explicit `Stop()` to prevent timer and memory leaks on context cancellation
- **Retry-After parsing consistency**: middleware now uses `strconv.Atoi`, matching the `error.go` implementation
- **Version constant**: updated the User-Agent from `go-autotask/0.1.0` to `go-autotask/1.3.0`

### Changed

- **Query**: documented that the builder is mutable and should not be shared across goroutines

## [1.3.0] - 2026-03-25

### Added

- **`WithMaxConcurrency(n)`**: semaphore-based middleware limiting concurrent in-flight API requests. Autotask enforces a per-integration-code thread limit (default 3); this prevents the client from exceeding it. It blocks with context-cancellation support and releases the slot on response completion.

## [1.2.0] - 2026-03-25

### Added

- **16 new entity types:** ProjectNotes, CompanyNotes, TicketAttachments, Quotes, QuoteItems, Opportunities, Invoices, BillingItems, BillingItemApprovalLevels, BillingCodes, ExpenseReports, ExpenseItems, Products, Services, ServiceBundles, Departments
- **Generator acronym normalization**: `goName()` recognizes 16 Go acronyms (ID, URL, API, SKU, and so on), producing idiomatic field names
- **Generator conditional `time` import**: the template only imports `"time"` when datetime fields are present, preventing build failures on entities without time fields
- **Generator configurable entity list**: the `-entities` flag accepts comma-separated names; defaults expanded from 9 to 25
- **Generator idiomatic filenames**: `toSnakeCase()` produces `ticket_notes.go` instead of `ticketnotes.go`, with acronym handling (`HTTPServer` becomes `http_server`)
- **Generator smarter pluralization**: `singular()` handles irregular plurals such as `Statuses` becoming `Status`
- **`ListChild` and `ListChildIter`**: automatic pagination for child entities, replacing the first-page-only `GetChild`
- **`ListChildRaw` and `CreateChildRaw`**: untyped child entity operations
- **`metadata.GetPickList`**: convenience function to fetch picklist values for a single field
- **`EntityWithID` interface**: `Create` and `CreateChild` parse `{"itemId": N}` from API responses and populate the entity's ID via an optional `SetID()` method
- **Pagination safety guards**: all pagination functions (`List`, `ListRaw`, `ListChild`, `ListChildIter`, `ListIter`) enforce a `maxPages` (1000) limit with `MaxPagesExceededError`

### Fixed

- **Zone discovery:** the API returns `apiVersions`, not `versions`; fixed the field tag in `zone.go`
- **Picklist values:** `PickListValue.Value` changed from `int` to `string` to match actual API responses; added `SortOrder`, `ParentValue`, `IsSystem` fields

### Deprecated

- `GetChild`: use `ListChild`, which provides automatic pagination

## [1.1.0] - 2026-03-24

### Added

- Generator: configurable `-entities` flag, expanded default entity list (25 entities)
- Generator: idiomatic `toSnakeCase()` filenames, improved `singular()` pluralization
- `ListChild` and `ListChildIter` for child entity pagination
- `metadata.GetPickList` convenience function
- Pagination safety guards (`MaxPagesExceededError`) on all pagination functions
- `EntityWithID` interface for parsing `itemId` from `Create`/`CreateChild` responses
- `ListChildRaw` and `CreateChildRaw` for untyped child operations
- `toSnakeCase` acronym handling (`HTTPServer` becomes `http_server`)

### Deprecated

- `GetChild`: use `ListChild`

## [1.0.0] - 2026-03-24

### Added

- Type-safe generic CRUD operations (`Get`, `List`, `Create`, `Update`, `Delete`)
- Query builder with a fluent API (`Where`, `Or`, `And`, field selection, limits)
- Iterator pagination via `ListIter` returning `iter.Seq2`
- Three-state `Optional[T]` type (unset / null / value) for correct API semantics
- Middleware: rate limiter, circuit breaker, API threshold monitor
- Raw operations (`GetRaw`, `ListRaw`, and friends) for undefined entities
- Child entity support (`GetChild`, `CreateChild`)
- Metadata introspection (field definitions, UDFs, entity capabilities)
- Code generation tool (`autotask-gen`) for entity structs from live API metadata
- Test support via `autotasktest.NewMockClient` with fixtures
- Automatic zone discovery for API endpoint resolution
- Structured error types mapped to HTTP status codes
- Entity types: Company, Contact, Ticket, Resource, Contract, Project, Task, ConfigurationItem, TicketNote, TimeEntry
- GitHub Actions: CI (test and lint), CodeQL, govulncheck, Dependabot, automated releases, stale issue cleanup

[Unreleased]: https://github.com/tphakala/go-autotask/compare/v1.11.0...HEAD
[1.11.0]: https://github.com/tphakala/go-autotask/compare/v1.10.0...v1.11.0
[1.10.0]: https://github.com/tphakala/go-autotask/compare/v1.9.0...v1.10.0
[1.9.0]: https://github.com/tphakala/go-autotask/compare/v1.8.0...v1.9.0
[1.8.0]: https://github.com/tphakala/go-autotask/compare/v1.7.0...v1.8.0
[1.7.0]: https://github.com/tphakala/go-autotask/compare/v1.6.0...v1.7.0
[1.6.0]: https://github.com/tphakala/go-autotask/compare/v1.5.1...v1.6.0
[1.5.1]: https://github.com/tphakala/go-autotask/compare/v1.5.0...v1.5.1
[1.5.0]: https://github.com/tphakala/go-autotask/compare/v1.4.4...v1.5.0
[1.4.4]: https://github.com/tphakala/go-autotask/compare/v1.4.3...v1.4.4
[1.4.3]: https://github.com/tphakala/go-autotask/compare/v1.4.2...v1.4.3
[1.4.2]: https://github.com/tphakala/go-autotask/compare/v1.4.1...v1.4.2
[1.4.1]: https://github.com/tphakala/go-autotask/compare/v1.4.0...v1.4.1
[1.4.0]: https://github.com/tphakala/go-autotask/compare/v1.3.0...v1.4.0
[1.3.0]: https://github.com/tphakala/go-autotask/compare/v1.2.0...v1.3.0
[1.2.0]: https://github.com/tphakala/go-autotask/compare/v1.1.0...v1.2.0
[1.1.0]: https://github.com/tphakala/go-autotask/compare/v1.0.0...v1.1.0
[1.0.0]: https://github.com/tphakala/go-autotask/releases/tag/v1.0.0
