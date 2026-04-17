package adapt

import (
	"testing"

	"github.com/codewandler/agentapis/api/unified"
)

func TestBuildCompletionsRequestIncludesStreamingDefaults(t *testing.T) {
	req := unified.Request{
		Model: "gpt-5",
		Messages: []unified.Message{
			{Role: unified.RoleUser, Parts: []unified.Part{{Type: unified.PartTypeText, Text: "hello"}}},
			{Role: unified.RoleAssistant, Parts: []unified.Part{{Type: unified.PartTypeToolCall, ToolCall: &unified.ToolCall{ID: "call_1", Name: "lookup", Args: map[string]any{"q": "x"}}}}},
			{Role: unified.RoleTool, Parts: []unified.Part{{Type: unified.PartTypeToolResult, ToolResult: &unified.ToolResult{ToolCallID: "call_1", ToolOutput: "done"}}}},
		},
		Tools: []unified.Tool{{Name: "lookup", Parameters: map[string]any{"type": "object"}}},
	}

	got, err := BuildCompletionsRequest(req)
	if err != nil {
		t.Fatalf("BuildCompletionsRequest() error = %v", err)
	}
	if !got.Stream {
		t.Fatalf("expected stream=true")
	}
	if got.StreamOptions == nil || !got.StreamOptions.IncludeUsage {
		t.Fatalf("expected stream_options.include_usage=true")
	}
	if len(got.Messages) != 3 {
		t.Fatalf("expected 3 wire messages, got %d", len(got.Messages))
	}
	if got.Messages[1].Role != "assistant" || len(got.Messages[1].ToolCalls) != 1 {
		t.Fatalf("expected assistant tool call message, got %#v", got.Messages[1])
	}
	if got.Messages[2].Role != "tool" || got.Messages[2].ToolCallID != "call_1" {
		t.Fatalf("expected tool result message, got %#v", got.Messages[2])
	}
}

func TestRequestFromCompletionsRoundTripsToolChoice(t *testing.T) {
	wire, err := BuildCompletionsRequest(unified.Request{
		Model:      "gpt-5",
		ToolChoice: unified.ToolChoiceTool{Name: "lookup"},
		Tools:      []unified.Tool{{Name: "lookup", Parameters: map[string]any{"type": "object"}}},
		Messages:   []unified.Message{{Role: unified.RoleUser, Parts: []unified.Part{{Type: unified.PartTypeText, Text: "hello"}}}},
	})
	if err != nil {
		t.Fatalf("BuildCompletionsRequest() error = %v", err)
	}

	got, err := RequestFromCompletions(*wire)
	if err != nil {
		t.Fatalf("RequestFromCompletions() error = %v", err)
	}
	tc, ok := got.ToolChoice.(unified.ToolChoiceTool)
	if !ok {
		t.Fatalf("expected ToolChoiceTool, got %T", got.ToolChoice)
	}
	if tc.Name != "lookup" {
		t.Fatalf("expected tool choice lookup, got %q", tc.Name)
	}
}
