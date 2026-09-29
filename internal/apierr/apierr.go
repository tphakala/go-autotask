// Package apierr reads the error envelope of an Autotask response and
// classifies validation failures. The root package and the middleware package
// share it, so a validation 500 is recognised the same way in both.
package apierr

import (
	"encoding/json"
	"strings"
)

// Entry is one element of the "errors" array of a response body.
type Entry struct {
	Message string
	Field   string
}

// validationPhrases are matched case-insensitively as substrings of an error
// message. Autotask answers HTTP 500 for a request it rejects as invalid, and
// nothing else marks such a 500 as permanent. The list is a heuristic: it
// holds only wording seen in real responses, and a 500 that matches none of it
// is treated as a server failure.
var validationPhrases = []string{
	"exceeds maximum length",
}

// Extract returns the entries of the "errors" array of body. An element is
// either an object with a "message" field or a plain string. Elements without
// a message are skipped, and a body that is not a JSON object yields nil.
func Extract(body []byte) []Entry {
	var envelope struct {
		Errors []json.RawMessage `json:"errors"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil || len(envelope.Errors) == 0 {
		return nil
	}
	var result []Entry
	for _, raw := range envelope.Errors {
		var obj struct {
			Message string `json:"message"`
			Field   string `json:"field"`
		}
		if err := json.Unmarshal(raw, &obj); err != nil {
			var s string
			if err := json.Unmarshal(raw, &s); err == nil && s != "" {
				result = append(result, Entry{Message: s})
			}
			continue
		}
		if obj.Message != "" {
			result = append(result, Entry(obj))
		}
	}
	return result
}

// IsValidation reports whether any message matches a known validation phrase.
func IsValidation(messages ...string) bool {
	for _, m := range messages {
		lower := strings.ToLower(m)
		for _, p := range validationPhrases {
			if strings.Contains(lower, p) {
				return true
			}
		}
	}
	return false
}
