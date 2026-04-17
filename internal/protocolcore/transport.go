package protocolcore

import (
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
