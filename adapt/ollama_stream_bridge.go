package adapt

import (
	"fmt"

	"github.com/codewandler/agentapis/api/ollama"
	"github.com/codewandler/agentapis/api/unified"
)

type OllamaMapper struct {
	started    bool
	responseID string
}

func NewOllamaMapper() *OllamaMapper { return &OllamaMapper{} }

func (m *OllamaMapper) MapEvent(ev *ollama.Response) (unified.StreamEvent, bool, error) {
	source := any(ev)
	if ev == nil {
		return unified.StreamEvent{}, true, nil
	}
	ref := unified.StreamRef{ResponseID: m.ensureResponseID(ev)}
	if ev.Message.Thinking != "" {
		return withRawEventPayload(withProviderExtras(unified.StreamEvent{
			Type: unified.StreamEventContentDelta,
			ContentDelta: &unified.ContentDelta{ContentBase: unified.ContentBase{
				Ref:      ref,
				Kind:     unified.ContentKindReasoning,
				Variant:  unified.ContentVariantRaw,
				Encoding: unified.ContentEncodingUTF8,
				Data:     ev.Message.Thinking,
			}},
			Delta: &unified.Delta{Kind: unified.DeltaKindThinking, Thinking: ev.Message.Thinking},
		}, ev), source), false, nil
	}
	if ev.Message.Content != "" {
		return withRawEventPayload(withProviderExtras(unified.StreamEvent{
			Type: unified.StreamEventContentDelta,
			ContentDelta: &unified.ContentDelta{ContentBase: unified.ContentBase{
				Ref:      ref,
				Kind:     unified.ContentKindText,
				Variant:  unified.ContentVariantPrimary,
				Encoding: unified.ContentEncodingUTF8,
				Data:     ev.Message.Content,
			}},
			Delta: &unified.Delta{Kind: unified.DeltaKindText, Text: ev.Message.Content},
		}, ev), source), false, nil
	}
	if len(ev.Message.ToolCalls) > 0 {
		tc := ev.Message.ToolCalls[0]
		return withRawEventPayload(withProviderExtras(unified.StreamEvent{
			Type: unified.StreamEventToolCall,
			StreamToolCall: &unified.StreamToolCall{
				Ref:      ref,
				Name:     tc.Function.Name,
				RawInput: mustJSON(tc.Function.Arguments),
				Args:     cloneAnyMap(tc.Function.Arguments),
			},
			ToolCall: &unified.ToolCall{Name: tc.Function.Name, Args: cloneAnyMap(tc.Function.Arguments)},
		}, ev), source), false, nil
	}
	if !m.started && ev.Model != "" && !ev.Done {
		m.started = true
		return withRawEventPayload(withProviderExtras(unified.StreamEvent{Type: unified.StreamEventStarted, Started: &unified.Started{RequestID: ref.ResponseID, Model: ev.Model, Provider: "ollama"}}, ev), source), false, nil
	}
	if ev.Done {
		out := unified.StreamEvent{Type: unified.StreamEventCompleted, Completed: &unified.Completed{StopReason: mapOllamaDoneReason(ev.DoneReason)}}
		if ev.PromptEvalCount > 0 || ev.EvalCount > 0 {
			out.Usage = &unified.StreamUsage{Provider: "ollama", Model: ev.Model, RequestID: ref.ResponseID, Tokens: unified.TokenItems{{Kind: unified.TokenKindInput, Count: ev.PromptEvalCount}, {Kind: unified.TokenKindOutput, Count: ev.EvalCount}}.NonZero()}
		}
		if ev.DoneReason != "" && out.Extras.Provider == nil {
			out.Extras.Provider = map[string]any{"done_reason": ev.DoneReason}
		}
		return withRawEventPayload(withProviderExtras(out, ev), source), false, nil
	}
	return unified.StreamEvent{}, true, nil
}

func (m *OllamaMapper) ensureResponseID(ev *ollama.Response) string {
	if m.responseID != "" {
		return m.responseID
	}
	switch {
	case ev.CreatedAt != "" && ev.Model != "":
		m.responseID = fmt.Sprintf("ollama:%s:%s", ev.Model, ev.CreatedAt)
	case ev.CreatedAt != "":
		m.responseID = "ollama:" + ev.CreatedAt
	case ev.Model != "":
		m.responseID = "ollama:" + ev.Model
	default:
		m.responseID = "ollama:stream"
	}
	return m.responseID
}

func mapOllamaDoneReason(s string) unified.StopReason {
	switch s {
	case "stop":
		return unified.StopReasonEndTurn
	case "length":
		return unified.StopReasonMaxTokens
	default:
		return ""
	}
}
