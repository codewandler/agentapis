package client

import (
	"context"
	"net/http"
	"testing"

	messagesapi "github.com/codewandler/agentapis/api/messages"
	"github.com/codewandler/agentapis/api/unified"
)

func TestMessagesClientStreamsUnifiedEvents(t *testing.T) {
	t.Parallel()

	sseBody := "event: message_start\n" +
		"data: {\"message\":{\"id\":\"msg_1\",\"model\":\"claude\",\"usage\":{\"input_tokens\":1}}}\n\n" +
		"event: message_delta\n" +
		"data: {\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":2}}\n\n"

	var gotWire messagesapi.Request
	protocol := messagesapi.NewClient(
		messagesapi.WithBaseURL("https://example.com"),
		messagesapi.WithHTTPClient(&http.Client{Transport: FixedSSEResponse(http.StatusOK, sseBody)}),
		messagesapi.WithRequestHook(func(_ context.Context, meta messagesapi.RequestMeta) { gotWire = *meta.Wire }),
	)

	client := NewMessagesClient(protocol,
		WithRequestTransform(func(_ context.Context, req *unified.Request) error {
			req.Model = "claude"
			return nil
		}),
	)

	stream, err := client.Stream(context.Background(), unified.Request{
		Model:     "alias",
		MaxTokens: 16,
		Messages:  []unified.Message{{Role: unified.RoleUser, Parts: []unified.Part{{Type: unified.PartTypeText, Text: "hi"}}}},
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

	if gotWire.Model != "claude" {
		t.Fatalf("expected transformed wire model, got %q", gotWire.Model)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 unified events, got %d", len(events))
	}
	if events[0].Type != unified.StreamEventStarted {
		t.Fatalf("expected started event, got %q", events[0].Type)
	}
	if events[1].Type != unified.StreamEventLifecycle {
		t.Fatalf("expected lifecycle event, got %q", events[1].Type)
	}
	if events[1].Lifecycle == nil || events[1].Lifecycle.Scope != unified.LifecycleScopeResponse || events[1].Lifecycle.State != unified.LifecycleStateDone {
		t.Fatalf("expected response done lifecycle, got %#v", events[1].Lifecycle)
	}
}

func TestMessagesClientEmitsCompletedOnMessageStop(t *testing.T) {
	t.Parallel()

	sseBody := "event: message_start\n" +
		"data: {\"message\":{\"id\":\"msg_1\",\"model\":\"claude\",\"usage\":{\"input_tokens\":1}}}\n\n" +
		"event: message_delta\n" +
		"data: {\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":2}}\n\n" +
		"event: message_stop\n" +
		"data: {}\n\n"

	protocol := messagesapi.NewClient(
		messagesapi.WithBaseURL("https://example.com"),
		messagesapi.WithHTTPClient(&http.Client{Transport: FixedSSEResponse(http.StatusOK, sseBody)}),
	)

	client := NewMessagesClient(protocol)
	stream, err := client.Stream(context.Background(), unified.Request{
		Model:     "claude",
		MaxTokens: 16,
		Messages:  []unified.Message{{Role: unified.RoleUser, Parts: []unified.Part{{Type: unified.PartTypeText, Text: "hi"}}}},
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

	if len(events) != 3 {
		t.Fatalf("expected 3 unified events, got %d", len(events))
	}
	if events[2].Type != unified.StreamEventCompleted || events[2].Completed == nil {
		t.Fatalf("expected completed event on message_stop, got %#v", events[2])
	}
}

func TestMessagesClientStreamWithOptionsForwardsMetadata(t *testing.T) {
	t.Parallel()

	sseBody := "event: message_start\n" +
		"data: {\"message\":{\"id\":\"msg_1\",\"model\":\"claude\",\"usage\":{\"input_tokens\":1}}}\n\n"

	protocol := messagesapi.NewClient(
		messagesapi.WithBaseURL("https://example.com"),
		messagesapi.WithHTTPClient(&http.Client{Transport: FixedSSEResponse(http.StatusOK, sseBody)}),
	)

	client := NewMessagesClient(protocol)
	var requestMeta RequestMeta
	var responseMeta ResponseMeta

	stream, err := client.StreamWithOptions(context.Background(), unified.Request{
		Model:     "claude",
		MaxTokens: 16,
		Messages:  []unified.Message{{Role: unified.RoleUser, Parts: []unified.Part{{Type: unified.PartTypeText, Text: "hi"}}}},
	}, StreamOptions{
		OnRequest: func(_ context.Context, meta RequestMeta) error {
			requestMeta = meta
			return nil
		},
		OnResponse: func(_ context.Context, meta ResponseMeta) error {
			responseMeta = meta
			return nil
		},
	})
	if err != nil {
		t.Fatalf("StreamWithOptions() error = %v", err)
	}
	for item := range stream {
		if item.Err != nil {
			t.Fatalf("unexpected stream error: %v", item.Err)
		}
	}

	if requestMeta.Target != TargetMessages || requestMeta.HTTP == nil || len(requestMeta.Body) == 0 {
		t.Fatalf("unexpected request meta: %#v", requestMeta)
	}
	if responseMeta.Target != TargetMessages || responseMeta.StatusCode != http.StatusOK {
		t.Fatalf("unexpected response meta: %#v", responseMeta)
	}
}

func TestMessagesClientStreamErrorsOnEmptyEventStream(t *testing.T) {
	t.Parallel()

	protocol := messagesapi.NewClient(
		messagesapi.WithBaseURL("https://example.com"),
		messagesapi.WithHTTPClient(&http.Client{Transport: FixedSSEResponse(http.StatusOK, "")}),
	)

	client := NewMessagesClient(protocol)
	stream, err := client.Stream(context.Background(), unified.Request{
		Model:     "claude",
		MaxTokens: 16,
		Messages:  []unified.Message{{Role: unified.RoleUser, Parts: []unified.Part{{Type: unified.PartTypeText, Text: "hi"}}}},
	})
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
		t.Fatal("expected stream error for empty event stream, got nil")
	}
}

func TestCostCalculatorEnrichesUsageEvents(t *testing.T) {
	t.Parallel()

	sseBody := "event: message_start\n" +
		"data: {\"message\":{\"id\":\"msg_1\",\"model\":\"claude\",\"usage\":{\"input_tokens\":100}}}\n\n" +
		"event: message_delta\n" +
		"data: {\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":50}}\n\n"

	protocol := messagesapi.NewClient(
		messagesapi.WithBaseURL("https://example.com"),
		messagesapi.WithHTTPClient(&http.Client{Transport: FixedSSEResponse(http.StatusOK, sseBody)}),
	)

	// Inject a cost calculator: $3/MTok input, $15/MTok output
	client := NewMessagesClient(protocol,
		WithCostCalculator(func(u unified.StreamUsage) unified.CostItems {
			var costs unified.CostItems
			if n := u.Tokens.Count(unified.TokenKindInputNew); n > 0 {
				costs = append(costs, unified.CostItem{Kind: unified.CostKindInput, Amount: float64(n) * 3.0 / 1_000_000})
			}
			if n := u.Tokens.Count(unified.TokenKindOutput); n > 0 {
				costs = append(costs, unified.CostItem{Kind: unified.CostKindOutput, Amount: float64(n) * 15.0 / 1_000_000})
			}
			return costs
		}),
	)

	stream, err := client.Stream(context.Background(), unified.Request{
		Model:     "claude",
		MaxTokens: 16,
		Messages:  []unified.Message{{Role: unified.RoleUser, Parts: []unified.Part{{Type: unified.PartTypeText, Text: "hi"}}}},
	})
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}

	var usageEvents []unified.StreamUsage
	for item := range stream {
		if item.Err != nil {
			t.Fatalf("unexpected stream error: %v", item.Err)
		}
		if item.Event.Usage != nil {
			usageEvents = append(usageEvents, *item.Event.Usage)
		}
	}

	if len(usageEvents) == 0 {
		t.Fatal("expected at least one usage event")
	}

	// The message_start event has input_tokens:100 → should have input cost
	firstUsage := usageEvents[0]
	if len(firstUsage.Costs) == 0 {
		t.Fatal("expected costs on first usage event")
	}
	inputCost := firstUsage.Costs.ByKind(unified.CostKindInput)
	wantInput := 100 * 3.0 / 1_000_000
	if inputCost != wantInput {
		t.Fatalf("expected input cost %g, got %g", wantInput, inputCost)
	}

	// The message_delta event has output_tokens:50 → should have output cost
	if len(usageEvents) < 2 {
		t.Fatal("expected two usage events")
	}
	secondUsage := usageEvents[1]
	outputCost := secondUsage.Costs.ByKind(unified.CostKindOutput)
	wantOutput := 50 * 15.0 / 1_000_000
	if outputCost != wantOutput {
		t.Fatalf("expected output cost %g, got %g", wantOutput, outputCost)
	}
}

func TestCostCalculatorNilIsNoOp(t *testing.T) {
	t.Parallel()

	sseBody := "event: message_start\n" +
		"data: {\"message\":{\"id\":\"msg_1\",\"model\":\"claude\",\"usage\":{\"input_tokens\":10}}}\n\n"

	protocol := messagesapi.NewClient(
		messagesapi.WithBaseURL("https://example.com"),
		messagesapi.WithHTTPClient(&http.Client{Transport: FixedSSEResponse(http.StatusOK, sseBody)}),
	)

	// No cost calculator — costs should remain nil
	client := NewMessagesClient(protocol)
	stream, err := client.Stream(context.Background(), unified.Request{
		Model:     "claude",
		MaxTokens: 16,
		Messages:  []unified.Message{{Role: unified.RoleUser, Parts: []unified.Part{{Type: unified.PartTypeText, Text: "hi"}}}},
	})
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}

	for item := range stream {
		if item.Err != nil {
			t.Fatalf("unexpected stream error: %v", item.Err)
		}
		if item.Event.Usage != nil && len(item.Event.Usage.Costs) > 0 {
			t.Fatalf("expected no costs without calculator, got %v", item.Event.Usage.Costs)
		}
	}
}

func TestCostCalculatorReturningNilDoesNotSetCosts(t *testing.T) {
	t.Parallel()

	sseBody := "event: message_start\n" +
		"data: {\"message\":{\"id\":\"msg_1\",\"model\":\"claude\",\"usage\":{\"input_tokens\":10}}}\n\n"

	protocol := messagesapi.NewClient(
		messagesapi.WithBaseURL("https://example.com"),
		messagesapi.WithHTTPClient(&http.Client{Transport: FixedSSEResponse(http.StatusOK, sseBody)}),
	)

	// Calculator returns nil → costs should remain nil
	client := NewMessagesClient(protocol,
		WithCostCalculator(func(u unified.StreamUsage) unified.CostItems {
			return nil
		}),
	)

	stream, err := client.Stream(context.Background(), unified.Request{
		Model:     "claude",
		MaxTokens: 16,
		Messages:  []unified.Message{{Role: unified.RoleUser, Parts: []unified.Part{{Type: unified.PartTypeText, Text: "hi"}}}},
	})
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}

	for item := range stream {
		if item.Err != nil {
			t.Fatalf("unexpected stream error: %v", item.Err)
		}
		if item.Event.Usage != nil && len(item.Event.Usage.Costs) > 0 {
			t.Fatalf("expected no costs when calculator returns nil, got %v", item.Event.Usage.Costs)
		}
	}
}