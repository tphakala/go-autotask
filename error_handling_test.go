package autotask_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	autotask "github.com/tphakala/go-autotask"
	"github.com/tphakala/go-autotask/autotasktest"
	"github.com/tphakala/go-autotask/entities"
)

func TestErrorAuthentication(t *testing.T) {
	t.Parallel()
	company := autotasktest.CompanyFixture()
	_, client := autotasktest.NewServer(t,
		autotasktest.WithEntity(company),
		autotasktest.WithErrorOn("GET", "Companies/1", http.StatusUnauthorized, []string{"Invalid credentials"}),
	)
	_, err := autotask.Get[entities.Company](t.Context(), client, 1)
	if err == nil {
		t.Fatal("expected error")
	}
	if _, ok := errors.AsType[*autotask.AuthenticationError](err); !ok {
		t.Fatalf("expected AuthenticationError, got %T: %v", err, err)
	}
}

func TestErrorNotFound(t *testing.T) {
	t.Parallel()
	company := autotasktest.CompanyFixture()
	_, client := autotasktest.NewServer(t,
		autotasktest.WithEntity(company),
	)
	_, err := autotask.Get[entities.Company](t.Context(), client, 99999)
	assertItemNotFound(t, err)
	_, err = autotask.GetRaw(t.Context(), client, "Companies", 99999)
	assertItemNotFound(t, err)
}

// assertItemNotFound checks for the NotFoundError the client builds from a
// 200 response with a null item, which is how the mock answers a missing id.
func assertItemNotFound(t *testing.T, err error) {
	t.Helper()
	nf, ok := errors.AsType[*autotask.NotFoundError](err)
	if !ok {
		t.Fatalf("expected NotFoundError, got %T: %v", err, err)
	}
	if nf.Err.StatusCode != http.StatusOK {
		t.Fatalf("StatusCode = %d; want %d (null item, not a 404)", nf.Err.StatusCode, http.StatusOK)
	}
}

func TestErrorNotFound404(t *testing.T) {
	t.Parallel()
	_, client := autotasktest.NewServer(t,
		autotasktest.WithEntity(autotasktest.CompanyFixture()),
		autotasktest.WithErrorOn("GET", "Companies/99999", http.StatusNotFound, []string{"not found"}),
	)
	_, getErr := autotask.Get[entities.Company](t.Context(), client, 99999)
	_, rawErr := autotask.GetRaw(t.Context(), client, "Companies", 99999)
	for name, err := range map[string]error{"Get": getErr, "GetRaw": rawErr} {
		nf, ok := errors.AsType[*autotask.NotFoundError](err)
		if !ok {
			t.Fatalf("%s: expected NotFoundError, got %T: %v", name, err, err)
		}
		if nf.Err.StatusCode != http.StatusNotFound {
			t.Fatalf("%s: StatusCode = %d; want %d", name, nf.Err.StatusCode, http.StatusNotFound)
		}
	}
}

func TestUpdateRawReturnsItemID(t *testing.T) {
	t.Parallel()
	company := autotasktest.CompanyFixture()
	_, client := autotasktest.NewServer(t,
		autotasktest.WithEntity(company),
	)
	id, _ := company.ID.Get()
	result, err := autotask.UpdateRaw(t.Context(), client, "Companies",
		map[string]any{"id": id, "companyName": "Renamed"})
	if err != nil {
		t.Fatal(err)
	}
	if result["itemId"] != float64(id) {
		t.Fatalf("itemId = %v; want %d (result %v)", result["itemId"], id, result)
	}
}

func TestErrorRateLimit(t *testing.T) {
	t.Parallel()
	company := autotasktest.CompanyFixture()
	_, client := autotasktest.NewServer(t,
		autotasktest.WithEntity(company),
		autotasktest.WithRetryAfterError("Companies/1", 30),
	)
	_, err := autotask.Get[entities.Company](t.Context(), client, 1)
	if err == nil {
		t.Fatal("expected error")
	}
	rlErr, ok := errors.AsType[*autotask.RateLimitError](err)
	if !ok {
		t.Fatalf("expected RateLimitError, got %T: %v", err, err)
	}
	if rlErr.RetryAfter != 30*time.Second {
		t.Fatalf("RetryAfter = %v, want 30s", rlErr.RetryAfter)
	}
}

func TestErrorValidation(t *testing.T) {
	t.Parallel()
	_, client := autotasktest.NewServer(t,
		autotasktest.WithRequiredFields("Companies", "companyName"),
	)
	// Create a company without the required companyName field.
	company := &entities.Company{
		CompanyType: autotask.Set(int64(1)),
	}
	_, err := autotask.Create[entities.Company](t.Context(), client, company)
	if err == nil {
		t.Fatal("expected error")
	}
	if _, ok := errors.AsType[*autotask.ValidationError](err); !ok {
		t.Fatalf("expected ValidationError, got %T: %v", err, err)
	}
}

func TestErrorServerError(t *testing.T) {
	t.Parallel()
	company := autotasktest.CompanyFixture()
	_, client := autotasktest.NewServer(t,
		autotasktest.WithEntity(company),
		autotasktest.WithErrorOn("GET", "Companies/1", http.StatusInternalServerError, []string{"Internal server error"}),
	)
	_, err := autotask.Get[entities.Company](t.Context(), client, 1)
	if err == nil {
		t.Fatal("expected error")
	}
	if _, ok := errors.AsType[*autotask.ServerError](err); !ok {
		t.Fatalf("expected ServerError, got %T: %v", err, err)
	}
}

func TestErrorMultipleMessages(t *testing.T) {
	t.Parallel()
	client := autotasktest.NewMockClient(t,
		autotasktest.WithFixture("GET", "/v1.0/Companies/1", http.StatusBadRequest,
			json.RawMessage(`{"errors":["msg1","msg2"]}`)),
	)
	_, err := autotask.Get[entities.Company](t.Context(), client, 1)
	if err == nil {
		t.Fatal("expected error")
	}
	errStr := err.Error()
	if !strings.Contains(errStr, "msg1") {
		t.Fatalf("error should contain 'msg1': %v", err)
	}
	// Verify msg2 is also captured in the parsed errors.
	vErr, ok := errors.AsType[*autotask.ValidationError](err)
	if !ok {
		t.Fatalf("expected ValidationError for 400 response, got %T: %v", err, err)
	}
	found := false
	for _, ae := range vErr.Err.Errors {
		if ae.Message == "msg2" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected 'msg2' in error list, got errors: %+v", vErr.Err.Errors)
	}
}

func TestErrorMixedPayloadFormat(t *testing.T) {
	t.Parallel()
	client := autotasktest.NewMockClient(t,
		autotasktest.WithFixture("GET", "/v1.0/Companies/1", http.StatusBadRequest,
			json.RawMessage(`{"errors":[{"message":"structured error"},"plain string error"]}`)),
	)
	_, err := autotask.Get[entities.Company](t.Context(), client, 1)
	if err == nil {
		t.Fatal("expected error")
	}
	errStr := err.Error()
	if !strings.Contains(errStr, "structured error") {
		t.Fatalf("error should contain 'structured error': %v", err)
	}
	// Verify the plain string error is also captured.
	vErr, ok := errors.AsType[*autotask.ValidationError](err)
	if !ok {
		t.Fatalf("expected ValidationError for 400 response, got %T: %v", err, err)
	}
	found := false
	for _, ae := range vErr.Err.Errors {
		if ae.Message == "plain string error" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected 'plain string error' in error list, got errors: %+v", vErr.Err.Errors)
	}
}

func TestErrorMalformedJSON(t *testing.T) {
	t.Parallel()
	client := autotasktest.NewMockClient(t,
		autotasktest.WithFixture("GET", "/v1.0/Companies/1", http.StatusBadRequest,
			json.RawMessage(`this is not valid json`)),
	)
	_, err := autotask.Get[entities.Company](t.Context(), client, 1)
	if err == nil {
		t.Fatal("expected error for malformed JSON response")
	}
}

func TestErrorEmptyBody(t *testing.T) {
	t.Parallel()
	client := autotasktest.NewMockClient(t,
		autotasktest.WithFixture("GET", "/v1.0/Companies/1", http.StatusOK, nil),
	)
	// A 200 response with an empty body should not panic.
	// Get expects {"item": ...}, so an empty body yields an unmarshaling error
	// because the client tries to unmarshal empty JSON into the item envelope.
	_, err := autotask.Get[entities.Company](t.Context(), client, 1)
	if err == nil {
		t.Fatal("expected error when 200 response has empty body (client cannot unmarshal empty JSON)")
	}
}

func TestErrorNetworkTimeout(t *testing.T) {
	t.Parallel()
	_, client := autotasktest.NewServer(t,
		autotasktest.WithEntity(autotasktest.CompanyFixture()),
		autotasktest.WithServerLatency(500*time.Millisecond),
	)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Millisecond)
	defer cancel()
	_, err := autotask.Get[entities.Company](ctx, client, 1)
	if err == nil {
		t.Fatal("expected error due to context timeout")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context.DeadlineExceeded, got %T: %v", err, err)
	}
}
