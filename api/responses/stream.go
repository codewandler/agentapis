package responses

type StreamEvent interface {
	responsesStreamEvent()
	EventType() string
}

type StreamResult struct {
	Event        StreamEvent
	Err          error
	RawEventName string
	RawJSON      []byte
}

func (*ResponseCreatedEvent) responsesStreamEvent()                 {}
func (*ResponseInProgressEvent) responsesStreamEvent()              {}
func (*ResponseCompletedEvent) responsesStreamEvent()               {}
func (*ResponseFailedEvent) responsesStreamEvent()                  {}
func (*ResponseIncompleteEvent) responsesStreamEvent()              {}
func (*ResponseQueuedEvent) responsesStreamEvent()                  {}
func (*OutputItemAddedEvent) responsesStreamEvent()                 {}
func (*OutputItemDoneEvent) responsesStreamEvent()                  {}
func (*ContentPartAddedEvent) responsesStreamEvent()                {}
func (*ContentPartDoneEvent) responsesStreamEvent()                 {}
func (*OutputTextDeltaEvent) responsesStreamEvent()                 {}
func (*OutputTextDoneEvent) responsesStreamEvent()                  {}
func (*OutputTextAnnotationAddedEvent) responsesStreamEvent()       {}
func (*RefusalDeltaEvent) responsesStreamEvent()                    {}
func (*RefusalDoneEvent) responsesStreamEvent()                     {}
func (*FunctionCallArgumentsDeltaEvent) responsesStreamEvent()      {}
func (*FunctionCallArgumentsDoneEvent) responsesStreamEvent()       {}
func (*FileSearchCallInProgressEvent) responsesStreamEvent()        {}
func (*FileSearchCallSearchingEvent) responsesStreamEvent()         {}
func (*FileSearchCallCompletedEvent) responsesStreamEvent()         {}
func (*WebSearchCallInProgressEvent) responsesStreamEvent()         {}
func (*WebSearchCallSearchingEvent) responsesStreamEvent()          {}
func (*WebSearchCallCompletedEvent) responsesStreamEvent()          {}
func (*ReasoningSummaryPartAddedEvent) responsesStreamEvent()       {}
func (*ReasoningSummaryPartDoneEvent) responsesStreamEvent()        {}
func (*ReasoningSummaryTextDeltaEvent) responsesStreamEvent()       {}
func (*ReasoningSummaryTextDoneEvent) responsesStreamEvent()        {}
func (*ReasoningTextDeltaEvent) responsesStreamEvent()              {}
func (*ReasoningTextDoneEvent) responsesStreamEvent()               {}
func (*ImageGenerationCallCompletedEvent) responsesStreamEvent()    {}
func (*ImageGenerationCallGeneratingEvent) responsesStreamEvent()   {}
func (*ImageGenerationCallInProgressEvent) responsesStreamEvent()   {}
func (*ImageGenerationCallPartialImageEvent) responsesStreamEvent() {}
func (*MCPCallArgumentsDeltaEvent) responsesStreamEvent()           {}
func (*MCPCallArgumentsDoneEvent) responsesStreamEvent()            {}
func (*MCPCallCompletedEvent) responsesStreamEvent()                {}
func (*MCPCallFailedEvent) responsesStreamEvent()                   {}
func (*MCPCallInProgressEvent) responsesStreamEvent()               {}
func (*MCPListToolsCompletedEvent) responsesStreamEvent()           {}
func (*MCPListToolsFailedEvent) responsesStreamEvent()              {}
func (*MCPListToolsInProgressEvent) responsesStreamEvent()          {}
func (*CodeInterpreterCallInProgressEvent) responsesStreamEvent()   {}
func (*CodeInterpreterCallInterpretingEvent) responsesStreamEvent() {}
func (*CodeInterpreterCallCompletedEvent) responsesStreamEvent()    {}
func (*CodeInterpreterCallCodeDeltaEvent) responsesStreamEvent()    {}
func (*CodeInterpreterCallCodeDoneEvent) responsesStreamEvent()     {}
func (*CustomToolCallInputDeltaEvent) responsesStreamEvent()        {}
func (*CustomToolCallInputDoneEvent) responsesStreamEvent()         {}
func (*APIErrorEvent) responsesStreamEvent()                        {}
func (*AudioTranscriptDeltaEvent) responsesStreamEvent()            {}
func (*AudioTranscriptDoneEvent) responsesStreamEvent()             {}
func (*AudioDeltaEvent) responsesStreamEvent()                      {}
func (*AudioDoneEvent) responsesStreamEvent()                       {}
