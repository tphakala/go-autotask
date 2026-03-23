package autotask

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

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

func parseResponse(resp *http.Response, result any) error {
	if resp == nil || resp.Body == nil {
		return fmt.Errorf("autotask: nil HTTP response or body")
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("autotask: reading response body: %w", err)
	}
	apiErrors := extractErrors(body)
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		if len(apiErrors) > 0 {
			return &Error{StatusCode: resp.StatusCode, Message: "unexpected error in success response", Errors: apiErrors}
		}
		if result != nil && len(body) > 0 {
			if err := json.Unmarshal(body, result); err != nil {
				return fmt.Errorf("autotask: decoding response: %w", err)
			}
		}
		return nil
	}
	base := Error{StatusCode: resp.StatusCode, Message: http.StatusText(resp.StatusCode), Errors: apiErrors}
	switch {
	case resp.StatusCode == 400:
		return &ValidationError{Err: base}
	case resp.StatusCode == 401:
		return &AuthenticationError{Err: base}
	case resp.StatusCode == 403:
		return &AuthorizationError{Err: base}
	case resp.StatusCode == 404:
		return &NotFoundError{Err: base}
	case resp.StatusCode == 409:
		return &ConflictError{Err: base}
	case resp.StatusCode == 422:
		return &BusinessLogicError{Err: base}
	case resp.StatusCode == 429:
		return &RateLimitError{Err: base, RetryAfter: parseRetryAfter(resp.Header.Get("Retry-After"))}
	case resp.StatusCode >= 500:
		return &ServerError{Err: base}
	default:
		return &base
	}
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
		return 60 * time.Second
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
	return 60 * time.Second
}
