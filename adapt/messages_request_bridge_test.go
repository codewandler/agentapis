package adapt

import (
	"testing"

	"github.com/codewandler/agentapis/api/unified"
)

func TestBuildMessagesRequest_DoesNotCoerceTemperatureWhenThinkingEnabled(t *testing.T) {
	models := []struct {
		model        string
		thinkingType string
	}{
		{"claude-sonnet-4-6", "adaptive"},
		{"claude-haiku-4-5-20251001", "enabled"},
	}
	for _, m := range models {
		t.Run(m.model, func(t *testing.T) {
			r := unified.Request{
				Model:       m.model,
				Temperature: 0.5,
				Messages:    []unified.Message{{Role: unified.RoleUser, Parts: []unified.Part{{Type: unified.PartTypeText, Text: "hi"}}}},
			}
			got, err := BuildMessagesRequest(r)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Thinking == nil || got.Thinking.Type != m.thinkingType {
				t.Fatalf("expected %s thinking, got %+v", m.thinkingType, got.Thinking)
			}
			if got.Temperature != 0.5 {
				t.Errorf("temperature = %v, want %v", got.Temperature, 0.5)
			}
		})
	}
}

func TestBuildMessagesRequest_ThinkingBudgetRespectsMaxTokens(t *testing.T) {
	tests := []struct {
		name      string
		effort    unified.Effort
		maxTokens int
		wantMax   int
	}{
		{"low small", unified.EffortLow, 256, 128},
		{"medium small", unified.EffortMedium, 512, 256},
		{"high small", unified.EffortHigh, 512, 256},
		{"max medium", unified.EffortMax, 2048, 1024},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := unified.Request{
				Model:     "claude-haiku-4-5-20251001",
				Effort:    tt.effort,
				MaxTokens: tt.maxTokens,
				Messages:  []unified.Message{{Role: unified.RoleUser, Parts: []unified.Part{{Type: unified.PartTypeText, Text: "hi"}}}},
			}
			got, err := BuildMessagesRequest(r)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Thinking == nil {
				t.Fatalf("expected thinking config")
			}
			if got.Thinking.BudgetTokens <= 0 {
				t.Fatalf("budget_tokens = %d, want > 0", got.Thinking.BudgetTokens)
			}
			if got.Thinking.BudgetTokens > tt.wantMax {
				t.Fatalf("budget_tokens = %d, want <= %d", got.Thinking.BudgetTokens, tt.wantMax)
			}
			if got.Thinking.BudgetTokens >= tt.maxTokens {
				t.Fatalf("budget_tokens = %d, want < max_tokens %d", got.Thinking.BudgetTokens, tt.maxTokens)
			}
		})
	}
}
