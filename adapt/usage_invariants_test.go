package adapt

import (
	"testing"

	"github.com/codewandler/agentapis/api/completions"
	"github.com/codewandler/agentapis/api/messages"
	"github.com/codewandler/agentapis/api/responses"
	"github.com/codewandler/agentapis/api/unified"
)

func TestMessagesUsageRespectsInputInvariant(t *testing.T) {
	event, ignored, err := MapMessagesEvent(&messages.MessageStartEvent{Message: messages.MessageStartPayload{
		ID:    "msg_1",
		Model: "claude",
		Usage: messages.MessageUsage{InputTokens: 10, CacheCreationInputTokens: 2, CacheReadInputTokens: 1},
	}})
	if err != nil || ignored {
		t.Fatalf("unexpected result: ev=%#v ignored=%v err=%v", event, ignored, err)
	}
	if event.Usage == nil {
		t.Fatal("expected usage")
	}
	if got, want := event.Usage.Input.New, 10; got != want {
		t.Fatalf("new = %d, want %d", got, want)
	}
	if got, want := event.Usage.Input.CacheWrite, 2; got != want {
		t.Fatalf("cache_write = %d, want %d", got, want)
	}
	if got, want := event.Usage.Input.CacheRead, 1; got != want {
		t.Fatalf("cache_read = %d, want %d", got, want)
	}
	if got, want := event.Usage.Input.Total, 13; got != want {
		t.Fatalf("total = %d, want %d", got, want)
	}
	if got := event.Usage.Input.CacheRead + event.Usage.Input.CacheWrite + event.Usage.Input.New; got != event.Usage.Input.Total {
		t.Fatalf("input invariant broken: total=%d sum=%d", event.Usage.Input.Total, got)
	}
}

func TestResponsesUsageDerivesNewFromTotalAndCacheRead(t *testing.T) {
	u := &responses.ResponseUsage{
		InputTokens:  100,
		OutputTokens: 40,
		InputTokensDetails: &struct {
			CachedTokens int `json:"cached_tokens"`
		}{CachedTokens: 25},
		OutputTokensDetails: &struct {
			ReasoningTokens int `json:"reasoning_tokens"`
		}{ReasoningTokens: 7},
	}
	usage := usageFromResponses(u)
	if usage == nil {
		t.Fatal("expected usage")
	}
	if got, want := usage.Input.New, 75; got != want {
		t.Fatalf("new = %d, want %d", got, want)
	}
	if got, want := usage.Input.CacheRead, 25; got != want {
		t.Fatalf("cache_read = %d, want %d", got, want)
	}
	if got, want := usage.Input.CacheWrite, 0; got != want {
		t.Fatalf("cache_write = %d, want %d", got, want)
	}
	if got, want := usage.Input.Total, 100; got != want {
		t.Fatalf("total = %d, want %d", got, want)
	}
	if got := usage.Input.CacheRead + usage.Input.CacheWrite + usage.Input.New; got != usage.Input.Total {
		t.Fatalf("input invariant broken: total=%d sum=%d", usage.Input.Total, got)
	}
	if got, want := usage.Output.Total, 40; got != want {
		t.Fatalf("output total = %d, want %d", got, want)
	}
	if got, want := usage.Output.Reasoning, 7; got != want {
		t.Fatalf("output reasoning = %d, want %d", got, want)
	}
}

func TestCompletionsUsageHandlesPromptCacheAndReasoningDetails(t *testing.T) {
	event, ignored, err := MapCompletionsEvent(&completions.Chunk{
		ID:    "chatcmpl-1",
		Model: "gpt",
		Usage: &completions.Usage{
			PromptTokens:     120,
			CompletionTokens: 30,
			PromptTokensDetails: &completions.TokDetails{
				CachedTokens: 20,
			},
			CompletionTokensDetails: &completions.TokDetails{
				ReasoningTokens: 9,
			},
		},
	})
	if err != nil || ignored {
		t.Fatalf("unexpected result: ev=%#v ignored=%v err=%v", event, ignored, err)
	}
	if event.Usage == nil {
		t.Fatal("expected usage")
	}
	if got, want := event.Usage.Input.New, 100; got != want {
		t.Fatalf("new = %d, want %d", got, want)
	}
	if got, want := event.Usage.Input.CacheRead, 20; got != want {
		t.Fatalf("cache_read = %d, want %d", got, want)
	}
	if got, want := event.Usage.Input.Total, 120; got != want {
		t.Fatalf("input total = %d, want %d", got, want)
	}
	if got, want := event.Usage.Output.Total, 30; got != want {
		t.Fatalf("output total = %d, want %d", got, want)
	}
	if got, want := event.Usage.Output.Reasoning, 9; got != want {
		t.Fatalf("output reasoning = %d, want %d", got, want)
	}
}

func TestNewUsageEventDerivesStructuredUsageFromTokens(t *testing.T) {
	ev := unified.NewUsageEvent(unified.TokenItems{
		{Kind: unified.TokenKindInputNew, Count: 11},
		{Kind: unified.TokenKindInputCacheRead, Count: 5},
		{Kind: unified.TokenKindInputCacheWrite, Count: 2},
		{Kind: unified.TokenKindOutput, Count: 17},
		{Kind: unified.TokenKindOutputReasoning, Count: 4},
	}, nil)
	if ev.Usage == nil {
		t.Fatal("expected usage")
	}
	if got, want := ev.Usage.Input.Total, 18; got != want {
		t.Fatalf("input total = %d, want %d", got, want)
	}
	if got, want := ev.Usage.Input.New, 11; got != want {
		t.Fatalf("input new = %d, want %d", got, want)
	}
	if got, want := ev.Usage.Output.Total, 17; got != want {
		t.Fatalf("output total = %d, want %d", got, want)
	}
	if got, want := ev.Usage.Output.Reasoning, 4; got != want {
		t.Fatalf("output reasoning = %d, want %d", got, want)
	}
}

func TestResponsesUsageClampsNewWhenCacheReadExceedsProviderInput(t *testing.T) {
	u := &responses.ResponseUsage{
		InputTokens:  10,
		OutputTokens: 4,
		InputTokensDetails: &struct {
			CachedTokens int `json:"cached_tokens"`
		}{CachedTokens: 20},
	}
	usage := usageFromResponses(u)
	if usage == nil {
		t.Fatal("expected usage")
	}
	if got, want := usage.Input.New, 0; got != want {
		t.Fatalf("new = %d, want %d", got, want)
	}
	if got, want := usage.Input.CacheRead, 20; got != want {
		t.Fatalf("cache_read = %d, want %d", got, want)
	}
	if got, want := usage.Input.Total, 20; got != want {
		t.Fatalf("total = %d, want %d", got, want)
	}
}

func TestCompletionsUsageClampsNewWhenCacheReadExceedsProviderInput(t *testing.T) {
	event, ignored, err := MapCompletionsEvent(&completions.Chunk{
		Usage: &completions.Usage{
			PromptTokens: 10,
			PromptTokensDetails: &completions.TokDetails{
				CachedTokens: 20,
			},
			CompletionTokens: 3,
		},
	})
	if err != nil || ignored {
		t.Fatalf("unexpected result: ev=%#v ignored=%v err=%v", event, ignored, err)
	}
	if event.Usage == nil {
		t.Fatal("expected usage")
	}
	if got, want := event.Usage.Input.New, 0; got != want {
		t.Fatalf("new = %d, want %d", got, want)
	}
	if got, want := event.Usage.Input.CacheRead, 20; got != want {
		t.Fatalf("cache_read = %d, want %d", got, want)
	}
	if got, want := event.Usage.Input.Total, 20; got != want {
		t.Fatalf("total = %d, want %d", got, want)
	}
}
