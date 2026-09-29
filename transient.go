package autotask

import (
	"context"
	"errors"
	"io"
	"net"

	"github.com/tphakala/go-autotask/middleware"
)

// IsTransient reports whether err is likely to succeed when the same request
// is retried later. A nil error and an error this package does not recognise
// return false.
//
// Transient: rate limiting (429), server errors (5xx) other than a validation
// 500 (see ServerError.IsValidation), a 2xx response with a blank body or with
// a body that is not JSON (the maintenance page), an open circuit breaker,
// timeouts, refused or reset connections and truncated bodies. A zone
// discovery answer with status 429 or 5xx is classified by its status like any
// other response.
//
// Permanent: 400, 401, 403, 404 (including a null item), 409 and 422,
// a cancelled context, a response over the size limit (ResponseTooLargeError),
// MaxPagesExceededError, and zone discovery finding no usable version or URL.
//
// A response with a status-typed error and a body that could not be read is
// classified by its status; a transient result says the failure may pass, not
// that the request is safe to send again, because a Create that failed after
// it was sent, by timeout, lost connection, 5xx or blank answer, may already
// have taken effect.
func IsTransient(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) {
		return false
	}
	if _, ok := errors.AsType[*RateLimitError](err); ok {
		return true
	}
	if se, ok := errors.AsType[*ServerError](err); ok {
		return !se.IsValidation()
	}
	if isPermanentAPIError(err) {
		return false
	}
	if _, ok := errors.AsType[*UnexpectedContentTypeError](err); ok {
		return true
	}
	if _, ok := errors.AsType[*EmptyResponseError](err); ok {
		return true
	}
	if _, ok := errors.AsType[*middleware.CircuitBreakerOpenError](err); ok {
		return true
	}
	return isTransientNetworkError(err)
}

func isPermanentAPIError(err error) bool {
	if _, ok := errors.AsType[*ValidationError](err); ok {
		return true
	}
	if _, ok := errors.AsType[*AuthenticationError](err); ok {
		return true
	}
	if _, ok := errors.AsType[*AuthorizationError](err); ok {
		return true
	}
	if _, ok := errors.AsType[*NotFoundError](err); ok {
		return true
	}
	if _, ok := errors.AsType[*ConflictError](err); ok {
		return true
	}
	if _, ok := errors.AsType[*BusinessLogicError](err); ok {
		return true
	}
	if _, ok := errors.AsType[*ResponseTooLargeError](err); ok {
		return true
	}
	_, ok := errors.AsType[*MaxPagesExceededError](err)
	return ok
}

// isTransientNetworkError reports timeouts, connection failures and a body cut
// short. A *url.Error alone does not qualify, because it also wraps permanent
// failures such as an untrusted certificate.
func isTransientNetworkError(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}
	if ne, ok := errors.AsType[net.Error](err); ok && ne.Timeout() {
		return true
	}
	if _, ok := errors.AsType[*net.OpError](err); ok {
		return true
	}
	_, ok := errors.AsType[*net.DNSError](err)
	return ok
}
