package openrouter

import (
	"errors"
	"fmt"

	"github.com/codewandler/agentapis/api/unified"
	"github.com/codewandler/agentapis/conversation"
)

// ErrResponsesReplayUnsupportedAssistantOrder indicates that canonical assistant
// history contains a mixed-content ordering that the OpenRouter Responses replay
// projection does not support faithfully.
var ErrResponsesReplayUnsupportedAssistantOrder = errors.New("conversation: openrouter responses replay does not support assistant text after tool calls")

// ConversationProjector validates replay safety for OpenRouter-style Responses
// projection while delegating baseline projection behavior to Base. It preserves
// canonical session history and fails early rather than rewriting unsupported
// assistant mixed-content ordering.
type ConversationProjector struct {
	Base conversation.MessageProjector
}

func (p ConversationProjector) ProjectMessages(state conversation.MessageProjectionState) ([]unified.Message, error) {
	base := p.Base
	if base == nil {
		base = conversation.DefaultMessageProjector{}
	}
	msgs, err := base.ProjectMessages(state)
	if err != nil {
		return nil, err
	}
	if state.Strategy != conversation.StrategyReplay && state.Strategy != conversation.StrategyAuto {
		return msgs, nil
	}
	for i, msg := range msgs {
		if msg.Role != unified.RoleAssistant {
			continue
		}
		seenToolCall := false
		for _, part := range msg.Parts {
			if part.ToolCall != nil || part.Type == unified.PartTypeToolCall {
				seenToolCall = true
				continue
			}
			if seenToolCall && part.Type == unified.PartTypeText && part.Text != "" {
				return nil, fmt.Errorf("%w (assistant message index %d)", ErrResponsesReplayUnsupportedAssistantOrder, i)
			}
		}
	}
	return msgs, nil
}
