package ollama

type Request struct {
	Model       string         `json:"model"`
	Messages    []Message      `json:"messages"`
	Tools       []Tool         `json:"tools,omitempty"`
	Format      any            `json:"format,omitempty"`
	Options     map[string]any `json:"options,omitempty"`
	Stream      bool           `json:"stream"`
	Think       any            `json:"think,omitempty"`
	KeepAlive   any            `json:"keep_alive,omitempty"`
	LogProbs    bool           `json:"logprobs,omitempty"`
	TopLogProbs int            `json:"top_logprobs,omitempty"`
}

type Message struct {
	Role      string     `json:"role"`
	Content   string     `json:"content,omitempty"`
	Thinking  string     `json:"thinking,omitempty"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
	ToolName  string     `json:"tool_name,omitempty"`
	Images    []string   `json:"images,omitempty"`
}

type Tool struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

type ToolFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

type ToolCall struct {
	Type     string           `json:"type,omitempty"`
	Function ToolCallFunction `json:"function"`
}

type ToolCallFunction struct {
	Index       int            `json:"index,omitempty"`
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Arguments   map[string]any `json:"arguments,omitempty"`
}

type Response struct {
	Model              string    `json:"model"`
	CreatedAt          string    `json:"created_at,omitempty"`
	Message            Message   `json:"message"`
	Done               bool      `json:"done"`
	DoneReason         string    `json:"done_reason,omitempty"`
	TotalDuration      int64     `json:"total_duration,omitempty"`
	LoadDuration       int64     `json:"load_duration,omitempty"`
	PromptEvalCount    int       `json:"prompt_eval_count,omitempty"`
	PromptEvalDuration int64     `json:"prompt_eval_duration,omitempty"`
	EvalCount          int       `json:"eval_count,omitempty"`
	EvalDuration       int64     `json:"eval_duration,omitempty"`
	LogProbs           []LogProb `json:"logprobs,omitempty"`
}

type LogProb struct {
	Token       string       `json:"token"`
	LogProb     float64      `json:"logprob"`
	Bytes       []int        `json:"bytes,omitempty"`
	TopLogProbs []TopLogProb `json:"top_logprobs,omitempty"`
}

type TopLogProb struct {
	Token   string  `json:"token"`
	LogProb float64 `json:"logprob"`
	Bytes   []int   `json:"bytes,omitempty"`
}


type TagsResponse struct {
	Models []TagModel `json:"models"`
}

type TagModel struct {
	Name       string         `json:"name"`
	Model      string         `json:"model"`
	ModifiedAt string         `json:"modified_at,omitempty"`
	Size       int64          `json:"size,omitempty"`
	Digest     string         `json:"digest,omitempty"`
	Details    map[string]any `json:"details,omitempty"`
}
