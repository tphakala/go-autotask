package autotasktest

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
)

func (ts *TestServer) handleQuery(w http.ResponseWriter, r *http.Request) {
	entityName, _, _ := parseEntityAndID(strings.TrimSuffix(r.URL.Path, "/query"))
	store, ok := ts.getStore(entityName)
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

	// Determine page from query param.
	pageNum := 1
	if p := r.URL.Query().Get("page"); p != "" {
		if n, err := strconv.Atoi(p); err == nil && n > 0 {
			pageNum = n
		}
	}

	pageSize := ts.opts.pageSize
	start := (pageNum - 1) * pageSize
	start = min(start, len(matched))
	end := start + pageSize
	end = min(end, len(matched))
	page := matched[start:end]

	var nextPageURL any
	if end < len(matched) {
		nextPageURL = fmt.Sprintf("%s%s?page=%d", ts.URL, r.URL.Path, pageNum+1)
	}
	var prevPageURL any
	if pageNum > 1 {
		prevPageURL = fmt.Sprintf("%s%s?page=%d", ts.URL, r.URL.Path, pageNum-1)
	}

	writeJSON(w, map[string]any{
		"items": page,
		"pageDetails": map[string]any{
			"count":        len(page),
			"requestCount": ts.opts.pageSize,
			"prevPageUrl":  prevPageURL,
			"nextPageUrl":  nextPageURL,
		},
	})
}

func (ts *TestServer) handleCount(w http.ResponseWriter, r *http.Request) {
	entityName, _, _ := parseEntityAndID(strings.TrimSuffix(r.URL.Path, "/query/count"))
	store, ok := ts.getStore(entityName)
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
// For simplicity, it evaluates top-level "eq" and "contains" conditions.
// This is not a full Autotask query engine — just enough for test validation.
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

	var result []json.RawMessage
	for _, item := range items {
		var m map[string]any
		if err := json.Unmarshal(item, &m); err != nil {
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
		if err := json.Unmarshal(r, &c); err != nil || c.Field == "" {
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
		return fmt.Sprintf("%v", val) == fmt.Sprintf("%v", c.value)
	case "noteq":
		return fmt.Sprintf("%v", val) != fmt.Sprintf("%v", c.value)
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

// matchesNumericOp handles numeric comparison operators.
func matchesNumericOp(val any, c filterCond) bool {
	switch c.op {
	case "gt":
		return toFloat(val) > toFloat(c.value)
	case "gte":
		return toFloat(val) >= toFloat(c.value)
	case "lt":
		return toFloat(val) < toFloat(c.value)
	case "lte":
		return toFloat(val) <= toFloat(c.value)
	default:
		return false
	}
}

func toFloat(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	case int64:
		return float64(n)
	case json.Number:
		f, _ := n.Float64()
		return f
	}
	return math.NaN()
}

func inSlice(val, sliceVal any) bool {
	arr, ok := sliceVal.([]any)
	if !ok {
		return false
	}
	s := fmt.Sprintf("%v", val)
	for _, item := range arr {
		if fmt.Sprintf("%v", item) == s {
			return true
		}
	}
	return false
}
