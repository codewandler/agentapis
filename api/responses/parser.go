package responses

import "encoding/json"

type Parser struct{}

func NewParser() *Parser { return &Parser{} }

func (p *Parser) Parse(name string, data []byte) (StreamEvent, error) {
	if name == "" {
		var envelope struct {
			Type string `json:"type"`
		}
		if json.Unmarshal(data, &envelope) == nil && envelope.Type != "" {
			name = envelope.Type
		}
	}

	switch name {
	case EventResponseCreated:
		return parseEvent[*ResponseCreatedEvent](name, data)
	case EventResponseInProgress:
		return parseEvent[*ResponseInProgressEvent](name, data)
	case EventResponseCompleted:
		return parseEvent[*ResponseCompletedEvent](name, data)
	case EventResponseFailed:
		return parseEvent[*ResponseFailedEvent](name, data)
	case EventResponseIncomplete:
		return parseEvent[*ResponseIncompleteEvent](name, data)
	case EventResponseQueued:
		return parseEvent[*ResponseQueuedEvent](name, data)

	case EventOutputItemAdded:
		return parseEvent[*OutputItemAddedEvent](name, data)
	case EventOutputItemDone:
		return parseEvent[*OutputItemDoneEvent](name, data)
	case EventContentPartAdded:
		return parseEvent[*ContentPartAddedEvent](name, data)
	case EventContentPartDone:
		return parseEvent[*ContentPartDoneEvent](name, data)

	case EventOutputTextDelta:
		return parseEvent[*OutputTextDeltaEvent](name, data)
	case EventOutputTextDone:
		return parseEvent[*OutputTextDoneEvent](name, data)
	case EventOutputTextAnnotationAdded:
		return parseEvent[*OutputTextAnnotationAddedEvent](name, data)
	case EventRefusalDelta:
		return parseEvent[*RefusalDeltaEvent](name, data)
	case EventRefusalDone:
		return parseEvent[*RefusalDoneEvent](name, data)

	case EventFunctionCallArgumentsDelta:
		return parseEvent[*FunctionCallArgumentsDeltaEvent](name, data)
	case EventFunctionCallArgumentsDone:
		return parseEvent[*FunctionCallArgumentsDoneEvent](name, data)

	case EventFileSearchCallInProgress:
		return parseEvent[*FileSearchCallInProgressEvent](name, data)
	case EventFileSearchCallSearching:
		return parseEvent[*FileSearchCallSearchingEvent](name, data)
	case EventFileSearchCallCompleted:
		return parseEvent[*FileSearchCallCompletedEvent](name, data)

	case EventWebSearchCallInProgress:
		return parseEvent[*WebSearchCallInProgressEvent](name, data)
	case EventWebSearchCallSearching:
		return parseEvent[*WebSearchCallSearchingEvent](name, data)
	case EventWebSearchCallCompleted:
		return parseEvent[*WebSearchCallCompletedEvent](name, data)

	case EventReasoningSummaryPartAdded:
		return parseEvent[*ReasoningSummaryPartAddedEvent](name, data)
	case EventReasoningSummaryPartDone:
		return parseEvent[*ReasoningSummaryPartDoneEvent](name, data)
	case EventReasoningSummaryTextDelta:
		return parseEvent[*ReasoningSummaryTextDeltaEvent](name, data)
	case EventReasoningSummaryTextDone:
		return parseEvent[*ReasoningSummaryTextDoneEvent](name, data)
	case EventReasoningTextDelta:
		return parseEvent[*ReasoningTextDeltaEvent](name, data)
	case EventReasoningTextDone:
		return parseEvent[*ReasoningTextDoneEvent](name, data)

	case EventImageGenerationCallCompleted:
		return parseEvent[*ImageGenerationCallCompletedEvent](name, data)
	case EventImageGenerationCallGenerating:
		return parseEvent[*ImageGenerationCallGeneratingEvent](name, data)
	case EventImageGenerationCallInProgress:
		return parseEvent[*ImageGenerationCallInProgressEvent](name, data)
	case EventImageGenerationCallPartialImage:
		return parseEvent[*ImageGenerationCallPartialImageEvent](name, data)

	case EventMCPCallArgumentsDelta:
		return parseEvent[*MCPCallArgumentsDeltaEvent](name, data)
	case EventMCPCallArgumentsDone:
		return parseEvent[*MCPCallArgumentsDoneEvent](name, data)
	case EventMCPCallCompleted:
		return parseEvent[*MCPCallCompletedEvent](name, data)
	case EventMCPCallFailed:
		return parseEvent[*MCPCallFailedEvent](name, data)
	case EventMCPCallInProgress:
		return parseEvent[*MCPCallInProgressEvent](name, data)
	case EventMCPListToolsCompleted:
		return parseEvent[*MCPListToolsCompletedEvent](name, data)
	case EventMCPListToolsFailed:
		return parseEvent[*MCPListToolsFailedEvent](name, data)
	case EventMCPListToolsInProgress:
		return parseEvent[*MCPListToolsInProgressEvent](name, data)

	case EventCodeInterpreterCallInProgress:
		return parseEvent[*CodeInterpreterCallInProgressEvent](name, data)
	case EventCodeInterpreterCallInterpreting:
		return parseEvent[*CodeInterpreterCallInterpretingEvent](name, data)
	case EventCodeInterpreterCallCompleted:
		return parseEvent[*CodeInterpreterCallCompletedEvent](name, data)
	case EventCodeInterpreterCallCodeDelta:
		return parseEvent[*CodeInterpreterCallCodeDeltaEvent](name, data)
	case EventCodeInterpreterCallCodeDone:
		return parseEvent[*CodeInterpreterCallCodeDoneEvent](name, data)

	case EventCustomToolCallInputDelta:
		return parseEvent[*CustomToolCallInputDeltaEvent](name, data)
	case EventCustomToolCallInputDone:
		return parseEvent[*CustomToolCallInputDoneEvent](name, data)

	case EventAudioTranscriptDone:
		return parseEvent[*AudioTranscriptDoneEvent](name, data)
	case EventAudioTranscriptDelta:
		return parseEvent[*AudioTranscriptDeltaEvent](name, data)
	case EventAudioDone:
		return parseEvent[*AudioDoneEvent](name, data)
	case EventAudioDelta:
		return parseEvent[*AudioDeltaEvent](name, data)

	case EventAPIError:
		return parseEvent[*APIErrorEvent](name, data)

	default:
		return nil, nil
	}
}
