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

// wireUser returns the user identifier for the wire `user` field.
func wireUser(id *unified.RequestIdentity) string {
	if id == nil {
		return ""
	}
	return id.User
}

// wireOpenAIMetadata clones explicitly declared OpenAI metadata for the wire.
// Never contains internal adapter state.
func wireOpenAIMetadata(m map[string]string) map[string]string {
	if len(m) == 0 {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// identityFromWire reconstructs RequestIdentity from the wire user field.
func identityFromWire(user string) *unified.RequestIdentity {
	if user == "" {
		return nil
	}
	return &unified.RequestIdentity{User: user}
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

func mustJSON(v any) string {
	if v == nil {
		return ""
	}
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(b)
}

func ensureOllamaExtras(r *unified.Request) *unified.OllamaExtras {
	if r.Extras.Ollama == nil {
		r.Extras.Ollama = &unified.OllamaExtras{}
	}
	return r.Extras.Ollama
}

func ptrBool(b bool) *bool       { return &b }
func ptrInt(i int) *int          { return &i }
func ptrFloat64(f float64) *float64 { return &f }
func ptrString(s string) *string { return &s }

func ptrStringIfNonEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// wireOpenAIMetadataAny converts map[string]string metadata to map[string]any for
// wire protocols that use map[string]any (e.g. completions).
func wireOpenAIMetadataAny(m map[string]string) map[string]any {
	if len(m) == 0 {
		return nil
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// openAIMetadataFromAny converts wire map[string]any metadata back to map[string]string.
func openAIMetadataFromAny(m map[string]any) map[string]string {
	if len(m) == 0 {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		if s, ok := v.(string); ok {
			out[k] = s
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
