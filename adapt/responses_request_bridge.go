package adapt

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/codewandler/agentapis/api/responses"
	"github.com/codewandler/agentapis/api/unified"
	"github.com/codewandler/agentapis/internal/sortmap"
)

// Field coverage for responses.Request:
//   model                  ← r.Model
//   input                  ← r.Messages (via item constructors)
//   instructions           ← first system message (if UseInstructions=true)
//   stream                 ← always true (streaming only)
//   tools                  ← r.Tools
//   tool_choice            ← r.ToolChoice
//   reasoning              ← r.Effort + rextras.ReasoningSummary
//   max_output_tokens      ← r.MaxTokens
//   temperature            ← r.Temperature
//   top_p                  ← r.TopP
//   metadata               ← rextras.OpenAIMetadata
//   user                   ← r.Identity.User
//   store                  ← rextras.Store
//   parallel_tool_calls    ← rextras.ParallelToolCalls
//   previous_response_id   ← rextras.PreviousResponseID
//   prompt_cache_retention ← rextras.PromptCacheRetention or r.CacheHint
//   prompt_cache_key       ← rextras.PromptCacheKey
//   text                   ← r.Output (format only)
//   service_tier           ← rextras.ServiceTier
//   truncation             ← rextras.Truncation
//   include                ← rextras.Include
//   background             ← rextras.Background
//   max_tool_calls         ← rextras.MaxToolCalls
//   top_logprobs           ← rextras.TopLogprobs
//   conversation           ← rextras.ConversationID
//   safety_identifier      NOT BRIDGED — use RequestTransform
//   prompt                 NOT BRIDGED — use RequestTransform
//   context_management     NOT BRIDGED — use RequestTransform
//   stream_options         NOT BRIDGED — use RequestTransform

// BuildResponsesRequest converts a canonical unified request to a Responses API wire request.
func BuildResponsesRequest(r unified.Request, _ ...ResponsesOption) (*responses.Request, error) {
	if err := Validate(r); err != nil {
		return nil, fmt.Errorf("validate unified request: %w", err)
	}

	out := &responses.Request{
		Model:  r.Model,
		Stream: ptrBool(true),
	}

	rextras := r.Extras.Responses

	if r.MaxTokens > 0 {
		out.MaxOutputTokens = ptrInt(r.MaxTokens)
	}
	if r.Temperature > 0 {
		out.Temperature = ptrFloat64(r.Temperature)
	}
	if r.TopP > 0 {
		out.TopP = ptrFloat64(r.TopP)
	}
	if r.Output != nil {
		switch r.Output.Mode {
		case unified.OutputModeText:
			// omit — text is the default
		case unified.OutputModeJSONObject:
			f := responses.FormatJSONObject()
			out.Text = &responses.ResponseTextParam{Format: &f}
		case unified.OutputModeJSONSchema:
			if r.Output.Schema == nil {
				return nil, fmt.Errorf("json_schema output mode requires a schema")
			}
			schemaMap := toMap(r.Output.Schema)
			f := responses.FormatJSONSchema("response", schemaMap, ptrBool(true), nil)
			out.Text = &responses.ResponseTextParam{Format: &f}
		default:
			return nil, fmt.Errorf("unsupported output mode %q", r.Output.Mode)
		}
	}
	if !r.Effort.IsEmpty() || (rextras != nil && rextras.ReasoningSummary != "") {
		reasoning := &responses.Reasoning{}
		if !r.Effort.IsEmpty() {
			e := responses.ReasoningEffort(r.Effort)
			reasoning.Effort = &e
		}
		if rextras != nil && rextras.ReasoningSummary != "" {
			s := responses.ReasoningSummary(rextras.ReasoningSummary)
			reasoning.Summary = &s
		}
		out.Reasoning = reasoning
	}
	if rextras != nil {
		if rextras.PromptCacheRetention != "" {
			pcr := responses.PromptCacheRetention(rextras.PromptCacheRetention)
			out.PromptCacheRetention = &pcr
		}
		out.PromptCacheKey = rextras.PromptCacheKey
		if rextras.PreviousResponseID != "" {
			out.PreviousResponseID = ptrString(rextras.PreviousResponseID)
		}
		if rextras.Store {
			out.Store = ptrBool(true)
		}
		if rextras.ParallelToolCalls {
			out.ParallelToolCalls = ptrBool(true)
		}
		if rextras.ServiceTier != "" {
			st := responses.ServiceTier(rextras.ServiceTier)
			out.ServiceTier = &st
		}
		if rextras.Truncation != "" {
			tr := responses.Truncation(rextras.Truncation)
			out.Truncation = &tr
		}
		if len(rextras.Include) > 0 {
			for _, inc := range rextras.Include {
				out.Include = append(out.Include, responses.IncludeItem(inc))
			}
		}
		if rextras.Background != nil {
			out.Background = rextras.Background
		}
		if rextras.MaxToolCalls != nil {
			out.MaxToolCalls = rextras.MaxToolCalls
		}
		if rextras.TopLogprobs != nil {
			out.TopLogprobs = rextras.TopLogprobs
		}
		if rextras.ConversationID != "" {
			c := responses.ConversationByID(rextras.ConversationID)
			out.Conversation = &c
		}
	}
	if retention := promptCacheRetentionFromHint(r.CacheHint); retention != "" {
		pcr := responses.PromptCacheRetention(retention)
		out.PromptCacheRetention = &pcr
	}
	out.User = wireUser(r.Identity)
	if rextras != nil {
		out.Metadata = wireOpenAIMetadata(rextras.OpenAIMetadata)
	}

	for _, t := range r.Tools {
		out.Tools = append(out.Tools, responses.ToolFromFunction(
			responses.NewFunctionTool(t.Name, ptrStringIfNonEmpty(t.Description), sortmap.NewSortedMap(t.Parameters), ptrBool(t.Strict)),
		))
	}

	if len(r.Tools) > 0 {
		switch tc := r.ToolChoice.(type) {
		case nil, unified.ToolChoiceAuto:
			v := responses.ToolChoiceAuto()
			out.ToolChoice = &v
		case unified.ToolChoiceRequired:
			v := responses.ToolChoiceRequired()
			out.ToolChoice = &v
		case unified.ToolChoiceNone:
			v := responses.ToolChoiceNone()
			out.ToolChoice = &v
		case unified.ToolChoiceTool:
			v := responses.ToolChoiceForFunction(tc.Name)
			out.ToolChoice = &v
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
	if instructions != "" {
		out.Instructions = ptrString(instructions)
	}

	var items []responses.InputItem
	for _, m := range remaining {
		switch m.Role {
		case unified.RoleSystem:
			return nil, fmt.Errorf("responses request cannot project additional system messages")
		case unified.RoleDeveloper:
			items = append(items, responses.InputItemFromMessage(
				responses.NewEasyInputMessage(string(unified.RoleDeveloper), responses.EasyInputContentText(partsText(m.Parts))),
			))
		case unified.RoleUser:
			items = append(items, responses.InputItemFromMessage(
				responses.NewEasyInputMessage(string(unified.RoleUser), responses.EasyInputContentText(partsText(m.Parts))),
			))
		case unified.RoleAssistant:
			assistantItems, err := buildResponsesAssistantItems(m)
			if err != nil {
				return nil, err
			}
			items = append(items, assistantItems...)
		case unified.RoleTool:
			for _, p := range m.Parts {
				if p.Type != unified.PartTypeToolResult || p.ToolResult == nil {
					continue
				}
				items = append(items, responses.InputItemFromFunctionOutput(
					responses.NewFunctionCallOutput(p.ToolResult.ToolCallID, p.ToolResult.ToolOutput),
				))
			}
		}
	}
	if len(items) > 0 {
		out.Input = responses.InputItems(items)
	}

	return out, nil
}

// RequestFromResponses converts a Responses wire request to unified.
func RequestFromResponses(r responses.Request) (unified.Request, error) {
	u := unified.Request{
		Model:    r.Model,
		Messages: make([]unified.Message, 0, 8),
	}
	if r.MaxOutputTokens != nil && *r.MaxOutputTokens > 0 {
		u.MaxTokens = *r.MaxOutputTokens
	}
	if r.Temperature != nil {
		u.Temperature = *r.Temperature
	}
	if r.TopP != nil {
		u.TopP = *r.TopP
	}
	if r.Text != nil && r.Text.Format != nil {
		switch r.Text.Format.Type() {
		case "json_object":
			u.Output = &unified.OutputSpec{Mode: unified.OutputModeJSONObject}
		case "text":
			u.Output = &unified.OutputSpec{Mode: unified.OutputModeText}
		case "json_schema":
			u.Output = &unified.OutputSpec{Mode: unified.OutputModeJSONSchema}
			// TODO: extract schema name and schema from the raw JSON if needed
		}
	}
	if r.Reasoning != nil {
		if r.Reasoning.Effort != nil {
			u.Effort = unified.Effort(*r.Reasoning.Effort)
		}
		if r.Reasoning.Summary != nil {
			ensureResponsesExtras(&u).ReasoningSummary = string(*r.Reasoning.Summary)
		}
	}
	if r.PromptCacheRetention != nil {
		ret := string(*r.PromptCacheRetention)
		if hint := cacheHintFromPromptCacheRetention(ret); hint != nil {
			u.CacheHint = hint
		}
		ensureResponsesExtras(&u).PromptCacheRetention = ret
	}
	// Identity and metadata.
	u.Identity = identityFromWire(r.User)
	if len(r.Metadata) > 0 {
		ensureResponsesExtras(&u).OpenAIMetadata = wireOpenAIMetadata(r.Metadata)
	}
	if r.PromptCacheKey != "" {
		ensureResponsesExtras(&u).PromptCacheKey = r.PromptCacheKey
	}
	if r.PreviousResponseID != nil && *r.PreviousResponseID != "" {
		ensureResponsesExtras(&u).PreviousResponseID = *r.PreviousResponseID
	}
	if r.Store != nil {
		ensureResponsesExtras(&u).Store = *r.Store
	}
	if r.ParallelToolCalls != nil {
		ensureResponsesExtras(&u).ParallelToolCalls = *r.ParallelToolCalls
	}
	if r.ServiceTier != nil {
		ensureResponsesExtras(&u).ServiceTier = string(*r.ServiceTier)
	}
	if r.Truncation != nil {
		ensureResponsesExtras(&u).Truncation = string(*r.Truncation)
	}
	if len(r.Include) > 0 {
		incs := make([]string, len(r.Include))
		for i, inc := range r.Include {
			incs[i] = string(inc)
		}
		ensureResponsesExtras(&u).Include = incs
	}
	if r.Background != nil {
		ensureResponsesExtras(&u).Background = r.Background
	}
	if r.MaxToolCalls != nil {
		ensureResponsesExtras(&u).MaxToolCalls = r.MaxToolCalls
	}
	if r.TopLogprobs != nil {
		ensureResponsesExtras(&u).TopLogprobs = r.TopLogprobs
	}

	// Decode tools from ToolParam wrappers.
	for _, tp := range r.Tools {
		if tp.Type() != "function" {
			// Non-function tools have no unified equivalent — skip silently.
			continue
		}
		var ft responses.FunctionTool
		if err := json.Unmarshal(tp.Raw(), &ft); err != nil {
			continue
		}
		strict := false
		if ft.Strict != nil {
			strict = *ft.Strict
		}
		desc := ""
		if ft.Description != nil {
			desc = *ft.Description
		}
		u.Tools = append(u.Tools, unified.Tool{
			Name:        ft.Name,
			Description: desc,
			Parameters:  toMap(ft.Parameters),
			Strict:      strict,
		})
	}
	u.ToolChoice = toolChoiceFromResponses(r.ToolChoice)

	if r.Instructions != nil && *r.Instructions != "" {
		useInstructions := true
		ensureResponsesExtras(&u).UseInstructions = &useInstructions
		u.Messages = append(u.Messages, unified.Message{Role: unified.RoleSystem, Parts: []unified.Part{{Type: unified.PartTypeText, Text: *r.Instructions}}})
	} else {
		useInstructions := false
		ensureResponsesExtras(&u).UseInstructions = &useInstructions
	}

	// Decode input items.
	var assistantTurn responsesAssistantTurn
	flushAssistantTurn := func() {
		if assistantTurn.empty() {
			return
		}
		u.Messages = append(u.Messages, unified.Message{Role: unified.RoleAssistant, Phase: assistantTurn.phase, Parts: assistantTurn.parts})
		assistantTurn.reset()
	}

	if !r.Input.IsText() {
		for _, item := range r.Input.Items() {
			raw := item.Raw()
			var probe struct {
				Type      string `json:"type"`
				Role      string `json:"role"`
				Content   string `json:"content"`
				Phase     string `json:"phase"`
				CallID    string `json:"call_id"`
				Name      string `json:"name"`
				Arguments string `json:"arguments"`
				Output    string `json:"output"`
			}
			if err := json.Unmarshal(raw, &probe); err != nil {
				continue
			}

			switch {
			case probe.Role == string(unified.RoleDeveloper):
				flushAssistantTurn()
				u.Messages = append(u.Messages, unified.Message{Role: unified.RoleDeveloper, Parts: []unified.Part{{Type: unified.PartTypeText, Text: probe.Content}}})
			case probe.Role == string(unified.RoleUser):
				flushAssistantTurn()
				u.Messages = append(u.Messages, unified.Message{Role: unified.RoleUser, Parts: []unified.Part{{Type: unified.PartTypeText, Text: probe.Content}}})
			case probe.Role == string(unified.RoleAssistant):
				phase, err := responsesAssistantPhase(probe.Phase)
				if err != nil {
					return unified.Request{}, err
				}
				if assistantTurn.shouldFlush(phase) {
					flushAssistantTurn()
				}
				assistantTurn.adoptPhase(phase)
				if probe.Content != "" {
					assistantTurn.parts = append(assistantTurn.parts, unified.Part{Type: unified.PartTypeText, Text: probe.Content})
				}
			case probe.Type == "function_call":
				phase, err := responsesAssistantPhase(probe.Phase)
				if err != nil {
					return unified.Request{}, err
				}
				if assistantTurn.shouldFlush(phase) {
					flushAssistantTurn()
				}
				assistantTurn.adoptPhase(phase)
				var args map[string]any
				if probe.Arguments != "" {
					_ = json.Unmarshal([]byte(probe.Arguments), &args)
				}
				assistantTurn.parts = append(assistantTurn.parts, unified.Part{Type: unified.PartTypeToolCall, ToolCall: &unified.ToolCall{ID: probe.CallID, Name: probe.Name, Args: args}})
			case probe.Type == "function_call_output":
				flushAssistantTurn()
				u.Messages = append(u.Messages, unified.Message{Role: unified.RoleTool, Parts: []unified.Part{{Type: unified.PartTypeToolResult, ToolResult: &unified.ToolResult{ToolCallID: probe.CallID, ToolOutput: probe.Output}}}})
			}
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

func buildResponsesAssistantItems(m unified.Message) ([]responses.InputItem, error) {
	items := make([]responses.InputItem, 0, len(m.Parts)+1)
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
			items = append(items, responses.InputItemFromFunctionCall(
				responses.NewFunctionCallInput(p.ToolCall.ID, p.ToolCall.Name, string(argRaw)),
			))
		case p.Type == unified.PartTypeThinking:
			// Thinking parts are produced by reasoning models but are not
			// echoed back as input in the Responses API. Reasoning is
			// controlled by the request-level `reasoning` config; the model
			// generates fresh reasoning each turn. Simply skip them.
			continue
		case p.Type == unified.PartTypeToolResult:
			return nil, fmt.Errorf("responses assistant message does not support tool result parts")
		default:
			return nil, fmt.Errorf("responses assistant message does not support part type %q", p.Type)
		}
	}

	if text.Len() > 0 {
		if phase != "" {
			msg := responses.NewEasyInputMessageWithPhase(string(unified.RoleAssistant), responses.EasyInputContentText(text.String()), phase)
			items = append([]responses.InputItem{responses.InputItemFromMessage(msg)}, items...)
		} else {
			msg := responses.NewEasyInputMessage(string(unified.RoleAssistant), responses.EasyInputContentText(text.String()))
			items = append([]responses.InputItem{responses.InputItemFromMessage(msg)}, items...)
		}
	}
	return items, nil
}

func toolChoiceFromResponses(tc *responses.ToolChoiceParam) unified.ToolChoice {
	if tc == nil {
		return nil
	}
	if s, ok := tc.AsString(); ok {
		switch s {
		case "auto":
			return unified.ToolChoiceAuto{}
		case "required":
			return unified.ToolChoiceRequired{}
		case "none":
			return unified.ToolChoiceNone{}
		}
	}
	if m, ok := tc.AsObject(); ok {
		if typ, _ := m["type"].(string); typ == "function" {
			if name, _ := m["name"].(string); name != "" {
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

