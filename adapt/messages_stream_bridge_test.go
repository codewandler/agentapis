package adapt

import (
	"testing"

	"github.com/codewandler/agentapis/api/messages"
	"github.com/codewandler/agentapis/api/unified"
)

func TestMapMessagesEventContentBlockStopIsSegmentDone(t *testing.T) {
	event, ignored, err := MapMessagesEvent(&messages.ContentBlockStopEvent{Index: 2})
	if err != nil {
		t.Fatalf("MapMessagesEvent() error = %v", err)
	}
	if ignored {
		t.Fatalf("expected event not to be ignored")
	}
	if event.Type != unified.StreamEventLifecycle {
		t.Fatalf("expected lifecycle event, got %q", event.Type)
	}
	if event.Lifecycle == nil {
		t.Fatalf("expected lifecycle payload")
	}
	if event.Lifecycle.Scope != unified.LifecycleScopeSegment || event.Lifecycle.State != unified.LifecycleStateDone {
		t.Fatalf("unexpected lifecycle payload: %#v", event.Lifecycle)
	}
	if event.Lifecycle.Ref.ItemIndex == nil || *event.Lifecycle.Ref.ItemIndex != 2 {
		t.Fatalf("unexpected item index: %#v", event.Lifecycle.Ref)
	}
}

func TestMapMessagesEventMessageStartIncludesUsage(t *testing.T) {
	event, ignored, err := MapMessagesEvent(&messages.MessageStartEvent{Message: messages.MessageStartPayload{
		ID:    "msg_1",
		Model: "claude",
		Usage: messages.MessageUsage{InputTokens: 10, CacheCreationInputTokens: 2, CacheReadInputTokens: 1},
	}})
	if err != nil {
		t.Fatalf("MapMessagesEvent() error = %v", err)
	}
	if ignored {
		t.Fatalf("expected event not to be ignored")
	}
	if event.Started == nil || event.Started.RequestID != "msg_1" {
		t.Fatalf("unexpected started payload: %#v", event.Started)
	}
	if event.Usage == nil {
		t.Fatalf("expected usage payload")
	}
	if got := event.Usage.Tokens.Count(unified.TokenKindInput); got != 10 {
		t.Fatalf("expected 10 input tokens, got %d", got)
	}
	if got := event.Usage.Tokens.Count(unified.TokenKindCacheWrite); got != 2 {
		t.Fatalf("expected 2 cache write tokens, got %d", got)
	}
	if got := event.Usage.Tokens.Count(unified.TokenKindCacheRead); got != 1 {
		t.Fatalf("expected 1 cache read token, got %d", got)
	}
}
