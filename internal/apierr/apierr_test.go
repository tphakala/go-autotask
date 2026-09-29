package apierr

import (
	"reflect"
	"testing"
)

func TestExtract(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		body string
		want []Entry
	}{
		{"strings", `{"errors":["a","b"]}`, []Entry{{Message: "a"}, {Message: "b"}}},
		{"objects", `{"errors":[{"message":"m","field":"f"}]}`, []Entry{{Message: "m", Field: "f"}}},
		{"mixed", `{"errors":["a",{"message":"m"}]}`, []Entry{{Message: "a"}, {Message: "m"}}},
		{"empty messages skipped", `{"errors":["",{"message":""},{"field":"f"}]}`, nil},
		{"no errors", `{"items":[]}`, nil},
		{"not an object", `<html>`, nil},
		{"empty", ``, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := Extract([]byte(tt.body)); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("Extract(%q) = %+v; want %+v", tt.body, got, tt.want)
			}
		})
	}
}

func TestIsValidation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		msgs []string
		want bool
	}{
		{"none", nil, false},
		{"match", []string{"String value exceeds maximum length (field:description)"}, true},
		{"case-insensitive", []string{"VALUE EXCEEDS MAXIMUM LENGTH"}, true},
		{"any message", []string{"unrelated", "exceeds maximum length"}, true},
		{"unrelated", []string{"Internal database error"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := IsValidation(tt.msgs...); got != tt.want {
				t.Fatalf("IsValidation(%q) = %v; want %v", tt.msgs, got, tt.want)
			}
		})
	}
}
