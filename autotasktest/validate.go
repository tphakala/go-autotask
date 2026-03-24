package autotasktest

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// maxRecordsLimit is the Autotask API maximum for the MaxRecords query parameter.
// This mirrors the unexported constant in the main autotask package.
const maxRecordsLimit = 500

// validOperators is the set of valid Autotask query filter operators.
var validOperators = map[string]bool{
	"eq": true, "noteq": true,
	"gt": true, "gte": true, "lt": true, "lte": true,
	"beginsWith": true, "endsWith": true, "contains": true,
	"exist": true, "notExist": true,
	"in": true, "notIn": true,
}

// noValueOperators don't require a "value" field.
var noValueOperators = map[string]bool{
	"exist": true, "notExist": true,
}

// groupOperators can contain nested "items".
var groupOperators = map[string]bool{
	"and": true, "or": true,
}

func (ts *TestServer) validateAuth(r *http.Request) error {
	if got := r.Header.Get("UserName"); got != ts.auth.username {
		return fmt.Errorf("username header mismatch: got %q", got)
	}
	if got := r.Header.Get("Secret"); got != ts.auth.secret {
		return fmt.Errorf("secret header mismatch")
	}
	if got := r.Header.Get("ApiIntegrationCode"); got != ts.auth.integrationCode {
		return fmt.Errorf("integration code header mismatch: got %q", got)
	}
	return nil
}

// validateFilter checks that a query filter JSON body is well-formed per the Autotask API spec.
func validateFilter(body []byte) error {
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(body, &payload); err != nil {
		return fmt.Errorf("filter body is not valid JSON: %w", err)
	}
	filterRaw, ok := payload["filter"]
	if !ok {
		return fmt.Errorf("missing required 'filter' field in query body")
	}
	var conditions []json.RawMessage
	if err := json.Unmarshal(filterRaw, &conditions); err != nil {
		return fmt.Errorf("'filter' must be an array: %w", err)
	}
	for _, cond := range conditions {
		if err := validateCondition(cond); err != nil {
			return err
		}
	}
	// Validate IncludeFields if present (the library sends this as a JSON array).
	if fieldsRaw, ok := payload["IncludeFields"]; ok {
		var fields []string
		if err := json.Unmarshal(fieldsRaw, &fields); err != nil {
			return fmt.Errorf("IncludeFields must be a JSON array of strings: %w", err)
		}
	}
	// Validate MaxRecords if present.
	if maxRaw, ok := payload["MaxRecords"]; ok {
		var maxRecords int
		if err := json.Unmarshal(maxRaw, &maxRecords); err != nil {
			return fmt.Errorf("MaxRecords must be an integer: %w", err)
		}
		if maxRecords < 1 || maxRecords > maxRecordsLimit {
			return fmt.Errorf("MaxRecords must be 1-%d, got %d", maxRecordsLimit, maxRecords)
		}
	}
	return nil
}

func validateCondition(raw json.RawMessage) error {
	var cond map[string]json.RawMessage
	if err := json.Unmarshal(raw, &cond); err != nil {
		return fmt.Errorf("condition must be a JSON object: %w", err)
	}
	opRaw, ok := cond["op"]
	if !ok {
		return fmt.Errorf("condition missing required 'op' field")
	}
	var op string
	if err := json.Unmarshal(opRaw, &op); err != nil {
		return fmt.Errorf("'op' must be a string: %w", err)
	}

	// Group operators (and/or).
	if groupOperators[op] {
		itemsRaw, ok := cond["items"]
		if !ok {
			return fmt.Errorf("group operator %q requires 'items' array", op)
		}
		var items []json.RawMessage
		if err := json.Unmarshal(itemsRaw, &items); err != nil {
			return fmt.Errorf("'items' must be an array: %w", err)
		}
		for _, item := range items {
			if err := validateCondition(item); err != nil {
				return err
			}
		}
		return nil
	}

	// Field operators.
	if !validOperators[op] {
		return fmt.Errorf("invalid operator %q", op)
	}
	if _, ok := cond["field"]; !ok {
		return fmt.Errorf("condition with op %q missing required 'field'", op)
	}
	if !noValueOperators[op] {
		if _, ok := cond["value"]; !ok {
			return fmt.Errorf("condition with op %q requires 'value' field", op)
		}
	}
	return nil
}
