package autotask_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	autotask "github.com/tphakala/go-autotask"
	"github.com/tphakala/go-autotask/entities"
	"github.com/tphakala/go-autotask/middleware"
)

func testAuth() autotask.AuthConfig {
	return autotask.AuthConfig{Username: "test@example.com", Secret: "secret", IntegrationCode: "code"}
}

// newAnswerClient returns a client whose every request is answered by handler.
func newAnswerClient(t *testing.T, handler http.HandlerFunc, opts ...autotask.ClientOption) *autotask.Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	opts = append([]autotask.ClientOption{autotask.WithBaseURL(srv.URL)}, opts...)
	client, err := autotask.NewClient(t.Context(), testAuth(), opts...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func answer(status int, contentType, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		if contentType != "" {
			w.Header().Set("Content-Type", contentType)
		}
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}
}

const validation500Body = `{"errors":["String value exceeds maximum length (field:description)"]}`

func TestIsTransientResponses(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		handler http.HandlerFunc
		opts    []autotask.ClientOption
		want    bool
	}{
		{"400", answer(400, "application/json", `{"errors":["bad"]}`), nil, false},
		{"401", answer(401, "", ""), nil, false},
		{"403", answer(403, "", ""), nil, false},
		{"404", answer(404, "", ""), nil, false},
		{"409", answer(409, "", ""), nil, false},
		{"422", answer(422, "", ""), nil, false},
		{"429", answer(429, "", ""), nil, true},
		{"500 validation", answer(500, "application/json", validation500Body), nil, false},
		{"500 unrelated message", answer(500, "application/json", `{"errors":["database is down"]}`), nil, true},
		{"500 no body", answer(500, "", ""), nil, true},
		{"503", answer(503, "", ""), nil, true},
		{"200 maintenance page", answer(200, "text/html", "<html>maintenance</html>"), nil, true},
		{"200 blank body", answer(200, "application/json", "  "), nil, true},
		{"200 over the size limit", answer(200, "application/json", `{"item":{"id":1}}`), []autotask.ClientOption{autotask.WithMaxResponseBytes(4)}, false},
		{"503 over the size limit keeps its status", answer(503, "application/json", `{"errors":["maintenance window"]}`), []autotask.ClientOption{autotask.WithMaxResponseBytes(4)}, true},
		{"400 over the size limit keeps its status", answer(400, "application/json", `{"errors":["bad request body"]}`), []autotask.ClientOption{autotask.WithMaxResponseBytes(4)}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			client := newAnswerClient(t, tt.handler, tt.opts...)
			_, err := autotask.Get[entities.Company](t.Context(), client, 1)
			if err == nil {
				t.Fatal("expected an error")
			}
			if got := autotask.IsTransient(err); got != tt.want {
				t.Fatalf("IsTransient(%T: %v) = %v; want %v", err, err, got, tt.want)
			}
		})
	}
}

func TestIsTransientNullItem(t *testing.T) {
	t.Parallel()
	client := newAnswerClient(t, answer(200, "application/json", `{"item":null}`))
	_, err := autotask.Get[entities.Company](t.Context(), client, 1)
	assertItemNotFound(t, err)
	if autotask.IsTransient(err) {
		t.Fatal("a null item is a permanent NotFound")
	}
}

func TestIsTransientValidation500SameTypeAsServerFailure(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		body       string
		validation bool
	}{
		{validation500Body, true},
		{`{"errors":[{"message":"Value EXCEEDS MAXIMUM LENGTH","field":"description"}]}`, true},
		{`{"errors":["database is down"]}`, false},
		{``, false},
	} {
		client := newAnswerClient(t, answer(500, "application/json", tt.body))
		_, err := autotask.Get[entities.Company](t.Context(), client, 1)
		se, ok := errors.AsType[*autotask.ServerError](err)
		if !ok {
			t.Fatalf("got %T: %v; want *ServerError", err, err)
		}
		if got := se.IsValidation(); got != tt.validation {
			t.Errorf("IsValidation(%q) = %v; want %v", tt.body, got, tt.validation)
		}
	}
}

func TestServerErrorIsValidationRequiresStatus500(t *testing.T) {
	t.Parallel()
	msg := []autotask.APIError{{Message: "exceeds maximum length"}}
	if (&autotask.ServerError{Err: autotask.Error{StatusCode: 503, Errors: msg}}).IsValidation() {
		t.Error("a 503 is never a validation failure")
	}
	if !(&autotask.ServerError{Err: autotask.Error{StatusCode: 500, Errors: msg}}).IsValidation() {
		t.Error("a 500 with a validation message is one")
	}
	var nilErr *autotask.ServerError
	if nilErr.IsValidation() {
		t.Error("nil receiver must be false")
	}
}

func TestIsTransientCallerErrors(t *testing.T) {
	t.Parallel()
	client := newAnswerClient(t, func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	})

	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := autotask.Get[entities.Company](canceled, client, 1)
	if err == nil || autotask.IsTransient(err) {
		t.Fatalf("cancelled context: IsTransient(%v) must be false", err)
	}

	expired, stop := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer stop()
	_, err = autotask.Get[entities.Company](expired, client, 1)
	if err == nil || !autotask.IsTransient(err) {
		t.Fatalf("deadline exceeded: IsTransient(%v) must be true", err)
	}
}

func TestIsTransientConnectionRefused(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	client, err := autotask.NewClient(t.Context(), testAuth(), autotask.WithBaseURL(url))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	srv.Close()
	_, err = autotask.Get[entities.Company](t.Context(), client, 1)
	if err == nil || !autotask.IsTransient(err) {
		t.Fatalf("IsTransient(%v) = false; want true for a refused connection", err)
	}
}

func TestIsTransientTruncatedBody(t *testing.T) {
	t.Parallel()
	client := newAnswerClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "100")
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"item":`)
	})
	_, err := autotask.Get[entities.Company](t.Context(), client, 1)
	if err == nil || !autotask.IsTransient(err) {
		t.Fatalf("IsTransient(%v) = false; want true for a body cut short", err)
	}
}

func TestIsTransientCircuitBreakerOpen(t *testing.T) {
	t.Parallel()
	client := newAnswerClient(t, answer(503, "", ""),
		autotask.WithCircuitBreaker(middleware.WithFailureThreshold(1)))
	_, _ = autotask.Get[entities.Company](t.Context(), client, 1)
	_, err := autotask.Get[entities.Company](t.Context(), client, 1)
	if _, ok := errors.AsType[*middleware.CircuitBreakerOpenError](err); !ok {
		t.Fatalf("got %T: %v; want the breaker open", err, err)
	}
	if !autotask.IsTransient(err) {
		t.Fatal("an open breaker is transient")
	}
}

func TestIsTransientBreakerIgnoresValidation500(t *testing.T) {
	t.Parallel()
	client := newAnswerClient(t, answer(500, "application/json", validation500Body),
		autotask.WithCircuitBreaker(middleware.WithFailureThreshold(2)))
	for range 5 {
		_, err := autotask.Get[entities.Company](t.Context(), client, 1)
		if _, open := errors.AsType[*middleware.CircuitBreakerOpenError](err); open {
			t.Fatal("validation 500s must not open the breaker")
		}
	}
}

func TestIsTransientPagesAndSentinels(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"unknown", errors.New("something else"), false},
		{"wrapped rate limit", fmt.Errorf("wrap: %w", &autotask.RateLimitError{}), true},
		{"max pages", &autotask.MaxPagesExceededError{EntityName: "Tickets", MaxPages: 3}, false},
		{"unexpected eof", fmt.Errorf("autotask: reading response body: %w", io.ErrUnexpectedEOF), true},
		{"dns", &net.DNSError{Err: "no such host", Name: "x"}, true},
		{"untrusted certificate is not a network blip", errors.New("tls: failed to verify certificate"), false},
		{"validation error then eof", errors.Join(&autotask.ValidationError{}, io.ErrUnexpectedEOF), false},
		{"cancel beats deadline", errors.Join(context.Canceled, context.DeadlineExceeded), false},
		{"503 whose body read was cancelled keeps its status", errors.Join(&autotask.ServerError{Err: autotask.Error{StatusCode: 503}}, context.Canceled), true},
		{"429 whose body read was cancelled keeps its status", errors.Join(&autotask.RateLimitError{Err: autotask.Error{StatusCode: 429}}, context.Canceled), true},
		{"validation 500 whose body read was cancelled stays permanent", errors.Join(&autotask.ServerError{Err: autotask.Error{StatusCode: 500, Errors: []autotask.APIError{{Message: "exceeds maximum length"}}}}, context.Canceled), false},
		{"400 whose body read was cancelled stays permanent", errors.Join(&autotask.ValidationError{Err: autotask.Error{StatusCode: 400}}, context.Canceled), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := autotask.IsTransient(tt.err); got != tt.want {
				t.Fatalf("IsTransient(%v) = %v; want %v", tt.err, got, tt.want)
			}
		})
	}
}

func zoneServer(t *testing.T, versions, zone http.HandlerFunc) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /atservicesrest/versioninformation", versions)
	mux.HandleFunc("GET /atservicesrest/V1.0/zoneInformation", zone)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestIsTransientZoneDiscovery(t *testing.T) {
	t.Parallel()
	okVersions := answer(200, "application/json", `{"apiVersions":["V1.0"]}`)
	tests := []struct {
		name     string
		versions http.HandlerFunc
		zone     http.HandlerFunc
		want     bool
	}{
		{"version step 503", answer(503, "", ""), nil, true},
		{"version step 429", answer(429, "", ""), nil, true},
		{"version step 404", answer(404, "", ""), nil, false},
		{"zone step 502", okVersions, answer(502, "", ""), true},
		{"zone step 401", okVersions, answer(401, "", ""), false},
		{"maintenance page", okVersions, answer(200, "text/html", "<html>"), true},
		{"no supported version", answer(200, "application/json", `{"apiVersions":["V9.0"]}`), nil, false},
		{"empty url", okVersions, answer(200, "application/json", `{"url":""}`), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			zone := tt.zone
			if zone == nil {
				zone = answer(500, "", "")
			}
			srv := zoneServer(t, tt.versions, zone)
			_, err := autotask.NewClient(t.Context(), testAuth(), autotask.WithZoneBaseURL(srv.URL))
			if err == nil {
				t.Fatal("expected a discovery error")
			}
			if got := autotask.IsTransient(err); got != tt.want {
				t.Fatalf("IsTransient(%v) = %v; want %v", err, got, tt.want)
			}
		})
	}
}

func TestZoneDiscoveryStatusErrorKeepsMessageAndType(t *testing.T) {
	t.Parallel()
	srv := zoneServer(t, answer(503, "", ""), answer(200, "", ""))
	_, err := autotask.NewClient(t.Context(), testAuth(), autotask.WithZoneBaseURL(srv.URL))
	if err == nil || !strings.Contains(err.Error(), "version request returned 503") {
		t.Fatalf("err = %v; want the version step status in the message", err)
	}
	if _, ok := errors.AsType[*autotask.ServerError](err); !ok {
		t.Fatalf("err = %T; want *ServerError reachable with errors.As", err)
	}
}

type timeoutError struct{}

func (timeoutError) Error() string   { return "i/o timeout" }
func (timeoutError) Timeout() bool   { return true }
func (timeoutError) Temporary() bool { return false }

func TestIsTransientNetErrorTimeout(t *testing.T) {
	t.Parallel()
	if !autotask.IsTransient(fmt.Errorf("autotask: request failed: %w", timeoutError{})) {
		t.Fatal("a net.Error timeout is transient")
	}
}
