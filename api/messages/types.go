package messages

import "encoding/json"

type Request struct {
	Model         string           `json:"model"`
	MaxTokens     int              `json:"max_tokens"`
	Stream        bool             `json:"stream"`
	System        SystemBlocks     `json:"system,omitempty"`
	Messages      []Message        `json:"messages"`
	Tools         []ToolDefinition `json:"tools,omitempty"`
	ToolChoice    any              `json:"tool_choice,omitempty"`
	Thinking      *ThinkingConfig  `json:"thinking,omitempty"`
	Metadata      *Metadata        `json:"metadata,omitempty"`
	CacheControl  *CacheControl    `json:"cache_control,omitempty"`
	TopK          int              `json:"top_k,omitempty"`
	TopP          float64          `json:"top_p,omitempty"`
	Temperature   float64          `json:"temperature,omitempty"`
	StopSequences []string         `json:"stop_sequences,omitempty"`
	OutputConfig  *OutputConfig    `json:"output_config,omitempty"`
}

type ThinkingConfig struct {
	Type         string `json:"type"`
	BudgetTokens int    `json:"budget_tokens,omitempty"`
	Display      string `json:"display,omitempty"`
}

type OutputConfig struct {
	Format *JSONOutputFormat `json:"format,omitempty"`
	Effort string            `json:"effort,omitempty"`
}

type JSONOutputFormat struct {
	Type   string `json:"type"`
	Schema any    `json:"schema,omitempty"`
}

type Metadata struct {
	UserID string `json:"user_id,omitempty"`
}

type SystemBlocks []*TextBlock

type Message struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

type TextBlock struct {
	Type         string        `json:"type"`
	Text         string        `json:"text"`
	CacheControl *CacheControl `json:"cache_control,omitempty"`
}

type ImageBlock struct {
	Type   string      `json:"type"`
	Source ImageSource `json:"source"`
}

type ImageSource struct {
	Type      string `json:"type"`
	MediaType string `json:"media_type,omitempty"`
	Data      string `json:"data,omitempty"`
	URL       string `json:"url,omitempty"`
}

type ToolUseBlock struct {
	Type         string          `json:"type"`
	ID           string          `json:"id"`
	Name         string          `json:"name"`
	Input        json.RawMessage `json:"input"`
	CacheControl *CacheControl   `json:"cache_control,omitempty"`
}

type ToolResultBlock struct {
	Type         string        `json:"type"`
	ToolUseID    string        `json:"tool_use_id"`
	Content      string        `json:"content"`
	IsError      bool          `json:"is_error,omitempty"`
	CacheControl *CacheControl `json:"cache_control,omitempty"`
}

type ThinkingBlock struct {
	Type         string        `json:"type"`
	Thinking     string        `json:"thinking"`
	Signature    string        `json:"signature,omitempty"`
	CacheControl *CacheControl `json:"cache_control,omitempty"`
}

type ToolDefinition struct {
	Name         string        `json:"name"`
	Description  string        `json:"description"`
	InputSchema  any           `json:"input_schema"`
	CacheControl *CacheControl `json:"cache_control,omitempty"`
}

type CacheControl struct {
	Type string `json:"type"`
	TTL  string `json:"ttl,omitempty"`
}

type MessageStartEvent struct {
	Message MessageStartPayload `json:"message"`
}

type MessageStartPayload struct {
	ID    string       `json:"id"`
	Model string       `json:"model"`
	Usage MessageUsage `json:"usage"`
}

type MessageUsage struct {
	InputTokens              int `json:"input_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
}

type ContentBlockStartEvent struct {
	Index        int             `json:"index"`
	ContentBlock json.RawMessage `json:"content_block"`
}

type StartBlockView struct {
	Type string `json:"type"`
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
	Data string `json:"data,omitempty"`
}

type ContentBlockDeltaEvent struct {
	Index int   `json:"index"`
	Delta Delta `json:"delta"`
}

type Delta struct {
	Type        string `json:"type"`
	Text        string `json:"text,omitempty"`
	PartialJSON string `json:"partial_json,omitempty"`
	Thinking    string `json:"thinking,omitempty"`
	Signature   string `json:"signature,omitempty"`
}

type ContentBlockStopEvent struct {
	Index int `json:"index"`
}

type TextCompleteEvent struct {
	Index int
	Text  string
}

type ThinkingCompleteEvent struct {
	Index     int
	Thinking  string
	Signature string
}

type ToolCompleteEvent struct {
	Index    int
	ID       string
	Name     string
	RawInput string
	Args     map[string]any
}

type MessageDeltaEvent struct {
	Delta struct {
		StopReason string `json:"stop_reason"`
	} `json:"delta"`
	Usage struct {
		InputTokens              int `json:"input_tokens,omitempty"`
		OutputTokens             int `json:"output_tokens"`
		CacheCreationInputTokens int `json:"cache_creation_input_tokens,omitempty"`
		CacheReadInputTokens     int `json:"cache_read_input_tokens,omitempty"`
	} `json:"usage"`
}

type MessageStopEvent struct{}

type StreamErrorEvent struct {
	Type string `json:"type"`
	Err  struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

func (e *StreamErrorEvent) Error() string {
	if e.Err.Type != "" {
		return "messages stream error " + e.Err.Type + ": " + e.Err.Message
	}
	return "messages stream error: " + e.Err.Message
}

type PingEvent struct{}
