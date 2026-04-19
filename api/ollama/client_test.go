package ollama

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
	body := "{" + `"model":"qwen3","message":{"role":"assistant","content":"hi"},"done":false` + "}\n" +
		"{" + `"model":"qwen3","message":{"role":"assistant","content":""},"done":true,"done_reason":"stop","prompt_eval_count":1,"eval_count":2` + "}\n"
	var requestMeta RequestMeta
	client := NewClient(
		WithBaseURL("https://example.com"),
		WithHTTPClient(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/x-ndjson"}}, Body: io.NopCloser(strings.NewReader(body))}, nil
		})}),
		WithRequestTransform(func(_ context.Context, req *Request) error { req.Model = "qwen3"; return nil }),
		WithRequestHook(func(_ context.Context, meta RequestMeta) { requestMeta = meta }),
		WithEventTransform(func(_ context.Context, ev *Response) (*Response, bool, error) { ev.Model += "-seen"; return ev, false, nil }),
	)
	stream, err := client.Stream(context.Background(), Request{Model: "alias", Messages: []Message{{Role: "user", Content: "hi"}}})
	if err != nil { t.Fatalf("Stream() error = %v", err) }
	var events []*Response
	for item := range stream {
		if item.Err != nil { t.Fatalf("unexpected stream error: %v", item.Err) }
		events = append(events, item.Event)
	}
	if len(events) != 2 { t.Fatalf("expected 2 events, got %d", len(events)) }
	if events[0].Model != "qwen3-seen" { t.Fatalf("expected transformed model, got %q", events[0].Model) }
	if requestMeta.Wire == nil || requestMeta.Wire.Model != "qwen3" { t.Fatalf("unexpected wire request: %#v", requestMeta.Wire) }
}

func TestClientListModels(t *testing.T) {
	t.Parallel()
	client := NewClient(
		WithBaseURL("https://example.com"),
		WithHTTPClient(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.Method != http.MethodGet || req.URL.Path != TagsPath {
				t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
			}
			body := `{"models":[{"name":"gemma4:latest","model":"gemma4:latest"}]}`
			return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(body))}, nil
		})}),
	)
	resp, err := client.ListModels(context.Background())
	if err != nil { t.Fatalf("ListModels() error = %v", err) }
	if len(resp.Models) != 1 || resp.Models[0].Name != "gemma4:latest" {
		t.Fatalf("unexpected models response: %#v", resp)
	}
}
