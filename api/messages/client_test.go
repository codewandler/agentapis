package messages

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestClientStreamAppliesTypedHooks(t *testing.T) {
	t.Parallel()

	sseBody := "event: message_start\n" +
		"data: {\"message\":{\"id\":\"msg_1\",\"model\":\"claude-real\",\"usage\":{\"input_tokens\":1}}}\n\n" +
		"event: message_stop\n" +
		"data: {}\n\n"

	var (
		requestMeta  RequestMeta
		responseMeta ResponseMeta
	)

	client := NewClient(
		WithBaseURL("https://example.com"),
		WithAPIKey("secret"),
		WithHTTPClient(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": {"text/event-stream"}, "Request-ID": {"req_1"}},
				Body:       io.NopCloser(strings.NewReader(sseBody)),
			}, nil
		})}),
		WithRequestTransform(func(_ context.Context, req *Request) error {
			req.Model = "claude-real"
			return nil
		}),
		WithHTTPRequestMutator(func(_ context.Context, httpReq *http.Request, _ *Request) error {
			httpReq.Header.Set("X-Test", "yes")
			q := httpReq.URL.Query()
			q.Set("beta", "true")
			httpReq.URL.RawQuery = q.Encode()
			return nil
		}),
		WithRequestHook(func(_ context.Context, meta RequestMeta) { requestMeta = meta }),
		WithResponseHook(func(_ context.Context, meta ResponseMeta) { responseMeta = meta }),
		WithEventTransform(func(_ context.Context, ev StreamEvent) (StreamEvent, bool, error) {
			_, ignore := ev.(*MessageStopEvent)
			return ev, ignore, nil
		}),
	)

	stream, err := client.Stream(context.Background(), Request{Model: "alias", MaxTokens: 16, Messages: []Message{{Role: "user", Content: "hi"}}})
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}

	var events []StreamEvent
	for item := range stream {
		if item.Err != nil {
			t.Fatalf("unexpected stream error: %v", item.Err)
		}
		events = append(events, item.Event)
	}

	if len(events) != 1 {
		t.Fatalf("expected 1 event after transform, got %d", len(events))
	}
	if _, ok := events[0].(*MessageStartEvent); !ok {
		t.Fatalf("expected MessageStartEvent, got %T", events[0])
	}
	if requestMeta.Wire == nil || requestMeta.Wire.Model != "claude-real" {
		t.Fatalf("unexpected request meta wire: %#v", requestMeta.Wire)
	}
	if requestMeta.HTTP == nil || requestMeta.HTTP.Header.Get(HeaderAPIKey) != "secret" || requestMeta.HTTP.Header.Get("X-Test") != "yes" {
		t.Fatalf("unexpected request headers: %#v", requestMeta.HTTP)
	}
	if requestMeta.HTTP.URL.Query().Get("beta") != "true" {
		t.Fatalf("expected mutated beta query, got %q", requestMeta.HTTP.URL.RawQuery)
	}
	if responseMeta.StatusCode != http.StatusOK || responseMeta.Headers["Request-ID"][0] != "req_1" {
		t.Fatalf("unexpected response meta: %#v", responseMeta)
	}
}

func TestClientStreamErrorsOnEmptyEventStream(t *testing.T) {
	t.Parallel()

	client := NewClient(
		WithBaseURL("https://example.com"),
		WithAPIKey("secret"),
		WithHTTPClient(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": {"text/event-stream"}},
				Body:       io.NopCloser(strings.NewReader("")),
			}, nil
		})}),
	)

	stream, err := client.Stream(context.Background(), Request{Model: "claude-real", MaxTokens: 16, Messages: []Message{{Role: "user", Content: "hi"}}})
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}

	var gotErr error
	for item := range stream {
		if item.Err != nil {
			gotErr = item.Err
			break
		}
	}
	if gotErr == nil {
		t.Fatal("expected stream error for empty SSE body, got nil")
	}
}
