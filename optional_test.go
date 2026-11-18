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

func TestOptionalUnmarshalState(t *testing.T) {
	t.Run("a decoded value clears null", func(t *testing.T) {
		o := Null[int64]()
		if err := json.Unmarshal([]byte("5"), &o); err != nil {
			t.Fatal(err)
		}
		if v, ok := o.Get(); !ok || v != 5 {
			t.Fatalf("Get() = %v, %v; want 5, true", v, ok)
		}
		if o.IsNull() {
			t.Fatal("IsNull() = true after decoding a value")
		}
	})

	t.Run("a failed decode keeps the previous state", func(t *testing.T) {
		null := Null[string]()
		if err := json.Unmarshal([]byte("5"), &null); err == nil {
			t.Fatal("decoding a number into Optional[string] succeeded")
		}
		if !null.IsNull() {
			t.Fatal("a failed decode changed an Optional that was null")
		}
		var unset Optional[string]
		if err := json.Unmarshal([]byte("5"), &unset); err == nil {
			t.Fatal("decoding a number into Optional[string] succeeded")
		}
		if unset.IsSet() {
			t.Fatal("a failed decode marked an unset Optional as set")
		}
	})
}

func TestOptionalUnmarshalFreshValue(t *testing.T) {
	t.Run("a failed decode keeps the previous value", func(t *testing.T) {
		o := Set([]int{9, 8})
		if err := json.Unmarshal([]byte(`[1, "bad"]`), &o); err == nil {
			t.Fatal("decoding a string element into Optional[[]int] succeeded")
		}
		if v, ok := o.Get(); !ok || len(v) != 2 || v[0] != 9 || v[1] != 8 {
			t.Fatalf("Get() = %v, %v; want [9 8], true", v, ok)
		}
	})

	t.Run("a value decoded after another value holds only the new value", func(t *testing.T) {
		o := Set(map[string]int{"old": 1})
		if err := json.Unmarshal([]byte("null"), &o); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal([]byte(`{}`), &o); err != nil {
			t.Fatal(err)
		}
		if v, ok := o.Get(); !ok || len(v) != 0 {
			t.Fatalf("Get() = %v, %v; want an empty map, true", v, ok)
		}
	})
}
