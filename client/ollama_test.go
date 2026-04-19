package client

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	ollamaapi "github.com/codewandler/agentapis/api/ollama"
	"github.com/codewandler/agentapis/api/unified"
)

func ioNop(s string) io.ReadCloser { return io.NopCloser(strings.NewReader(s)) }

func TestOllamaClientStreamsUnifiedEvents(t *testing.T) {
	t.Parallel()
	body := "{" + `"model":"qwen3","message":{"role":"assistant","thinking":"h"},"done":false` + "}\n" +
		"{" + `"model":"qwen3","message":{"role":"assistant","content":"hi"},"done":false` + "}\n" +
		"{" + `"model":"qwen3","message":{"role":"assistant","content":""},"done":true,"done_reason":"stop","prompt_eval_count":1,"eval_count":2` + "}\n"
	protocol := ollamaapi.NewClient(
		ollamaapi.WithBaseURL("https://example.com"),
		ollamaapi.WithHTTPClient(&http.Client{Transport: RoundTripFunc(func(_ *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/x-ndjson"}}, Body: ioNop(body)}, nil
		})}),
	)
	client := NewOllamaClient(protocol)
	stream, err := client.Stream(context.Background(), unified.Request{Model: "qwen3", Messages: []unified.Message{{Role: unified.RoleUser, Parts: []unified.Part{{Type: unified.PartTypeText, Text: "hi"}}}}})
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}
	var sawThinking, sawText, sawDone bool
	for item := range stream {
		if item.Err != nil {
			t.Fatalf("unexpected stream error: %v", item.Err)
		}
		sawThinking = sawThinking || (item.Event.Type == unified.StreamEventContentDelta && item.Event.Delta != nil && item.Event.Delta.Kind == unified.DeltaKindThinking)
		sawText = sawText || (item.Event.Type == unified.StreamEventContentDelta && item.Event.Delta != nil && item.Event.Delta.Kind == unified.DeltaKindText)
		sawDone = sawDone || item.Event.Type == unified.StreamEventCompleted
	}
	if !sawThinking || !sawText || !sawDone {
		t.Fatalf("expected thinking/text/completed, got thinking=%v text=%v done=%v", sawThinking, sawText, sawDone)
	}
}

func TestOllamaClientListModels(t *testing.T) {
	t.Parallel()
	protocol := ollamaapi.NewClient(
		ollamaapi.WithBaseURL("https://example.com"),
		ollamaapi.WithHTTPClient(&http.Client{Transport: RoundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.Path != ollamaapi.TagsPath {
				t.Fatalf("unexpected path: %s", req.URL.Path)
			}
			return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/json"}}, Body: ioNop(`{"models":[{"name":"gemma4:e4b","model":"gemma4:e4b"}]}`)}, nil
		})}),
	)
	client := NewOllamaClient(protocol)
	resp, err := client.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels() error = %v", err)
	}
	if len(resp.Models) != 1 || resp.Models[0].Name != "gemma4:e4b" {
		t.Fatalf("unexpected models response: %#v", resp)
	}
}

func TestOllamaClientStreamWithOptionsForwardsTargetedMetadata(t *testing.T) {
	t.Parallel()
	body := "{" + `"model":"qwen3","message":{"role":"assistant","content":"ok"},"done":true` + "}\n"
	protocol := ollamaapi.NewClient(
		ollamaapi.WithBaseURL("https://example.com"),
		ollamaapi.WithHTTPClient(&http.Client{Transport: RoundTripFunc(func(_ *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/x-ndjson"}, "X-Test": {"yes"}}, Body: ioNop(body)}, nil
		})}),
	)
	client := NewOllamaClient(protocol)
	var requestMeta RequestMeta
	var responseMeta ResponseMeta
	stream, err := client.StreamWithOptions(context.Background(), unified.Request{Model: "qwen3", Messages: []unified.Message{{Role: unified.RoleUser, Parts: []unified.Part{{Type: unified.PartTypeText, Text: "hi"}}}}}, StreamOptions{
		OnRequest:  func(_ context.Context, meta RequestMeta) error { requestMeta = meta; return nil },
		OnResponse: func(_ context.Context, meta ResponseMeta) error { responseMeta = meta; return nil },
	})
	if err != nil {
		t.Fatalf("StreamWithOptions() error = %v", err)
	}
	for item := range stream {
		if item.Err != nil {
			t.Fatalf("unexpected stream error: %v", item.Err)
		}
	}
	if requestMeta.Target != TargetOllama || requestMeta.HTTP == nil || len(requestMeta.Body) == 0 {
		t.Fatalf("unexpected request meta: %#v", requestMeta)
	}
	if responseMeta.Target != TargetOllama || responseMeta.StatusCode != http.StatusOK || responseMeta.Headers.Get("X-Test") != "yes" {
		t.Fatalf("unexpected response meta: %#v", responseMeta)
	}
}
