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
