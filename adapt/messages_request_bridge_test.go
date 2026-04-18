package adapt

import (
	"testing"

	"github.com/codewandler/agentapis/api/unified"
)

func TestBuildMessagesRequest_ThinkingCoercesTemperature(t *testing.T) {
	models := []struct {
		model        string
		thinkingType string
	}{
		{"claude-sonnet-4-6", "adaptive"},
		{"claude-haiku-4-5-20251001", "enabled"},
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

	for _, m := range models {
		for _, tt := range tests {
			t.Run(m.model+"/"+tt.name, func(t *testing.T) {
				r := unified.Request{
					Model:       m.model,
					Temperature: tt.temp,
					Messages:    []unified.Message{{Role: unified.RoleUser, Parts: []unified.Part{{Type: unified.PartTypeText, Text: "hi"}}}},
				}
				got, err := BuildMessagesRequest(r)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if got.Thinking == nil || got.Thinking.Type != m.thinkingType {
					t.Fatalf("expected %s thinking, got %+v", m.thinkingType, got.Thinking)
				}
				if got.Temperature != tt.wantTmp {
					t.Errorf("temperature = %v, want %v", got.Temperature, tt.wantTmp)
				}
			})
		}
	}
}
