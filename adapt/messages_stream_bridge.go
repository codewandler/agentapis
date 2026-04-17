package adapt

import (
	"encoding/json"

	"github.com/codewandler/agentapis/api/messages"
	"github.com/codewandler/agentapis/api/unified"
)

// MapMessagesEvent converts a Messages parser event into a unified stream event.
func MapMessagesEvent(ev messages.StreamEvent) (unified.StreamEvent, bool, error) {
	source := any(ev)
	switch e := ev.(type) {
	case *messages.MessageStartEvent:
		return withRawEventPayload(withRawEventName(unified.StreamEvent{
			Type:    unified.StreamEventStarted,
			Started: &unified.Started{RequestID: e.Message.ID, Model: e.Message.Model},
			Usage:   &unified.StreamUsage{Tokens: usageFromMessages(e.Message.Usage)},
		}, messages.EventMessageStart), source), false, nil

	case *messages.ContentBlockStartEvent:
		out := unified.StreamEvent{
			Type: unified.StreamEventLifecycle,
			Lifecycle: &unified.Lifecycle{
				Scope: unified.LifecycleScopeSegment,
				State: unified.LifecycleStateAdded,
				Ref:   unified.StreamRef{ItemIndex: uint32Ptr(e.Index)},
			},
		}
		var block messages.StartBlockView
		if err := json.Unmarshal(e.ContentBlock, &block); err == nil {
			out.Lifecycle.ItemType = block.Type
			out.Lifecycle.Kind, out.Lifecycle.Variant = messagesBlockKindVariant(block.Type)
			out.Extras.Provider = map[string]any{"content_block": providerMap(block)}
		} else {
			out.Extras.Provider = map[string]any{"content_block": string(e.ContentBlock)}
		}
		return withRawEventPayload(withRawEventName(out, messages.EventContentBlockStart), source), false, nil

	case *messages.ContentBlockDeltaEvent:
		ref := unified.StreamRef{ItemIndex: uint32Ptr(e.Index)}
		switch e.Delta.Type {
		case messages.DeltaTypeText:
			return withRawEventPayload(withRawEventName(unified.StreamEvent{
				Type: unified.StreamEventContentDelta,
				ContentDelta: &unified.ContentDelta{ContentBase: unified.ContentBase{
					Ref:      ref,
					Kind:     unified.ContentKindText,
					Variant:  unified.ContentVariantPrimary,
					Encoding: unified.ContentEncodingUTF8,
					Data:     e.Delta.Text,
				}},
				Delta: &unified.Delta{Kind: unified.DeltaKindText, Index: ref.ItemIndex, Text: e.Delta.Text},
			}, messages.EventContentBlockDelta), source), false, nil
		case messages.DeltaTypeThinking:
			return withRawEventPayload(withRawEventName(unified.StreamEvent{
				Type: unified.StreamEventContentDelta,
				ContentDelta: &unified.ContentDelta{ContentBase: unified.ContentBase{
					Ref:      ref,
					Kind:     unified.ContentKindReasoning,
					Encoding: unified.ContentEncodingUTF8,
					Data:     e.Delta.Thinking,
				}},
				Delta: &unified.Delta{Kind: unified.DeltaKindThinking, Index: ref.ItemIndex, Thinking: e.Delta.Thinking},
			}, messages.EventContentBlockDelta), source), false, nil
		case messages.DeltaTypeInputJSON:
			return withRawEventPayload(withRawEventName(unified.StreamEvent{
				Type: unified.StreamEventToolDelta,
				ToolDelta: &unified.ToolDelta{
					Ref:  ref,
					Kind: unified.ToolDeltaKindFunctionArguments,
					Data: e.Delta.PartialJSON,
				},
				Delta: &unified.Delta{Kind: unified.DeltaKindTool, Index: ref.ItemIndex, ToolArgs: e.Delta.PartialJSON},
			}, messages.EventContentBlockDelta), source), false, nil
		case messages.DeltaTypeSignature:
			return withRawEventPayload(withRawEventName(unified.StreamEvent{
				Type: unified.StreamEventContentDelta,
				ContentDelta: &unified.ContentDelta{ContentBase: unified.ContentBase{
					Ref:       ref,
					Kind:      unified.ContentKindReasoning,
					Signature: e.Delta.Signature,
				}},
			}, messages.EventContentBlockDelta), source), false, nil
		default:
			return withRawEventPayload(withProviderExtras(withRawEventName(unified.StreamEvent{Type: unified.StreamEventUnknown}, messages.EventContentBlockDelta), e), source), false, nil
		}

	case *messages.TextCompleteEvent:
		ref := unified.StreamRef{ItemIndex: uint32Ptr(e.Index)}
		return withRawEventPayload(withRawEventName(unified.StreamEvent{
			Type: unified.StreamEventContent,
			Lifecycle: &unified.Lifecycle{
				Scope:   unified.LifecycleScopeSegment,
				State:   unified.LifecycleStateDone,
				Ref:     ref,
				Kind:    unified.ContentKindText,
				Variant: unified.ContentVariantPrimary,
			},
			StreamContent: &unified.StreamContent{ContentBase: unified.ContentBase{
				Ref:      ref,
				Kind:     unified.ContentKindText,
				Variant:  unified.ContentVariantPrimary,
				Encoding: unified.ContentEncodingUTF8,
				Data:     e.Text,
			}},
			Content: &unified.ContentPart{Part: unified.Part{Type: unified.PartTypeText, Text: e.Text}, Index: e.Index},
		}, messages.EventContentBlockStop), source), false, nil

	case *messages.ThinkingCompleteEvent:
		ref := unified.StreamRef{ItemIndex: uint32Ptr(e.Index)}
		return withRawEventPayload(withRawEventName(unified.StreamEvent{
			Type: unified.StreamEventContent,
			Lifecycle: &unified.Lifecycle{
				Scope: unified.LifecycleScopeSegment,
				State: unified.LifecycleStateDone,
				Ref:   ref,
				Kind:  unified.ContentKindReasoning,
			},
			StreamContent: &unified.StreamContent{ContentBase: unified.ContentBase{
				Ref:       ref,
				Kind:      unified.ContentKindReasoning,
				Encoding:  unified.ContentEncodingUTF8,
				Data:      e.Thinking,
				Signature: e.Signature,
			}},
			Content: &unified.ContentPart{Part: unified.Part{
				Type: unified.PartTypeThinking,
				Thinking: &unified.ThinkingPart{
					Text:      e.Thinking,
					Signature: e.Signature,
				},
			}, Index: e.Index},
		}, messages.EventContentBlockStop), source), false, nil

	case *messages.ToolCompleteEvent:
		ref := unified.StreamRef{ItemIndex: uint32Ptr(e.Index)}
		return withRawEventPayload(withRawEventName(unified.StreamEvent{
			Type: unified.StreamEventToolCall,
			Lifecycle: &unified.Lifecycle{
				Scope:    unified.LifecycleScopeSegment,
				State:    unified.LifecycleStateDone,
				Ref:      ref,
				ItemType: messages.BlockTypeToolUse,
			},
			StreamToolCall: &unified.StreamToolCall{Ref: ref, ID: e.ID, Name: e.Name, RawInput: e.RawInput, Args: e.Args},
			ToolCall:       &unified.ToolCall{ID: e.ID, Name: e.Name, Args: e.Args},
		}, messages.EventContentBlockStop), source), false, nil

	case *messages.MessageDeltaEvent:
		return withRawEventPayload(withRawEventName(unified.StreamEvent{
			Type:      unified.StreamEventCompleted,
			Completed: &unified.Completed{StopReason: mapMessagesStopReason(e.Delta.StopReason)},
			Usage: &unified.StreamUsage{Tokens: usageFromMessagesFields(
				e.Usage.InputTokens,
				e.Usage.CacheCreationInputTokens,
				e.Usage.CacheReadInputTokens,
				e.Usage.OutputTokens,
			)},
		}, messages.EventMessageDelta), source), false, nil

	case *messages.StreamErrorEvent:
		return withRawEventPayload(withRawEventName(unified.StreamEvent{Type: unified.StreamEventError, Error: &unified.StreamError{Err: e}}, messages.EventError), source), false, nil

	case *messages.PingEvent, *messages.MessageStopEvent:
		return unified.StreamEvent{}, true, nil

	case *messages.ContentBlockStopEvent:
		return withRawEventPayload(withRawEventName(unified.StreamEvent{
			Type: unified.StreamEventLifecycle,
			Lifecycle: &unified.Lifecycle{
				Scope: unified.LifecycleScopeSegment,
				State: unified.LifecycleStateDone,
				Ref:   unified.StreamRef{ItemIndex: uint32Ptr(e.Index)},
			},
		}, messages.EventContentBlockStop), source), false, nil

	default:
		return unified.StreamEvent{Type: unified.StreamEventUnknown}, false, nil
	}
}

func messagesBlockKindVariant(blockType string) (unified.ContentKind, unified.ContentVariant) {
	switch blockType {
	case messages.BlockTypeText:
		return unified.ContentKindText, unified.ContentVariantPrimary
	case messages.BlockTypeThinking, messages.BlockTypeRedactedThinking:
		return unified.ContentKindReasoning, ""
	default:
		return "", ""
	}
}

func mapMessagesStopReason(s string) unified.StopReason {
	switch s {
	case messages.StopReasonEndTurn:
		return unified.StopReasonEndTurn
	case messages.StopReasonToolUse:
		return unified.StopReasonToolUse
	case messages.StopReasonMaxTok:
		return unified.StopReasonMaxTokens
	default:
		return unified.StopReason(s)
	}
}

func usageFromMessages(u messages.MessageUsage) unified.TokenItems {
	return usageFromMessagesFields(u.InputTokens, u.CacheCreationInputTokens, u.CacheReadInputTokens, 0)
}

func usageFromMessagesFields(input, cacheWrite, cacheRead, output int) unified.TokenItems {
	return unified.TokenItems{
		{Kind: unified.TokenKindInput, Count: input},
		{Kind: unified.TokenKindCacheWrite, Count: cacheWrite},
		{Kind: unified.TokenKindCacheRead, Count: cacheRead},
		{Kind: unified.TokenKindOutput, Count: output},
	}.NonZero()
}
