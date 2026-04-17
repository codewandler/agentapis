package client

import (
	"context"
	"net/http"
	"testing"

	responsesapi "github.com/codewandler/agentapis/api/responses"
	"github.com/codewandler/agentapis/api/unified"
)

func TestResponsesClientStreamsUnifiedEvents(t *testing.T) {
	t.Parallel()

	sseBody := "event: response.created\n" +
		"data: {\"response\":{\"id\":\"resp_1\",\"model\":\"gpt-5\"}}\n\n" +
		"event: response.completed\n" +
		"data: {\"response\":{\"id\":\"resp_1\",\"model\":\"gpt-5\",\"status\":\"completed\",\"usage\":{\"input_tokens\":1,\"output_tokens\":2}}}\n\n"

	protocol := responsesapi.NewClient(
		responsesapi.WithBaseURL("https://example.com"),
		responsesapi.WithHTTPClient(&http.Client{Transport: FixedSSEResponse(http.StatusOK, sseBody)}),
	)

	client := NewResponsesClient(protocol)
	stream, err := client.Stream(context.Background(), unified.Request{
		Model:    "gpt-5",
		Messages: []unified.Message{{Role: unified.RoleUser, Parts: []unified.Part{{Type: unified.PartTypeText, Text: "hi"}}}},
	})
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}

	var sawStarted, sawCompleted bool
	for item := range stream {
		if item.Err != nil {
			t.Fatalf("unexpected stream error: %v", item.Err)
		}
		sawStarted = sawStarted || item.Event.Type == unified.StreamEventStarted
		sawCompleted = sawCompleted || item.Event.Type == unified.StreamEventCompleted
	}
	if !sawStarted || !sawCompleted {
		t.Fatalf("expected started and completed events, got started=%v completed=%v", sawStarted, sawCompleted)
	}
}
