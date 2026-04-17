package adapt

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/codewandler/agentapis/api/responses"
	"github.com/codewandler/agentapis/api/unified"
	"github.com/codewandler/agentapis/internal/sortmap"
)

// BuildResponsesRequest converts a canonical unified request to a Responses API wire request.
func BuildResponsesRequest(r unified.Request, _ ...ResponsesOption) (*responses.Request, error) {
	if err := Validate(r); err != nil {
		return nil, fmt.Errorf("validate unified request: %w", err)
	}

	out := &responses.Request{
		Model:  r.Model,
		Stream: true,
		Input:  make([]responses.Input, 0, len(r.Messages)),
	}

	rextras := r.Extras.Responses
	usedMaxField := "max_output_tokens"
	if rextras != nil && rextras.UsedMaxTokenField != "" {
		usedMaxField = rextras.UsedMaxTokenField
	}
	if r.MaxTokens > 0 {
		if usedMaxField == "max_tokens" {
			out.MaxTokens = r.MaxTokens
		} else {
			out.MaxOutputTokens = r.MaxTokens
		}
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
			out.ResponseFormat = &responses.ResponseFormat{Type: "json_object"}
		case unified.OutputModeJSONSchema:
			return nil, fmt.Errorf("responses request does not support output mode %q", r.Output.Mode)
		default:
			return nil, fmt.Errorf("unsupported output mode %q", r.Output.Mode)
		}
	}
	if !r.Effort.IsEmpty() || (rextras != nil && rextras.ReasoningSummary != "") {
		out.Reasoning = &responses.Reasoning{Effort: string(r.Effort)}
		if rextras != nil {
			out.Reasoning.Summary = rextras.ReasoningSummary
		}
	}
	if rextras != nil {
		out.PromptCacheRetention = rextras.PromptCacheRetention
		out.PreviousResponseID = rextras.PreviousResponseID
		out.Store = rextras.Store
		out.ParallelToolCalls = rextras.ParallelToolCalls
	}
	if retention := promptCacheRetentionFromHint(r.CacheHint); retention != "" {
		out.PromptCacheRetention = retention
	}
	out.User, out.Metadata = metadataToOpenAI(r.Metadata, nil)
	if rextras != nil {
		_, out.Metadata = metadataToOpenAI(r.Metadata, rextras.ExtraMetadata)
		out.User, _ = metadataToOpenAI(r.Metadata, nil)
	}

	for _, t := range r.Tools {
		out.Tools = append(out.Tools, responses.Tool{
			Type:        "function",
			Name:        t.Name,
			Description: t.Description,
			Parameters:  sortmap.NewSortedMap(t.Parameters),
			Strict:      t.Strict,
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
			out.ToolChoice = map[string]any{"type": "function", "name": tc.Name}
		default:
			return nil, fmt.Errorf("unsupported tool choice type %T", r.ToolChoice)
		}
	}

	useInstructions := true
	if rextras != nil && rextras.UseInstructions != nil {
		useInstructions = *rextras.UseInstructions
	}
	instructions, remaining, err := consumeResponsesInstruction(r.Messages, useInstructions)
	if err != nil {
		return nil, err
	}
	out.Instructions = instructions
	for _, m := range remaining {
		switch m.Role {
		case unified.RoleSystem:
			return nil, fmt.Errorf("responses request cannot project additional system messages")
		case unified.RoleDeveloper:
			out.Input = append(out.Input, responses.Input{Role: string(unified.RoleDeveloper), Content: partsText(m.Parts)})
		case unified.RoleUser:
			out.Input = append(out.Input, responses.Input{Role: string(unified.RoleUser), Content: partsText(m.Parts)})
		case unified.RoleAssistant:
			inputs, err := buildResponsesAssistantInputs(m)
			if err != nil {
				return nil, err
			}
			out.Input = append(out.Input, inputs...)
		case unified.RoleTool:
			for _, p := range m.Parts {
				if p.Type != unified.PartTypeToolResult || p.ToolResult == nil {
					continue
				}
				out.Input = append(out.Input, responses.Input{
					Type:   "function_call_output",
					CallID: p.ToolResult.ToolCallID,
					Output: p.ToolResult.ToolOutput,
				})
			}
		}
	}

	return out, nil
}

// RequestFromResponses converts a Responses wire request to unified.
func RequestFromResponses(r responses.Request) (unified.Request, error) {
	u := unified.Request{
		Model:       r.Model,
		Temperature: r.Temperature,
		TopP:        r.TopP,
		TopK:        r.TopK,
		Messages:    make([]unified.Message, 0, len(r.Input)+1),
	}
	if r.MaxOutputTokens > 0 {
		u.MaxTokens = r.MaxOutputTokens
		ensureResponsesExtras(&u).UsedMaxTokenField = "max_output_tokens"
	} else if r.MaxTokens > 0 {
		u.MaxTokens = r.MaxTokens
		ensureResponsesExtras(&u).UsedMaxTokenField = "max_tokens"
	}
	if r.ResponseFormat != nil {
		switch r.ResponseFormat.Type {
		case "json_object":
			u.Output = &unified.OutputSpec{Mode: unified.OutputModeJSONObject}
		case "text":
			u.Output = &unified.OutputSpec{Mode: unified.OutputModeText}
		}
	}
	if r.Reasoning != nil {
		if r.Reasoning.Effort != "" {
			u.Effort = unified.Effort(r.Reasoning.Effort)
		}
		if r.Reasoning.Summary != "" {
			ensureResponsesExtras(&u).ReasoningSummary = r.Reasoning.Summary
		}
	}
	if hint := cacheHintFromPromptCacheRetention(r.PromptCacheRetention); hint != nil {
		u.CacheHint = hint
	}
	if meta, extra := metadataFromOpenAI(r.User, r.Metadata); meta != nil {
		u.Metadata = meta
		if extra != nil {
			ensureResponsesExtras(&u).ExtraMetadata = extra
		}
	} else if extra != nil {
		ensureResponsesExtras(&u).ExtraMetadata = extra
	}
	if r.PromptCacheRetention != "" {
		ensureResponsesExtras(&u).PromptCacheRetention = r.PromptCacheRetention
	}
	if r.PreviousResponseID != "" {
		ensureResponsesExtras(&u).PreviousResponseID = r.PreviousResponseID
	}
	if r.Store || r.ParallelToolCalls {
		extras := ensureResponsesExtras(&u)
		extras.Store = r.Store
		extras.ParallelToolCalls = r.ParallelToolCalls
	}

	for _, t := range r.Tools {
		u.Tools = append(u.Tools, unified.Tool{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  toMap(t.Parameters),
			Strict:      t.Strict,
		})
	}
	u.ToolChoice = toolChoiceFromResponses(r.ToolChoice)

	if r.Instructions != "" {
		useInstructions := true
		ensureResponsesExtras(&u).UseInstructions = &useInstructions
		u.Messages = append(u.Messages, unified.Message{Role: unified.RoleSystem, Parts: []unified.Part{{Type: unified.PartTypeText, Text: r.Instructions}}})
	} else {
		useInstructions := false
		ensureResponsesExtras(&u).UseInstructions = &useInstructions
	}

	var assistantTurn responsesAssistantTurn
	flushAssistantTurn := func() {
		if assistantTurn.empty() {
			return
		}
		u.Messages = append(u.Messages, unified.Message{Role: unified.RoleAssistant, Phase: assistantTurn.phase, Parts: assistantTurn.parts})
		assistantTurn.reset()
	}

	for _, in := range r.Input {
		switch {
		case in.Role == string(unified.RoleDeveloper):
			flushAssistantTurn()
			u.Messages = append(u.Messages, unified.Message{Role: unified.RoleDeveloper, Parts: []unified.Part{{Type: unified.PartTypeText, Text: in.Content}}})
		case in.Role == string(unified.RoleUser):
			flushAssistantTurn()
			u.Messages = append(u.Messages, unified.Message{Role: unified.RoleUser, Parts: []unified.Part{{Type: unified.PartTypeText, Text: in.Content}}})
		case in.Role == string(unified.RoleAssistant):
			phase, err := responsesAssistantPhase(in.Phase)
			if err != nil {
				return unified.Request{}, err
			}
			if assistantTurn.shouldFlush(phase) {
				flushAssistantTurn()
			}
			assistantTurn.adoptPhase(phase)
			if in.Content != "" {
				assistantTurn.parts = append(assistantTurn.parts, unified.Part{Type: unified.PartTypeText, Text: in.Content})
			}
		case in.Type == "function_call":
			phase, err := responsesAssistantPhase(in.Phase)
			if err != nil {
				return unified.Request{}, err
			}
			if assistantTurn.shouldFlush(phase) {
				flushAssistantTurn()
			}
			assistantTurn.adoptPhase(phase)
			var args map[string]any
			if in.Arguments != "" {
				_ = json.Unmarshal([]byte(in.Arguments), &args)
			}
			assistantTurn.parts = append(assistantTurn.parts, unified.Part{Type: unified.PartTypeToolCall, ToolCall: &unified.ToolCall{ID: in.CallID, Name: in.Name, Args: args}})
		case in.Type == "function_call_output":
			flushAssistantTurn()
			u.Messages = append(u.Messages, unified.Message{Role: unified.RoleTool, Parts: []unified.Part{{Type: unified.PartTypeToolResult, ToolResult: &unified.ToolResult{ToolCallID: in.CallID, ToolOutput: in.Output}}}})
		}
	}
	flushAssistantTurn()

	if err := Validate(u); err != nil {
		return unified.Request{}, err
	}
	return u, nil
}

type ResponsesOption func(*responsesOptions)

type responsesOptions struct{}

func consumeResponsesInstruction(messages []unified.Message, useInstructions bool) (string, []unified.Message, error) {
	if len(messages) == 0 {
		return "", messages, nil
	}
	if messages[0].Role == unified.RoleSystem {
		if !useInstructions {
			return "", nil, fmt.Errorf("responses request cannot project system messages when UseInstructions=false")
		}
		if !isTextOnlyMessage(messages[0]) {
			return "", nil, fmt.Errorf("responses request requires leading system message to be text-only")
		}
		for i := 1; i < len(messages); i++ {
			if messages[i].Role == unified.RoleSystem {
				return "", nil, fmt.Errorf("responses request cannot project multiple system messages")
			}
		}
		return partsText(messages[0].Parts), messages[1:], nil
	}
	for _, m := range messages {
		if m.Role == unified.RoleSystem {
			if !useInstructions {
				return "", nil, fmt.Errorf("responses request cannot project system messages when UseInstructions=false")
			}
			return "", nil, fmt.Errorf("responses request requires system message to be first")
		}
	}
	return "", messages, nil
}

func buildResponsesAssistantInputs(m unified.Message) ([]responses.Input, error) {
	inputs := make([]responses.Input, 0, len(m.Parts)+1)
	var text strings.Builder
	seenToolCall := false
	phase := string(m.Phase)

	for _, p := range m.Parts {
		switch {
		case p.Native != nil:
			return nil, fmt.Errorf("responses assistant message does not support native parts")
		case p.Type == unified.PartTypeText:
			if seenToolCall && p.Text != "" {
				return nil, fmt.Errorf("responses assistant message cannot contain text after tool calls")
			}
			text.WriteString(p.Text)
		case p.Type == unified.PartTypeToolCall:
			if p.ToolCall == nil {
				return nil, fmt.Errorf("responses assistant message has invalid tool call part")
			}
			seenToolCall = true
			argRaw, _ := json.Marshal(p.ToolCall.Args)
			inputs = append(inputs, responses.Input{
				Type:      "function_call",
				CallID:    p.ToolCall.ID,
				Name:      p.ToolCall.Name,
				Arguments: string(argRaw),
				Phase:     phase,
			})
		case p.Type == unified.PartTypeThinking:
			return nil, fmt.Errorf("responses assistant message does not support thinking parts")
		case p.Type == unified.PartTypeToolResult:
			return nil, fmt.Errorf("responses assistant message does not support tool result parts")
		default:
			return nil, fmt.Errorf("responses assistant message does not support part type %q", p.Type)
		}
	}

	if text.Len() > 0 {
		inputs = append([]responses.Input{{Role: string(unified.RoleAssistant), Content: text.String(), Phase: phase}}, inputs...)
	}
	return inputs, nil
}

func toolChoiceFromResponses(v any) unified.ToolChoice {
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
			if name, _ := t["name"].(string); name != "" {
				return unified.ToolChoiceTool{Name: name}
			}
		}
	}
	return nil
}

type responsesAssistantTurn struct {
	phase unified.AssistantPhase
	parts []unified.Part
}

func (t responsesAssistantTurn) empty() bool {
	return len(t.parts) == 0
}

func (t *responsesAssistantTurn) reset() {
	t.phase = ""
	t.parts = nil
}

func (t *responsesAssistantTurn) shouldFlush(phase unified.AssistantPhase) bool {
	if t.empty() || phase.IsEmpty() || t.phase.IsEmpty() {
		return false
	}
	return t.phase != phase
}

func (t *responsesAssistantTurn) adoptPhase(phase unified.AssistantPhase) {
	if t.phase.IsEmpty() {
		t.phase = phase
	}
}

func responsesAssistantPhase(raw string) (unified.AssistantPhase, error) {
	phase := unified.AssistantPhase(raw)
	if !phase.Valid() {
		return "", fmt.Errorf("responses assistant item has invalid phase %q", raw)
	}
	return phase, nil
}
