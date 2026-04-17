package adapt

import (
	"encoding/json"
	"strings"

	"github.com/codewandler/agentapis/api/unified"
)

func partsText(parts []unified.Part) string {
	var b strings.Builder
	for _, p := range parts {
		if p.Type == unified.PartTypeText {
			b.WriteString(p.Text)
		}
	}
	return b.String()
}

func toMap(v any) map[string]any {
	if v == nil {
		return nil
	}
	if m, ok := v.(map[string]any); ok {
		return m
	}
	raw, _ := json.Marshal(v)
	var out map[string]any
	_ = json.Unmarshal(raw, &out)
	return out
}

func contentString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func cloneAnyMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func promptCacheRetentionFromHint(h *unified.CacheHint) string {
	if h != nil && h.Enabled && h.TTL == "1h" {
		return "24h"
	}
	return ""
}

func cacheHintFromPromptCacheRetention(ret string) *unified.CacheHint {
	if ret == "24h" {
		return &unified.CacheHint{Enabled: true, TTL: "1h"}
	}
	return nil
}

func metadataToOpenAI(meta *unified.RequestMetadata, extra map[string]any) (string, map[string]any) {
	user := ""
	out := cloneAnyMap(extra)
	if meta != nil {
		user = meta.User
		if len(meta.Metadata) > 0 {
			if out == nil {
				out = map[string]any{}
			}
			for k, v := range meta.Metadata {
				out[k] = v
			}
		}
	}
	if len(out) == 0 {
		out = nil
	}
	return user, out
}

func metadataFromOpenAI(user string, raw map[string]any) (*unified.RequestMetadata, map[string]any) {
	if user == "" && len(raw) == 0 {
		return nil, nil
	}
	return &unified.RequestMetadata{User: user, Metadata: cloneAnyMap(raw)}, nil
}

func ensureMessagesExtras(r *unified.Request) *unified.MessagesExtras {
	if r.Extras.Messages == nil {
		r.Extras.Messages = &unified.MessagesExtras{}
	}
	return r.Extras.Messages
}

func messagesCachePartIndex(r unified.Request, messageIndex int) *int {
	if r.Extras.Messages == nil || r.Extras.Messages.MessageCachePartIndex == nil {
		return nil
	}
	idx, ok := r.Extras.Messages.MessageCachePartIndex[messageIndex]
	if !ok {
		return nil
	}
	return &idx
}

func ensureCompletionsExtras(r *unified.Request) *unified.CompletionsExtras {
	if r.Extras.Completions == nil {
		r.Extras.Completions = &unified.CompletionsExtras{}
	}
	return r.Extras.Completions
}

func ensureResponsesExtras(r *unified.Request) *unified.ResponsesExtras {
	if r.Extras.Responses == nil {
		r.Extras.Responses = &unified.ResponsesExtras{}
	}
	return r.Extras.Responses
}

func hasPerMessageCacheHints(msgs []unified.Message) bool {
	for _, m := range msgs {
		if m.CacheHint != nil && m.CacheHint.Enabled {
			return true
		}
	}
	return false
}

func isTextOnlyMessage(m unified.Message) bool {
	if len(m.Parts) == 0 {
		return false
	}
	for _, p := range m.Parts {
		if p.Type != unified.PartTypeText {
			return false
		}
	}
	return true
}

func partHasCanonicalFields(p unified.Part) bool {
	return p.Type != "" || p.Text != "" || p.Thinking != nil || p.ToolCall != nil || p.ToolResult != nil
}

func partContributesCanonicalContent(p unified.Part) bool {
	if p.Native != nil {
		return false
	}
	if p.Thinking != nil || p.ToolCall != nil || p.ToolResult != nil {
		return true
	}
	return p.Type == unified.PartTypeText && p.Text != ""
}

func ensureMessagesCachePartIndexMap(in map[int]int) map[int]int {
	if in != nil {
		return in
	}
	return map[int]int{}
}
