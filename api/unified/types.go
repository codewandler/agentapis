package unified

import (
	"encoding/json"
	"fmt"
	"time"
)

// === Enums ===

type Role string

const (
	RoleSystem    Role = "system"
	RoleDeveloper Role = "developer"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

type PartType string

const (
	PartTypeText       PartType = "text"
	PartTypeThinking   PartType = "thinking"
	PartTypeToolCall   PartType = "tool_call"
	PartTypeToolResult PartType = "tool_result"
)

type StopReason string

const (
	StopReasonEndTurn       StopReason = "end_turn"
	StopReasonToolUse       StopReason = "tool_use"
	StopReasonMaxTokens     StopReason = "max_tokens"
	StopReasonContentFilter StopReason = "content_filter"
	StopReasonError         StopReason = "error"
)

type Effort string

const (
	EffortUnspecified Effort = ""
	EffortLow         Effort = "low"
	EffortMedium      Effort = "medium"
	EffortHigh        Effort = "high"
	EffortMax         Effort = "max"
)

func (e Effort) IsEmpty() bool { return e == EffortUnspecified }

func (e Effort) ToBudget(low, high int) (int, bool) {
	switch e {
	case EffortLow:
		return low, true
	case EffortMedium:
		return low + (high-low)*2/3, true
	case EffortHigh:
		return high - (high-low)/10, true
	case EffortMax:
		return high, true
	default:
		return high, false
	}
}

type ThinkingMode string

const (
	ThinkingModeAuto ThinkingMode = "auto"
	ThinkingModeOn   ThinkingMode = "on"
	ThinkingModeOff  ThinkingMode = "off"
)

func (m ThinkingMode) IsOff() bool  { return m == ThinkingModeOff }
func (m ThinkingMode) IsOn() bool   { return m == ThinkingModeOn }
func (m ThinkingMode) IsAuto() bool { return m == ThinkingModeAuto }

type OutputMode string

const (
	OutputModeText       OutputMode = "text"
	OutputModeJSONObject OutputMode = "json_object"
	OutputModeJSONSchema OutputMode = "json_schema"
)

type StreamEventType string

const (
	StreamEventStarted      StreamEventType = "started"
	StreamEventDelta        StreamEventType = "delta"
	StreamEventToolCall     StreamEventType = "tool_call"
	StreamEventContent      StreamEventType = "content"
	StreamEventUsage        StreamEventType = "usage"
	StreamEventCompleted    StreamEventType = "completed"
	StreamEventError        StreamEventType = "error"
	StreamEventLifecycle    StreamEventType = "lifecycle"
	StreamEventContentDelta StreamEventType = "content_delta"
	StreamEventToolDelta    StreamEventType = "tool_delta"
	StreamEventAnnotation   StreamEventType = "annotation"
	StreamEventUnknown      StreamEventType = "unknown"
)

type LifecycleScope string

const (
	LifecycleScopeResponse LifecycleScope = "response"
	LifecycleScopeItem     LifecycleScope = "item"
	LifecycleScopeSegment  LifecycleScope = "segment"
)

type LifecycleState string

const (
	LifecycleStateQueued     LifecycleState = "queued"
	LifecycleStateInProgress LifecycleState = "in_progress"
	LifecycleStateAdded      LifecycleState = "added"
	LifecycleStateDone       LifecycleState = "done"
	LifecycleStateFailed     LifecycleState = "failed"
	LifecycleStateIncomplete LifecycleState = "incomplete"
)

type ContentKind string

const (
	ContentKindText      ContentKind = "text"
	ContentKindReasoning ContentKind = "reasoning"
	ContentKindRefusal   ContentKind = "refusal"
	ContentKindMedia     ContentKind = "media"
)

type ContentVariant string

const (
	ContentVariantPrimary    ContentVariant = "primary"
	ContentVariantSummary    ContentVariant = "summary"
	ContentVariantRaw        ContentVariant = "raw"
	ContentVariantTranscript ContentVariant = "transcript"
)

type ContentEncoding string

const (
	ContentEncodingUTF8   ContentEncoding = "utf8"
	ContentEncodingBase64 ContentEncoding = "base64"
)

type ToolDeltaKind string

const (
	ToolDeltaKindFunctionArguments ToolDeltaKind = "function_arguments"
	ToolDeltaKindCustomInput       ToolDeltaKind = "custom_input"
)

type DeltaKind string

const (
	DeltaKindText     DeltaKind = "text"
	DeltaKindThinking DeltaKind = "thinking"
	DeltaKindTool     DeltaKind = "tool"
)

type AssistantPhase string

const (
	AssistantPhaseCommentary  AssistantPhase = "commentary"
	AssistantPhaseFinalAnswer AssistantPhase = "final_answer"
)

func (p AssistantPhase) Valid() bool {
	switch p {
	case "", AssistantPhaseCommentary, AssistantPhaseFinalAnswer:
		return true
	default:
		return false
	}
}

func (p AssistantPhase) IsEmpty() bool { return p == "" }

// === Token Usage ===

type TokenKind string

const (
	TokenKindInput      TokenKind = "input"
	TokenKindOutput     TokenKind = "output"
	TokenKindCacheRead  TokenKind = "cache_read"
	TokenKindCacheWrite TokenKind = "cache_write"
	TokenKindReasoning  TokenKind = "reasoning"
)

type TokenItem struct {
	Kind  TokenKind `json:"kind"`
	Count int       `json:"count"`
}

type TokenItems []TokenItem

func (t TokenItems) Count(kind TokenKind) int {
	for _, item := range t {
		if item.Kind == kind {
			return item.Count
		}
	}
	return 0
}

func (t TokenItems) TotalInput() int {
	return t.Count(TokenKindInput) + t.Count(TokenKindCacheRead) + t.Count(TokenKindCacheWrite)
}

func (t TokenItems) TotalOutput() int {
	return t.Count(TokenKindOutput) + t.Count(TokenKindReasoning)
}

func (t TokenItems) Total() int {
	return t.TotalInput() + t.TotalOutput()
}

func (t TokenItems) NonZero() TokenItems {
	var result TokenItems
	for _, item := range t {
		if item.Count > 0 {
			result = append(result, item)
		}
	}
	return result
}

// === Cost ===

type CostKind string

const (
	CostKindInput      CostKind = "input"
	CostKindOutput     CostKind = "output"
	CostKindCacheRead  CostKind = "cache_read"
	CostKindCacheWrite CostKind = "cache_write"
	CostKindReasoning  CostKind = "reasoning"
	CostKindImage      CostKind = "image"
	CostKindAudio      CostKind = "audio"
	CostKindVideo      CostKind = "video"
)

type CostItem struct {
	Kind   CostKind `json:"kind"`
	Amount float64  `json:"amount"`
}

type CostItems []CostItem

func (c CostItems) Total() float64 {
	var total float64
	for _, item := range c {
		total += item.Amount
	}
	return total
}

func (c CostItems) ByKind(kind CostKind) float64 {
	for _, item := range c {
		if item.Kind == kind {
			return item.Amount
		}
	}
	return 0
}

func (c CostItems) NonZero() CostItems {
	var result CostItems
	for _, item := range c {
		if item.Amount != 0 {
			result = append(result, item)
		}
	}
	return result
}

// === Cache Hint ===

type CacheHint struct {
	Enabled bool   `json:"enabled,omitempty"`
	TTL     string `json:"ttl,omitempty"`
}

// === Request Types ===

type Request struct {
	Model       string           `json:"model"`
	Messages    []Message        `json:"messages"`
	MaxTokens   int              `json:"max_tokens,omitempty"`
	Temperature float64          `json:"temperature,omitempty"`
	TopP        float64          `json:"top_p,omitempty"`
	TopK        int              `json:"top_k,omitempty"`
	Output      *OutputSpec      `json:"output,omitempty"`
	Tools       []Tool           `json:"tools,omitempty"`
	ToolChoice  ToolChoice       `json:"tool_choice,omitempty"`
	Effort      Effort           `json:"effort,omitempty"`
	Thinking    ThinkingMode     `json:"thinking,omitempty"`
	Metadata    *RequestMetadata `json:"metadata,omitempty"`
	CacheHint   *CacheHint       `json:"cache_hint,omitempty"`
	Extras      RequestExtras    `json:"extras,omitempty"`
}

type Message struct {
	Role      Role           `json:"role"`
	Parts     []Part         `json:"parts"`
	Phase     AssistantPhase `json:"phase,omitempty"`
	CacheHint *CacheHint     `json:"cache_hint,omitempty"`
}

type Part struct {
	Type       PartType        `json:"type"`
	Text       string          `json:"text,omitempty"`
	Thinking   *ThinkingPart   `json:"thinking,omitempty"`
	ToolCall   *ToolCall       `json:"tool_call,omitempty"`
	ToolResult *ToolResult     `json:"tool_result,omitempty"`
	Native     json.RawMessage `json:"native,omitempty"`
}

type ThinkingPart struct {
	Provider  string `json:"provider,omitempty"`
	Text      string `json:"text"`
	Signature string `json:"signature,omitempty"`
}

type ToolCall struct {
	ID   string         `json:"id"`
	Name string         `json:"name"`
	Args map[string]any `json:"args,omitempty"`
}

type ToolResult struct {
	ToolCallID string `json:"tool_call_id"`
	ToolOutput string `json:"tool_output"`
	IsError    bool   `json:"is_error,omitempty"`
}

type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
	Strict      bool           `json:"strict,omitempty"`
}

type ToolChoice interface {
	toolChoice()
	String() string
}

type ToolChoiceAuto struct{}

type ToolChoiceRequired struct{}

type ToolChoiceNone struct{}

type ToolChoiceTool struct {
	Name string `json:"name"`
}

func (ToolChoiceAuto) toolChoice()     {}
func (ToolChoiceRequired) toolChoice() {}
func (ToolChoiceNone) toolChoice()     {}
func (ToolChoiceTool) toolChoice()     {}

func (ToolChoiceAuto) String() string     { return "ToolChoice(auto)" }
func (ToolChoiceRequired) String() string { return "ToolChoice(required)" }
func (ToolChoiceNone) String() string     { return "ToolChoice(none)" }
func (t ToolChoiceTool) String() string   { return fmt.Sprintf("ToolChoice(tool=%s)", t.Name) }

type OutputSpec struct {
	Mode   OutputMode `json:"mode"`
	Schema any        `json:"schema,omitempty"`
}

type RequestMetadata struct {
	User     string         `json:"user,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// === Request Extras ===

type RequestExtras struct {
	Messages    *MessagesExtras    `json:"messages,omitempty"`
	Completions *CompletionsExtras `json:"completions,omitempty"`
	Responses   *ResponsesExtras   `json:"responses,omitempty"`
	Provider    map[string]any     `json:"provider,omitempty"`
}

type MessagesExtras struct {
	AnthropicBeta         []string      `json:"anthropic_beta,omitempty"`
	StopSequences         []string      `json:"stop_sequences,omitempty"`
	ThinkingType          string        `json:"thinking_type,omitempty"`
	ThinkingBudgetTokens  int           `json:"thinking_budget_tokens,omitempty"`
	ThinkingDisplay       string        `json:"thinking_display,omitempty"`
	RequestCacheControl   *CacheControl `json:"request_cache_control,omitempty"`
	MessageCachePartIndex map[int]int   `json:"message_cache_part_index,omitempty"`
}

type CompletionsExtras struct {
	PromptCacheRetention string         `json:"prompt_cache_retention,omitempty"`
	Stop                 []string       `json:"stop,omitempty"`
	N                    int            `json:"n,omitempty"`
	PresencePenalty      float64        `json:"presence_penalty,omitempty"`
	FrequencyPenalty     float64        `json:"frequency_penalty,omitempty"`
	LogProbs             bool           `json:"logprobs,omitempty"`
	TopLogProbs          int            `json:"top_logprobs,omitempty"`
	Store                bool           `json:"store,omitempty"`
	ParallelToolCalls    bool           `json:"parallel_tool_calls,omitempty"`
	ServiceTier          string         `json:"service_tier,omitempty"`
	ExtraMetadata        map[string]any `json:"extra_metadata,omitempty"`
}

type ResponsesExtras struct {
	PromptCacheRetention string         `json:"prompt_cache_retention,omitempty"`
	PreviousResponseID   string         `json:"previous_response_id,omitempty"`
	ReasoningSummary     string         `json:"reasoning_summary,omitempty"`
	Store                bool           `json:"store,omitempty"`
	ParallelToolCalls    bool           `json:"parallel_tool_calls,omitempty"`
	UseInstructions      *bool          `json:"use_instructions,omitempty"`
	UsedMaxTokenField    string         `json:"used_max_token_field,omitempty"`
	ExtraMetadata        map[string]any `json:"extra_metadata,omitempty"`
}

type CacheControl struct {
	Type string `json:"type"`
	TTL  string `json:"ttl,omitempty"`
}

// === Stream Event Types ===

type StreamEvent struct {
	Type           StreamEventType `json:"type"`
	Started        *Started        `json:"started,omitempty"`
	Delta          *Delta          `json:"delta,omitempty"`
	ToolCall       *ToolCall       `json:"tool_call,omitempty"`
	Content        *ContentPart    `json:"content,omitempty"`
	Usage          *StreamUsage    `json:"usage,omitempty"`
	Completed      *Completed      `json:"completed,omitempty"`
	Error          *StreamError    `json:"error,omitempty"`
	Lifecycle      *Lifecycle      `json:"lifecycle,omitempty"`
	ContentDelta   *ContentDelta   `json:"content_delta,omitempty"`
	StreamContent  *StreamContent  `json:"stream_content,omitempty"`
	ToolDelta      *ToolDelta      `json:"tool_delta,omitempty"`
	StreamToolCall *StreamToolCall `json:"stream_tool_call,omitempty"`
	Annotation     *Annotation     `json:"annotation,omitempty"`
	Extras         EventExtras     `json:"extras,omitempty"`
}

type EventExtras struct {
	RawEventName string          `json:"raw_event_name,omitempty"`
	RawJSON      json.RawMessage `json:"raw_json,omitempty"`
	Provider     map[string]any  `json:"provider,omitempty"`
}

type Started struct {
	RequestID string         `json:"request_id,omitempty"`
	Model     string         `json:"model,omitempty"`
	Provider  string         `json:"provider,omitempty"`
	Extra     map[string]any `json:"extra,omitempty"`
}

type Delta struct {
	Kind     DeltaKind `json:"kind"`
	Index    *uint32   `json:"index,omitempty"`
	Text     string    `json:"text,omitempty"`
	Thinking string    `json:"thinking,omitempty"`
	ToolID   string    `json:"tool_id,omitempty"`
	ToolName string    `json:"tool_name,omitempty"`
	ToolArgs string    `json:"tool_args,omitempty"`
}

type ContentPart struct {
	Part  Part `json:"part"`
	Index int  `json:"index,omitempty"`
}

type StreamUsage struct {
	Provider   string         `json:"provider,omitempty"`
	Model      string         `json:"model,omitempty"`
	RequestID  string         `json:"request_id,omitempty"`
	Tokens     TokenItems     `json:"tokens,omitempty"`
	Costs      CostItems      `json:"costs,omitempty"`
	RecordedAt time.Time      `json:"recorded_at,omitempty"`
	Extras     map[string]any `json:"extras,omitempty"`
}

type Completed struct {
	StopReason StopReason `json:"stop_reason"`
}

type StreamError struct {
	Err error `json:"-"`
}

func (e *StreamError) MarshalJSON() ([]byte, error) {
	if e.Err != nil {
		return json.Marshal(e.Err.Error())
	}
	return []byte(`null`), nil
}

func (e *StreamError) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	e.Err = fmt.Errorf("%s", s)
	return nil
}

type Lifecycle struct {
	Scope    LifecycleScope `json:"scope"`
	State    LifecycleState `json:"state"`
	Ref      StreamRef      `json:"ref"`
	Kind     ContentKind    `json:"kind,omitempty"`
	Variant  ContentVariant `json:"variant,omitempty"`
	ItemType string         `json:"item_type,omitempty"`
	Mime     string         `json:"mime,omitempty"`
}

type ContentBase struct {
	Ref       StreamRef       `json:"ref"`
	Kind      ContentKind     `json:"kind"`
	Variant   ContentVariant  `json:"variant,omitempty"`
	Mime      string          `json:"mime,omitempty"`
	Encoding  ContentEncoding `json:"encoding,omitempty"`
	Data      string          `json:"data,omitempty"`
	Signature string          `json:"signature,omitempty"`
}

type ContentDelta struct {
	ContentBase
	Final bool `json:"final,omitempty"`
}

type StreamContent struct {
	ContentBase
	Annotations []Annotation `json:"annotations,omitempty"`
}

type ToolDelta struct {
	Ref      StreamRef     `json:"ref"`
	Kind     ToolDeltaKind `json:"kind"`
	ToolID   string        `json:"tool_id,omitempty"`
	ToolName string        `json:"tool_name,omitempty"`
	Data     string        `json:"data,omitempty"`
	Final    bool          `json:"final,omitempty"`
}

type StreamToolCall struct {
	Ref      StreamRef      `json:"ref"`
	ID       string         `json:"id"`
	Name     string         `json:"name"`
	RawInput string         `json:"raw_input,omitempty"`
	Args     map[string]any `json:"args,omitempty"`
}

type Annotation struct {
	Ref         StreamRef `json:"ref"`
	Type        string    `json:"type,omitempty"`
	Text        string    `json:"text,omitempty"`
	FileID      string    `json:"file_id,omitempty"`
	Filename    string    `json:"filename,omitempty"`
	URL         string    `json:"url,omitempty"`
	Title       string    `json:"title,omitempty"`
	ContainerID string    `json:"container_id,omitempty"`
	StartIndex  int       `json:"start_index,omitempty"`
	EndIndex    int       `json:"end_index,omitempty"`
	Offset      int       `json:"offset,omitempty"`
	Index       int       `json:"index,omitempty"`
}

type StreamRef struct {
	ResponseID      string  `json:"response_id,omitempty"`
	ItemIndex       *uint32 `json:"item_index,omitempty"`
	ItemID          string  `json:"item_id,omitempty"`
	SegmentIndex    *uint32 `json:"segment_index,omitempty"`
	AnnotationIndex *uint32 `json:"annotation_index,omitempty"`
}

// === Event Builders ===

func NewStartedEvent(requestID, model string) StreamEvent {
	return StreamEvent{
		Type:    StreamEventStarted,
		Started: &Started{RequestID: requestID, Model: model},
	}
}

func NewTextDeltaEvent(ref StreamRef, index *uint32, text string) StreamEvent {
	return StreamEvent{
		Type:  StreamEventDelta,
		Delta: &Delta{Kind: DeltaKindText, Index: index, Text: text},
	}
}

func NewThinkingDeltaEvent(ref StreamRef, index *uint32, thinking string) StreamEvent {
	return StreamEvent{
		Type:  StreamEventDelta,
		Delta: &Delta{Kind: DeltaKindThinking, Index: index, Thinking: thinking},
	}
}

func NewToolDeltaEvent(ref StreamRef, index *uint32, toolID, toolName, args string) StreamEvent {
	return StreamEvent{
		Type:  StreamEventDelta,
		Delta: &Delta{Kind: DeltaKindTool, Index: index, ToolID: toolID, ToolName: toolName, ToolArgs: args},
	}
}

func NewToolCallEvent(id, name string, args map[string]any) StreamEvent {
	return StreamEvent{
		Type:     StreamEventToolCall,
		ToolCall: &ToolCall{ID: id, Name: name, Args: args},
	}
}

func NewCompletedEvent(reason StopReason) StreamEvent {
	return StreamEvent{
		Type:      StreamEventCompleted,
		Completed: &Completed{StopReason: reason},
	}
}

func NewUsageEvent(tokens TokenItems, costs CostItems) StreamEvent {
	return StreamEvent{
		Type:  StreamEventUsage,
		Usage: &StreamUsage{Tokens: tokens, Costs: costs},
	}
}

func NewErrorEvent(err error) StreamEvent {
	return StreamEvent{
		Type:  StreamEventError,
		Error: &StreamError{Err: err},
	}
}

func NewContentDeltaEvent(ref StreamRef, kind ContentKind, variant ContentVariant, encoding ContentEncoding, data string) StreamEvent {
	return StreamEvent{
		Type: StreamEventContentDelta,
		ContentDelta: &ContentDelta{
			ContentBase: ContentBase{Ref: ref, Kind: kind, Variant: variant, Encoding: encoding, Data: data},
		},
	}
}

func NewLifecycleEvent(scope LifecycleScope, state LifecycleState, ref StreamRef) StreamEvent {
	return StreamEvent{
		Type:      StreamEventLifecycle,
		Lifecycle: &Lifecycle{Scope: scope, State: state, Ref: ref},
	}
}
