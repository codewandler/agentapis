package adapt

import (
	"testing"

	"github.com/codewandler/agentapis/api/unified"
)

func TestBuildResponsesRequest_ThinkingPartsStripped(t *testing.T) {
	t.Parallel()

	req := unified.Request{
		Model: "codex-mini",
		Messages: []unified.Message{
			{Role: unified.RoleSystem, Parts: []unified.Part{{Type: unified.PartTypeText, Text: "You are helpful."}}},
			{Role: unified.RoleUser, Parts: []unified.Part{{Type: unified.PartTypeText, Text: "Hello"}}},
			{
				Role: unified.RoleAssistant,
				Parts: []unified.Part{
					{Type: unified.PartTypeThinking, Thinking: &unified.ThinkingPart{Text: "Let me think...", Signature: "sig123"}},
					{Type: unified.PartTypeText, Text: "Hi there!"},
				},
			},
			{Role: unified.RoleUser, Parts: []unified.Part{{Type: unified.PartTypeText, Text: "How are you?"}}},
		},
	}

	out, err := BuildResponsesRequest(req)
	if err != nil {
		t.Fatalf("BuildResponsesRequest returned error: %v", err)
	}

	// Should have 3 inputs: first user, assistant text, second user
	// (system goes to instructions, thinking is stripped)
	if len(out.Input) != 3 {
		t.Fatalf("expected 3 inputs, got %d: %+v", len(out.Input), out.Input)
	}

	// Verify assistant input has text but no thinking
	assistant := out.Input[1]
	if assistant.Role != "assistant" {
		t.Errorf("expected assistant role, got %q", assistant.Role)
	}
	if assistant.Content != "Hi there!" {
		t.Errorf("expected assistant content 'Hi there!', got %q", assistant.Content)
	}
}

func TestBuildResponsesRequest_ThinkingOnlyAssistant(t *testing.T) {
	t.Parallel()

	// Edge case: assistant message with ONLY thinking parts (no text)
	req := unified.Request{
		Model: "codex-mini",
		Messages: []unified.Message{
			{Role: unified.RoleUser, Parts: []unified.Part{{Type: unified.PartTypeText, Text: "Hello"}}},
			{
				Role: unified.RoleAssistant,
				Parts: []unified.Part{
					{Type: unified.PartTypeThinking, Thinking: &unified.ThinkingPart{Text: "thinking only"}},
				},
			},
			{Role: unified.RoleUser, Parts: []unified.Part{{Type: unified.PartTypeText, Text: "Next"}}},
		},
	}

	out, err := BuildResponsesRequest(req)
	if err != nil {
		t.Fatalf("BuildResponsesRequest returned error: %v", err)
	}

	// With thinking stripped and no text, the assistant produces 0 inputs.
	// We should have: user + user = 2 inputs
	if len(out.Input) != 2 {
		t.Fatalf("expected 2 inputs, got %d: %+v", len(out.Input), out.Input)
	}
}


func TestBuildResponsesRequest_AssistantMixedContentTextBeforeToolCallsProjects(t *testing.T) {
	t.Parallel()

	req := unified.Request{
		Model: "codex-mini",
		Messages: []unified.Message{
			{Role: unified.RoleUser, Parts: []unified.Part{{Type: unified.PartTypeText, Text: "Start"}}},
			{
				Role: unified.RoleAssistant,
				Parts: []unified.Part{
					{Type: unified.PartTypeThinking, Thinking: &unified.ThinkingPart{Text: "internal"}},
					{Type: unified.PartTypeText, Text: "I will call a tool."},
					{Type: unified.PartTypeToolCall, ToolCall: &unified.ToolCall{ID: "call_1", Name: "get_weather", Args: map[string]any{"city": "Berlin"}}},
				},
			},
		},
	}

	out, err := BuildResponsesRequest(req)
	if err != nil {
		t.Fatalf("BuildResponsesRequest returned error: %v", err)
	}
	if len(out.Input) != 3 {
		t.Fatalf("expected 3 inputs, got %d: %+v", len(out.Input), out.Input)
	}
	if out.Input[1].Role != "assistant" || out.Input[1].Content != "I will call a tool." {
		t.Fatalf("expected assistant text input first, got %+v", out.Input[1])
	}
	if out.Input[2].Type != "function_call" || out.Input[2].CallID != "call_1" || out.Input[2].Name != "get_weather" {
		t.Fatalf("expected assistant tool call input after text, got %+v", out.Input[2])
	}
}

func TestBuildResponsesRequest_AssistantToolCallThenTextFails(t *testing.T) {
	t.Parallel()

	req := unified.Request{
		Model: "codex-mini",
		Messages: []unified.Message{
			{Role: unified.RoleUser, Parts: []unified.Part{{Type: unified.PartTypeText, Text: "Start"}}},
			{
				Role: unified.RoleAssistant,
				Parts: []unified.Part{
					{Type: unified.PartTypeToolCall, ToolCall: &unified.ToolCall{ID: "call_1", Name: "get_weather", Args: map[string]any{"city": "Berlin"}}},
					{Type: unified.PartTypeText, Text: "The weather is sunny."},
				},
			},
		},
	}

	_, err := BuildResponsesRequest(req)
	if err == nil {
		t.Fatal("expected BuildResponsesRequest to reject assistant text after tool call for Responses API")
	}
	if got := err.Error(); got != "responses assistant message cannot contain text after tool calls" {
		t.Fatalf("unexpected error: %v", err)
	}
}
