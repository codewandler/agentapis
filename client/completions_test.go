package client

import (
	"context"
	"net/http"
	"testing"

	completionsapi "github.com/codewandler/agentapis/api/completions"
	"github.com/codewandler/agentapis/api/unified"
)

func TestCompletionsClientAppliesUnifiedEventTransforms(t *testing.T) {
	t.Parallel()

	sseBody := "data: {\"id\":\"chatcmpl_1\",\"model\":\"gpt-5\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"hi\"}}]}\n\n" +
		"data: {\"id\":\"chatcmpl_1\",\"model\":\"gpt-5\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n" +
		"data: [DONE]\n\n"

	protocol := completionsapi.NewClient(
		completionsapi.WithBaseURL("https://example.com"),
		completionsapi.WithHTTPClient(&http.Client{Transport: FixedSSEResponse(http.StatusOK, sseBody)}),
	)

	client := NewCompletionsClient(protocol,
		WithEventTransform(func(_ context.Context, ev unified.StreamEvent) (unified.StreamEvent, bool, error) {
			return ev, ev.Type == unified.StreamEventCompleted, nil
		}),
	)

	stream, err := client.Stream(context.Background(), unified.Request{
		Model:    "gpt-5",
		Messages: []unified.Message{{Role: unified.RoleUser, Parts: []unified.Part{{Type: unified.PartTypeText, Text: "hi"}}}},
	})
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}

	var events []unified.StreamEvent
	for item := range stream {
		if item.Err != nil {
			t.Fatalf("unexpected stream error: %v", item.Err)
		}
		events = append(events, item.Event)
	}

	if len(events) == 0 {
		t.Fatalf("expected at least one unified event")
	}
	for _, event := range events {
		if event.Type == unified.StreamEventCompleted {
			t.Fatalf("expected completed event to be filtered")
		}
	}
}
