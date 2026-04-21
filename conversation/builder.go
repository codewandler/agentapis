package conversation

import "github.com/codewandler/agentapis/api/unified"

// Builder incrementally constructs a Request.
type Builder struct {
	req Request
}

func NewRequest() *Builder { return &Builder{} }

func (b *Builder) Model(model string) *Builder { b.req.Model = model; return b }
func (b *Builder) MaxTokens(max int) *Builder { b.req.MaxTokens = max; return b }
func (b *Builder) Temperature(v float64) *Builder { b.req.Temperature = v; return b }
func (b *Builder) Effort(v unified.Effort) *Builder { b.req.Effort = v; return b }
func (b *Builder) Thinking(v unified.ThinkingMode) *Builder { b.req.Thinking = v; return b }
func (b *Builder) Instructions(lines ...string) *Builder { b.req.Instructions = append(b.req.Instructions, lines...); return b }
func (b *Builder) Tools(tools []unified.Tool) *Builder { b.req.Tools = append([]unified.Tool(nil), tools...); return b }
func (b *Builder) ToolChoice(choice unified.ToolChoice) *Builder { b.req.ToolChoice = choice; return b }
func (b *Builder) CachePolicy(p CachePolicy) *Builder { b.req.CachePolicy = p; return b }
func (b *Builder) CacheHint(h *unified.CacheHint) *Builder {
	if h == nil {
		b.req.CacheHint = nil
		return b
	}
	cpy := *h
	b.req.CacheHint = &cpy
	return b
}
func (b *Builder) Input(in Input) *Builder { b.req.Inputs = append(b.req.Inputs, in); return b }
func (b *Builder) User(text string) *Builder { b.req.Inputs = append(b.req.Inputs, Input{Role: unified.RoleUser, Text: text}); return b }
func (b *Builder) Developer(text string) *Builder { b.req.Inputs = append(b.req.Inputs, Input{Role: unified.RoleDeveloper, Text: text}); return b }
func (b *Builder) System(text string) *Builder { b.req.Inputs = append(b.req.Inputs, Input{Role: unified.RoleSystem, Text: text}); return b }
func (b *Builder) ToolResult(callID, output string) *Builder {
	b.req.Inputs = append(b.req.Inputs, Input{Role: unified.RoleTool, ToolResult: &ToolResult{ToolCallID: callID, Output: output}})
	return b
}

func (b *Builder) ToolResultWithError(callID, output string, isError bool) *Builder {
	b.req.Inputs = append(b.req.Inputs, Input{Role: unified.RoleTool, ToolResult: &ToolResult{ToolCallID: callID, Output: output, IsError: isError}})
	return b
}
func (b *Builder) Build() Request { return b.req }
