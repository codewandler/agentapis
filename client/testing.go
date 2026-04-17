package client

import (
	"io"
	"net/http"
	"strings"
)

type RoundTripFunc func(*http.Request) (*http.Response, error)

func (f RoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func FixedSSEResponse(statusCode int, sseBody string) RoundTripFunc {
	return func(_ *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: statusCode,
			Header:     http.Header{"Content-Type": {"text/event-stream"}},
			Body:       io.NopCloser(strings.NewReader(sseBody)),
		}, nil
	}
}

func NewTestStream(events ...StreamResult) <-chan StreamResult {
	ch := make(chan StreamResult, len(events))
	for _, event := range events {
		ch <- event
	}
	close(ch)
	return ch
}
