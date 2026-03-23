package autotask

import (
	"bytes"
	"encoding/json"
)

// Optional represents a three-state field: unset, null, or set to a value.
// Use with the `omitzero` struct tag so unset fields are omitted from JSON.
//   - Zero value: unset (omitted from JSON via omitzero + IsZero)
//   - Null[T](): explicitly null (serializes as JSON null, clears field in Autotask)
//   - Set(v): has a value (serializes as the JSON representation of v)
type Optional[T any] struct {
	value T
	set   bool
	null  bool
}

// Set creates an Optional with a value.
func Set[T any](v T) Optional[T] {
	return Optional[T]{value: v, set: true}
}

// Null creates an Optional that is explicitly null.
func Null[T any]() Optional[T] {
	return Optional[T]{set: true, null: true}
}

// Get returns the value and whether it is set (non-null).
func (o Optional[T]) Get() (T, bool) {
	if o.set && !o.null {
		return o.value, true
	}
	var zero T
	return zero, false
}

// IsSet returns true if the field was explicitly set (to a value or null).
func (o Optional[T]) IsSet() bool { return o.set }

// IsNull returns true if the field was explicitly set to null.
func (o Optional[T]) IsNull() bool { return o.null }

// IsZero returns true when the field is unset. Used by encoding/json with
// the omitzero struct tag to omit unset fields from JSON output.
func (o Optional[T]) IsZero() bool { return !o.set }

// MarshalJSON implements json.Marshaler. Note: when called directly on an unset
// Optional (not via a struct field with omitzero), this marshals the zero value
// of T. Optional is designed for use as struct fields with the `omitzero` tag.
func (o Optional[T]) MarshalJSON() ([]byte, error) {
	if o.null {
		return []byte("null"), nil
	}
	return json.Marshal(o.value)
}

// UnmarshalJSON implements json.Unmarshaler.
func (o *Optional[T]) UnmarshalJSON(data []byte) error {
	o.set = true
	if bytes.Equal(data, []byte("null")) {
		o.null = true
		return nil
	}
	return json.Unmarshal(data, &o.value)
}
