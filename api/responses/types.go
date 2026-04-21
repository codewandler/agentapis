package responses

import "encoding/json"

type Request struct {
	Model              string               `json:"model"`
	Input              InputParam           `json:"input"                       jsonschema:"required"`
	Instructions       *string              `json:"instructions,omitempty"       jsonschema:"description=System or developer message inserted into model context"`
	Tools              []ToolParam          `json:"tools,omitempty"`
	ToolChoice         *ToolChoiceParam     `json:"tool_choice,omitempty"`
	Reasoning          *Reasoning           `json:"reasoning,omitempty"`
	MaxOutputTokens    *int                 `json:"max_output_tokens,omitempty"  jsonschema:"minimum=16"`
	Temperature        *float64             `json:"temperature,omitempty"        jsonschema:"minimum=0,maximum=2,default=1"`
	TopP               *float64             `json:"top_p,omitempty"              jsonschema:"minimum=0,maximum=1,default=1"`
	PromptCacheRetention *PromptCacheRetention `json:"prompt_cache_retention,omitempty"`
	Stream             *bool                `json:"stream,omitempty"             jsonschema:"default=false"`
	PreviousResponseID *string              `json:"previous_response_id,omitempty" jsonschema:"description=ID of previous response for multi-turn conversations"`
	Metadata           map[string]string    `json:"metadata,omitempty"           jsonschema:"maxProperties=16"`
	User               string               `json:"user,omitempty"               jsonschema:"deprecated=true,description=Replaced by safety_identifier and prompt_cache_key"`
	Store              *bool                `json:"store,omitempty"              jsonschema:"default=true"`
	ParallelToolCalls  *bool                `json:"parallel_tool_calls,omitempty" jsonschema:"default=true"`
	PromptCacheKey     string               `json:"prompt_cache_key,omitempty"`

	// New fields from OpenAI Responses API spec:
	Text              *ResponseTextParam       `json:"text,omitempty"`
	Truncation        *Truncation              `json:"truncation,omitempty"             jsonschema:"description=auto truncates to fit context window; disabled returns 400. Default: disabled"`
	Include           []IncludeItem            `json:"include,omitempty"                jsonschema:"description=Additional output data to include in the response"`
	StreamOptions     *StreamOptions           `json:"stream_options,omitempty"         jsonschema:"description=Streaming options. Only valid when stream=true"`
	Background        *bool                    `json:"background,omitempty"             jsonschema:"default=false,description=Run the response in the background"`
	MaxToolCalls      *int                     `json:"max_tool_calls,omitempty"         jsonschema:"description=Maximum total built-in tool calls across the response"`
	Conversation      *ConversationParam       `json:"conversation,omitempty"           jsonschema:"description=Links response to a conversation. Cannot combine with previous_response_id"`
	ContextManagement []ContextManagementParam `json:"context_management,omitempty"     jsonschema:"minItems=1"`
	Prompt            *Prompt                  `json:"prompt,omitempty"                 jsonschema:"description=Reference to a stored prompt template"`
	SafetyIdentifier  string                   `json:"safety_identifier,omitempty"      jsonschema:"maxLength=64,description=Hashed stable user ID for abuse detection"`
	ServiceTier       *ServiceTier             `json:"service_tier,omitempty"           jsonschema:"description=Processing tier. Default: auto"`
	TopLogprobs       *int                     `json:"top_logprobs,omitempty"           jsonschema:"minimum=0,maximum=20"`

	// Extra holds non-serialized adapter state for RequestTransform hooks.
	// This field is never sent to OpenAI.
	Extra map[string]any `json:"-"`
}

type Reasoning struct {
	Effort  *ReasoningEffort  `json:"effort"`
	Summary *ReasoningSummary `json:"summary"`
}



type EventMeta struct {
	Type           string `json:"type"`
	SequenceNumber int    `json:"sequence_number,omitempty"`
}

func (m *EventMeta) SetType(name string) {
	if m.Type == "" {
		m.Type = name
	}
}

func (m *EventMeta) EventType() string { return m.Type }

type ResponseRef struct {
	ResponseID string `json:"response_id,omitempty"`
}

type OutputRef struct {
	OutputIndex int    `json:"output_index,omitempty"`
	ItemID      string `json:"item_id,omitempty"`
}

type ContentRef struct {
	OutputIndex  int    `json:"output_index,omitempty"`
	ItemID       string `json:"item_id,omitempty"`
	ContentIndex int    `json:"content_index,omitempty"`
}

type SummaryRef struct {
	OutputIndex  int    `json:"output_index,omitempty"`
	ItemID       string `json:"item_id,omitempty"`
	SummaryIndex int    `json:"summary_index,omitempty"`
}

type ResponsePayload struct {
	ID                string               `json:"id,omitempty"`
	Model             string               `json:"model,omitempty"`
	CreatedAt         int64                `json:"created_at,omitempty"`
	Status            string               `json:"status,omitempty"`
	Error             *ResponseError       `json:"error,omitempty"`
	IncompleteDetails *IncompleteDetails   `json:"incomplete_details,omitempty"`
	Instructions      any                  `json:"instructions,omitempty"`
	Output            []ResponseOutputItem `json:"output,omitempty"`
	Usage             *ResponseUsage       `json:"usage,omitempty"`
	User              string               `json:"user,omitempty"`
	Metadata          map[string]any       `json:"metadata,omitempty"`
}

type ResponseError struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

type IncompleteDetails struct {
	Reason string `json:"reason,omitempty"`
}

type ResponseOutputItem struct {
	ID          string                 `json:"id,omitempty"`
	Type        string                 `json:"type,omitempty"`
	Status      string                 `json:"status,omitempty"`
	Role        string                 `json:"role,omitempty"`
	Phase       string                 `json:"phase,omitempty"`
	Content     []ResponseContentPart  `json:"content,omitempty"`
	CallID      string                 `json:"call_id,omitempty"`
	Name        string                 `json:"name,omitempty"`
	Arguments   string                 `json:"arguments,omitempty"`
	Output      string                 `json:"output,omitempty"`
	Input       string                 `json:"input,omitempty"`
	Results     []map[string]any       `json:"results,omitempty"`
	Summary     []ReasoningSummaryPart `json:"summary,omitempty"`
	Queries     []string               `json:"queries,omitempty"`
	Code        string                 `json:"code,omitempty"`
	ContainerID string                 `json:"container_id,omitempty"`
	FileID      string                 `json:"file_id,omitempty"`
	ServerLabel string                 `json:"server_label,omitempty"`
	ToolName    string                 `json:"tool_name,omitempty"`
}

type ResponseContentPart struct {
	Type        string                 `json:"type,omitempty"`
	Text        string                 `json:"text,omitempty"`
	Refusal     string                 `json:"refusal,omitempty"`
	Annotations []OutputTextAnnotation `json:"annotations,omitempty"`
	Logprobs    []TokenLogprob         `json:"logprobs,omitempty"`
	Transcript  string                 `json:"transcript,omitempty"`
}

type OutputTextAnnotation struct {
	Type        string `json:"type,omitempty"`
	FileID      string `json:"file_id,omitempty"`
	Filename    string `json:"filename,omitempty"`
	Index       int    `json:"index,omitempty"`
	StartIndex  int    `json:"start_index,omitempty"`
	EndIndex    int    `json:"end_index,omitempty"`
	Title       string `json:"title,omitempty"`
	URL         string `json:"url,omitempty"`
	ContainerID string `json:"container_id,omitempty"`
	Offset      int    `json:"offset,omitempty"`
	Text        string `json:"text,omitempty"`
}

type TokenLogprob struct {
	Token       string            `json:"token,omitempty"`
	Logprob     float64           `json:"logprob,omitempty"`
	TopLogprobs []TopTokenLogprob `json:"top_logprobs,omitempty"`
}

type TopTokenLogprob struct {
	Token   string  `json:"token,omitempty"`
	Logprob float64 `json:"logprob,omitempty"`
}

type ReasoningSummaryPart struct {
	Type string `json:"type,omitempty"`
	Text string `json:"text,omitempty"`
}

type ResponseCreatedEvent struct {
	EventMeta
	Response ResponsePayload `json:"response"`
}

type ResponseInProgressEvent struct {
	EventMeta
	Response ResponsePayload `json:"response"`
}

type ResponseQueuedEvent struct {
	EventMeta
	Response ResponsePayload `json:"response"`
}

type ResponseCompletedEvent struct {
	EventMeta
	Response struct {
		ID                string `json:"id"`
		Model             string `json:"model"`
		Status            string `json:"status"`
		IncompleteDetails *struct {
			Reason string `json:"reason"`
		} `json:"incomplete_details,omitempty"`
		Error *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error,omitempty"`
		Usage *ResponseUsage `json:"usage,omitempty"`
	} `json:"response"`
}

type ResponseFailedEvent struct {
	EventMeta
	Response ResponsePayload `json:"response"`
}

type ResponseIncompleteEvent struct {
	EventMeta
	Response ResponsePayload `json:"response"`
}

type ResponseUsage struct {
	InputTokens        int `json:"input_tokens"`
	OutputTokens       int `json:"output_tokens"`
	InputTokensDetails *struct {
		CachedTokens int `json:"cached_tokens"`
	} `json:"input_tokens_details,omitempty"`
	OutputTokensDetails *struct {
		ReasoningTokens int `json:"reasoning_tokens"`
	} `json:"output_tokens_details,omitempty"`
}

type OutputItemAddedEvent struct {
	EventMeta
	OutputIndex int                `json:"output_index"`
	Item        ResponseOutputItem `json:"item"`
}

type OutputItemDoneEvent struct {
	EventMeta
	OutputIndex int                `json:"output_index"`
	Item        ResponseOutputItem `json:"item"`
}

type ContentPartAddedEvent struct {
	EventMeta
	ContentRef
	Part ResponseContentPart `json:"part"`
}

type ContentPartDoneEvent struct {
	EventMeta
	ContentRef
	Part ResponseContentPart `json:"part"`
}

type OutputTextDeltaEvent struct {
	EventMeta
	ContentRef
	Delta    string         `json:"delta"`
	Logprobs []TokenLogprob `json:"logprobs,omitempty"`
}

type OutputTextDoneEvent struct {
	EventMeta
	ContentRef
	Text     string         `json:"text"`
	Logprobs []TokenLogprob `json:"logprobs,omitempty"`
}

type OutputTextAnnotationAddedEvent struct {
	EventMeta
	ContentRef
	AnnotationIndex int                  `json:"annotation_index,omitempty"`
	Annotation      OutputTextAnnotation `json:"annotation"`
}

type RefusalDeltaEvent struct {
	EventMeta
	ContentRef
	Delta string `json:"delta"`
}

type RefusalDoneEvent struct {
	EventMeta
	ContentRef
	Refusal string `json:"refusal"`
}

type FunctionCallArgumentsDeltaEvent struct {
	EventMeta
	OutputRef
	Delta string `json:"delta"`
}

type FunctionCallArgumentsDoneEvent struct {
	EventMeta
	OutputRef
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments"`
}

type FileSearchCallInProgressEvent struct {
	EventMeta
	OutputRef
}

type FileSearchCallSearchingEvent struct {
	EventMeta
	OutputRef
}

type FileSearchCallCompletedEvent struct {
	EventMeta
	OutputRef
}

type WebSearchCallInProgressEvent struct {
	EventMeta
	OutputRef
}

type WebSearchCallSearchingEvent struct {
	EventMeta
	OutputRef
}

type WebSearchCallCompletedEvent struct {
	EventMeta
	OutputRef
}

type ReasoningSummaryPartAddedEvent struct {
	EventMeta
	SummaryRef
	Part ReasoningSummaryPart `json:"part"`
}

type ReasoningSummaryPartDoneEvent struct {
	EventMeta
	SummaryRef
	Part ReasoningSummaryPart `json:"part"`
}

type ReasoningSummaryTextDeltaEvent struct {
	EventMeta
	OutputRef
	Delta string `json:"delta"`
}

type ReasoningSummaryTextDoneEvent struct {
	EventMeta
	OutputRef
	Text string `json:"text"`
}

type ReasoningTextDeltaEvent struct {
	EventMeta
	OutputRef
	Delta string `json:"delta"`
}

type ReasoningTextDoneEvent struct {
	EventMeta
	OutputRef
	Text string `json:"text"`
}

type ImageGenerationCallInProgressEvent struct {
	EventMeta
	OutputRef
}

type ImageGenerationCallGeneratingEvent struct {
	EventMeta
	OutputRef
}

type ImageGenerationCallCompletedEvent struct {
	EventMeta
	OutputRef
}

type ImageGenerationCallPartialImageEvent struct {
	EventMeta
	OutputRef
	PartialImageIndex int    `json:"partial_image_index"`
	PartialImageB64   string `json:"partial_image_b64"`
}

type MCPCallArgumentsDeltaEvent struct {
	EventMeta
	OutputRef
	Delta string `json:"delta"`
}

type MCPCallArgumentsDoneEvent struct {
	EventMeta
	OutputRef
	Arguments string `json:"arguments"`
}

type MCPCallInProgressEvent struct {
	EventMeta
	OutputRef
}

type MCPCallFailedEvent struct {
	EventMeta
	OutputRef
}

type MCPCallCompletedEvent struct {
	EventMeta
	OutputRef
}

type MCPListToolsInProgressEvent struct {
	EventMeta
	OutputRef
}

type MCPListToolsFailedEvent struct {
	EventMeta
	OutputRef
}

type MCPListToolsCompletedEvent struct {
	EventMeta
	OutputRef
}

type CodeInterpreterCallInProgressEvent struct {
	EventMeta
	OutputRef
}

type CodeInterpreterCallInterpretingEvent struct {
	EventMeta
	OutputRef
}

type CodeInterpreterCallCompletedEvent struct {
	EventMeta
	OutputRef
}

type CodeInterpreterCallCodeDeltaEvent struct {
	EventMeta
	OutputRef
	Delta string `json:"delta"`
}

type CodeInterpreterCallCodeDoneEvent struct {
	EventMeta
	OutputRef
	Code string `json:"code"`
}

type CustomToolCallInputDeltaEvent struct {
	EventMeta
	OutputRef
	Delta string `json:"delta"`
}

type CustomToolCallInputDoneEvent struct {
	EventMeta
	OutputRef
	Input string `json:"input"`
}

type APIErrorEvent struct {
	EventMeta
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
	Param   any    `json:"param,omitempty"`
}

func (e *APIErrorEvent) Error() string {
	if e.Code != "" {
		return "responses API error " + e.Code + ": " + e.Message
	}
	if e.Type != "" && e.Type != EventAPIError {
		return "responses API error " + e.Type + ": " + e.Message
	}
	return "responses API error: " + e.Message
}

type AudioTranscriptDeltaEvent struct {
	EventMeta
	ResponseRef
	Delta string `json:"delta"`
}

type AudioTranscriptDoneEvent struct {
	EventMeta
	ResponseRef
}

type AudioDeltaEvent struct {
	EventMeta
	ResponseRef
	Delta string `json:"delta"`
}

type AudioDoneEvent struct {
	EventMeta
	ResponseRef
}

type eventTypeSetter interface {
	SetType(string)
}

func parseEvent[T StreamEvent](name string, data []byte) (StreamEvent, error) {
	var evt T
	if err := json.Unmarshal(data, &evt); err != nil {
		return nil, err
	}
	if s, ok := any(evt).(eventTypeSetter); ok {
		s.SetType(name)
	}
	return evt, nil
}
