package adapt

import (
	"testing"

	"github.com/codewandler/agentapis/api/ollama"
	"github.com/codewandler/agentapis/api/unified"
)

func TestOllamaMapperMapsThinkingTextToolAndDone(t *testing.T) {
	t.Parallel()
	m := NewOllamaMapper()

	ev, ignored, err := m.MapEvent(&ollama.Response{Model: "qwen3", Message: ollama.Message{Thinking: "hmm"}})
	if err != nil || ignored || ev.Type != unified.StreamEventContentDelta || ev.Delta == nil || ev.Delta.Kind != unified.DeltaKindThinking {
		t.Fatalf("thinking map mismatch: ev=%#v ignored=%v err=%v", ev, ignored, err)
	}

	ev, ignored, err = m.MapEvent(&ollama.Response{Model: "qwen3", Message: ollama.Message{Content: "hello"}})
	if err != nil || ignored || ev.Type != unified.StreamEventContentDelta || ev.Delta == nil || ev.Delta.Kind != unified.DeltaKindText {
		t.Fatalf("text map mismatch: ev=%#v ignored=%v err=%v", ev, ignored, err)
	}

	ev, ignored, err = m.MapEvent(&ollama.Response{Model: "qwen3", Message: ollama.Message{ToolCalls: []ollama.ToolCall{{Function: ollama.ToolCallFunction{Name: "weather", Arguments: map[string]any{"city": "Berlin"}}}}}})
	if err != nil || ignored || ev.Type != unified.StreamEventToolCall || ev.ToolCall == nil || ev.ToolCall.Name != "weather" {
		t.Fatalf("tool map mismatch: ev=%#v ignored=%v err=%v", ev, ignored, err)
	}

	ev, ignored, err = m.MapEvent(&ollama.Response{Model: "qwen3", Done: true, DoneReason: "stop", PromptEvalCount: 2, EvalCount: 3})
	if err != nil || ignored || ev.Type != unified.StreamEventCompleted || ev.Completed == nil || ev.Completed.StopReason != unified.StopReasonEndTurn || ev.Usage == nil || ev.Usage.Tokens.Total() != 5 {
		t.Fatalf("done map mismatch: ev=%#v ignored=%v err=%v", ev, ignored, err)
	}
	if ev.Lifecycle == nil || ev.Lifecycle.Scope != unified.LifecycleScopeResponse || ev.Lifecycle.Ref.ResponseID == "" {
		t.Fatalf("expected completed event lifecycle response id, got %#v", ev.Lifecycle)
	}
}

func TestOllamaMapperCanEmitStartedForEmptyFirstChunk(t *testing.T) {
	t.Parallel()
	m := NewOllamaMapper()
	ev, ignored, err := m.MapEvent(&ollama.Response{Model: "qwen3", Done: false})
	if err != nil || ignored {
		t.Fatalf("unexpected err/ignored: %v %v", err, ignored)
	}
	if ev.Type != unified.StreamEventStarted || ev.Started == nil || ev.Started.Model != "qwen3" || ev.Started.RequestID == "" {
		t.Fatalf("started mismatch: %#v", ev)
	}
}

func TestOllamaMapperKeepsStableResponseID(t *testing.T) {
	t.Parallel()
	m := NewOllamaMapper()
	first, ignored, err := m.MapEvent(&ollama.Response{Model: "qwen3", CreatedAt: "2025-01-01T00:00:00Z", Message: ollama.Message{Content: "a"}})
	if err != nil || ignored { t.Fatalf("unexpected first result: ev=%#v ignored=%v err=%v", first, ignored, err) }
	second, ignored, err := m.MapEvent(&ollama.Response{Model: "qwen3", CreatedAt: "2025-01-01T00:00:01Z", Message: ollama.Message{Thinking: "b"}})
	if err != nil || ignored { t.Fatalf("unexpected second result: ev=%#v ignored=%v err=%v", second, ignored, err) }
	if first.ContentDelta == nil || second.ContentDelta == nil { t.Fatalf("expected content deltas") }
	if first.ContentDelta.Ref.ResponseID == "" || first.ContentDelta.Ref.ResponseID != second.ContentDelta.Ref.ResponseID {
		t.Fatalf("expected stable response id, got first=%q second=%q", first.ContentDelta.Ref.ResponseID, second.ContentDelta.Ref.ResponseID)
	}
}
