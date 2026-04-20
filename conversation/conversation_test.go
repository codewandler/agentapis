package conversation

import (
	"testing"

	"github.com/codewandler/agentapis/api/unified"
)

func TestNewRequestBuilderBuildsRequest(t *testing.T) {
	req := NewRequest().
		Model("gpt-4o-mini").
		MaxTokens(256).
		Temperature(0.2).
		Effort(unified.EffortHigh).
		Thinking(unified.ThinkingModeOn).
		CachePolicy(CachePolicyProgressive).
		Instructions("Answer tersely.", "Prefer bullets.").
		ToolChoice(unified.ToolChoiceAuto{}).
		CacheHint(&unified.CacheHint{Enabled: true, TTL: "1h"}).
		User("hello").
		ToolResult("call_1", `{"ok":true}`).
		Build()
	if req.Model != "gpt-4o-mini" || req.MaxTokens != 256 || req.Temperature != 0.2 || req.Effort != unified.EffortHigh || req.Thinking != unified.ThinkingModeOn || req.CachePolicy != CachePolicyProgressive || len(req.Instructions) != 2 || len(req.Inputs) != 2 || req.CacheHint == nil || !req.CacheHint.Enabled {
		t.Fatalf("unexpected request: %#v", req)
	}
	if _, ok := req.ToolChoice.(unified.ToolChoiceAuto); !ok {
		t.Fatalf("unexpected tool choice: %#v", req.ToolChoice)
	}
}
