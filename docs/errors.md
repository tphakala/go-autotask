# Error handling and retries

Back to [README](../README.md).

## Typed API errors

API errors are returned as typed errors for easy matching:

```go
ticket, err := autotask.Get[entities.Ticket](ctx, client, 999)
if nf, ok := errors.AsType[*autotask.NotFoundError](err); ok {
	fmt.Println("not found:", nf.Err.Message)
}
```

| Type | HTTP Status |
|---|---|
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

## Protocol and response body errors

- **`UnexpectedContentTypeError`**: returned for a 2xx response whose body is not blank (empty or only whitespace), not labelled as JSON, and not valid JSON. Its `Err.StatusCode` holds the status, `ContentType` the header, and `Snippet` up to the first 256 bytes of the body. It unwraps to `*Error` like the other typed errors, so check for it before a generic `*Error` branch. Autotask answers with HTTP 200 and an HTML page during planned maintenance, so `IsTransient` reports this error as transient. A create call, such as `Create` or `CreateChild`, that got it may already have taken effect, so sending it again can duplicate the record; see [Deciding whether to retry](#deciding-whether-to-retry). `NewClient` returns it (wrapped) when zone discovery hits the same page.
- **`EmptyResponseError`**: returned for a 2xx response with an empty or whitespace-only body from any call that decodes a result, such as `Get`, `Count`, `Create`, `UpdateRaw`, or a list page. It is not a `NotFoundError`: a blank body says nothing about whether the record exists. `Delete`, `DeleteRaw`, `DeleteChild`, and `DeleteChildRaw` still accept a blank body.
- **`MissingItemsError`**: returned for a 2xx list page whose JSON body has no `items` member, such as `{}` from `List`, `ListRaw`, `ListChild`, `ListChildRaw`, their iterators, and `GetChild`. It is not an empty result, so do not read it as "no match" before a create. A page with `"items": []` or `"items": null` is an empty page. `MissingItemsError` unwraps to `*Error` with `StatusCode` 200, and `IsTransient` reports it as transient. An iterator can yield it after the items of earlier pages, and a new iteration starts again at the first page, so work done per item must be safe to repeat before a loop is retried.
- **`MissingMemberError`**: returned for a 2xx response whose JSON body has no member holding the result: `metadata.GetFields`, `metadata.GetUDFs` (and `metadata.GetPickList`, through `GetFields`) without `fields`, `metadata.GetEntityInfo` without `info` (or with `"info": null`), and `Count` without `queryCount` (or with `"queryCount": null`), such as `{}`. `Member` names the missing member. It is not an empty result or a count of zero, so do not read it as "no fields" or "no match". `"fields": []` and `"fields": null` are an empty list, and `"queryCount": 0` is a count of zero. `MissingMemberError` unwraps to `*Error` with `StatusCode` 200, and `IsTransient` reports it as transient.
- **`ResponseTooLargeError`**: the client reads at most 128 MiB of an API or zone discovery response body; change the limit with [`WithMaxResponseBytes`](configuration.md#client-options). A longer 2xx body returns `ResponseTooLargeError`. For an API response with any other status the status-typed error is returned (a 503 is still a `ServerError`), with the `ResponseTooLargeError` reachable through `errors.As`.

`EmptyResponseError` and `ResponseTooLargeError` unwrap to `*Error` and carry the status in `Err.StatusCode`, and `NewClient` returns them (wrapped) when a zone discovery response is blank or over the limit.

## Deciding whether to retry

```go
if err != nil && autotask.IsTransient(err) {
	// Back off and retry. Do not resend a create call (Create, CreateRaw, CreateChild, CreateChildRaw) on
	// this alone: it may already have taken effect, see below.
}
```

`IsTransient` is true for rate limiting, server errors, a blank or non-JSON 2xx response, a list page without `items`, a metadata or count response without its result member, an open circuit breaker, timeouts, refused or dropped connections, and truncated bodies.

`IsTransient` is false for 400, 401, 403, 404, 409, and 422, a cancelled context (unless it cut short the body of a 429 or 5xx response, which is classified by its status), `ResponseTooLargeError`, `MaxPagesExceededError`, an unrecognised error, and nil. A zone discovery failure is classified by its status.

Autotask answers HTTP 500 for some validation failures, and a retry returns the same error. `(*ServerError).IsValidation()` reports such a 500, and `IsTransient` treats it as permanent. It matches the error message against a short list of known wording, so a validation 500 with other wording still looks like a server failure.

A transient result says the failure may pass, not that the request is safe to send again, because a create call, such as `Create` or `CreateChild`, that failed after it was sent may already have taken effect.
