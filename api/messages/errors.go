package messages

import (
	"encoding/json"
	"errors"
	"fmt"
)

// Common Anthropic API error types
const (
	ErrTypeInvalidRequest    = "invalid_request_error"
	ErrTypeAuthentication    = "authentication_error"
	ErrTypePermission        = "permission_error"
	ErrTypeNotFound          = "not_found_error"
	ErrTypeRateLimit         = "rate_limit_error"
	ErrTypeAPI               = "api_error"
	ErrTypeOverloaded        = "overloaded_error"
	ErrTypeInsufficientQuota = "insufficient_quota_error"
)

// APIError represents a structured error response from the Anthropic API.
type APIError struct {
	// Type is the error type (e.g., "invalid_request_error", "rate_limit_error")
	Type string `json:"type"`

	// Message is the human-readable error message
	Message string `json:"message"`

	// StatusCode is the HTTP status code
	StatusCode int `json:"-"`

	// RawBody is the original response body (for debugging)
	RawBody []byte `json:"-"`
}

// Error implements the error interface.
func (e *APIError) Error() string {
	if e.Type != "" {
		return fmt.Sprintf("%s: %s (HTTP %d)", e.Type, e.Message, e.StatusCode)
	}
	return fmt.Sprintf("%s (HTTP %d)", e.Message, e.StatusCode)
}

// Is implements errors.Is for error matching.
func (e *APIError) Is(target error) bool {
	if target == nil {
		return false
	}
	var other *APIError
	if errors.As(target, &other) {
		// Match by type if both have types
		if e.Type != "" && other.Type != "" {
			return e.Type == other.Type
		}
	}
	return false
}

// IsRateLimit returns true if this is a rate limit error.
func (e *APIError) IsRateLimit() bool {
	return e.Type == ErrTypeRateLimit
}

// IsAuthentication returns true if this is an authentication error.
func (e *APIError) IsAuthentication() bool {
	return e.Type == ErrTypeAuthentication
}

// IsOverloaded returns true if the API is overloaded.
func (e *APIError) IsOverloaded() bool {
	return e.Type == ErrTypeOverloaded
}

// IsInvalidRequest returns true if the request was invalid.
func (e *APIError) IsInvalidRequest() bool {
	return e.Type == ErrTypeInvalidRequest
}

// IsInsufficientQuota returns true if there's insufficient quota.
func (e *APIError) IsInsufficientQuota() bool {
	return e.Type == ErrTypeInsufficientQuota
}

// IsRetryable returns true if the error is potentially retryable.
func (e *APIError) IsRetryable() bool {
	switch e.Type {
	case ErrTypeRateLimit, ErrTypeOverloaded, ErrTypeAPI:
		return true
	}
	return e.StatusCode >= 500
}

// Sentinel errors for type checking with errors.Is
var (
	ErrRateLimit  = &APIError{Type: ErrTypeRateLimit}
	ErrAuth       = &APIError{Type: ErrTypeAuthentication}
	ErrOverloaded = &APIError{Type: ErrTypeOverloaded}
	ErrInvalidReq = &APIError{Type: ErrTypeInvalidRequest}
	ErrNoQuota    = &APIError{Type: ErrTypeInsufficientQuota}
)

// ParseAPIError parses an API error from a response body and status code.
// Returns nil if the body doesn't contain a valid error structure.
func ParseAPIError(statusCode int, body []byte) *APIError {
	var resp struct {
		Type  string `json:"type"`
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil
	}
	if resp.Error.Message == "" {
		return nil
	}
	return &APIError{
		Type:       resp.Error.Type,
		Message:    resp.Error.Message,
		StatusCode: statusCode,
		RawBody:    body,
	}
}

// AsAPIError attempts to extract an *APIError from err.
// Returns nil if err is not an *APIError.
func AsAPIError(err error) *APIError {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr
	}
	return nil
}
