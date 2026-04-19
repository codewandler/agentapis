package adapt

import (
	"fmt"
	"strings"

	"github.com/codewandler/agentapis/api/ollama"
	"github.com/codewandler/agentapis/api/unified"
)

func BuildOllamaRequest(r unified.Request) (*ollama.Request, error) {
	if err := Validate(r); err != nil {
		return nil, fmt.Errorf("validate unified request: %w", err)
	}
	if tc := r.ToolChoice; tc != nil {
		switch tc.(type) {
		case unified.ToolChoiceAuto:
			// okay
		default:
			return nil, fmt.Errorf("ollama request does not support tool choice %s", tc.String())
		}
	}

	out := &ollama.Request{Model: r.Model, Stream: true, Messages: make([]ollama.Message, 0, len(r.Messages))}
	options := map[string]any{}
	if r.MaxTokens > 0 { options["num_predict"] = r.MaxTokens }
	if r.Temperature > 0 { options["temperature"] = r.Temperature }
	if r.TopP > 0 { options["top_p"] = r.TopP }
	if r.TopK > 0 { options["top_k"] = r.TopK }

	if ox := r.Extras.Ollama; ox != nil {
		for k, v := range ox.Options {
			options[k] = v
		}
		if ox.KeepAlive != nil {
			out.KeepAlive = ox.KeepAlive
		}
		if ox.LogProbs {
			out.LogProbs = true
		}
		if ox.TopLogProbs > 0 {
			out.TopLogProbs = ox.TopLogProbs
		}
		if lvl := strings.TrimSpace(ox.ThinkLevel); lvl != "" {
			out.Think = lvl
		}
	}
	if len(options) > 0 { out.Options = options }
	if out.Think == nil {
		if r.Thinking.IsOn() { out.Think = true }
		if r.Thinking.IsOff() { out.Think = false }
	}
	if r.Output != nil {
		switch r.Output.Mode {
		case unified.OutputModeText:
		case unified.OutputModeJSONObject:
			out.Format = "json"
		case unified.OutputModeJSONSchema:
			out.Format = r.Output.Schema
		default:
			return nil, fmt.Errorf("unsupported output mode %q", r.Output.Mode)
		}
	}
	for _, t := range r.Tools {
		out.Tools = append(out.Tools, ollama.Tool{Type: "function", Function: ollama.ToolFunction{Name: t.Name, Description: t.Description, Parameters: cloneAnyMap(t.Parameters)}})
	}

	toolNamesByID := ollamaToolNamesByID(r.Messages)
	for _, m := range r.Messages {
		switch m.Role {
		case unified.RoleTool:
			for _, p := range m.Parts {
				if p.Type != unified.PartTypeToolResult || p.ToolResult == nil { continue }
				toolName := toolNamesByID[p.ToolResult.ToolCallID]
				if toolName == "" { toolName = p.ToolResult.ToolCallID }
				out.Messages = append(out.Messages, ollama.Message{Role: "tool", ToolName: toolName, Content: p.ToolResult.ToolOutput})
			}
			continue
		}

		wire := ollama.Message{}
		switch m.Role {
		case unified.RoleDeveloper, unified.RoleSystem:
			wire.Role = "system"
		case unified.RoleUser:
			wire.Role = "user"
		case unified.RoleAssistant:
			wire.Role = "assistant"
		default:
			wire.Role = string(m.Role)
		}
		for _, p := range m.Parts {
			switch p.Type {
			case unified.PartTypeText:
				wire.Content += p.Text
			case unified.PartTypeThinking:
				if p.Thinking != nil { wire.Thinking += p.Thinking.Text }
			case unified.PartTypeToolCall:
				if p.ToolCall != nil {
					wire.ToolCalls = append(wire.ToolCalls, ollama.ToolCall{Type: "function", Function: ollama.ToolCallFunction{Name: p.ToolCall.Name, Arguments: cloneAnyMap(p.ToolCall.Args)}})
				}
			case unified.PartTypeToolResult:
				return nil, fmt.Errorf("ollama assistant message cannot contain tool_result parts")
			}
		}
		out.Messages = append(out.Messages, wire)
	}
	return out, nil
}

func RequestFromOllama(r ollama.Request) (unified.Request, error) {
	u := unified.Request{Model: r.Model, Messages: make([]unified.Message, 0, len(r.Messages))}
	if r.Options != nil {
		ensureOllamaExtras(&u).Options = cloneAnyMap(r.Options)
		if v, ok := r.Options["num_predict"].(float64); ok { u.MaxTokens = int(v) }
		if v, ok := r.Options["num_predict"].(int); ok { u.MaxTokens = v }
		if v, ok := r.Options["temperature"].(float64); ok { u.Temperature = v }
		if v, ok := r.Options["top_p"].(float64); ok { u.TopP = v }
		if v, ok := r.Options["top_k"].(float64); ok { u.TopK = int(v) }
		if v, ok := r.Options["top_k"].(int); ok { u.TopK = v }
	}
	if r.KeepAlive != nil { ensureOllamaExtras(&u).KeepAlive = r.KeepAlive }
	if r.LogProbs { ensureOllamaExtras(&u).LogProbs = true }
	if r.TopLogProbs > 0 { ensureOllamaExtras(&u).TopLogProbs = r.TopLogProbs }
	switch v := r.Think.(type) {
	case bool:
		if v { u.Thinking = unified.ThinkingModeOn } else { u.Thinking = unified.ThinkingModeOff }
	case string:
		ensureOllamaExtras(&u).ThinkLevel = v
	}
	switch v := r.Format.(type) {
	case string:
		if v == "json" { u.Output = &unified.OutputSpec{Mode: unified.OutputModeJSONObject} }
	case map[string]any:
		u.Output = &unified.OutputSpec{Mode: unified.OutputModeJSONSchema, Schema: v}
	}
	for _, t := range r.Tools {
		u.Tools = append(u.Tools, unified.Tool{Name: t.Function.Name, Description: t.Function.Description, Parameters: cloneAnyMap(t.Function.Parameters)})
	}
	for _, m := range r.Messages {
		um := unified.Message{Parts: []unified.Part{}}
		switch m.Role {
		case "system": um.Role = unified.RoleSystem
		case "user": um.Role = unified.RoleUser
		case "assistant": um.Role = unified.RoleAssistant
		case "tool": um.Role = unified.RoleTool
		default: um.Role = unified.Role(m.Role)
		}
		if m.Content != "" {
			if um.Role == unified.RoleTool {
				um.Parts = append(um.Parts, unified.Part{Type: unified.PartTypeToolResult, ToolResult: &unified.ToolResult{ToolCallID: m.ToolName, ToolOutput: m.Content}})
			} else {
				um.Parts = append(um.Parts, unified.Part{Type: unified.PartTypeText, Text: m.Content})
			}
		}
		if m.Thinking != "" {
			um.Parts = append(um.Parts, unified.Part{Type: unified.PartTypeThinking, Thinking: &unified.ThinkingPart{Provider: "ollama", Text: m.Thinking}})
		}
		for _, tc := range m.ToolCalls {
			um.Parts = append(um.Parts, unified.Part{Type: unified.PartTypeToolCall, ToolCall: &unified.ToolCall{Name: tc.Function.Name, Args: cloneAnyMap(tc.Function.Arguments)}})
		}
		u.Messages = append(u.Messages, um)
	}
	return u, nil
}

func ollamaToolNamesByID(messages []unified.Message) map[string]string {
	out := map[string]string{}
	for _, m := range messages {
		if m.Role != unified.RoleAssistant { continue }
		for _, p := range m.Parts {
			if p.Type != unified.PartTypeToolCall || p.ToolCall == nil || p.ToolCall.ID == "" || p.ToolCall.Name == "" { continue }
			out[p.ToolCall.ID] = p.ToolCall.Name
		}
	}
	return out
}
