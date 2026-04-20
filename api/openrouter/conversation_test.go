package openrouter

import (
	"errors"
	"testing"

	"github.com/codewandler/agentapis/api/unified"
	"github.com/codewandler/agentapis/conversation"
)

func TestConversationProjectorRejectsAssistantTextAfterToolCall(t *testing.T) {
	t.Parallel()
	proj := ConversationProjector{}
	_, err := proj.ProjectMessages(conversation.MessageProjectionState{
		Strategy: conversation.StrategyReplay,
		History: []unified.Message{{
			Role: unified.RoleAssistant,
			Parts: []unified.Part{
				{Type: unified.PartTypeToolCall, ToolCall: &unified.ToolCall{ID: "call_1", Name: "weather", Args: map[string]any{"city": "Berlin"}}},
				{Type: unified.PartTypeText, Text: "and now some text"},
			},
		}},
	})
	if err == nil {
		t.Fatal("expected replay projector to reject assistant text after tool call")
	}
	if !errors.Is(err, ErrResponsesReplayUnsupportedAssistantOrder) {
		t.Fatalf("expected ErrResponsesReplayUnsupportedAssistantOrder, got %v", err)
	}
}

func TestConversationProjectorAllowsAssistantTextBeforeToolCall(t *testing.T) {
	t.Parallel()
	proj := ConversationProjector{}
	msgs, err := proj.ProjectMessages(conversation.MessageProjectionState{
		Strategy: conversation.StrategyReplay,
		History: []unified.Message{{
			Role: unified.RoleAssistant,
			Parts: []unified.Part{
				{Type: unified.PartTypeText, Text: "I will call a tool."},
				{Type: unified.PartTypeToolCall, ToolCall: &unified.ToolCall{ID: "call_1", Name: "weather", Args: map[string]any{"city": "Berlin"}}},
			},
		}},
		Pending: []unified.Message{{Role: unified.RoleUser, Parts: []unified.Part{{Type: unified.PartTypeText, Text: "continue"}}}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(msgs) != 2 || msgs[0].Role != unified.RoleAssistant || msgs[1].Role != unified.RoleUser {
		t.Fatalf("unexpected projected messages: %#v", msgs)
	}
}
