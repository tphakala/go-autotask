package autotasktest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
)

func (ts *TestServer) handleQuery(w http.ResponseWriter, r *http.Request) {
	store, ok := ts.getStore(parsePath(strings.TrimSuffix(r.URL.Path, "/query")).entity)
	if !ok {
		writeJSON(w, map[string]any{
			"items":       []any{},
			"pageDetails": map[string]any{"count": 0, "requestCount": ts.opts.pageSize},
		})
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, []string{"Failed to read request body"})
		return
	}

	// Validate filter structure if body is present.
	if len(body) > 0 {
		if err := validateFilter(body); err != nil {
			writeErrorResponse(w, http.StatusBadRequest, []string{err.Error()})
			return
		}
	}

	// Get all matching items (for simplicity, return all items from store).
	// In-memory filter evaluation applies basic matching.
	allItems := store.all()
	matched := filterItems(allItems, body)

	// Pagination is governed solely by pageSize. The query's MaxRecords is
	// validated for range (see validate.go) but intentionally NOT applied to the
	// returned set: the client caps to MaxRecords itself, and the List/ListRaw
	// MaxRecords tests rely on this mock returning the full page so they exercise
	// the client-side cap. Do not add server-side MaxRecords capping here.
	ts.writePage(w, r, matched)
}

func (ts *TestServer) handleCount(w http.ResponseWriter, r *http.Request) {
	store, ok := ts.getStore(parsePath(strings.TrimSuffix(r.URL.Path, "/query/count")).entity)
	if !ok {
		writeJSON(w, map[string]any{"queryCount": 0})
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, []string{"Failed to read request body"})
		return
	}

	if len(body) > 0 {
		if err := validateFilter(body); err != nil {
			writeErrorResponse(w, http.StatusBadRequest, []string{err.Error()})
			return
		}
	}

	matched := filterItems(store.all(), body)
	writeJSON(w, map[string]any{"queryCount": len(matched)})
}

// filterItems applies basic in-memory filter matching.
// It evaluates top-level conditions for all standard Autotask operators
// (eq, noteq, gt, gte, lt, lte, beginsWith, endsWith, contains, exist, notExist, in, notIn).
// Numbers compare by exact value and other values as text. This is not a full
// Autotask query engine: group conditions (and/or) are not evaluated.
func filterItems(items []json.RawMessage, queryBody []byte) []json.RawMessage {
	if len(queryBody) == 0 {
		return items
	}

	var query struct {
		Filter []json.RawMessage `json:"filter"`
	}
	if err := json.Unmarshal(queryBody, &query); err != nil || len(query.Filter) == 0 {
		return items
	}

	conditions := parseConditions(query.Filter)
	if len(conditions) == 0 {
		return items
	}

	result := []json.RawMessage{}
	for _, item := range items {
		m, err := decodeObject(item)
		if err != nil {
			continue
		}
		if matchesAll(m, conditions) {
			result = append(result, item)
		}
	}
	return result
}

type filterCond struct {
	field string
	op    string
	value any
}

func parseConditions(raw []json.RawMessage) []filterCond {
	var result []filterCond
	for _, r := range raw {
		var c struct {
			Field string `json:"field"`
			Op    string `json:"op"`
			Value any    `json:"value"`
		}
		dec := json.NewDecoder(bytes.NewReader(r))
		dec.UseNumber()
		if err := dec.Decode(&c); err != nil || c.Field == "" {
			continue // skip group conditions for now
		}
		result = append(result, filterCond{field: c.Field, op: c.Op, value: c.Value})
	}
	return result
}

func matchesAll(m map[string]any, conditions []filterCond) bool {
	for _, c := range conditions {
		if !matchesCond(m, c) {
			return false
		}
	}
	return true
}

func matchesCond(m map[string]any, c filterCond) bool {
	val, exists := m[c.field]
	switch c.op {
	case "exist":
		return exists && val != nil
	case "notExist":
		return !exists || val == nil
	case "eq":
		return valuesEqual(val, c.value)
	case "noteq":
		return !valuesEqual(val, c.value)
	case "in":
		return inSlice(val, c.value)
	case "notIn":
		return !inSlice(val, c.value)
	default:
		return matchesStringOp(val, c) || matchesNumericOp(val, c)
	}
}

// matchesStringOp handles string comparison operators.
func matchesStringOp(val any, c filterCond) bool {
	switch c.op {
	case "contains":
		return strings.Contains(fmt.Sprintf("%v", val), fmt.Sprintf("%v", c.value))
	case "beginsWith":
		return strings.HasPrefix(fmt.Sprintf("%v", val), fmt.Sprintf("%v", c.value))
	case "endsWith":
		return strings.HasSuffix(fmt.Sprintf("%v", val), fmt.Sprintf("%v", c.value))
	default:
		return false
	}
}

// matchesNumericOp handles numeric comparison operators. A value that is not a
// number matches none of them.
func matchesNumericOp(val any, c filterCond) bool {
	cmp, ok := compareNumbers(val, c.value)
	if !ok {
		return false
	}
	switch c.op {
	case "gt":
		return cmp > 0
	case "gte":
		return cmp >= 0
	case "lt":
		return cmp < 0
	case "lte":
		return cmp <= 0
	default:
		return false
	}
}

// numberValue returns v as an exact rational when it is a JSON number.
func numberValue(v any) (*big.Rat, bool) {
	n, ok := v.(json.Number)
	if !ok {
		return nil, false
	}
	return new(big.Rat).SetString(n.String())
}

// compareNumbers orders two JSON numbers by exact value. It reports false when
// either is not a number.
func compareNumbers(a, b any) (int, bool) {
	x, ok := numberValue(a)
	if !ok {
		return 0, false
	}
	y, ok := numberValue(b)
	if !ok {
		return 0, false
	}
	return x.Cmp(y), true
}

// valuesEqual compares two numbers by exact value, so 1 equals 1.0, and any
// other pair as text.
func valuesEqual(a, b any) bool {
	if cmp, ok := compareNumbers(a, b); ok {
		return cmp == 0
	}
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

func inSlice(val, sliceVal any) bool {
	arr, ok := sliceVal.([]any)
	if !ok {
		return false
	}
	for _, item := range arr {
		if valuesEqual(val, item) {
			return true
		}
	}
	return false
}
