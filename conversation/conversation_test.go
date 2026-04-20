package conversation

import (
	"testing"

	"github.com/codewandler/agentapis/api/unified"
)

func TestNewRequestBuilderBuildsRequest(t *testing.T) {
	req := NewRequest().
		Model("gpt-4o-mini").
		Instructions("Answer tersely.", "Prefer bullets.").
		ToolChoice(unified.ToolChoiceAuto{}).
		User("hello").
		ToolResult("call_1", `{"ok":true}`).
		Build()
	if req.Model != "gpt-4o-mini" || len(req.Instructions) != 2 || len(req.Inputs) != 2 {
		t.Fatalf("unexpected request: %#v", req)
	}
	if _, ok := req.ToolChoice.(unified.ToolChoiceAuto); !ok {
		t.Fatalf("unexpected tool choice: %#v", req.ToolChoice)
	}
}
