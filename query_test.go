package autotask

import (
	"encoding/json"
	"testing"
)

func TestQuerySimpleWhere(t *testing.T) {
	q := NewQuery().Where("status", OpEq, 1)
	b, err := json.Marshal(q)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	json.Unmarshal(b, &m)
	filter := m["filter"].([]any)
	if len(filter) != 1 {
		t.Fatalf("filter length = %d; want 1", len(filter))
	}
	f := filter[0].(map[string]any)
	if f["op"] != "eq" || f["field"] != "status" {
		t.Fatalf("filter = %v", f)
	}
}

func TestQueryMultipleWhere(t *testing.T) {
	q := NewQuery().Where("status", OpEq, 1).Where("queueID", OpEq, 8)
	b, _ := json.Marshal(q)
	var m map[string]any
	json.Unmarshal(b, &m)
	filter := m["filter"].([]any)
	if len(filter) != 2 {
		t.Fatalf("filter length = %d; want 2", len(filter))
	}
}

func TestQueryOr(t *testing.T) {
	q := NewQuery().Or(Field("priority", OpEq, 1), Field("priority", OpEq, 2))
	b, _ := json.Marshal(q)
	var m map[string]any
	json.Unmarshal(b, &m)
	filter := m["filter"].([]any)
	orGroup := filter[0].(map[string]any)
	if orGroup["op"] != "or" {
		t.Fatalf("op = %v; want or", orGroup["op"])
	}
	items := orGroup["items"].([]any)
	if len(items) != 2 {
		t.Fatalf("items length = %d; want 2", len(items))
	}
}

func TestQueryNestedAndOr(t *testing.T) {
	q := NewQuery().Or(
		And(Field("status", OpEq, 1), Field("queueID", OpEq, 8)),
		And(Field("priority", OpEq, 1), Field("priority", OpEq, 2)),
	)
	b, _ := json.Marshal(q)
	var m map[string]any
	json.Unmarshal(b, &m)
	filter := m["filter"].([]any)
	orGroup := filter[0].(map[string]any)
	items := orGroup["items"].([]any)
	if len(items) != 2 {
		t.Fatalf("OR items = %d; want 2", len(items))
	}
	andGroup := items[0].(map[string]any)
	if andGroup["op"] != "and" {
		t.Fatalf("nested op = %v; want and", andGroup["op"])
	}
}

func TestQueryUDF(t *testing.T) {
	q := NewQuery().WhereUDF("CustomField", OpEq, "value")
	b, _ := json.Marshal(q)
	var m map[string]any
	json.Unmarshal(b, &m)
	filter := m["filter"].([]any)
	f := filter[0].(map[string]any)
	if f["udf"] != true {
		t.Fatalf("udf = %v; want true", f["udf"])
	}
}

func TestQueryFields(t *testing.T) {
	q := NewQuery().Where("status", OpEq, 1).Fields("id", "title", "status")
	b, _ := json.Marshal(q)
	var m map[string]any
	json.Unmarshal(b, &m)
	fields := m["IncludeFields"].([]any)
	if len(fields) != 3 {
		t.Fatalf("IncludeFields length = %d; want 3", len(fields))
	}
}

func TestQueryLimit(t *testing.T) {
	q := NewQuery().Where("status", OpEq, 1).Limit(100)
	b, _ := json.Marshal(q)
	var m map[string]any
	json.Unmarshal(b, &m)
	if m["MaxRecords"] != float64(100) {
		t.Fatalf("MaxRecords = %v; want 100", m["MaxRecords"])
	}
}

func TestQueryLimitClampedTo500(t *testing.T) {
	q := NewQuery().Limit(1000)
	b, _ := json.Marshal(q)
	var m map[string]any
	json.Unmarshal(b, &m)
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
