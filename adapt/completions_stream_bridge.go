package adapt

import (
	"github.com/codewandler/agentapis/api/completions"
	"github.com/codewandler/agentapis/api/unified"
)

// MapCompletionsEvent converts a Chat Completions parser event into a unified stream event.
func MapCompletionsEvent(chunk *completions.Chunk) (unified.StreamEvent, bool, error) {
	source := any(chunk)
	if chunk == nil {
		return unified.StreamEvent{Type: unified.StreamEventUnknown}, false, nil
	}

	out := withRawEventPayload(withProviderExtras(unified.StreamEvent{}, chunk), source)
	if chunk.ID != "" || chunk.Model != "" {
		out.Type = unified.StreamEventStarted
		out.Started = &unified.Started{RequestID: chunk.ID, Model: chunk.Model}
	}

	if chunk.Usage != nil {
		cacheRead := 0
		reasoning := 0
		if chunk.Usage.PromptTokensDetails != nil {
			cacheRead = chunk.Usage.PromptTokensDetails.CachedTokens
		}
		if chunk.Usage.CompletionTokensDetails != nil {
			reasoning = chunk.Usage.CompletionTokensDetails.ReasoningTokens
		}
		newInput := chunk.Usage.PromptTokens - cacheRead
		if newInput < 0 {
			newInput = 0
		}
		output := chunk.Usage.CompletionTokens - reasoning
		if output < 0 {
			output = 0
		}
		tokens := unified.TokenItems{
			{Kind: unified.TokenKindInputNew, Count: newInput},
			{Kind: unified.TokenKindInputCacheRead, Count: cacheRead},
			{Kind: unified.TokenKindOutput, Count: output},
			{Kind: unified.TokenKindOutputReasoning, Count: reasoning},
		}.NonZero()
		out.Type = unified.StreamEventUsage
		out.Usage = &unified.StreamUsage{Input: tokens.InputTokens(), Output: tokens.OutputTokens(), Tokens: tokens}
	}

	if len(chunk.Choices) > 0 {
		choice := chunk.Choices[0]
		if choice.Delta.Content != "" {
			out.Type = unified.StreamEventContentDelta
			out.ContentDelta = &unified.ContentDelta{ContentBase: unified.ContentBase{
				Kind:     unified.ContentKindText,
				Variant:  unified.ContentVariantPrimary,
				Encoding: unified.ContentEncodingUTF8,
				Data:     choice.Delta.Content,
			}}
			out.Delta = &unified.Delta{Kind: unified.DeltaKindText, Text: choice.Delta.Content}
		}
		if len(choice.Delta.ToolCalls) > 0 {
			tc := choice.Delta.ToolCalls[0]
			ref := unified.StreamRef{ItemIndex: uint32Ptr(tc.Index)}
			out.Type = unified.StreamEventToolDelta
			out.ToolDelta = &unified.ToolDelta{Ref: ref, Kind: unified.ToolDeltaKindFunctionArguments, ToolID: tc.ID, ToolName: tc.Function.Name, Data: tc.Function.Arguments}
			out.Delta = &unified.Delta{Kind: unified.DeltaKindTool, Index: ref.ItemIndex, ToolID: tc.ID, ToolName: tc.Function.Name, ToolArgs: tc.Function.Arguments}
		}
		if choice.FinishReason != nil {
			out.Type = unified.StreamEventCompleted
			out.Completed = &unified.Completed{StopReason: mapOpenAIFinishReason(*choice.FinishReason)}
		}
	}

	if out.Started == nil && out.Delta == nil && out.Usage == nil && out.Completed == nil && out.ContentDelta == nil && out.ToolDelta == nil {
		return unified.StreamEvent{}, true, nil
	}
	return out, false, nil
}

func mapOpenAIFinishReason(s string) unified.StopReason {
	switch s {
	case completions.FinishReasonStop:
		return unified.StopReasonEndTurn
	case completions.FinishReasonToolCalls:
		return unified.StopReasonToolUse
	case completions.FinishReasonLength:
		return unified.StopReasonMaxTokens
	case completions.FinishReasonContentFilter:
		return unified.StopReasonContentFilter
	default:
		return unified.StopReason(s)
	}
}
