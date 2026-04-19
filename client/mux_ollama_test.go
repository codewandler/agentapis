package client

import (
	"context"
	"net/http"
	"testing"

	ollamaapi "github.com/codewandler/agentapis/api/ollama"
	"github.com/codewandler/agentapis/api/unified"
)

func TestMuxClientRoutesToOllamaPreferredTarget(t *testing.T) {
	t.Parallel()
	body := "{" + `"model":"qwen3","message":{"role":"assistant","content":"hi"},"done":true,"done_reason":"stop"` + "}\n"
	ollamaClient := NewOllamaClient(ollamaapi.NewClient(
		ollamaapi.WithBaseURL("https://example.com"),
		ollamaapi.WithHTTPClient(&http.Client{Transport: RoundTripFunc(func(_ *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/x-ndjson"}}, Body: ioNop(body)}, nil
		})}),
	))
	mux := NewMuxClient(WithOllamaClient(ollamaClient))
	preferred := TargetOllama
	stream, err := mux.StreamWithOptions(context.Background(), unified.Request{Model: "qwen3", Messages: []unified.Message{{Role: unified.RoleUser, Parts: []unified.Part{{Type: unified.PartTypeText, Text: "hi"}}}}}, StreamOptions{PreferredTarget: &preferred})
	if err != nil { t.Fatalf("StreamWithOptions() error = %v", err) }
	var saw bool
	for item := range stream {
		if item.Err != nil { t.Fatalf("unexpected stream error: %v", item.Err) }
		saw = true
	}
	if !saw { t.Fatalf("expected ollama event") }
}
