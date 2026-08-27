package tekmetric

import "fmt"

// temporaryError represents a temporary error that should be retried.
// This includes rate limit errors (429) and server errors (5xx).
type temporaryError struct {
	statusCode int
	message    string
	cause      error
}

func (e *temporaryError) Error() string {
	return e.message
}

// Temporary returns true indicating this error is temporary and should be retried.
func (e *temporaryError) Temporary() bool {
	return true
}

// Unwrap exposes the underlying error to errors.Is and errors.As.
func (e *temporaryError) Unwrap() error {
	return e.cause
}

// StatusCode returns the HTTP status that produced this error, or zero when the
// error came from the transport rather than a response.
func (e *temporaryError) StatusCode() int {
	return e.statusCode
}

// newTemporaryStatusError reports a status the API may answer differently on
// another attempt.
func newTemporaryStatusError(status int) *temporaryError {
	return &temporaryError{
		statusCode: status,
		message:    fmt.Sprintf("temporary error with status %d", status),
	}
}

// newTransportError reports a request that never produced a response. A network
// fault is worth another attempt.
func newTransportError(cause error) *temporaryError {
	return &temporaryError{
		message: fmt.Sprintf("request failed: %v", cause),
		cause:   cause,
	}
}
