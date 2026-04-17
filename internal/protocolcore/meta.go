package protocolcore

import "net/http"

type RequestMeta[Req any] struct {
	Wire *Req
	HTTP *http.Request
	Body []byte
}

type ResponseMeta[Req any] struct {
	Wire       *Req
	StatusCode int
	Headers    http.Header
}

type RawEvent struct {
	Name string
	Data []byte
	Err  error
}

type HTTPRequest struct {
	Method  string
	URL     string
	Headers http.Header
	Body    []byte
}
