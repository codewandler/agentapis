package adapt

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/codewandler/agentapis/api/messages"
	"github.com/codewandler/agentapis/api/unified"
	"github.com/codewandler/agentapis/internal/sortmap"
)

type MessagesOption func(*messagesOptions)

type messagesOptions struct {
	modelCaps ModelCapsFunc
}

func WithMessagesModelCaps(fn ModelCapsFunc) MessagesOption {
	return func(o *messagesOptions) { o.modelCaps = fn }
}

func BuildMessagesRequest(r unified.Request, opts ...MessagesOption) (*messages.Request, error) {
	mopts := &messagesOptions{modelCaps: DefaultAnthropicMessagesModelCaps}
	for _, o := range opts {
		o(mopts)
	}
	if err := Validate(r); err != nil {
		return nil, fmt.Errorf("validate unified request: %w", err)
	}

	maxTokens := r.MaxTokens
	if maxTokens == 0 {
		maxTokens = 32000
	}

	out := &messages.Request{
		Model:     r.Model,
		MaxTokens: maxTokens,
		Stream:    true,
		Messages:  make([]messages.Message, 0, len(r.Messages)),
	}
	if r.Temperature > 0 {
		out.Temperature = r.Temperature
	}
	if r.TopK > 0 {
		out.TopK = r.TopK
	}
	if r.TopP > 0 {
		out.TopP = r.TopP
	}

	mextras := r.Extras.Messages
	if mextras != nil && len(mextras.StopSequences) > 0 {
		out.StopSequences = append([]string(nil), mextras.StopSequences...)
	}
	if err := applyMessagesOutput(out, r.Output); err != nil {
		return nil, err
	}
	if err := applyMessagesMetadata(out, r.Metadata, mextras); err != nil {
		return nil, err
	}

	for _, t := range r.Tools {
		out.Tools = append(out.Tools, messages.ToolDefinition{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: sortmap.NewSortedMap(t.Parameters),
		})
	}

	if len(r.Tools) > 0 {
		switch tc := r.ToolChoice.(type) {
		case nil, unified.ToolChoiceAuto:
			out.ToolChoice = map[string]string{"type": "auto"}
		case unified.ToolChoiceRequired:
			out.ToolChoice = map[string]string{"type": "any"}
		case unified.ToolChoiceNone:
		case unified.ToolChoiceTool:
			out.ToolChoice = map[string]any{"type": "tool", "name": tc.Name}
		default:
			return nil, fmt.Errorf("unsupported tool choice type %T", r.ToolChoice)
		}
	}

	caps := mopts.modelCaps(r.Model)
	out.Thinking = messagesThinkingFromRequest(r, mextras, caps)
	if out.Thinking != nil && out.Thinking.Type != "disabled" {
		switch r.ToolChoice.(type) {
		case unified.ToolChoiceRequired, unified.ToolChoiceTool:
			out.ToolChoice = map[string]string{"type": "auto"}
		}
	}
	if caps.SupportsEffort {
		if out.OutputConfig == nil {
			out.OutputConfig = &messages.OutputConfig{}
		}
		e := r.Effort
		if e == unified.EffortUnspecified {
			e = unified.EffortMedium
		}
		if e == unified.EffortMax && !caps.SupportsMaxEffort {
			e = unified.EffortHigh
		}
		out.OutputConfig.Effort = string(e)
	}
	// Anthropic requires temperature=1 when adaptive thinking is enabled.
	// Coerce any explicitly set non-1 temperature to 1; leave zero (omitted) alone.
	if out.Thinking != nil && out.Thinking.Type == "adaptive" && out.Temperature != 0 && out.Temperature != 1 {
		out.Temperature = 1
	}

	for _, m := range r.Messages {
		msgIndex := len(out.System) + len(out.Messages)
		switch m.Role {
		case unified.RoleSystem, unified.RoleDeveloper:
			text := strings.TrimSpace(partsText(m.Parts))
			if text != "" {
				out.System = append(out.System, &messages.TextBlock{
					Type:         messages.BlockTypeText,
					Text:         text,
					CacheControl: cacheHintToMessages(m.CacheHint),
				})
			}
			continue
		}

		wire, err := messageToMessages(m, messagesCachePartIndex(r, msgIndex))
		if err != nil {
			return nil, err
		}
		out.Messages = append(out.Messages, *wire)
	}

	if out.Messages == nil {
		out.Messages = make([]messages.Message, 0)
	}

	if !hasPerMessageCacheHints(r.Messages) {
		if cc := cacheHintToMessages(r.CacheHint); cc != nil {
			out.CacheControl = cc
		} else if mextras != nil && mextras.RequestCacheControl != nil {
			out.CacheControl = cloneCacheControl(mextras.RequestCacheControl)
		}
	}

	return out, nil
}

func RequestFromMessages(r messages.Request) (unified.Request, error) {
	u := unified.Request{
		Model:       r.Model,
		MaxTokens:   r.MaxTokens,
		Temperature: r.Temperature,
		TopK:        r.TopK,
		TopP:        r.TopP,
		Messages:    make([]unified.Message, 0, len(r.Messages)+len(r.System)),
	}
	if r.OutputConfig != nil {
		if r.OutputConfig.Format != nil {
			switch {
			case r.OutputConfig.Format.Type == "json_schema" && r.OutputConfig.Format.Schema != nil:
				u.Output = &unified.OutputSpec{Mode: unified.OutputModeJSONSchema, Schema: r.OutputConfig.Format.Schema}
			case r.OutputConfig.Format.Type == "json_schema":
				u.Output = &unified.OutputSpec{Mode: unified.OutputModeJSONObject}
			}
		}
		if r.OutputConfig.Effort != "" {
			u.Effort = unified.Effort(r.OutputConfig.Effort)
		}
	}
	if r.Metadata != nil && r.Metadata.UserID != "" {
		u.Metadata = &unified.RequestMetadata{User: r.Metadata.UserID}
	}
	if r.CacheControl != nil {
		u.CacheHint = cacheHintFromMessages(r.CacheControl)
		ensureMessagesExtras(&u).RequestCacheControl = cloneMessagesRequestCacheControl(r.CacheControl)
	}
	if len(r.StopSequences) > 0 {
		ensureMessagesExtras(&u).StopSequences = append([]string(nil), r.StopSequences...)
	}
	if r.Thinking != nil {
		if r.Thinking.Type == "disabled" {
			u.Thinking = unified.ThinkingModeOff
		} else {
			u.Thinking = unified.ThinkingModeOn
		}
		mextras := ensureMessagesExtras(&u)
		mextras.ThinkingType = r.Thinking.Type
		mextras.ThinkingBudgetTokens = r.Thinking.BudgetTokens
		mextras.ThinkingDisplay = r.Thinking.Display
	}
	for _, t := range r.Tools {
		u.Tools = append(u.Tools, unified.Tool{Name: t.Name, Description: t.Description, Parameters: toMap(t.InputSchema)})
	}
	u.ToolChoice = toolChoiceFromMessages(r.ToolChoice)
	for _, s := range r.System {
		if s == nil {
			continue
		}
		u.Messages = append(u.Messages, unified.Message{Role: unified.RoleSystem, Parts: []unified.Part{{Type: unified.PartTypeText, Text: s.Text}}, CacheHint: cacheHintFromMessages(s.CacheControl)})
	}
	for _, m := range r.Messages {
		canonicalIndex := len(u.Messages)
		x, cachePartIndex, err := messageFromMessages(m)
		if err != nil {
			return unified.Request{}, err
		}
		u.Messages = append(u.Messages, x)
		if cachePartIndex != nil {
			ensureMessagesExtras(&u).MessageCachePartIndex = ensureMessagesCachePartIndexMap(ensureMessagesExtras(&u).MessageCachePartIndex)
			u.Extras.Messages.MessageCachePartIndex[canonicalIndex] = *cachePartIndex
		}
	}
	if err := Validate(u); err != nil {
		return unified.Request{}, err
	}
	return u, nil
}

func messageToMessages(m unified.Message, cachePartIndex *int) (*messages.Message, error) {
	wire := &messages.Message{}
	switch m.Role {
	case unified.RoleUser:
		wire.Role = "user"
	case unified.RoleAssistant:
		wire.Role = "assistant"
	case unified.RoleTool:
		wire.Role = "user"
	default:
		wire.Role = string(m.Role)
	}

	blocks := make([]any, 0, len(m.Parts))
	for _, p := range m.Parts {
		switch p.Type {
		case unified.PartTypeText:
			blocks = append(blocks, &messages.TextBlock{Type: messages.BlockTypeText, Text: p.Text})
		case unified.PartTypeThinking:
			if p.Thinking != nil {
				blocks = append(blocks, &messages.ThinkingBlock{Type: messages.BlockTypeThinking, Thinking: p.Thinking.Text, Signature: p.Thinking.Signature})
			}
		case unified.PartTypeToolCall:
			if p.ToolCall != nil {
				argRaw, _ := json.Marshal(p.ToolCall.Args)
				blocks = append(blocks, &messages.ToolUseBlock{Type: messages.BlockTypeToolUse, ID: p.ToolCall.ID, Name: p.ToolCall.Name, Input: argRaw})
			}
		case unified.PartTypeToolResult:
			if p.ToolResult != nil {
				blocks = append(blocks, &messages.ToolResultBlock{Type: "tool_result", ToolUseID: p.ToolResult.ToolCallID, Content: p.ToolResult.ToolOutput, IsError: p.ToolResult.IsError})
			}
		}
	}
	if len(blocks) == 0 {
		blocks = append(blocks, &messages.TextBlock{Type: messages.BlockTypeText, Text: ""})
	}
	wire.Content = blocks

	if h := cacheHintToMessages(m.CacheHint); h != nil {
		if attachIndex := resolveMessagesCacheBlockIndex(blocks, cachePartIndex); attachIndex >= 0 {
			switch tb := blocks[attachIndex].(type) {
			case *messages.TextBlock:
				tb.CacheControl = h
			case *messages.ToolUseBlock:
				tb.CacheControl = h
			case *messages.ToolResultBlock:
				tb.CacheControl = h
			case *messages.ThinkingBlock:
				tb.CacheControl = h
			}
		}
	}

	return wire, nil
}

func messageFromMessages(m messages.Message) (unified.Message, *int, error) {
	um := unified.Message{Parts: make([]unified.Part, 0)}
	switch m.Role {
	case "user":
		um.Role = unified.RoleUser
	case "assistant":
		um.Role = unified.RoleAssistant
	default:
		um.Role = unified.Role(m.Role)
	}
	var cachePartIndex *int

	switch c := m.Content.(type) {
	case string:
		if c != "" {
			um.Parts = append(um.Parts, unified.Part{Type: unified.PartTypeText, Text: c})
		}
	case []any:
		for i, item := range c {
			part, hint, err := partFromMessagesRaw(item)
			if err != nil {
				return unified.Message{}, nil, err
			}
			if part != nil {
				um.Parts = append(um.Parts, *part)
			}
			if hint != nil {
				um.CacheHint = hint
				idx := i
				cachePartIndex = &idx
			}
		}
	}

	if len(um.Parts) == 0 {
		um.Parts = []unified.Part{{Type: unified.PartTypeText, Text: ""}}
	}
	return um, cachePartIndex, nil
}

func partFromMessagesRaw(v any) (*unified.Part, *unified.CacheHint, error) {
	m, ok := v.(map[string]any)
	if !ok {
		return nil, nil, nil
	}
	typ, _ := m["type"].(string)
	hint := cacheHintFromRaw(m)
	switch typ {
	case messages.BlockTypeText:
		text, _ := m["text"].(string)
		return &unified.Part{Type: unified.PartTypeText, Text: text}, hint, nil
	case messages.BlockTypeThinking:
		thinking, _ := m["thinking"].(string)
		sig, _ := m["signature"].(string)
		return &unified.Part{Type: unified.PartTypeThinking, Thinking: &unified.ThinkingPart{Text: thinking, Signature: sig}}, hint, nil
	case messages.BlockTypeToolUse:
		id, _ := m["id"].(string)
		name, _ := m["name"].(string)
		args, _ := m["input"].(map[string]any)
		return &unified.Part{Type: unified.PartTypeToolCall, ToolCall: &unified.ToolCall{ID: id, Name: name, Args: args}}, hint, nil
	case "tool_result":
		toolID, _ := m["tool_use_id"].(string)
		content, _ := m["content"].(string)
		isErr, _ := m["is_error"].(bool)
		return &unified.Part{Type: unified.PartTypeToolResult, ToolResult: &unified.ToolResult{ToolCallID: toolID, ToolOutput: content, IsError: isErr}}, hint, nil
	default:
		raw, _ := json.Marshal(m)
		return &unified.Part{Native: raw}, hint, nil
	}
}

func applyMessagesOutput(out *messages.Request, spec *unified.OutputSpec) error {
	if spec == nil {
		return nil
	}
	switch spec.Mode {
	case unified.OutputModeText:
		return nil
	case unified.OutputModeJSONObject:
		if out.OutputConfig == nil {
			out.OutputConfig = &messages.OutputConfig{}
		}
		out.OutputConfig.Format = &messages.JSONOutputFormat{Type: "json_schema"}
		return nil
	case unified.OutputModeJSONSchema:
		if out.OutputConfig == nil {
			out.OutputConfig = &messages.OutputConfig{}
		}
		out.OutputConfig.Format = &messages.JSONOutputFormat{Type: "json_schema", Schema: spec.Schema}
		return nil
	default:
		return fmt.Errorf("unsupported output mode %q", spec.Mode)
	}
}

func applyMessagesMetadata(out *messages.Request, meta *unified.RequestMetadata, _ *unified.MessagesExtras) error {
	if meta == nil {
		return nil
	}
	if meta.User != "" {
		out.Metadata = &messages.Metadata{UserID: meta.User}
	}
	return nil
}

func messagesThinkingFromRequest(r unified.Request, extras *unified.MessagesExtras, caps ModelCaps) *messages.ThinkingConfig {
	var thinking *messages.ThinkingConfig
	if extras != nil && extras.ThinkingType != "" {
		thinking = &messages.ThinkingConfig{
			Type:         extras.ThinkingType,
			BudgetTokens: extras.ThinkingBudgetTokens,
			Display:      extras.ThinkingDisplay,
		}
		return thinking
	}

	if r.Thinking == unified.ThinkingModeOff {
		thinking = &messages.ThinkingConfig{Type: "disabled"}
	} else if caps.SupportsAdaptiveThinking {
		thinking = &messages.ThinkingConfig{Type: "adaptive", Display: caps.DefaultThinkingDisplay}
	} else {
		thinking = &messages.ThinkingConfig{Type: "enabled", BudgetTokens: effortToBudget(r.Effort), Display: caps.DefaultThinkingDisplay}
	}
	if extras != nil && extras.ThinkingDisplay != "" {
		thinking.Display = extras.ThinkingDisplay
	}
	return thinking
}

func effortToBudget(e unified.Effort) int {
	switch e {
	case unified.EffortLow:
		return 1024
	case unified.EffortMedium, unified.EffortUnspecified:
		return 31999
	case unified.EffortHigh:
		return 31999
	case unified.EffortMax:
		return 31999
	default:
		return 31999
	}
}

func toolChoiceFromMessages(v any) unified.ToolChoice {
	typ, name, ok := messagesToolChoiceFields(v)
	if !ok {
		return nil
	}
	switch typ {
	case "auto":
		return unified.ToolChoiceAuto{}
	case "any":
		return unified.ToolChoiceRequired{}
	case "tool":
		if name != "" {
			return unified.ToolChoiceTool{Name: name}
		}
	}
	return nil
}

func messagesToolChoiceFields(v any) (typ, name string, ok bool) {
	switch t := v.(type) {
	case map[string]any:
		typ, _ = t["type"].(string)
		name, _ = t["name"].(string)
		return typ, name, typ != ""
	case map[string]string:
		return t["type"], t["name"], t["type"] != ""
	default:
		return "", "", false
	}
}

func cacheHintToMessages(h *unified.CacheHint) *messages.CacheControl {
	if h == nil || !h.Enabled {
		return nil
	}
	cc := &messages.CacheControl{Type: "ephemeral"}
	if h.TTL == "1h" {
		cc.TTL = h.TTL
	}
	return cc
}

func cacheHintFromMessages(cc *messages.CacheControl) *unified.CacheHint {
	if cc == nil {
		return nil
	}
	ttl := cc.TTL
	if ttl == "" {
		ttl = "5m"
	}
	return &unified.CacheHint{Enabled: true, TTL: ttl}
}

func cacheHintFromRaw(m map[string]any) *unified.CacheHint {
	raw, ok := m["cache_control"].(map[string]any)
	if !ok {
		return nil
	}
	ttl, _ := raw["ttl"].(string)
	if ttl == "" {
		ttl = "5m"
	}
	return &unified.CacheHint{Enabled: true, TTL: ttl}
}

func cloneCacheControl(cc *unified.CacheControl) *messages.CacheControl {
	if cc == nil {
		return nil
	}
	return &messages.CacheControl{Type: cc.Type, TTL: cc.TTL}
}

func cloneMessagesRequestCacheControl(cc *messages.CacheControl) *unified.CacheControl {
	if cc == nil {
		return nil
	}
	return &unified.CacheControl{Type: cc.Type, TTL: cc.TTL}
}

func resolveMessagesCacheBlockIndex(blocks []any, preferred *int) int {
	if len(blocks) == 0 {
		return -1
	}
	if preferred != nil && *preferred >= 0 && *preferred < len(blocks) {
		return *preferred
	}
	return len(blocks) - 1
}
