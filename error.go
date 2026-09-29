package autotask

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const retryAfterDefault = 60 * time.Second

// maxSnippetBytes caps how much of a non-JSON body UnexpectedContentTypeError keeps.
const maxSnippetBytes = 256

type Error struct {
	StatusCode int
	Message    string
	Errors     []APIError
}

type APIError struct {
	Message string `json:"message"`
	Field   string `json:"field,omitempty"`
}

func (e *Error) Error() string {
	if len(e.Errors) > 0 {
		return fmt.Sprintf("autotask: %d %s: %s", e.StatusCode, e.Message, e.Errors[0].Message)
	}
	return fmt.Sprintf("autotask: %d %s", e.StatusCode, e.Message)
}

type ValidationError struct{ Err Error }

func (e *ValidationError) Error() string { return e.Err.Error() }
func (e *ValidationError) Unwrap() error { return &e.Err }

type AuthenticationError struct{ Err Error }

func (e *AuthenticationError) Error() string { return e.Err.Error() }
func (e *AuthenticationError) Unwrap() error { return &e.Err }

type AuthorizationError struct{ Err Error }

func (e *AuthorizationError) Error() string { return e.Err.Error() }
func (e *AuthorizationError) Unwrap() error { return &e.Err }

type NotFoundError struct{ Err Error }

func (e *NotFoundError) Error() string { return e.Err.Error() }
func (e *NotFoundError) Unwrap() error { return &e.Err }

type ConflictError struct{ Err Error }

func (e *ConflictError) Error() string { return e.Err.Error() }
func (e *ConflictError) Unwrap() error { return &e.Err }

type BusinessLogicError struct{ Err Error }

func (e *BusinessLogicError) Error() string { return e.Err.Error() }
func (e *BusinessLogicError) Unwrap() error { return &e.Err }

type RateLimitError struct {
	Err        Error
	RetryAfter time.Duration
}

func (e *RateLimitError) Error() string { return e.Err.Error() }
func (e *RateLimitError) Unwrap() error { return &e.Err }

type ServerError struct{ Err Error }

func (e *ServerError) Error() string { return e.Err.Error() }
func (e *ServerError) Unwrap() error { return &e.Err }

// UnexpectedContentTypeError reports a successful (2xx) response whose body is
// not blank, not labelled as JSON and not valid JSON. During planned
// maintenance Autotask answers with HTTP 200 and an HTML page, so a caller can
// treat this error as transient and back off.
type UnexpectedContentTypeError struct {
	Err         Error
	ContentType string
	// Snippet holds up to the first 256 bytes of the body, for logging.
	Snippet string
}

func (e *UnexpectedContentTypeError) Error() string {
	return fmt.Sprintf("%s (Content-Type %q)", e.Err.Error(), e.ContentType)
}
func (e *UnexpectedContentTypeError) Unwrap() error { return &e.Err }

// bodyReadError is returned when the body of a non-2xx response cannot be read.
// It matches the status-typed error and the read error under errors.As and
// errors.Is.
type bodyReadError struct {
	status error
	read   error
}

func (e *bodyReadError) Error() string   { return e.status.Error() + ": " + e.read.Error() }
func (e *bodyReadError) Unwrap() []error { return []error{e.status, e.read} }

func statusToError(resp *http.Response, base Error) error {
	switch {
	case resp.StatusCode == http.StatusBadRequest:
		return &ValidationError{Err: base}
	case resp.StatusCode == http.StatusUnauthorized:
		return &AuthenticationError{Err: base}
	case resp.StatusCode == http.StatusForbidden:
		return &AuthorizationError{Err: base}
	case resp.StatusCode == http.StatusNotFound:
		return &NotFoundError{Err: base}
	case resp.StatusCode == http.StatusConflict:
		return &ConflictError{Err: base}
	case resp.StatusCode == http.StatusUnprocessableEntity:
		return &BusinessLogicError{Err: base}
	case resp.StatusCode == http.StatusTooManyRequests:
		return &RateLimitError{Err: base, RetryAfter: parseRetryAfter(resp.Header.Get("Retry-After"))}
	case resp.StatusCode >= http.StatusInternalServerError:
		return &ServerError{Err: base}
	default:
		return &base
	}
}

func parseResponse(resp *http.Response, result any) error {
	if resp == nil || resp.Body == nil {
		return fmt.Errorf("autotask: nil HTTP response or body")
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		readErr := fmt.Errorf("autotask: reading response body: %w", err)
		if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
			return readErr
		}
		// The status line already classifies the failure, so keep the typed
		// error (and RetryAfter) and carry the read error beside it.
		base := Error{StatusCode: resp.StatusCode, Message: http.StatusText(resp.StatusCode)}
		return &bodyReadError{status: statusToError(resp, base), read: readErr}
	}
	apiErrors := extractErrors(body)
	if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
		if len(apiErrors) > 0 {
			return &Error{StatusCode: resp.StatusCode, Message: "unexpected error in success response", Errors: apiErrors}
		}
		// Checked even when result is nil, so a Delete answered by the
		// maintenance page does not read as success.
		if err := checkJSONBody(resp, body); err != nil {
			return err
		}
		if result != nil && len(body) > 0 {
			if err := json.Unmarshal(body, result); err != nil {
				return fmt.Errorf("autotask: decoding response: %w", err)
			}
		}
		return nil
	}
	base := Error{StatusCode: resp.StatusCode, Message: http.StatusText(resp.StatusCode), Errors: apiErrors}
	return statusToError(resp, base)
}

// checkJSONBody returns an *UnexpectedContentTypeError when body is neither
// labelled as JSON nor valid JSON. A body that is valid JSON passes under any
// Content-Type, because JSON can arrive mislabelled: a Go net/http handler that
// does not set the header gets a sniffed "text/plain; charset=utf-8"
// (net/http/server.go:1486 in Go 1.27.1). An empty or whitespace-only body
// passes and is left to the caller.
func checkJSONBody(resp *http.Response, body []byte) error {
	if len(bytes.TrimSpace(body)) == 0 {
		return nil
	}
	contentType := resp.Header.Get("Content-Type")
	if isJSONMediaType(contentType) || json.Valid(body) {
		return nil
	}
	snippet := body
	if len(snippet) > maxSnippetBytes {
		snippet = snippet[:maxSnippetBytes]
	}
	return &UnexpectedContentTypeError{
		Err:         Error{StatusCode: resp.StatusCode, Message: "response body is not JSON"},
		ContentType: contentType,
		// The cut can split a multi-byte character; drop the partial bytes.
		Snippet: strings.ToValidUTF8(string(snippet), ""),
	}
}

// isJSONMediaType reports whether contentType is application/json or a
// +json structured syntax type such as application/problem+json.
func isJSONMediaType(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return false
	}
	return mediaType == "application/json" || strings.HasSuffix(mediaType, "+json")
}

func extractErrors(body []byte) []APIError {
	var envelope struct {
		Errors []json.RawMessage `json:"errors"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil || len(envelope.Errors) == 0 {
		return nil
	}
	var result []APIError
	for _, raw := range envelope.Errors {
		var ae APIError
		if err := json.Unmarshal(raw, &ae); err != nil {
			var s string
			if err := json.Unmarshal(raw, &s); err == nil && s != "" {
				result = append(result, APIError{Message: s})
			}
			continue
		}
		if ae.Message != "" {
			result = append(result, ae)
		}
	}
	return result
}

func parseRetryAfter(header string) time.Duration {
	if header == "" {
		return retryAfterDefault
	}
	if seconds, err := strconv.Atoi(header); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	if t, err := http.ParseTime(header); err == nil {
		d := time.Until(t)
		if d > 0 {
			return d
		}
	}
	return retryAfterDefault
}
