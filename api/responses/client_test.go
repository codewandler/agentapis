package responses

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

	sseBody := "event: response.created\n" +
		"data: {\"response\":{\"id\":\"resp_1\",\"model\":\"gpt-5\"}}\n\n" +
		"event: response.completed\n" +
		"data: {\"response\":{\"id\":\"resp_1\",\"model\":\"gpt-5\",\"status\":\"completed\"}}\n\n"

	var responseMeta ResponseMeta

	client := NewClient(
		WithBaseURL("https://example.com"),
		WithAPIKey("secret"),
		WithHTTPClient(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": {"text/event-stream"}, "X-Request-ID": {"req_123"}},
				Body:       io.NopCloser(strings.NewReader(sseBody)),
			}, nil
		})}),
		WithRequestTransform(func(_ context.Context, req *Request) error {
			req.Model = "gpt-5"
			return nil
		}),
		WithResponseHook(func(_ context.Context, meta ResponseMeta) { responseMeta = meta }),
		WithEventTransform(func(_ context.Context, ev StreamEvent) (StreamEvent, bool, error) {
			_, ignore := ev.(*ResponseCompletedEvent)
			return ev, ignore, nil
		}),
	)

	stream, err := client.Stream(context.Background(), Request{Model: "alias", Input: []Input{{Role: "user", Content: "hi"}}})
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
	if _, ok := events[0].(*ResponseCreatedEvent); !ok {
		t.Fatalf("expected ResponseCreatedEvent, got %T", events[0])
	}
	if responseMeta.StatusCode != http.StatusOK || responseMeta.Headers["X-Request-ID"][0] != "req_123" {
		t.Fatalf("unexpected response meta: %#v", responseMeta)
	}
}
