package client

import (
	"context"
	"net/http"
	"testing"

	messagesapi "github.com/codewandler/agentapis/api/messages"
	responsesapi "github.com/codewandler/agentapis/api/responses"
	"github.com/codewandler/agentapis/api/unified"
)

func TestMuxClientRoutesAfterRequestTransform(t *testing.T) {
	t.Parallel()

	messageSSE := "event: message_start\n" +
		"data: {\"message\":{\"id\":\"msg_1\",\"model\":\"claude\",\"usage\":{\"input_tokens\":1}}}\n\n"
	responseSSE := "event: response.created\n" +
		"data: {\"response\":{\"id\":\"resp_1\",\"model\":\"gpt-5\"}}\n\n"

	messagesClient := NewMessagesClient(messagesapi.NewClient(
		messagesapi.WithBaseURL("https://example.com"),
		messagesapi.WithHTTPClient(&http.Client{Transport: FixedSSEResponse(http.StatusOK, messageSSE)}),
	))
	responsesClient := NewResponsesClient(responsesapi.NewClient(
		responsesapi.WithBaseURL("https://example.com"),
		responsesapi.WithHTTPClient(&http.Client{Transport: FixedSSEResponse(http.StatusOK, responseSSE)}),
	))

	mux := NewMuxClient(
		WithMessagesClient(messagesClient),
		WithResponsesClient(responsesClient),
		WithTargetResolver(func(_ context.Context, req *unified.Request) (Target, error) {
			if req.Model == "claude" {
				return TargetMessages, nil
			}
			return TargetResponses, nil
		}),
		WithMuxRequestTransform(func(_ context.Context, req *unified.Request) error {
			if req.Model == "anthropic/claude" {
				req.Model = "claude"
			}
			return nil
		}),
		WithMuxEventTransform(func(_ context.Context, ev unified.StreamEvent) (unified.StreamEvent, bool, error) {
			return ev, ev.Type == unified.StreamEventCompleted, nil
		}),
	)

	stream, err := mux.Stream(context.Background(), unified.Request{
		Model:     "anthropic/claude",
		MaxTokens: 16,
		Messages:  []unified.Message{{Role: unified.RoleUser, Parts: []unified.Part{{Type: unified.PartTypeText, Text: "hi"}}}},
	})
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}

	count := 0
	for item := range stream {
		if item.Err != nil {
			t.Fatalf("unexpected stream error: %v", item.Err)
		}
		count++
		if item.Event.Type != unified.StreamEventStarted {
			t.Fatalf("expected messages started event, got %q", item.Event.Type)
		}
	}
	if count != 1 {
		t.Fatalf("expected exactly one forwarded event, got %d", count)
	}
}

func TestMuxClientPreferredTargetOverridesResolver(t *testing.T) {
	t.Parallel()

	messageSSE := "event: message_start\n" +
		"data: {\"message\":{\"id\":\"msg_1\",\"model\":\"claude\",\"usage\":{\"input_tokens\":1}}}\n\n"
	responseSSE := "event: response.created\n" +
		"data: {\"response\":{\"id\":\"resp_1\",\"model\":\"gpt-5\"}}\n\n"

	messagesClient := NewMessagesClient(messagesapi.NewClient(
		messagesapi.WithBaseURL("https://example.com"),
		messagesapi.WithHTTPClient(&http.Client{Transport: FixedSSEResponse(http.StatusOK, messageSSE)}),
	))
	responsesClient := NewResponsesClient(responsesapi.NewClient(
		responsesapi.WithBaseURL("https://example.com"),
		responsesapi.WithHTTPClient(&http.Client{Transport: FixedSSEResponse(http.StatusOK, responseSSE)}),
	))

	mux := NewMuxClient(
		WithMessagesClient(messagesClient),
		WithResponsesClient(responsesClient),
		WithTargetResolver(func(_ context.Context, _ *unified.Request) (Target, error) {
			return TargetMessages, nil
		}),
	)

	preferred := TargetResponses
	stream, err := mux.StreamWithOptions(context.Background(), unified.Request{
		Model:    "gpt-5",
		Messages: []unified.Message{{Role: unified.RoleUser, Parts: []unified.Part{{Type: unified.PartTypeText, Text: "hi"}}}},
	}, StreamOptions{PreferredTarget: &preferred})
	if err != nil {
		t.Fatalf("StreamWithOptions() error = %v", err)
	}

	var sawStarted bool
	for item := range stream {
		if item.Err != nil {
			t.Fatalf("unexpected stream error: %v", item.Err)
		}
		if item.Event.Type == unified.StreamEventStarted {
			sawStarted = true
		}
	}
	if !sawStarted {
		t.Fatalf("expected preferred target responses to stream started event")
	}
}
