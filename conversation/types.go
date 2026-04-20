package conversation

import (
	"context"
	"sync"

	"github.com/codewandler/agentapis/api/unified"
	"github.com/codewandler/agentapis/client"
)

// Streamer is the minimal unified streaming dependency required by Session.
type Streamer interface {
	Stream(ctx context.Context, req unified.Request) (<-chan client.StreamResult, error)
}

// Strategy controls how conversation state is projected upstream.
type Strategy int

const (
	StrategyAuto Strategy = iota
	StrategyReplay
	StrategyResponsesPreviousResponseID
)

// Capabilities describes backend conversation optimization support.
type Capabilities struct {
	SupportsResponsesPreviousResponseID bool
}

// MessageProjector derives outbound messages from canonical session state.
type MessageProjector interface {
	ProjectMessages(state MessageProjectionState) ([]unified.Message, error)
}

// MessageProjectionState describes the canonical inputs available when projecting
// outbound messages for the next turn.
type MessageProjectionState struct {
	Defaults       ProjectionDefaults `json:"defaults"`
	History        []unified.Message  `json:"history,omitempty"`
	Pending        []unified.Message  `json:"pending,omitempty"`
	Strategy       Strategy           `json:"strategy"`
	Capabilities   Capabilities       `json:"capabilities,omitempty"`
	LastResponseID string             `json:"last_response_id,omitempty"`
}

// ProjectionDefaults contains session defaults relevant to outbound message projection.
type ProjectionDefaults struct {
	Model      string             `json:"model,omitempty"`
	Tools      []unified.Tool     `json:"tools,omitempty"`
	ToolChoice unified.ToolChoice `json:"tool_choice,omitempty"`
	System     []string           `json:"system,omitempty"`
	Developer  []string           `json:"developer,omitempty"`
}

// Request is the caller-facing payload for the next conversation step.
type Request struct {
	Model        string               `json:"model,omitempty"`
	Instructions []string             `json:"instructions,omitempty"`
	Tools        []unified.Tool       `json:"tools,omitempty"`
	ToolChoice   unified.ToolChoice   `json:"tool_choice,omitempty"`
	CacheHint    *unified.CacheHint   `json:"cache_hint,omitempty"`
	Inputs       []Input              `json:"inputs,omitempty"`
}

// Input is one request-local conversational input.
type Input struct {
	Role       unified.Role `json:"role"`
	Text       string       `json:"text,omitempty"`
	ToolResult *ToolResult  `json:"tool_result,omitempty"`
}

// ToolResult feeds tool output back into the conversation.
type ToolResult struct {
	ToolCallID string `json:"tool_call_id"`
	Output     string `json:"output"`
	IsError    bool   `json:"is_error,omitempty"`
}

// ReasoningRecord captures reasoning state observed for one committed turn.
type ReasoningRecord struct {
	Raw     string `json:"raw,omitempty"`
	Summary string `json:"summary,omitempty"`
}

func (r ReasoningRecord) HasContent() bool { return r.Raw != "" || r.Summary != "" }

// Session owns state for one ongoing conversation.
type Session struct {
	streamer Streamer
	defaults sessionDefaults
	strategy  Strategy
	caps      Capabilities
	projector MessageProjector

	mu        sync.Mutex
	history   []unified.Message
	reasoning []ReasoningRecord
	native    nativeState
	inFlight  bool
}

type sessionDefaults struct {
	model      string
	tools      []unified.Tool
	toolChoice unified.ToolChoice
	system     []string
	developer  []string
}

type nativeState struct {
	lastResponseID string
}
