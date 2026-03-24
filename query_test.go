package autotask

import (
	"encoding/json"
	"testing"
)

// marshalQuery is a test helper that marshals q and unmarshals into a map.
func marshalQuery(t *testing.T, q *Query) map[string]any {
	t.Helper()
	b, err := json.Marshal(q)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestQuerySimpleWhere(t *testing.T) {
	m := marshalQuery(t, NewQuery().Where("status", OpEq, 1))
	filter, ok := m["filter"].([]any)
	if !ok {
		t.Fatalf("expected []any for filter, got %T", m["filter"])
	}
	if len(filter) != 1 {
		t.Fatalf("filter length = %d; want 1", len(filter))
	}
	f, ok := filter[0].(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any for filter[0], got %T", filter[0])
	}
	if f["op"] != "eq" || f["field"] != "status" {
		t.Fatalf("filter = %v", f)
	}
}

func TestQueryMultipleWhere(t *testing.T) {
	m := marshalQuery(t, NewQuery().Where("status", OpEq, 1).Where("queueID", OpEq, 8))
	filter, ok := m["filter"].([]any)
	if !ok {
		t.Fatalf("expected []any for filter, got %T", m["filter"])
	}
	if len(filter) != 2 {
		t.Fatalf("filter length = %d; want 2", len(filter))
	}
}

func TestQueryOr(t *testing.T) {
	m := marshalQuery(t, NewQuery().Or(Field("priority", OpEq, 1), Field("priority", OpEq, 2)))
	filter, ok := m["filter"].([]any)
	if !ok {
		t.Fatalf("expected []any for filter, got %T", m["filter"])
	}
	orGroup, ok := filter[0].(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any for filter[0], got %T", filter[0])
	}
	if orGroup["op"] != "or" {
		t.Fatalf("op = %v; want or", orGroup["op"])
	}
	items, ok := orGroup["items"].([]any)
	if !ok {
		t.Fatalf("expected []any for items, got %T", orGroup["items"])
	}
	if len(items) != 2 {
		t.Fatalf("items length = %d; want 2", len(items))
	}
}

func TestQueryNestedAndOr(t *testing.T) {
	m := marshalQuery(t, NewQuery().Or(
		And(Field("status", OpEq, 1), Field("queueID", OpEq, 8)),
		And(Field("priority", OpEq, 1), Field("priority", OpEq, 2)),
	))
	filter, ok := m["filter"].([]any)
	if !ok {
		t.Fatalf("expected []any for filter, got %T", m["filter"])
	}
	orGroup, ok := filter[0].(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any for filter[0], got %T", filter[0])
	}
	items, ok := orGroup["items"].([]any)
	if !ok {
		t.Fatalf("expected []any for items, got %T", orGroup["items"])
	}
	if len(items) != 2 {
		t.Fatalf("OR items = %d; want 2", len(items))
	}
	andGroup, ok := items[0].(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any for items[0], got %T", items[0])
	}
	if andGroup["op"] != "and" {
		t.Fatalf("nested op = %v; want and", andGroup["op"])
	}
}

func TestQueryUDF(t *testing.T) {
	m := marshalQuery(t, NewQuery().WhereUDF("CustomField", OpEq, "value"))
	filter, ok := m["filter"].([]any)
	if !ok {
		t.Fatalf("expected []any for filter, got %T", m["filter"])
	}
	f, ok := filter[0].(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any for filter[0], got %T", filter[0])
	}
	if f["udf"] != true {
		t.Fatalf("udf = %v; want true", f["udf"])
	}
}

func TestQueryFields(t *testing.T) {
	m := marshalQuery(t, NewQuery().Where("status", OpEq, 1).Fields("id", "title", "status"))
	fields, ok := m["IncludeFields"].([]any)
	if !ok {
		t.Fatalf("expected []any for IncludeFields, got %T", m["IncludeFields"])
	}
	if len(fields) != 3 {
		t.Fatalf("IncludeFields length = %d; want 3", len(fields))
	}
}

func TestQueryLimit(t *testing.T) {
	m := marshalQuery(t, NewQuery().Where("status", OpEq, 1).Limit(100))
	if m["MaxRecords"] != float64(100) {
		t.Fatalf("MaxRecords = %v; want 100", m["MaxRecords"])
	}
}

func TestQueryLimitClampedTo500(t *testing.T) {
	m := marshalQuery(t, NewQuery().Limit(1000))
	if m["MaxRecords"] != float64(500) {
		t.Fatalf("MaxRecords = %v; want 500 (clamped)", m["MaxRecords"])
	}
}

func TestFieldConstructor(t *testing.T) {
	f := Field("status", OpEq, 1)
	if f.Field != "status" || f.Op != OpEq {
		t.Fatalf("Field = %+v", f)
	}
}

func TestUDFieldConstructor(t *testing.T) {
	f := UDField("Custom", OpContains, "test")
	if !f.UDF {
		t.Fatal("UDField should set UDF=true")
	}
}
