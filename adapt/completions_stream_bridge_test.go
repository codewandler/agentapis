package adapt

import (
	"testing"

	"github.com/codewandler/agentapis/api/completions"
	"github.com/codewandler/agentapis/api/unified"
)

func TestMapCompletionsEventCompletedCarriesResponseID(t *testing.T) {
	t.Parallel()
	finish := completions.FinishReasonStop
	ev, ignored, err := MapCompletionsEvent(&completions.Chunk{
		ID:    "chatcmpl_123",
		Model: "gpt-4o-mini",
		Choices: []completions.Choice{{
			FinishReason: &finish,
		}},
	})
	if err != nil || ignored {
		t.Fatalf("unexpected result: ev=%#v ignored=%v err=%v", ev, ignored, err)
	}
	if ev.Type != unified.StreamEventCompleted || ev.Completed == nil {
		t.Fatalf("expected completed event, got %#v", ev)
	}
	if ev.Lifecycle == nil || ev.Lifecycle.Scope != unified.LifecycleScopeResponse || ev.Lifecycle.Ref.ResponseID != "chatcmpl_123" {
		t.Fatalf("expected response lifecycle ref, got %#v", ev.Lifecycle)
	}
}
