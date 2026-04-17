package adapt

import (
	"encoding/json"
	"fmt"

	"github.com/codewandler/agentapis/api/completions"
	"github.com/codewandler/agentapis/api/unified"
	"github.com/codewandler/agentapis/internal/sortmap"
)

// BuildCompletionsRequest converts a canonical unified request to a Chat Completions wire request.
func BuildCompletionsRequest(r unified.Request, _ ...CompletionsOption) (*completions.Request, error) {
	if err := Validate(r); err != nil {
		return nil, fmt.Errorf("validate unified request: %w", err)
	}

	out := &completions.Request{
		Model:         r.Model,
		Stream:        true,
		StreamOptions: &completions.StreamOptions{IncludeUsage: true},
		Messages:      make([]completions.Message, 0, len(r.Messages)),
	}

	if r.MaxTokens > 0 {
		out.MaxTokens = r.MaxTokens
	}
	if r.Temperature > 0 {
		out.Temperature = r.Temperature
	}
	if r.TopP > 0 {
		out.TopP = r.TopP
	}
	if r.TopK > 0 {
		out.TopK = r.TopK
	}
	if r.Output != nil {
		switch r.Output.Mode {
		case unified.OutputModeText:
			// omit
		case unified.OutputModeJSONObject:
			out.ResponseFormat = &completions.ResponseFormat{Type: "json_object"}
		case unified.OutputModeJSONSchema:
			return nil, fmt.Errorf("chat completions request does not support output mode %q", r.Output.Mode)
		default:
			return nil, fmt.Errorf("unsupported output mode %q", r.Output.Mode)
		}
	}

	if !r.Effort.IsEmpty() {
		out.ReasoningEffort = string(r.Effort)
	}

	cextras := r.Extras.Completions
	if cextras != nil {
		out.Stop = append([]string(nil), cextras.Stop...)
		out.N = cextras.N
		out.PresencePenalty = cextras.PresencePenalty
		out.FrequencyPenalty = cextras.FrequencyPenalty
		out.LogProbs = cextras.LogProbs
		out.TopLogProbs = cextras.TopLogProbs
		out.Store = cextras.Store
		out.ParallelToolCalls = cextras.ParallelToolCalls
		out.ServiceTier = cextras.ServiceTier
		out.PromptCacheRetention = cextras.PromptCacheRetention
	}
	if retention := promptCacheRetentionFromHint(r.CacheHint); retention != "" {
		out.PromptCacheRetention = retention
	}
	out.User, out.Metadata = metadataToOpenAI(r.Metadata, nil)
	if cextras != nil {
		_, out.Metadata = metadataToOpenAI(r.Metadata, cextras.ExtraMetadata)
		out.User, _ = metadataToOpenAI(r.Metadata, nil)
	}

	for _, t := range r.Tools {
		out.Tools = append(out.Tools, completions.Tool{
			Type: "function",
			Function: completions.FuncPayload{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  sortmap.NewSortedMap(t.Parameters),
				Strict:      t.Strict,
			},
		})
	}

	if len(r.Tools) > 0 {
		switch tc := r.ToolChoice.(type) {
		case nil, unified.ToolChoiceAuto:
			out.ToolChoice = "auto"
		case unified.ToolChoiceRequired:
			out.ToolChoice = "required"
		case unified.ToolChoiceNone:
			out.ToolChoice = "none"
		case unified.ToolChoiceTool:
			out.ToolChoice = map[string]any{
				"type":     "function",
				"function": map[string]string{"name": tc.Name},
			}
		default:
			return nil, fmt.Errorf("unsupported tool choice type %T", r.ToolChoice)
		}
	}

	for _, m := range r.Messages {
		wire := completions.Message{}
		content, err := buildCompletionsContent(m)
		if err != nil {
			return nil, err
		}
		wire.Content = content
		switch m.Role {
		case unified.RoleSystem, unified.RoleDeveloper:
			wire.Role = string(unified.RoleSystem)
		case unified.RoleUser:
			wire.Role = string(unified.RoleUser)
		case unified.RoleAssistant:
			wire.Role = string(unified.RoleAssistant)
			for _, p := range m.Parts {
				if p.Type != unified.PartTypeToolCall || p.ToolCall == nil {
					continue
				}
				argRaw, _ := json.Marshal(p.ToolCall.Args)
				wire.ToolCalls = append(wire.ToolCalls, completions.ToolCall{
					ID:   p.ToolCall.ID,
					Type: "function",
					Function: completions.FuncCall{
						Name:      p.ToolCall.Name,
						Arguments: string(argRaw),
					},
				})
			}
		case unified.RoleTool:
			for _, p := range m.Parts {
				if p.Type != unified.PartTypeToolResult || p.ToolResult == nil {
					continue
				}
				out.Messages = append(out.Messages, completions.Message{
					Role:       string(unified.RoleTool),
					Content:    p.ToolResult.ToolOutput,
					ToolCallID: p.ToolResult.ToolCallID,
				})
			}
			continue
		default:
			wire.Role = string(m.Role)
		}
		out.Messages = append(out.Messages, wire)
	}

	return out, nil
}

// RequestFromCompletions converts a Chat Completions wire request to unified.
func RequestFromCompletions(r completions.Request) (unified.Request, error) {
	u := unified.Request{
		Model:       r.Model,
		MaxTokens:   r.MaxTokens,
		Temperature: r.Temperature,
		TopP:        r.TopP,
		TopK:        r.TopK,
		Messages:    make([]unified.Message, 0, len(r.Messages)),
	}

	if r.ResponseFormat != nil {
		switch r.ResponseFormat.Type {
		case "json_object":
			u.Output = &unified.OutputSpec{Mode: unified.OutputModeJSONObject}
		case "text":
			u.Output = &unified.OutputSpec{Mode: unified.OutputModeText}
		}
	}
	if r.ReasoningEffort != "" {
		u.Effort = unified.Effort(r.ReasoningEffort)
	}
	if hint := cacheHintFromPromptCacheRetention(r.PromptCacheRetention); hint != nil {
		u.CacheHint = hint
	}
	if meta, extra := metadataFromOpenAI(r.User, r.Metadata); meta != nil {
		u.Metadata = meta
		if extra != nil {
			ensureCompletionsExtras(&u).ExtraMetadata = extra
		}
	} else if extra != nil {
		ensureCompletionsExtras(&u).ExtraMetadata = extra
	}
	if r.PromptCacheRetention != "" {
		ensureCompletionsExtras(&u).PromptCacheRetention = r.PromptCacheRetention
	}
	if len(r.Stop) > 0 {
		ensureCompletionsExtras(&u).Stop = append([]string(nil), r.Stop...)
	}
	if r.N > 0 || r.PresencePenalty != 0 || r.FrequencyPenalty != 0 || r.LogProbs || r.TopLogProbs > 0 || r.Store || r.ParallelToolCalls || r.ServiceTier != "" {
		extras := ensureCompletionsExtras(&u)
		extras.N = r.N
		extras.PresencePenalty = r.PresencePenalty
		extras.FrequencyPenalty = r.FrequencyPenalty
		extras.LogProbs = r.LogProbs
		extras.TopLogProbs = r.TopLogProbs
		extras.Store = r.Store
		extras.ParallelToolCalls = r.ParallelToolCalls
		extras.ServiceTier = r.ServiceTier
	}

	for _, t := range r.Tools {
		u.Tools = append(u.Tools, unified.Tool{
			Name:        t.Function.Name,
			Description: t.Function.Description,
			Parameters:  toMap(t.Function.Parameters),
			Strict:      t.Function.Strict,
		})
	}
	u.ToolChoice = toolChoiceFromCompletions(r.ToolChoice)

	for _, m := range r.Messages {
		um := unified.Message{Role: unified.Role(m.Role), Parts: make([]unified.Part, 0, 2)}
		switch m.Role {
		case string(unified.RoleSystem):
			um.Role = unified.RoleSystem
		case string(unified.RoleUser):
			um.Role = unified.RoleUser
		case string(unified.RoleAssistant):
			um.Role = unified.RoleAssistant
		case string(unified.RoleTool):
			um.Role = unified.RoleTool
		}

		if text, ok := m.Content.(string); ok && text != "" {
			um.Parts = append(um.Parts, unified.Part{Type: unified.PartTypeText, Text: text})
		} else if m.Content != nil {
			if raw, err := json.Marshal(m.Content); err == nil {
				um.Parts = append(um.Parts, unified.Part{Native: raw})
			}
		}
		for _, tc := range m.ToolCalls {
			var args map[string]any
			if tc.Function.Arguments != "" {
				_ = json.Unmarshal([]byte(tc.Function.Arguments), &args)
			}
			um.Parts = append(um.Parts, unified.Part{Type: unified.PartTypeToolCall, ToolCall: &unified.ToolCall{ID: tc.ID, Name: tc.Function.Name, Args: args}})
		}
		if m.ToolCallID != "" {
			um.Parts = append(um.Parts, unified.Part{Type: unified.PartTypeToolResult, ToolResult: &unified.ToolResult{ToolCallID: m.ToolCallID, ToolOutput: contentString(m.Content)}})
		}
		if len(um.Parts) == 0 {
			um.Parts = []unified.Part{{Type: unified.PartTypeText, Text: ""}}
		}
		u.Messages = append(u.Messages, um)
	}

	if err := Validate(u); err != nil {
		return unified.Request{}, err
	}
	return u, nil
}

type CompletionsOption func(*completionsOptions)

type completionsOptions struct{}

func toolChoiceFromCompletions(v any) unified.ToolChoice {
	switch t := v.(type) {
	case nil:
		return nil
	case string:
		switch t {
		case "auto":
			return unified.ToolChoiceAuto{}
		case "required":
			return unified.ToolChoiceRequired{}
		case "none":
			return unified.ToolChoiceNone{}
		}
	case map[string]any:
		if typ, _ := t["type"].(string); typ == "function" {
			if fn, ok := t["function"].(map[string]any); ok {
				if name, _ := fn["name"].(string); name != "" {
					return unified.ToolChoiceTool{Name: name}
				}
			}
			if fn, ok := t["function"].(map[string]string); ok {
				if name := fn["name"]; name != "" {
					return unified.ToolChoiceTool{Name: name}
				}
			}
		}
	}
	return nil
}

func buildCompletionsContent(m unified.Message) (any, error) {
	var native any
	hasNative := false
	hasCanonical := false

	for _, p := range m.Parts {
		if p.Native != nil {
			if hasNative {
				return nil, fmt.Errorf("chat completions message cannot project multiple native content parts")
			}
			if partHasCanonicalFields(p) {
				return nil, fmt.Errorf("chat completions native content part must not also carry canonical fields")
			}
			native = json.RawMessage(p.Native)
			hasNative = true
			continue
		}
		if partContributesCanonicalContent(p) {
			hasCanonical = true
		}
	}

	if hasNative && hasCanonical {
		return nil, fmt.Errorf("chat completions message cannot mix native content with canonical parts")
	}
	if hasNative {
		return native, nil
	}
	return partsText(m.Parts), nil
}
