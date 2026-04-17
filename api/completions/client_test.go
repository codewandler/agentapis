package completions

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

	sseBody := "data: {\"id\":\"chatcmpl_1\",\"model\":\"gpt-5\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"hi\"}}]}\n\n" +
		"data: [DONE]\n\n"

	var requestMeta RequestMeta

	client := NewClient(
		WithBaseURL("https://example.com"),
		WithAPIKey("secret"),
		WithHTTPClient(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": {"text/event-stream"}},
				Body:       io.NopCloser(strings.NewReader(sseBody)),
			}, nil
		})}),
		WithRequestTransform(func(_ context.Context, req *Request) error {
			req.Model = "gpt-5"
			return nil
		}),
		WithRequestHook(func(_ context.Context, meta RequestMeta) { requestMeta = meta }),
		WithEventTransform(func(_ context.Context, ev *Chunk) (*Chunk, bool, error) {
			if ev != nil {
				ev.Model = ev.Model + "-seen"
			}
			return ev, false, nil
		}),
	)

	stream, err := client.Stream(context.Background(), Request{Model: "alias", Messages: []Message{{Role: "user", Content: "hi"}}})
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}

	var chunks []*Chunk
	for item := range stream {
		if item.Err != nil {
			t.Fatalf("unexpected stream error: %v", item.Err)
		}
		chunks = append(chunks, item.Event)
	}

	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	if chunks[0].Model != "gpt-5-seen" {
		t.Fatalf("expected transformed model, got %q", chunks[0].Model)
	}
	if requestMeta.Wire == nil || requestMeta.Wire.Model != "gpt-5" {
		t.Fatalf("unexpected wire request: %#v", requestMeta.Wire)
	}
	if requestMeta.HTTP == nil || requestMeta.HTTP.Header.Get("Authorization") != "Bearer secret" {
		t.Fatalf("unexpected request headers: %#v", requestMeta.HTTP)
	}
}
