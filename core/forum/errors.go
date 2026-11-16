package forum

import (
	"errors"
	"fmt"
)

var (
	ErrUnauthenticated  = errors.New("forum access token is required")
	ErrEmptyResponse    = errors.New("forum returned an empty response")
	ErrResponseTooLarge = errors.New("forum response exceeded the size limit")
	ErrRedirectOrigin   = errors.New("forum redirect to another origin is not allowed")
)

type APIError struct {
	Code    string
	Message string
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("forum API error %s: %s", e.Code, e.Message)
	}
	return fmt.Sprintf("forum API error %s", e.Code)
}

// HTTPError deliberately omits the authenticated URL and response body.
type HTTPError struct{ StatusCode int }

func (e *HTTPError) Error() string {
	return fmt.Sprintf("forum HTTP status %d", e.StatusCode)
}

// DecodeError separates a response contract failure from authentication and
// transport errors. UI adapters can explain it without showing parser internals.
type DecodeError struct {
	Part  string
	Cause error
}

func (e *DecodeError) Error() string { return fmt.Sprintf("decode forum %s: %v", e.Part, e.Cause) }
func (e *DecodeError) Unwrap() error { return e.Cause }
