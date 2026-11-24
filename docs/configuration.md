# Configuration and zone discovery

Back to [README](../README.md).

## Client options

Create a client with `autotask.NewClient(ctx, auth, opts...)`. Always call `client.Close()` when finished to stop background middleware such as the threshold monitor.

| Option | Description |
|---|---|
| `WithBaseURL(url)` | Override automatic zone discovery with a fixed API base URL; with a non-empty URL, `Client.Zone` reports no zone |
| `WithZoneInfo(z)` | Use a zone saved from `Client.Zone` on an earlier run; no zone discovery request is made (see [Zone discovery](#zone-discovery)) |
| `WithZoneBaseURL(url)` | Override the base URL used for zone discovery (mainly for testing) |
| `WithHTTPClient(hc)` | Use a custom `*http.Client`; a nil `hc` keeps the default client |
| `WithLogger(l)` | Log circuit breaker, rate limiter, zone discovery, TLS fallback, and threshold monitor events to `l` (see [Logging](middleware.md#logging)); nil is ignored |
| `WithUserAgent(ua)` | Custom `User-Agent` header |
| `WithImpersonation(id)` | Perform API calls as another resource |
| `WithMiddleware(m)` | Add custom `http.RoundTripper` middleware |
| `WithRateLimiter(opts...)` | Enable rate limiting (see [Middleware](middleware.md#rate-limiter)) |
| `WithCircuitBreaker(opts...)` | Enable circuit breaker (see [Middleware](middleware.md#circuit-breaker)) |
| `WithMaxConcurrency(n)` | Cap the number of concurrent in-flight requests (see [Middleware](middleware.md#concurrency-limiter)) |
| `WithThresholdMonitor(opts...)` | Enable API usage monitoring; pass at least one option to start the background monitor (see [Middleware](middleware.md#threshold-monitor)) |
| `WithMaxResponseBytes(n)` | Largest API or zone discovery response body the client reads (default 128 MiB) |

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

With `WithZoneInfo`, `NewClient` sends no zone discovery request, uses `zone.URL` as the base URL, and `client.Zone()` returns the same value, `WebURL` included. `NewClient` returns an error when `zone.URL` is not an absolute `http` or `https` URL with a host and no `@`, query, or fragment, or when a non-empty `WithBaseURL` is also set.

The client sends the API credentials to `zone.URL`, so protect the saved file like the credentials and keep one per API username. A saved zone does not expire: if calls start to fail in a way that points at the endpoint, delete it and let `NewClient` discover the zone again.
