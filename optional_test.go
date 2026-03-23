package autotask

import (
	"encoding/json"
	"testing"
	"time"
)

func TestOptionalZeroValueIsUnset(t *testing.T) {
	var o Optional[string]
	if o.IsSet() {
		t.Fatal("zero value Optional should not be set")
	}
	if o.IsNull() {
		t.Fatal("zero value Optional should not be null")
	}
	if !o.IsZero() {
		t.Fatal("zero value Optional should be zero")
	}
}

func TestOptionalSet(t *testing.T) {
	o := Set("hello")
	if !o.IsSet() {
		t.Fatal("Set Optional should be set")
	}
	if o.IsNull() {
		t.Fatal("Set Optional should not be null")
	}
	if o.IsZero() {
		t.Fatal("Set Optional should not be zero")
	}
	v, ok := o.Get()
	if !ok || v != "hello" {
		t.Fatalf("Get() = %q, %v; want %q, true", v, ok, "hello")
	}
}

func TestOptionalNull(t *testing.T) {
	o := Null[string]()
	if !o.IsSet() {
		t.Fatal("Null Optional should be set (explicitly set to null)")
	}
	if !o.IsNull() {
		t.Fatal("Null Optional should be null")
	}
	if o.IsZero() {
		t.Fatal("Null Optional should not be zero (it's explicitly set)")
	}
}

func TestOptionalMarshalJSONSet(t *testing.T) {
	o := Set(42)
	b, err := json.Marshal(o)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "42" {
		t.Fatalf("Marshal Set(42) = %s; want 42", b)
	}
}

func TestOptionalMarshalJSONNull(t *testing.T) {
	o := Null[int]()
	b, err := json.Marshal(o)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "null" {
		t.Fatalf("Marshal Null = %s; want null", b)
	}
}

func TestOptionalOmitzeroInStruct(t *testing.T) {
	type S struct {
		Name  Optional[string] `json:"name,omitzero"`
		Value Optional[int]    `json:"value,omitzero"`
		Clear Optional[string] `json:"clear,omitzero"`
	}
	s := S{
		Name:  Set("test"),
		Clear: Null[string](),
	}
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	expected := `{"name":"test","clear":null}`
	if string(b) != expected {
		t.Fatalf("Marshal = %s; want %s", b, expected)
	}
}

func TestOptionalUnmarshalJSON(t *testing.T) {
	type S struct {
		Name  Optional[string] `json:"name,omitzero"`
		Value Optional[int]    `json:"value,omitzero"`
		Clear Optional[string] `json:"clear,omitzero"`
	}
	input := `{"name":"hello","clear":null}`
	var s S
	if err := json.Unmarshal([]byte(input), &s); err != nil {
		t.Fatal(err)
	}
	if v, ok := s.Name.Get(); !ok || v != "hello" {
		t.Fatalf("Name = %q, %v; want hello, true", v, ok)
	}
	if s.Value.IsSet() {
		t.Fatal("Value should not be set (missing from JSON)")
	}
	if !s.Clear.IsNull() {
		t.Fatal("Clear should be null")
	}
}

func TestOptionalTime(t *testing.T) {
	ts := time.Date(2026, 3, 23, 12, 0, 0, 0, time.UTC)
	o := Set(ts)
	b, err := json.Marshal(o)
	if err != nil {
		t.Fatal(err)
	}
	var o2 Optional[time.Time]
	if err := json.Unmarshal(b, &o2); err != nil {
		t.Fatal(err)
	}
	v, ok := o2.Get()
	if !ok || !v.Equal(ts) {
		t.Fatalf("round-trip time = %v, %v; want %v, true", v, ok, ts)
	}
}
