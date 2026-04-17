package client

import (
	"context"
	"net/http"
)

type RequestMeta struct {
	Target Target
	Wire   any
	HTTP   *http.Request
	Body   []byte
}

type ResponseMeta struct {
	Target     Target
	Wire       any
	StatusCode int
	Headers    http.Header
}

type StreamOptions struct {
	PreferredTarget *Target
	OnRequest       func(context.Context, RequestMeta) error
	OnResponse      func(context.Context, ResponseMeta) error
}
