package autotask_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	autotask "github.com/tphakala/go-autotask"
	"github.com/tphakala/go-autotask/autotasktest"
	"github.com/tphakala/go-autotask/entities"
)

const (
	opAnd = "and"
	opOr  = "or"
)

// marshalQueryToMap is a test helper that marshals a query and returns a generic map.
func marshalQueryToMap(t *testing.T, q *autotask.Query) map[string]any {
	t.Helper()
	data, err := json.Marshal(q)
	if err != nil {
		t.Fatalf("failed to marshal query: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("failed to unmarshal query JSON: %v", err)
	}
	return m
}

// firstFilter extracts the first filter condition from a marshaled query map.
func firstFilter(t *testing.T, m map[string]any) map[string]any {
	t.Helper()
	filter, ok := m["filter"].([]any)
	if !ok {
		t.Fatalf("expected []any for filter, got %T", m["filter"])
	}
	if len(filter) == 0 {
		t.Fatal("filter array is empty")
	}
	entry, ok := filter[0].(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any for filter[0], got %T", filter[0])
	}
	return entry
}

// requireGroup asserts that m has the expected "op" and returns the "items" slice.
func requireGroup(t *testing.T, m map[string]any, wantOp, label string) []any {
	t.Helper()
	if m["op"] != wantOp {
		t.Fatalf("%s op = %v; want %v", label, m["op"], wantOp)
	}
	items, ok := m["items"].([]any)
	if !ok {
		t.Fatalf("%s: expected []any for items, got %T", label, m["items"])
	}
	return items
}

// requireMapAt asserts that items[index] is a map and returns it.
func requireMapAt(t *testing.T, items []any, index int, label string) map[string]any {
	t.Helper()
	if index >= len(items) {
		t.Fatalf("%s: index %d out of range (len=%d)", label, index, len(items))
	}
	m, ok := items[index].(map[string]any)
	if !ok {
		t.Fatalf("%s: expected map at index %d, got %T", label, index, items[index])
	}
	return m
}

func TestQueryFilterOperators(t *testing.T) {
	t.Parallel()
	company := autotasktest.CompanyFixture()
	_, client := autotasktest.NewServer(t, autotasktest.WithEntity(company))

	tests := []struct {
		name  string
		op    autotask.Operator
		value any
	}{
		{"Eq", autotask.OpEq, "test"},
		{"NotEq", autotask.OpNotEq, "test"},
		{"Gt", autotask.OpGt, 10},
		{"Gte", autotask.OpGte, 10},
		{"Lt", autotask.OpLt, 10},
		{"Lte", autotask.OpLte, 10},
		{"BeginsWith", autotask.OpBeginsWith, "Acme"},
		{"EndsWith", autotask.OpEndsWith, "Corp"},
		{"Contains", autotask.OpContains, "me"},
		{"Exist", autotask.OpExist, nil},
		{"NotExist", autotask.OpNotExist, nil},
		{"In", autotask.OpIn, []string{"A", "B"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			q := autotask.NewQuery().Where("companyName", tt.op, tt.value)

			// Verify JSON structure.
			m := marshalQueryToMap(t, q)
			entry := firstFilter(t, m)
			if entry["op"] != string(tt.op) {
				t.Fatalf("op = %v; want %v", entry["op"], tt.op)
			}
			if entry["field"] != "companyName" {
				t.Fatalf("field = %v; want companyName", entry["field"])
			}
			// For operators that take a value, verify it is present.
			if tt.op != autotask.OpExist && tt.op != autotask.OpNotExist {
				if _, ok := entry["value"]; !ok {
					t.Fatal("expected value field to be present")
				}
			}

			// Verify the mock server accepts the query without error.
			_, err := autotask.List[entities.Company](t.Context(), client, q)
			if err != nil {
				t.Fatalf("server rejected query with op %v: %v", tt.op, err)
			}
		})
	}
}

func TestQueryNestedGroups(t *testing.T) {
	t.Parallel()
	company := autotasktest.CompanyFixture()
	_, client := autotasktest.NewServer(t, autotasktest.WithEntity(company))

	t.Run("TwoLevels", func(t *testing.T) {
		t.Parallel()
		q := autotask.NewQuery().Or(
			autotask.And(
				autotask.Field("companyName", autotask.OpEq, "Acme"),
				autotask.Field("isActive", autotask.OpEq, true),
			),
			autotask.Field("companyType", autotask.OpEq, 2),
		)

		m := marshalQueryToMap(t, q)
		orGroup := firstFilter(t, m)
		items := requireGroup(t, orGroup, opOr, "top-level")
		if len(items) != 2 {
			t.Fatalf("OR items length = %d; want 2", len(items))
		}
		andGroup := requireMapAt(t, items, 0, "nested AND")
		andItems := requireGroup(t, andGroup, opAnd, "nested AND")
		if len(andItems) != 2 {
			t.Fatalf("AND items length = %d; want 2", len(andItems))
		}

		_, err := autotask.List[entities.Company](t.Context(), client, q)
		if err != nil {
			t.Fatalf("server rejected two-level nested query: %v", err)
		}
	})

	t.Run("ThreeLevels", func(t *testing.T) {
		t.Parallel()
		q := autotask.NewQuery().And(
			autotask.Or(
				autotask.And(
					autotask.Field("companyName", autotask.OpBeginsWith, "Acme"),
					autotask.Field("isActive", autotask.OpEq, true),
				),
				autotask.Field("companyType", autotask.OpEq, 2),
			),
			autotask.Field("country", autotask.OpEq, "US"),
		)

		m := marshalQueryToMap(t, q)
		topGroup := firstFilter(t, m)
		topItems := requireGroup(t, topGroup, opAnd, "top-level AND")
		if len(topItems) != 2 {
			t.Fatalf("top AND items length = %d; want 2", len(topItems))
		}

		orGroup := requireMapAt(t, topItems, 0, "second-level OR")
		orItems := requireGroup(t, orGroup, opOr, "second-level OR")

		innerAnd := requireMapAt(t, orItems, 0, "third-level AND")
		requireGroup(t, innerAnd, opAnd, "third-level AND")

		_, err := autotask.List[entities.Company](t.Context(), client, q)
		if err != nil {
			t.Fatalf("server rejected three-level nested query: %v", err)
		}
	})
}

func TestQueryUDFFilter(t *testing.T) {
	t.Parallel()
	company := autotasktest.CompanyFixture()
	_, client := autotasktest.NewServer(t, autotasktest.WithEntity(company))

	q := autotask.NewQuery().WhereUDF("CustomerRanking", autotask.OpEq, "Gold")

	m := marshalQueryToMap(t, q)
	entry := firstFilter(t, m)
	if entry["udf"] != true {
		t.Fatalf("udf = %v; want true", entry["udf"])
	}
	if entry["field"] != "CustomerRanking" {
		t.Fatalf("field = %v; want CustomerRanking", entry["field"])
	}
	if entry["op"] != "eq" {
		t.Fatalf("op = %v; want eq", entry["op"])
	}

	_, err := autotask.List[entities.Company](t.Context(), client, q)
	if err != nil {
		t.Fatalf("server rejected UDF filter query: %v", err)
	}
}

func TestQueryIncludeFields(t *testing.T) {
	t.Parallel()
	company := autotasktest.CompanyFixture()
	_, client := autotasktest.NewServer(t, autotasktest.WithEntity(company))

	q := autotask.NewQuery().Fields("id", "companyName")

	m := marshalQueryToMap(t, q)
	fields, ok := m["IncludeFields"].([]any)
	if !ok {
		t.Fatalf("expected []any for IncludeFields, got %T", m["IncludeFields"])
	}
	if len(fields) != 2 {
		t.Fatalf("IncludeFields length = %d; want 2", len(fields))
	}
	if fields[0] != "id" {
		t.Fatalf("IncludeFields[0] = %v; want id", fields[0])
	}
	if fields[1] != "companyName" {
		t.Fatalf("IncludeFields[1] = %v; want companyName", fields[1])
	}

	_, err := autotask.List[entities.Company](t.Context(), client, q)
	if err != nil {
		t.Fatalf("server rejected query with IncludeFields: %v", err)
	}
}

func TestQueryMaxRecords(t *testing.T) {
	t.Parallel()

	q := autotask.NewQuery().Limit(100)

	m := marshalQueryToMap(t, q)
	maxRecords, ok := m["MaxRecords"].(float64)
	if !ok {
		t.Fatalf("expected float64 for MaxRecords, got %T", m["MaxRecords"])
	}
	if maxRecords != 100 {
		t.Fatalf("MaxRecords = %v; want 100", maxRecords)
	}
}

func TestQueryMaxRecordsTruncation(t *testing.T) {
	t.Parallel()

	q := autotask.NewQuery().Limit(1000)

	m := marshalQueryToMap(t, q)
	maxRecords, ok := m["MaxRecords"].(float64)
	if !ok {
		t.Fatalf("expected float64 for MaxRecords, got %T", m["MaxRecords"])
	}
	if maxRecords != 500 { //nolint:mnd // 500 is the hard cap defined in query.go
		t.Fatalf("MaxRecords = %v; want 500 (clamped)", maxRecords)
	}
}

func TestQueryEmpty(t *testing.T) {
	t.Parallel()
	company := autotasktest.CompanyFixture()
	_, client := autotasktest.NewServer(t, autotasktest.WithEntity(company))

	q := autotask.NewQuery()

	m := marshalQueryToMap(t, q)
	filter, ok := m["filter"].([]any)
	if !ok {
		t.Fatalf("expected []any for filter, got %T", m["filter"])
	}
	if len(filter) != 0 {
		t.Fatalf("filter length = %d; want 0", len(filter))
	}

	results, err := autotask.List[entities.Company](t.Context(), client, q)
	if err != nil {
		t.Fatalf("server rejected empty query: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least one result from empty query with seeded data")
	}
}

func TestQueryInvalidFilterRejected(t *testing.T) {
	t.Parallel()
	company := autotasktest.CompanyFixture()
	ts, _ := autotasktest.NewServer(t, autotasktest.WithEntity(company))

	// Send a raw POST with malformed filter JSON to the query endpoint.
	malformed := []byte(`{"filter": "not-an-array"}`)
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, ts.URL+"/v1.0/Companies/query", bytes.NewReader(malformed))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("UserName", "test-user")
	req.Header.Set("Secret", "test-secret")
	req.Header.Set("ApiIntegrationCode", "test-code")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d; want %d", resp.StatusCode, http.StatusBadRequest)
	}

	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	errors, ok := body["errors"].([]any)
	if !ok || len(errors) == 0 {
		t.Fatal("expected non-empty errors array in response")
	}
}
