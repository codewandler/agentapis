package adapt

import (
	"testing"

	"github.com/codewandler/agentapis/api/unified"
)

func TestBuildMessagesRequest_AdaptiveThinkingCoercesTemperature(t *testing.T) {
	base := unified.Request{
		Model:    "claude-sonnet-4-6",
		Messages: []unified.Message{{Role: unified.RoleUser, Parts: []unified.Part{{Type: unified.PartTypeText, Text: "hi"}}}},
	}

	tests := []struct {
		name    string
		temp    float64
		wantTmp float64
	}{
		{"zero stays zero (omitted)", 0, 0},
		{"1.0 stays 1.0", 1.0, 1.0},
		{"0.5 coerced to 1.0", 0.5, 1.0},
		{"0.7 coerced to 1.0", 0.7, 1.0},
		{"1.5 coerced to 1.0", 1.5, 1.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := base
			r.Temperature = tt.temp
			got, err := BuildMessagesRequest(r)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Thinking == nil || got.Thinking.Type != "adaptive" {
				t.Fatalf("expected adaptive thinking, got %+v", got.Thinking)
			}
			if got.Temperature != tt.wantTmp {
				t.Errorf("temperature = %v, want %v", got.Temperature, tt.wantTmp)
			}
		})
	}
}

func TestBuildMessagesRequest_NonAdaptiveThinkingKeepsTemperature(t *testing.T) {
	r := unified.Request{
		Model:       "claude-haiku-4-5-20251001",
		Temperature: 0.5,
		Messages:    []unified.Message{{Role: unified.RoleUser, Parts: []unified.Part{{Type: unified.PartTypeText, Text: "hi"}}}},
	}
	got, err := BuildMessagesRequest(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Thinking == nil || got.Thinking.Type != "enabled" {
		t.Fatalf("expected enabled thinking, got %+v", got.Thinking)
	}
	if got.Temperature != 0.5 {
		t.Errorf("temperature = %v, want 0.5 (non-adaptive should not coerce)", got.Temperature)
	}
}
