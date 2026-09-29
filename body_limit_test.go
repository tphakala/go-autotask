package autotask

import (
	"context"
	"errors"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func newStaticClient(t *testing.T, status int, body string, opts ...ClientOption) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Retry-After", "7")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	c, err := NewClient(t.Context(), AuthConfig{Username: "u", Secret: "s", IntegrationCode: "i"},
		append([]ClientOption{WithBaseURL(srv.URL)}, opts...)...)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// resultCalls are the calls that decode a result from a 2xx body.
func resultCalls() map[string]func(context.Context, *Client) error {
	return map[string]func(context.Context, *Client) error{
		"Get": func(ctx context.Context, c *Client) error {
			_, err := Get[testEntity](ctx, c, 1)
			return err
		},
		"GetRaw": func(ctx context.Context, c *Client) error {
			_, err := GetRaw(ctx, c, "Tickets", 1)
			return err
		},
		"Count": func(ctx context.Context, c *Client) error {
			_, err := Count[testEntity](ctx, c, nil)
			return err
		},
		"Create": func(ctx context.Context, c *Client) error {
			_, err := Create(ctx, c, &testEntity{})
			return err
		},
		"CreateRaw": func(ctx context.Context, c *Client) error {
			_, err := CreateRaw(ctx, c, "Tickets", map[string]any{"a": 1})
			return err
		},
		"UpdateRaw": func(ctx context.Context, c *Client) error {
			_, err := UpdateRaw(ctx, c, "Tickets", map[string]any{"id": 1})
			return err
		},
		"ListRaw": func(ctx context.Context, c *Client) error {
			_, err := ListRaw(ctx, c, "Tickets", nil)
			return err
		},
	}
}

func TestBlankSuccessBodyIsEmptyResponseError(t *testing.T) {
	bodies := []struct {
		name   string
		status int
		body   string
	}{
		{"200 empty", http.StatusOK, ""},
		{"200 whitespace", http.StatusOK, " \r\n"},
		{"204", http.StatusNoContent, ""},
	}
	for _, b := range bodies {
		for name, call := range resultCalls() {
			t.Run(b.name+"/"+name, func(t *testing.T) {
				err := call(t.Context(), newStaticClient(t, b.status, b.body))
				ee, ok := errors.AsType[*EmptyResponseError](err)
				if !ok {
					t.Fatalf("expected EmptyResponseError, got %T: %v", err, err)
				}
				if ee.Err.StatusCode != b.status {
					t.Fatalf("StatusCode = %d; want %d", ee.Err.StatusCode, b.status)
				}
				if _, nf := errors.AsType[*NotFoundError](err); nf {
					t.Fatalf("a blank body must not read as NotFound: %v", err)
				}
				if _, base := errors.AsType[*Error](err); !base {
					t.Fatalf("EmptyResponseError must unwrap to *Error: %v", err)
				}
			})
		}
	}
}

func TestBlankSuccessBodyWithoutResult(t *testing.T) {
	c := newStaticClient(t, http.StatusOK, "")
	if err := DeleteRaw(t.Context(), c, "Tickets", 1); err != nil {
		t.Fatalf("DeleteRaw on a blank 200: %v", err)
	}
	if err := Delete[testEntity](t.Context(), c, 1); err != nil {
		t.Fatalf("Delete on a blank 200: %v", err)
	}
}

func TestSuccessBodyOverLimit(t *testing.T) {
	const limit = 64
	body := `{"itemId":1,"pad":"` + strings.Repeat("x", limit) + `"}`
	for name, call := range resultCalls() {
		t.Run(name, func(t *testing.T) {
			err := call(t.Context(), newStaticClient(t, http.StatusOK, body, WithMaxResponseBytes(limit)))
			tl, ok := errors.AsType[*ResponseTooLargeError](err)
			if !ok {
				t.Fatalf("expected ResponseTooLargeError, got %T: %v", err, err)
			}
			if tl.Limit != limit || tl.Err.StatusCode != http.StatusOK {
				t.Fatalf("Limit = %d, StatusCode = %d; want %d, 200", tl.Limit, tl.Err.StatusCode, limit)
			}
		})
	}
}

func TestSuccessBodyAtLimit(t *testing.T) {
	body := `{"itemId":123}`
	c := newStaticClient(t, http.StatusOK, body, WithMaxResponseBytes(int64(len(body))))
	got, err := CreateRaw(t.Context(), c, "Tickets", map[string]any{"a": 1})
	if err != nil {
		t.Fatalf("body of exactly the limit: %v", err)
	}
	if got["itemId"] != float64(123) {
		t.Fatalf("itemId = %v; want 123", got["itemId"])
	}
}

// A limit of math.MaxInt64 turns the cap off; the extra byte readLimited asks
// for must not overflow into a negative read limit.
func TestMaxInt64LimitReadsBody(t *testing.T) {
	c := newStaticClient(t, http.StatusOK, `{"itemId":123}`, WithMaxResponseBytes(math.MaxInt64))
	got, err := CreateRaw(t.Context(), c, "Tickets", map[string]any{"a": 1})
	if err != nil {
		t.Fatalf("CreateRaw with a MaxInt64 limit: %v", err)
	}
	if got["itemId"] != float64(123) {
		t.Fatalf("itemId = %v; want 123", got["itemId"])
	}
}

func TestErrorBodyOverLimitKeepsStatusType(t *testing.T) {
	body := `{"errors":["` + strings.Repeat("x", 128) + `"]}`
	t.Run("503", func(t *testing.T) {
		err := DeleteRaw(t.Context(), newStaticClient(t, http.StatusServiceUnavailable, body, WithMaxResponseBytes(32)), "Tickets", 1)
		if _, ok := errors.AsType[*ServerError](err); !ok {
			t.Fatalf("expected ServerError, got %T: %v", err, err)
		}
		if tl, ok := errors.AsType[*ResponseTooLargeError](err); !ok || tl.Limit != 32 {
			t.Fatalf("size error not reachable: %v", err)
		}
	})
	t.Run("429 keeps RetryAfter", func(t *testing.T) {
		err := DeleteRaw(t.Context(), newStaticClient(t, http.StatusTooManyRequests, body, WithMaxResponseBytes(32)), "Tickets", 1)
		rl, ok := errors.AsType[*RateLimitError](err)
		if !ok || rl.RetryAfter != 7*time.Second {
			t.Fatalf("expected RateLimitError with RetryAfter 7s, got %T: %v", err, err)
		}
		if _, ok := errors.AsType[*ResponseTooLargeError](err); !ok {
			t.Fatalf("size error not reachable: %v", err)
		}
	})
}

// countingReader counts the bytes read from it.
type countingReader struct {
	r io.Reader
	n int
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += n
	return n, err
}

func TestParseResponseStopsReadingAtLimit(t *testing.T) {
	const limit = 1024
	src := &countingReader{r: strings.NewReader(strings.Repeat(" ", 1<<20))}
	resp := &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(src)}
	var out map[string]any
	if _, ok := errors.AsType[*ResponseTooLargeError](parseResponse(resp, &out, limit)); !ok {
		t.Fatal("expected ResponseTooLargeError")
	}
	if src.n > limit+1 {
		t.Fatalf("read %d bytes; want at most %d", src.n, limit+1)
	}
}

func TestWithMaxResponseBytesIgnoresNonPositive(t *testing.T) {
	for _, n := range []int64{0, -1} {
		c := newStaticClient(t, http.StatusOK, "{}", WithMaxResponseBytes(n))
		if c.maxResponseBytes != defaultMaxResponseBytes {
			t.Fatalf("WithMaxResponseBytes(%d): limit = %d; want default %d", n, c.maxResponseBytes, defaultMaxResponseBytes)
		}
	}
}

// rawZoneServer answers the version step with versionBody and the zone step with
// zoneBody.
func rawZoneServer(t *testing.T, versionBody, zoneBody string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/versioninformation") {
			_, _ = w.Write([]byte(versionBody))
			return
		}
		_, _ = w.Write([]byte(zoneBody))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestZoneDiscoveryBodyChecks(t *testing.T) {
	const versions = `{"apiVersions":["V1.0"]}`
	zone := `{"zoneName":"z","url":"https://example.invalid/","webUrl":"w","ci":1}`
	tests := []struct {
		name       string
		version    string
		zone       string
		limit      int64
		wantEmpty  bool
		wantTooBig bool
	}{
		{name: "blank version body", version: "", zone: zone, wantEmpty: true},
		{name: "blank zone body", version: versions, zone: " ", wantEmpty: true},
		{name: "version body over limit", version: versions, zone: zone, limit: 8, wantTooBig: true},
		{name: "zone body over limit", version: versions, zone: zone, limit: int64(len(versions)), wantTooBig: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := rawZoneServer(t, tt.version, tt.zone)
			opts := []ClientOption{WithZoneBaseURL(srv.URL), WithHTTPClient(srv.Client())}
			if tt.limit > 0 {
				opts = append(opts, WithMaxResponseBytes(tt.limit))
			}
			_, err := NewClient(t.Context(), AuthConfig{Username: "u@example.com"}, opts...)
			if _, ok := errors.AsType[*EmptyResponseError](err); ok != tt.wantEmpty {
				t.Fatalf("EmptyResponseError = %v; want %v (err %v)", ok, tt.wantEmpty, err)
			}
			if _, ok := errors.AsType[*ResponseTooLargeError](err); ok != tt.wantTooBig {
				t.Fatalf("ResponseTooLargeError = %v; want %v (err %v)", ok, tt.wantTooBig, err)
			}
		})
	}
}
