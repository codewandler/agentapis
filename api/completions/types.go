package completions

type Request struct {
	Model                string          `json:"model"`
	Messages             []Message       `json:"messages"`
	Tools                []Tool          `json:"tools,omitempty"`
	ToolChoice           any             `json:"tool_choice,omitempty"`
	ReasoningEffort      string          `json:"reasoning_effort,omitempty"`
	PromptCacheRetention string          `json:"prompt_cache_retention,omitempty"`
	MaxTokens            int             `json:"max_tokens,omitempty"`
	Temperature          float64         `json:"temperature,omitempty"`
	TopP                 float64         `json:"top_p,omitempty"`
	TopK                 int             `json:"top_k,omitempty"`
	Stop                 []string        `json:"stop,omitempty"`
	N                    int             `json:"n,omitempty"`
	PresencePenalty      float64         `json:"presence_penalty,omitempty"`
	FrequencyPenalty     float64         `json:"frequency_penalty,omitempty"`
	LogProbs             bool            `json:"logprobs,omitempty"`
	TopLogProbs          int             `json:"top_logprobs,omitempty"`
	ResponseFormat       *ResponseFormat `json:"response_format,omitempty"`
	Stream               bool            `json:"stream"`
	StreamOptions        *StreamOptions  `json:"stream_options,omitempty"`
	User                 string          `json:"user,omitempty"`
	Metadata             map[string]any  `json:"metadata,omitempty"`
	Store                bool            `json:"store,omitempty"`
	ParallelToolCalls    bool            `json:"parallel_tool_calls,omitempty"`
	ServiceTier          string          `json:"service_tier,omitempty"`
}

type Message struct {
	Role       string     `json:"role"`
	Content    any        `json:"content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

type ToolCall struct {
	ID       string   `json:"id"`
	Type     string   `json:"type"`
	Function FuncCall `json:"function"`
}

type FuncCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type Tool struct {
	Type     string      `json:"type"`
	Function FuncPayload `json:"function"`
}

type FuncPayload struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Parameters  any    `json:"parameters"`
	Strict      bool   `json:"strict,omitempty"`
}

type ResponseFormat struct {
	Type string `json:"type"`
}

type StreamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

type Chunk struct {
	ID      string   `json:"id"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   *Usage   `json:"usage,omitempty"`
}

type Choice struct {
	Index        int     `json:"index"`
	Delta        Delta   `json:"delta"`
	FinishReason *string `json:"finish_reason"`
}

type Delta struct {
	Role      string          `json:"role,omitempty"`
	Content   string          `json:"content,omitempty"`
	ToolCalls []ToolCallDelta `json:"tool_calls,omitempty"`
}

type ToolCallDelta struct {
	Index    int           `json:"index"`
	ID       string        `json:"id,omitempty"`
	Type     string        `json:"type,omitempty"`
	Function FuncCallDelta `json:"function"`
}

type FuncCallDelta struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

type Usage struct {
	PromptTokens            int         `json:"prompt_tokens"`
	CompletionTokens        int         `json:"completion_tokens"`
	TotalTokens             int         `json:"total_tokens"`
	PromptTokensDetails     *TokDetails `json:"prompt_tokens_details,omitempty"`
	CompletionTokensDetails *TokDetails `json:"completion_tokens_details,omitempty"`
}

type TokDetails struct {
	CachedTokens    int `json:"cached_tokens,omitempty"`
	ReasoningTokens int `json:"reasoning_tokens,omitempty"`
}
