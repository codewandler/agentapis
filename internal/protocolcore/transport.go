package protocolcore

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
)

const (
	HeaderRetryAfter  = "Retry-After"
	HeaderContentType = "Content-Type"

	ContentTypeJSON = "application/json"
)

type ErrorParser func(statusCode int, body []byte) error

type HTTPError struct {
	StatusCode int
	Body       []byte
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Body)
}

type StatusError struct {
	StatusCode int
	Body       []byte
	Err        error
}

func (e *StatusError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("HTTP %d: %v", e.StatusCode, e.Err)
	}
	return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Body)
}

func (e *StatusError) Unwrap() error {
	if e.Err != nil {
		return e.Err
	}
	return &HTTPError{StatusCode: e.StatusCode, Body: e.Body}
}

func StatusCodeOf(err error) (int, bool) {
	var statusErr *StatusError
	if errors.As(err, &statusErr) {
		return statusErr.StatusCode, true
	}
	var httpErr *HTTPError
	if errors.As(err, &httpErr) {
		return httpErr.StatusCode, true
	}
	return 0, false
}

func CloneHeaders(headers http.Header) http.Header {
	if headers == nil {
		return make(http.Header)
	}
	return headers.Clone()
}

func JoinURL(baseURL, path string) string {
	baseURL = strings.TrimRight(baseURL, "/")
	path = "/" + strings.TrimLeft(path, "/")
	return baseURL + path
}
