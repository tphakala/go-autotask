# Middleware and logging

Back to [README](../README.md).

## Rate limiter

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

## Circuit breaker

```go
client, err := autotask.NewClient(ctx, auth,
	autotask.WithCircuitBreaker(
		middleware.WithFailureThreshold(5),
		middleware.WithOpenTimeout(30*time.Second),
	),
)
```

Three-state circuit breaker (closed, open, half-open) that stops sending requests after repeated failures. `WithFailureWindow` and `WithSuccessThreshold` tune the trip and recovery behavior.

By default a round trip counts as a failure when it is a transport error (a caller's context cancellation and a request that gave up waiting in the concurrency or rate limiter excepted), a 429 or a 5xx, or a 2xx response whose `Content-Type` header is present and not JSON or `text/plain` (excluding 204 and `Content-Length: 0`), such as the HTML maintenance page. A 500 whose error message matches the validation wording that `(*ServerError).IsValidation` uses does not count, so repeated validation failures cannot open the breaker for all traffic (see [Deciding whether to retry](errors.md#deciding-whether-to-retry)). A validation 500 worded differently still counts. `WithFailurePredicate(fn)` replaces that rule. `middleware.DefaultFailure` is the default and `middleware.StatusFailure` is the rule the breaker used before: transport errors, 429, and 5xx. The predicate can read the first 64 KiB of the response body and the caller still gets the whole body.

## Concurrency limiter

```go
client, err := autotask.NewClient(ctx, auth,
	autotask.WithCircuitBreaker(),  // fail fast if the circuit is open
	autotask.WithRateLimiter(),     // then enforce the hourly rate
	autotask.WithMaxConcurrency(3), // then cap in-flight requests
)
```

Caps the number of requests in flight at once. The first option is the outermost middleware, so put the circuit breaker first, then the rate limiter, then the concurrency limiter: an open circuit rejects a request before it waits for a token or a slot, and the rate limit is enforced before the concurrency gate. A request that times out or is cancelled while waiting in either limiter fails with a `*middleware.LimiterWaitError`, which the default failure predicate does not count against the circuit breaker.

## Threshold monitor

```go
client, err := autotask.NewClient(ctx, auth,
	autotask.WithThresholdMonitor(
		middleware.WithCheckInterval(5*time.Minute),
		middleware.WithWarningCallback(func(info middleware.ThresholdInfo) {
			log.Printf("API usage at %.0f%%", info.UsagePercent)
		}),
		middleware.WithCriticalCallback(func(info middleware.ThresholdInfo) {
			log.Printf("CRITICAL: API usage at %.0f%%", info.UsagePercent)
		}),
	),
)
```

Polls the Autotask `ThresholdInformation` endpoint in the background (starting with an immediate check) and invokes callbacks on each check when usage is at or above 75% (warning) or 90% (critical); critical usage without a critical callback falls through to the warning callback. It also logs each check at or above those levels and each failed check to the client's logger (see [Logging](#logging)). Pass at least one `ThresholdMonitorOption` to `WithThresholdMonitor` to start the monitor, and call `client.Close()` when shutting down to stop the polling loop.

## Logging

```go
client, err := autotask.NewClient(ctx, auth,
	autotask.WithLogger(slog.Default()),
	autotask.WithCircuitBreaker(),
	autotask.WithRateLimiter(),
)
```

`WithLogger` hands its logger to the rate limiter, circuit breaker, and threshold monitor the client builds, wherever it is listed among the options. The client writes these records, each message starting with `autotask: `:

| Event | Level |
|---|---|
| Circuit breaker state change | Warn |
| 429 with a `Retry-After` header | Warn |
| Request starts waiting for the `Retry-After` period | Info |
| Request waited for a rate limit token, or starts the adaptive delay | Debug |
| API usage at or above 75% or 90%, on every check | Warn |
| Threshold check failed | Warn |
| Zone discovered | Info |
| Zone discovery failed | Warn |
| Host refused TLS 1.3 and is pinned to TLS 1.2 | Warn |

`middleware.WithRateLimitLogger`, `middleware.WithCircuitBreakerLogger`, and `middleware.WithThresholdMonitorLogger` set the logger for middleware built directly with `middleware.NewRateLimiter` and the like. Passed to `WithRateLimiter`, `WithCircuitBreaker`, or `WithThresholdMonitor`, they replace the client's logger for that component. Middleware added with `WithMiddleware` or `WithMaxConcurrency` does not log, and the TLS record needs the default HTTP client rather than one set with `WithHTTPClient`.

Records carry method, URL path, host, counts, durations, state names, and errors. They never carry credentials, and the records the client writes itself carry no request or response bodies or URL query strings. An error value is logged as returned to the caller, so it can still quote part of a redirect `Location` header or the API versions a zone discovery response advertised (see the `WithLogger` doc). The error `NewClient` returns for a failed zone discovery request, and the error a threshold monitor logs or passes to its error callback for a failed request, have the request URL's query, user info, and fragment removed. A threshold check that fails because the monitor is stopping is not logged (though the error callback still runs).
