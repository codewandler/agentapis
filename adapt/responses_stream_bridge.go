package adapt

import (
	"encoding/json"
	"fmt"

	"github.com/codewandler/agentapis/api/responses"
	"github.com/codewandler/agentapis/api/unified"
)

type funcCallMeta struct {
	name   string
	callID string
}

// ResponsesMapper converts Responses API events to unified stream events.
type ResponsesMapper struct {
	pending map[int]funcCallMeta
}

func NewResponsesMapper() *ResponsesMapper {
	return &ResponsesMapper{pending: make(map[int]funcCallMeta)}
}

func (m *ResponsesMapper) MapEvent(ev responses.StreamEvent) (unified.StreamEvent, bool, error) {
	source := any(ev)
	switch e := ev.(type) {
	case *responses.ResponseCreatedEvent:
		return withRawEventPayload(withRawEventName(unified.StreamEvent{Type: unified.StreamEventStarted, Started: &unified.Started{RequestID: e.Response.ID, Model: e.Response.Model}}, responses.EventResponseCreated), source), false, nil

	case *responses.ResponseQueuedEvent:
		return withRawEventPayload(withProviderExtras(withRawEventName(unified.StreamEvent{Type: unified.StreamEventLifecycle, Lifecycle: &unified.Lifecycle{Scope: unified.LifecycleScopeResponse, State: unified.LifecycleStateQueued, Ref: unified.StreamRef{ResponseID: e.Response.ID}}}, responses.EventResponseQueued), e), source), false, nil

	case *responses.ResponseInProgressEvent:
		return withRawEventPayload(withProviderExtras(withRawEventName(unified.StreamEvent{Type: unified.StreamEventLifecycle, Lifecycle: &unified.Lifecycle{Scope: unified.LifecycleScopeResponse, State: unified.LifecycleStateInProgress, Ref: unified.StreamRef{ResponseID: e.Response.ID}}}, responses.EventResponseInProgress), e), source), false, nil

	case *responses.ResponseFailedEvent:
		out := unified.StreamEvent{Type: unified.StreamEventCompleted, Lifecycle: &unified.Lifecycle{Scope: unified.LifecycleScopeResponse, State: unified.LifecycleStateFailed, Ref: unified.StreamRef{ResponseID: e.Response.ID}}, Completed: &unified.Completed{StopReason: unified.StopReasonError}}
		if u := usageFromResponses(e.Response.Usage); u != nil {
			out.Usage = u
		}
		if e.Response.Error != nil {
			out.Error = &unified.StreamError{Err: fmt.Errorf("responses response failed %s: %s", e.Response.Error.Code, e.Response.Error.Message)}
		}
		return withRawEventPayload(withProviderExtras(withRawEventName(out, responses.EventResponseFailed), e), source), false, nil

	case *responses.ResponseIncompleteEvent:
		out := unified.StreamEvent{Type: unified.StreamEventCompleted, Lifecycle: &unified.Lifecycle{Scope: unified.LifecycleScopeResponse, State: unified.LifecycleStateIncomplete, Ref: unified.StreamRef{ResponseID: e.Response.ID}}, Completed: &unified.Completed{StopReason: mapResponsesIncompleteReason(e.Response.IncompleteDetails)}}
		if u := usageFromResponses(e.Response.Usage); u != nil {
			out.Usage = u
		}
		return withRawEventPayload(withProviderExtras(withRawEventName(out, responses.EventResponseIncomplete), e), source), false, nil

	case *responses.OutputTextDeltaEvent:
		ref := responsesContentRef(e.OutputIndex, e.ItemID, e.ContentIndex)
		out := unified.StreamEvent{Type: unified.StreamEventContentDelta, ContentDelta: &unified.ContentDelta{ContentBase: unified.ContentBase{Ref: ref, Kind: unified.ContentKindText, Variant: unified.ContentVariantPrimary, Encoding: unified.ContentEncodingUTF8, Data: e.Delta}}, Delta: &unified.Delta{Kind: unified.DeltaKindText, Index: ref.ItemIndex, Text: e.Delta}}
		if len(e.Logprobs) > 0 {
			out.Extras.Provider = map[string]any{"logprobs": providerMap(e.Logprobs)}
		}
		return withRawEventPayload(withRawEventName(out, responses.EventOutputTextDelta), source), false, nil

	case *responses.OutputTextDoneEvent:
		ref := responsesContentRef(e.OutputIndex, e.ItemID, e.ContentIndex)
		out := unified.StreamEvent{Type: unified.StreamEventContent, StreamContent: &unified.StreamContent{ContentBase: unified.ContentBase{Ref: ref, Kind: unified.ContentKindText, Variant: unified.ContentVariantPrimary, Encoding: unified.ContentEncodingUTF8, Data: e.Text}}, Content: &unified.ContentPart{Part: unified.Part{Type: unified.PartTypeText, Text: e.Text}, Index: e.ContentIndex}}
		if len(e.Logprobs) > 0 {
			out.Extras.Provider = map[string]any{"logprobs": providerMap(e.Logprobs)}
		}
		return withRawEventPayload(withRawEventName(out, responses.EventOutputTextDone), source), false, nil

	case *responses.OutputTextAnnotationAddedEvent:
		return withRawEventPayload(withRawEventName(unified.StreamEvent{Type: unified.StreamEventAnnotation, Annotation: &unified.Annotation{Ref: responsesAnnotationRef(e.OutputIndex, e.ItemID, e.ContentIndex, e.AnnotationIndex), Type: e.Annotation.Type, Text: e.Annotation.Text, FileID: e.Annotation.FileID, Filename: e.Annotation.Filename, URL: e.Annotation.URL, Title: e.Annotation.Title, ContainerID: e.Annotation.ContainerID, StartIndex: e.Annotation.StartIndex, EndIndex: e.Annotation.EndIndex, Offset: e.Annotation.Offset, Index: e.Annotation.Index}}, responses.EventOutputTextAnnotationAdded), source), false, nil

	case *responses.RefusalDeltaEvent:
		ref := responsesContentRef(e.OutputIndex, e.ItemID, e.ContentIndex)
		return withRawEventPayload(withRawEventName(unified.StreamEvent{Type: unified.StreamEventContentDelta, ContentDelta: &unified.ContentDelta{ContentBase: unified.ContentBase{Ref: ref, Kind: unified.ContentKindRefusal, Variant: unified.ContentVariantPrimary, Encoding: unified.ContentEncodingUTF8, Data: e.Delta}}}, responses.EventRefusalDelta), source), false, nil

	case *responses.RefusalDoneEvent:
		ref := responsesContentRef(e.OutputIndex, e.ItemID, e.ContentIndex)
		return withRawEventPayload(withRawEventName(unified.StreamEvent{Type: unified.StreamEventContent, StreamContent: &unified.StreamContent{ContentBase: unified.ContentBase{Ref: ref, Kind: unified.ContentKindRefusal, Variant: unified.ContentVariantPrimary, Encoding: unified.ContentEncodingUTF8, Data: e.Refusal}}}, responses.EventRefusalDone), source), false, nil

	case *responses.ReasoningTextDeltaEvent:
		ref := responsesItemRef(e.OutputIndex, e.ItemID)
		return withRawEventPayload(withRawEventName(unified.StreamEvent{Type: unified.StreamEventContentDelta, ContentDelta: &unified.ContentDelta{ContentBase: unified.ContentBase{Ref: ref, Kind: unified.ContentKindReasoning, Variant: unified.ContentVariantRaw, Encoding: unified.ContentEncodingUTF8, Data: e.Delta}}, Delta: &unified.Delta{Kind: unified.DeltaKindThinking, Index: ref.ItemIndex, Thinking: e.Delta}}, responses.EventReasoningTextDelta), source), false, nil

	case *responses.ReasoningTextDoneEvent:
		ref := responsesItemRef(e.OutputIndex, e.ItemID)
		return withRawEventPayload(withRawEventName(unified.StreamEvent{Type: unified.StreamEventContent, StreamContent: &unified.StreamContent{ContentBase: unified.ContentBase{Ref: ref, Kind: unified.ContentKindReasoning, Variant: unified.ContentVariantRaw, Encoding: unified.ContentEncodingUTF8, Data: e.Text}}, Content: &unified.ContentPart{Part: unified.Part{Type: unified.PartTypeThinking, Thinking: &unified.ThinkingPart{Text: e.Text}}, Index: e.OutputIndex}}, responses.EventReasoningTextDone), source), false, nil

	case *responses.ReasoningSummaryPartAddedEvent:
		return withRawEventPayload(withRawEventName(unified.StreamEvent{Type: unified.StreamEventLifecycle, Lifecycle: &unified.Lifecycle{Scope: unified.LifecycleScopeSegment, State: unified.LifecycleStateAdded, Ref: responsesSummaryRef(e.OutputIndex, e.ItemID, e.SummaryIndex), Kind: unified.ContentKindReasoning, Variant: unified.ContentVariantSummary}}, responses.EventReasoningSummaryPartAdded), source), false, nil

	case *responses.ReasoningSummaryPartDoneEvent:
		return withRawEventPayload(withRawEventName(unified.StreamEvent{Type: unified.StreamEventLifecycle, Lifecycle: &unified.Lifecycle{Scope: unified.LifecycleScopeSegment, State: unified.LifecycleStateDone, Ref: responsesSummaryRef(e.OutputIndex, e.ItemID, e.SummaryIndex), Kind: unified.ContentKindReasoning, Variant: unified.ContentVariantSummary}}, responses.EventReasoningSummaryPartDone), source), false, nil

	case *responses.ReasoningSummaryTextDeltaEvent:
		ref := responsesItemRef(e.OutputIndex, e.ItemID)
		return withRawEventPayload(withRawEventName(unified.StreamEvent{Type: unified.StreamEventContentDelta, ContentDelta: &unified.ContentDelta{ContentBase: unified.ContentBase{Ref: ref, Kind: unified.ContentKindReasoning, Variant: unified.ContentVariantSummary, Encoding: unified.ContentEncodingUTF8, Data: e.Delta}}, Delta: &unified.Delta{Kind: unified.DeltaKindThinking, Index: ref.ItemIndex, Thinking: e.Delta}}, responses.EventReasoningSummaryTextDelta), source), false, nil

	case *responses.ReasoningSummaryTextDoneEvent:
		ref := responsesItemRef(e.OutputIndex, e.ItemID)
		return withRawEventPayload(withRawEventName(unified.StreamEvent{Type: unified.StreamEventContent, StreamContent: &unified.StreamContent{ContentBase: unified.ContentBase{Ref: ref, Kind: unified.ContentKindReasoning, Variant: unified.ContentVariantSummary, Encoding: unified.ContentEncodingUTF8, Data: e.Text}}, Content: &unified.ContentPart{Part: unified.Part{Type: unified.PartTypeThinking, Thinking: &unified.ThinkingPart{Text: e.Text}}, Index: e.OutputIndex}}, responses.EventReasoningSummaryTextDone), source), false, nil

	case *responses.FunctionCallArgumentsDeltaEvent:
		ref := responsesItemRef(e.OutputIndex, e.ItemID)
		return withRawEventPayload(withRawEventName(unified.StreamEvent{Type: unified.StreamEventToolDelta, ToolDelta: &unified.ToolDelta{Ref: ref, Kind: unified.ToolDeltaKindFunctionArguments, Data: e.Delta}, Delta: &unified.Delta{Kind: unified.DeltaKindTool, Index: ref.ItemIndex, ToolArgs: e.Delta}}, responses.EventFunctionCallArgumentsDelta), source), false, nil

	case *responses.FunctionCallArgumentsDoneEvent:
		var args map[string]any
		_ = json.Unmarshal([]byte(e.Arguments), &args)
		name, callID := e.Name, e.ItemID
		if meta, ok := m.pending[e.OutputIndex]; ok {
			if name == "" {
				name = meta.name
			}
			if meta.callID != "" {
				callID = meta.callID
			}
			delete(m.pending, e.OutputIndex)
		}
		return withRawEventPayload(withRawEventName(unified.StreamEvent{Type: unified.StreamEventToolCall, StreamToolCall: &unified.StreamToolCall{Ref: responsesItemRef(e.OutputIndex, e.ItemID), ID: callID, Name: name, RawInput: e.Arguments, Args: args}, ToolCall: &unified.ToolCall{ID: callID, Name: name, Args: args}}, responses.EventFunctionCallArgumentsDone), source), false, nil

	case *responses.CustomToolCallInputDeltaEvent:
		return withRawEventPayload(withRawEventName(unified.StreamEvent{Type: unified.StreamEventToolDelta, ToolDelta: &unified.ToolDelta{Ref: responsesItemRef(e.OutputIndex, e.ItemID), Kind: unified.ToolDeltaKindCustomInput, Data: e.Delta}}, responses.EventCustomToolCallInputDelta), source), false, nil

	case *responses.CustomToolCallInputDoneEvent:
		return withRawEventPayload(withRawEventName(unified.StreamEvent{Type: unified.StreamEventToolDelta, ToolDelta: &unified.ToolDelta{Ref: responsesItemRef(e.OutputIndex, e.ItemID), Kind: unified.ToolDeltaKindCustomInput, Data: e.Input, Final: true}}, responses.EventCustomToolCallInputDone), source), false, nil

	case *responses.OutputItemAddedEvent:
		if e.Item.Type == "function_call" && (e.Item.Name != "" || e.Item.CallID != "") {
			m.pending[e.OutputIndex] = funcCallMeta{name: e.Item.Name, callID: e.Item.CallID}
		}
		return withRawEventPayload(withProviderExtras(withRawEventName(unified.StreamEvent{Type: unified.StreamEventLifecycle, Lifecycle: &unified.Lifecycle{Scope: unified.LifecycleScopeItem, State: unified.LifecycleStateAdded, Ref: responsesItemRef(e.OutputIndex, e.Item.ID), ItemType: e.Item.Type}}, responses.EventOutputItemAdded), map[string]any{"item": e.Item}), source), false, nil

	case *responses.OutputItemDoneEvent:
		return withRawEventPayload(withProviderExtras(withRawEventName(unified.StreamEvent{Type: unified.StreamEventLifecycle, Lifecycle: &unified.Lifecycle{Scope: unified.LifecycleScopeItem, State: unified.LifecycleStateDone, Ref: responsesItemRef(e.OutputIndex, e.Item.ID), ItemType: e.Item.Type}}, responses.EventOutputItemDone), map[string]any{"item": e.Item}), source), false, nil

	case *responses.ContentPartAddedEvent:
		kind, variant := responsesPartKindVariant(e.Part.Type)
		return withRawEventPayload(withProviderExtras(withRawEventName(unified.StreamEvent{Type: unified.StreamEventLifecycle, Lifecycle: &unified.Lifecycle{Scope: unified.LifecycleScopeSegment, State: unified.LifecycleStateAdded, Ref: responsesContentRef(e.OutputIndex, e.ItemID, e.ContentIndex), Kind: kind, Variant: variant, Mime: responsesPartMime(e.Part.Type)}}, responses.EventContentPartAdded), map[string]any{"part": e.Part}), source), false, nil

	case *responses.ContentPartDoneEvent:
		kind, variant := responsesPartKindVariant(e.Part.Type)
		return withRawEventPayload(withProviderExtras(withRawEventName(unified.StreamEvent{Type: unified.StreamEventLifecycle, Lifecycle: &unified.Lifecycle{Scope: unified.LifecycleScopeSegment, State: unified.LifecycleStateDone, Ref: responsesContentRef(e.OutputIndex, e.ItemID, e.ContentIndex), Kind: kind, Variant: variant, Mime: responsesPartMime(e.Part.Type)}}, responses.EventContentPartDone), map[string]any{"part": e.Part}), source), false, nil

	case *responses.AudioDeltaEvent:
		return withRawEventPayload(withRawEventName(unified.StreamEvent{Type: unified.StreamEventContentDelta, ContentDelta: &unified.ContentDelta{ContentBase: unified.ContentBase{Ref: unified.StreamRef{ResponseID: e.ResponseID}, Kind: unified.ContentKindMedia, Variant: unified.ContentVariantPrimary, Encoding: unified.ContentEncodingBase64, Data: e.Delta}}}, responses.EventAudioDelta), source), false, nil

	case *responses.AudioDoneEvent:
		return withRawEventPayload(withRawEventName(unified.StreamEvent{Type: unified.StreamEventContentDelta, ContentDelta: &unified.ContentDelta{ContentBase: unified.ContentBase{Ref: unified.StreamRef{ResponseID: e.ResponseID}, Kind: unified.ContentKindMedia, Variant: unified.ContentVariantPrimary, Encoding: unified.ContentEncodingBase64}, Final: true}}, responses.EventAudioDone), source), false, nil

	case *responses.AudioTranscriptDeltaEvent:
		return withRawEventPayload(withRawEventName(unified.StreamEvent{Type: unified.StreamEventContentDelta, ContentDelta: &unified.ContentDelta{ContentBase: unified.ContentBase{Ref: unified.StreamRef{ResponseID: e.ResponseID}, Kind: unified.ContentKindMedia, Variant: unified.ContentVariantTranscript, Encoding: unified.ContentEncodingUTF8, Data: e.Delta}}}, responses.EventAudioTranscriptDelta), source), false, nil

	case *responses.AudioTranscriptDoneEvent:
		return withRawEventPayload(withRawEventName(unified.StreamEvent{Type: unified.StreamEventContentDelta, ContentDelta: &unified.ContentDelta{ContentBase: unified.ContentBase{Ref: unified.StreamRef{ResponseID: e.ResponseID}, Kind: unified.ContentKindMedia, Variant: unified.ContentVariantTranscript, Encoding: unified.ContentEncodingUTF8}, Final: true}}, responses.EventAudioTranscriptDone), source), false, nil

	case *responses.ResponseCompletedEvent:
		out := unified.StreamEvent{Type: unified.StreamEventCompleted, Lifecycle: &unified.Lifecycle{Scope: unified.LifecycleScopeResponse, State: unified.LifecycleStateDone, Ref: unified.StreamRef{ResponseID: e.Response.ID}}, Completed: &unified.Completed{StopReason: unified.StopReasonEndTurn}}
		if u := usageFromResponses(e.Response.Usage); u != nil {
			out.Usage = u
		}
		return withRawEventPayload(withProviderExtras(withRawEventName(out, responses.EventResponseCompleted), e), source), false, nil

	case *responses.APIErrorEvent:
		return withRawEventPayload(withRawEventName(unified.StreamEvent{Type: unified.StreamEventError, Error: &unified.StreamError{Err: e}}, responses.EventAPIError), source), false, nil

	case *responses.FileSearchCallInProgressEvent:
		return unknownResponsesEvent(responses.EventFileSearchCallInProgress, source, e), false, nil
	case *responses.FileSearchCallSearchingEvent:
		return unknownResponsesEvent(responses.EventFileSearchCallSearching, source, e), false, nil
	case *responses.FileSearchCallCompletedEvent:
		return unknownResponsesEvent(responses.EventFileSearchCallCompleted, source, e), false, nil
	case *responses.WebSearchCallInProgressEvent:
		return unknownResponsesEvent(responses.EventWebSearchCallInProgress, source, e), false, nil
	case *responses.WebSearchCallSearchingEvent:
		return unknownResponsesEvent(responses.EventWebSearchCallSearching, source, e), false, nil
	case *responses.WebSearchCallCompletedEvent:
		return unknownResponsesEvent(responses.EventWebSearchCallCompleted, source, e), false, nil
	case *responses.MCPCallArgumentsDeltaEvent:
		return unknownResponsesEvent(responses.EventMCPCallArgumentsDelta, source, e), false, nil
	case *responses.MCPCallArgumentsDoneEvent:
		return unknownResponsesEvent(responses.EventMCPCallArgumentsDone, source, e), false, nil
	case *responses.MCPCallCompletedEvent:
		return unknownResponsesEvent(responses.EventMCPCallCompleted, source, e), false, nil
	case *responses.MCPCallFailedEvent:
		return unknownResponsesEvent(responses.EventMCPCallFailed, source, e), false, nil
	case *responses.MCPCallInProgressEvent:
		return unknownResponsesEvent(responses.EventMCPCallInProgress, source, e), false, nil
	case *responses.MCPListToolsCompletedEvent:
		return unknownResponsesEvent(responses.EventMCPListToolsCompleted, source, e), false, nil
	case *responses.MCPListToolsFailedEvent:
		return unknownResponsesEvent(responses.EventMCPListToolsFailed, source, e), false, nil
	case *responses.MCPListToolsInProgressEvent:
		return unknownResponsesEvent(responses.EventMCPListToolsInProgress, source, e), false, nil
	case *responses.CodeInterpreterCallInProgressEvent:
		return unknownResponsesEvent(responses.EventCodeInterpreterCallInProgress, source, e), false, nil
	case *responses.CodeInterpreterCallInterpretingEvent:
		return unknownResponsesEvent(responses.EventCodeInterpreterCallInterpreting, source, e), false, nil
	case *responses.CodeInterpreterCallCompletedEvent:
		return unknownResponsesEvent(responses.EventCodeInterpreterCallCompleted, source, e), false, nil
	case *responses.CodeInterpreterCallCodeDeltaEvent:
		return unknownResponsesEvent(responses.EventCodeInterpreterCallCodeDelta, source, e), false, nil
	case *responses.CodeInterpreterCallCodeDoneEvent:
		return unknownResponsesEvent(responses.EventCodeInterpreterCallCodeDone, source, e), false, nil
	case *responses.ImageGenerationCallCompletedEvent:
		return unknownResponsesEvent(responses.EventImageGenerationCallCompleted, source, e), false, nil
	case *responses.ImageGenerationCallGeneratingEvent:
		return unknownResponsesEvent(responses.EventImageGenerationCallGenerating, source, e), false, nil
	case *responses.ImageGenerationCallInProgressEvent:
		return unknownResponsesEvent(responses.EventImageGenerationCallInProgress, source, e), false, nil
	case *responses.ImageGenerationCallPartialImageEvent:
		return unknownResponsesEvent(responses.EventImageGenerationCallPartialImage, source, e), false, nil

	default:
		return unified.StreamEvent{Type: unified.StreamEventUnknown}, false, nil
	}
}

// MapResponsesEvent is a stateless convenience wrapper around ResponsesMapper.MapEvent.
func MapResponsesEvent(ev responses.StreamEvent) (unified.StreamEvent, bool, error) {
	return NewResponsesMapper().MapEvent(ev)
}

func unknownResponsesEvent(name string, source any, provider any) unified.StreamEvent {
	return withRawEventPayload(withProviderExtras(withRawEventName(unified.StreamEvent{Type: unified.StreamEventUnknown}, name), provider), source)
}

func responsesItemRef(outputIndex int, itemID string) unified.StreamRef {
	return unified.StreamRef{ItemIndex: uint32Ptr(outputIndex), ItemID: itemID}
}

func responsesContentRef(outputIndex int, itemID string, contentIndex int) unified.StreamRef {
	return unified.StreamRef{ItemIndex: uint32Ptr(outputIndex), ItemID: itemID, SegmentIndex: uint32Ptr(contentIndex)}
}

func responsesSummaryRef(outputIndex int, itemID string, summaryIndex int) unified.StreamRef {
	return unified.StreamRef{ItemIndex: uint32Ptr(outputIndex), ItemID: itemID, SegmentIndex: uint32Ptr(summaryIndex)}
}

func responsesAnnotationRef(outputIndex int, itemID string, contentIndex int, annotationIndex int) unified.StreamRef {
	return unified.StreamRef{ItemIndex: uint32Ptr(outputIndex), ItemID: itemID, SegmentIndex: uint32Ptr(contentIndex), AnnotationIndex: uint32Ptr(annotationIndex)}
}

func usageFromResponses(u *responses.ResponseUsage) *unified.StreamUsage {
	if u == nil {
		return nil
	}
	cacheRead := 0
	if u.InputTokensDetails != nil {
		cacheRead = u.InputTokensDetails.CachedTokens
	}
	newInput := u.InputTokens - cacheRead
	if newInput < 0 {
		newInput = 0
	}
	reasoning := 0
	if u.OutputTokensDetails != nil {
		reasoning = u.OutputTokensDetails.ReasoningTokens
	}
	output := u.OutputTokens - reasoning
	if output < 0 {
		output = 0
	}
	tokens := unified.TokenItems{
		{Kind: unified.TokenKindInputNew, Count: newInput},
		{Kind: unified.TokenKindInputCacheRead, Count: cacheRead},
		{Kind: unified.TokenKindOutput, Count: output},
		{Kind: unified.TokenKindOutputReasoning, Count: reasoning},
	}
	tokens = tokens.NonZero()
	return &unified.StreamUsage{Input: tokens.InputTokens(), Output: tokens.OutputTokens(), Tokens: tokens}
}

func mapResponsesIncompleteReason(d *responses.IncompleteDetails) unified.StopReason {
	if d == nil {
		return unified.StopReasonEndTurn
	}
	switch d.Reason {
	case responses.ReasonMaxOutputTokens:
		return unified.StopReasonMaxTokens
	case responses.ReasonContentFilter:
		return unified.StopReasonContentFilter
	default:
		return unified.StopReason(d.Reason)
	}
}

func responsesPartKindVariant(partType string) (unified.ContentKind, unified.ContentVariant) {
	switch partType {
	case "output_text":
		return unified.ContentKindText, unified.ContentVariantPrimary
	case "refusal":
		return unified.ContentKindRefusal, unified.ContentVariantPrimary
	case "audio":
		return unified.ContentKindMedia, unified.ContentVariantPrimary
	default:
		return "", ""
	}
}

func responsesPartMime(partType string) string {
	switch partType {
	case "audio":
		return "audio/*"
	default:
		return ""
	}
}
