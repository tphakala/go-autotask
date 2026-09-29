package autotasktest

import (
	"io"
	"net/http"
	"testing"
)

func ruleGet(t *testing.T, ts *TestServer) (status int, header http.Header, body string) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, ts.URL+"/v1.0/Companies/1", http.NoBody)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, resp.Header, string(b)
}

func TestWithResponseOnServesRawBody(t *testing.T) {
	ts, _ := NewServer(t,
		WithResponseOn(http.MethodGet, "/Companies/1", http.StatusOK, "text/html", "<html>Unavailable</html>"),
	)

	status, header, body := ruleGet(t, ts)
	if status != http.StatusOK || header.Get("Content-Type") != "text/html" || body != "<html>Unavailable</html>" {
		t.Fatalf("got %d %q %q; want 200 text/html and the served body", status, header.Get("Content-Type"), body)
	}
}

// The body is not empty: net/http sniffs a Content-Type for a non-empty body
// unless the header is present with a nil value, which is what
// writeRawResponse does for an empty contentType.
func TestWithResponseOnEmptyContentTypeSendsNoHeader(t *testing.T) {
	ts, _ := NewServer(t,
		WithResponseOn(http.MethodGet, "/Companies/1", http.StatusOK, "", "<html>x</html>"),
	)

	_, header, body := ruleGet(t, ts)
	if body != "<html>x</html>" {
		t.Fatalf("body = %q; want the served body", body)
	}
	if _, present := header["Content-Type"]; present {
		t.Fatalf("Content-Type = %q; want the header absent", header.Get("Content-Type"))
	}
}

func TestTimesLimitsRulesThenFallsThrough(t *testing.T) {
	ts, _ := NewServer(t,
		WithErrorOn(http.MethodGet, "/Companies/1", http.StatusServiceUnavailable, []string{"first"}, Times(1)),
		WithResponseOn(http.MethodGet, "/Companies/1", http.StatusTeapot, "text/plain", "second", Times(2)),
	)

	// The raw request carries no credentials, so once both rules are used up the
	// normal handler answers with its 401.
	want := []int{http.StatusServiceUnavailable, http.StatusTeapot, http.StatusTeapot, http.StatusUnauthorized}
	for i, w := range want {
		if status, _, _ := ruleGet(t, ts); status != w {
			t.Fatalf("request %d: status %d; want %d", i+1, status, w)
		}
	}
}

func TestTimesNonPositiveIsUnlimited(t *testing.T) {
	for _, n := range []int{0, -3} {
		ts, _ := NewServer(t,
			WithRetryAfterError("/Companies/1", 3, Times(n)),
		)
		for i := range 4 {
			status, header, _ := ruleGet(t, ts)
			if status != http.StatusTooManyRequests || header.Get("Retry-After") != "3" {
				t.Fatalf("Times(%d) request %d: status %d Retry-After %q; want 429 and 3", n, i+1, status, header.Get("Retry-After"))
			}
		}
	}
}
