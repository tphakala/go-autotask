package autotask_test

import (
	"errors"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"

	autotask "github.com/tphakala/go-autotask"
	"github.com/tphakala/go-autotask/autotasktest"
	"github.com/tphakala/go-autotask/entities"
)

const maintenanceHTML = "<html>Unavailable</html>"

func TestWithResponseOnHTMLMaintenancePage(t *testing.T) {
	t.Parallel()
	company := autotasktest.CompanyFixture(func(c *entities.Company) { c.ID = autotask.Set(int64(1)) })
	_, client := autotasktest.NewServer(t,
		autotasktest.WithEntity(company),
		autotasktest.WithResponseOn(http.MethodGet, "/Companies/1", http.StatusOK, "text/html", maintenanceHTML),
	)

	_, err := autotask.Get[entities.Company](t.Context(), client, 1)
	ct, ok := errors.AsType[*autotask.UnexpectedContentTypeError](err)
	if !ok {
		t.Fatalf("got %T: %v; want *UnexpectedContentTypeError", err, err)
	}
	if ct.ContentType != "text/html" {
		t.Fatalf("ContentType = %q; want text/html", ct.ContentType)
	}
	if ct.Snippet != maintenanceHTML {
		t.Fatalf("Snippet = %q; want the served body", ct.Snippet)
	}
	if ct.Err.StatusCode != http.StatusOK {
		t.Fatalf("StatusCode = %d; want 200", ct.Err.StatusCode)
	}
}

func TestWithResponseOnTimesThenFallsThrough(t *testing.T) {
	t.Parallel()
	company := autotasktest.CompanyFixture(func(c *entities.Company) { c.ID = autotask.Set(int64(1)) })
	ts, client := autotasktest.NewServer(t,
		autotasktest.WithEntity(company),
		autotasktest.WithResponseOn(http.MethodGet, "/Companies/1", http.StatusBadGateway, "text/plain", "bad gateway", autotasktest.Times(2)),
	)

	for i := 1; i <= 2; i++ {
		_, err := autotask.Get[entities.Company](t.Context(), client, 1)
		if _, ok := errors.AsType[*autotask.ServerError](err); !ok {
			t.Fatalf("request %d: got %T: %v; want *ServerError", i, err, err)
		}
	}
	got, err := autotask.Get[entities.Company](t.Context(), client, 1)
	if err != nil {
		t.Fatalf("third request: %v", err)
	}
	if v, _ := got.ID.Get(); v != 1 {
		t.Fatalf("third request returned ID %d; want the seeded entity 1", v)
	}
	if n := len(ts.Requests()); n < 3 {
		t.Fatalf("recorded %d requests; want at least 3", n)
	}
}

func TestRuleOrderAndExhaustedRuleFallsToNextRule(t *testing.T) {
	t.Parallel()
	ts, _ := autotasktest.NewServer(t,
		autotasktest.WithErrorOn(http.MethodGet, "/Companies/1", http.StatusServiceUnavailable, []string{"first"}, autotasktest.Times(1)),
		autotasktest.WithResponseOn(http.MethodGet, "/Companies/1", http.StatusTeapot, "text/plain", "second"),
	)

	want := []int{http.StatusServiceUnavailable, http.StatusTeapot, http.StatusTeapot}
	for i, w := range want {
		status, _, _ := rawGet(t, ts.URL+"/v1.0/Companies/1")
		if status != w {
			t.Fatalf("request %d: status %d; want %d", i+1, status, w)
		}
	}
}

func TestRuleMethodMismatchDoesNotConsumeTimes(t *testing.T) {
	t.Parallel()
	ts, _ := autotasktest.NewServer(t,
		autotasktest.WithResponseOn(http.MethodGet, "/Companies/1", http.StatusTeapot, "text/plain", "x", autotasktest.Times(1)),
	)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodDelete, ts.URL+"/v1.0/Companies/1", http.NoBody)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode == http.StatusTeapot {
		t.Fatal("a DELETE matched a GET rule")
	}
	if status, _, _ := rawGet(t, ts.URL+"/v1.0/Companies/1"); status != http.StatusTeapot {
		t.Fatalf("GET status %d; want 418, the mismatched DELETE must not use up the rule", status)
	}
}

func TestTimesNonPositiveIsUnlimited(t *testing.T) {
	t.Parallel()
	for _, n := range []int{0, -3} {
		ts, _ := autotasktest.NewServer(t,
			autotasktest.WithResponseOn(http.MethodGet, "/Companies/1", http.StatusTeapot, "text/plain", "x", autotasktest.Times(n)),
		)
		for i := range 5 {
			if status, _, _ := rawGet(t, ts.URL+"/v1.0/Companies/1"); status != http.StatusTeapot {
				t.Fatalf("Times(%d) request %d: status %d; want 418", n, i+1, status)
			}
		}
	}
}

func TestWithResponseOnEmptyContentTypeSendsNoHeader(t *testing.T) {
	t.Parallel()
	ts, client := autotasktest.NewServer(t,
		autotasktest.WithResponseOn(http.MethodGet, "/Companies/1", http.StatusOK, "", ""),
	)

	status, header, body := rawGet(t, ts.URL+"/v1.0/Companies/1")
	if status != http.StatusOK || body != "" {
		t.Fatalf("got %d %q; want 200 with an empty body", status, body)
	}
	if _, present := header["Content-Type"]; present {
		t.Fatalf("Content-Type = %q; want the header absent", header.Get("Content-Type"))
	}

	_, err := autotask.Get[entities.Company](t.Context(), client, 1)
	if _, ok := errors.AsType[*autotask.EmptyResponseError](err); !ok {
		t.Fatalf("got %T: %v; want *EmptyResponseError", err, err)
	}
}

func TestWithRetryAfterErrorTimes(t *testing.T) {
	t.Parallel()
	company := autotasktest.CompanyFixture(func(c *entities.Company) { c.ID = autotask.Set(int64(1)) })
	_, client := autotasktest.NewServer(t,
		autotasktest.WithEntity(company),
		autotasktest.WithRetryAfterError("/Companies/1", 7, autotasktest.Times(1)),
	)

	_, err := autotask.Get[entities.Company](t.Context(), client, 1)
	rl, ok := errors.AsType[*autotask.RateLimitError](err)
	if !ok {
		t.Fatalf("got %T: %v; want *RateLimitError", err, err)
	}
	if rl.RetryAfter.Seconds() != 7 {
		t.Fatalf("RetryAfter = %v; want 7s", rl.RetryAfter)
	}
	if _, err := autotask.Get[entities.Company](t.Context(), client, 1); err != nil {
		t.Fatalf("second request: %v", err)
	}
}

// TestRuleTimesConcurrent runs under -race: the hit counter is shared by the
// concurrent handlers and exactly n requests may be answered by the rule.
func TestRuleTimesConcurrent(t *testing.T) {
	t.Parallel()
	const limit, callers = 5, 40
	ts, _ := autotasktest.NewServer(t,
		autotasktest.WithResponseOn(http.MethodGet, "/Companies/1", http.StatusTeapot, "text/plain", "x", autotasktest.Times(limit)),
	)

	var teapots atomic.Int64
	var wg sync.WaitGroup
	for range callers {
		wg.Go(func() {
			status, _, _ := rawGet(t, ts.URL+"/v1.0/Companies/1")
			if status == http.StatusTeapot {
				teapots.Add(1)
			}
		})
	}
	wg.Wait()
	if got := teapots.Load(); got != limit {
		t.Fatalf("rule answered %d requests; want exactly %d", got, limit)
	}
}

func rawGet(t *testing.T, url string) (status int, header http.Header, body string) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, http.NoBody)
	if err != nil {
		t.Error(err)
		return 0, nil, ""
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Error(err)
		return 0, nil, ""
	}
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Error(err)
	}
	return resp.StatusCode, resp.Header, string(b)
}
