package httpx

import (
	"net/http"

	"github.com/codewandler/agentapis/internal/protocolcore"
)

// DefaultTransport returns the shared default HTTP transport used by agentapis.
//
// It supports response decompression for gzip, deflate, br, and zstd.
func DefaultTransport() http.RoundTripper {
	return protocolcore.DefaultHTTPTransport()
}

// DefaultClient returns the shared default HTTP client used by agentapis.
func DefaultClient() *http.Client {
	return protocolcore.DefaultHTTPClient()
}

// CloneDefaultClient returns a shallow clone of the shared default HTTP client.
//
// This is useful when callers want to preserve agentapis defaults while replacing
// or wrapping the Transport on their own client instance.
func CloneDefaultClient() *http.Client {
	base := protocolcore.DefaultHTTPClient()
	cloned := &http.Client{}
	if base != nil {
		*cloned = *base
	}
	return cloned
}
