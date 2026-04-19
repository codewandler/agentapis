package adapt

import (
	"testing"

	"github.com/codewandler/agentapis/api/ollama"
	"github.com/codewandler/agentapis/api/unified"
)

func TestBuildOllamaRequestMapsCoreFields(t *testing.T) {
	t.Parallel()
	req := unified.Request{
		Model:       "qwen3",
		MaxTokens:   123,
		Temperature: 0.7,
		TopP:        0.8,
		TopK:        20,
		Thinking:    unified.ThinkingModeOn,
		Output:      &unified.OutputSpec{Mode: unified.OutputModeJSONSchema, Schema: map[string]any{"type": "object"}},
		Tools:       []unified.Tool{{Name: "get_weather", Description: "Get weather", Parameters: map[string]any{"type": "object"}}},
		Messages: []unified.Message{
			{Role: unified.RoleDeveloper, Parts: []unified.Part{{Type: unified.PartTypeText, Text: "dev"}}},
			{Role: unified.RoleUser, Parts: []unified.Part{{Type: unified.PartTypeText, Text: "hi"}}},
			{Role: unified.RoleAssistant, Parts: []unified.Part{{Type: unified.PartTypeThinking, Thinking: &unified.ThinkingPart{Text: "think"}}, {Type: unified.PartTypeText, Text: "answer"}, {Type: unified.PartTypeToolCall, ToolCall: &unified.ToolCall{Name: "get_weather", Args: map[string]any{"city": "Berlin"}}}}},
			{Role: unified.RoleTool, Parts: []unified.Part{{Type: unified.PartTypeToolResult, ToolResult: &unified.ToolResult{ToolCallID: "get_weather", ToolOutput: "18C"}}}},
		},
	}
	wire, err := BuildOllamaRequest(req)
	if err != nil { t.Fatalf("BuildOllamaRequest() error = %v", err) }
	if wire.Model != "qwen3" || wire.Think != true { t.Fatalf("unexpected model/think: %#v", wire) }
	if got := wire.Options["num_predict"]; got != 123 { t.Fatalf("num_predict = %#v", got) }
	if got := wire.Options["temperature"]; got != 0.7 { t.Fatalf("temperature = %#v", got) }
	if got := wire.Options["top_p"]; got != 0.8 { t.Fatalf("top_p = %#v", got) }
	if got := wire.Options["top_k"]; got != 20 { t.Fatalf("top_k = %#v", got) }
	if len(wire.Messages) != 4 { t.Fatalf("messages len = %d", len(wire.Messages)) }
	if wire.Messages[0].Role != "system" { t.Fatalf("developer should map to system, got %q", wire.Messages[0].Role) }
	if wire.Messages[2].Thinking != "think" || wire.Messages[2].Content != "answer" { t.Fatalf("assistant mapping mismatch: %#v", wire.Messages[2]) }
	if len(wire.Messages[2].ToolCalls) != 1 || wire.Messages[2].ToolCalls[0].Function.Name != "get_weather" { t.Fatalf("tool call mapping mismatch: %#v", wire.Messages[2].ToolCalls) }
	if wire.Messages[3].Role != "tool" || wire.Messages[3].ToolName != "get_weather" { t.Fatalf("tool result mapping mismatch: %#v", wire.Messages[3]) }
}

func TestBuildOllamaRequestRejectsExplicitToolChoice(t *testing.T) {
	t.Parallel()
	_, err := BuildOllamaRequest(unified.Request{Model: "qwen3", Messages: []unified.Message{{Role: unified.RoleUser, Parts: []unified.Part{{Type: unified.PartTypeText, Text: "hi"}}}}, ToolChoice: unified.ToolChoiceRequired{}})
	if err == nil { t.Fatalf("expected error") }
}

func TestRequestFromOllamaRoundTripsMainFields(t *testing.T) {
	t.Parallel()
	wire, err := BuildOllamaRequest(unified.Request{Model: "qwen3", MaxTokens: 9, Temperature: 0.2, TopP: 0.9, TopK: 7, Thinking: unified.ThinkingModeOff, Output: &unified.OutputSpec{Mode: unified.OutputModeJSONObject}, Messages: []unified.Message{{Role: unified.RoleUser, Parts: []unified.Part{{Type: unified.PartTypeText, Text: "hi"}}}}})
	if err != nil { t.Fatalf("BuildOllamaRequest() error = %v", err) }
	round, err := RequestFromOllama(*wire)
	if err != nil { t.Fatalf("RequestFromOllama() error = %v", err) }
	if round.MaxTokens != 9 || round.TopK != 7 || round.Thinking != unified.ThinkingModeOff { t.Fatalf("roundtrip mismatch: %#v", round) }
	if round.Output == nil || round.Output.Mode != unified.OutputModeJSONObject { t.Fatalf("output mismatch: %#v", round.Output) }
}

func TestBuildOllamaRequestResolvesToolResultNameFromPriorToolCallID(t *testing.T) {
	t.Parallel()
	wire, err := BuildOllamaRequest(unified.Request{
		Model: "qwen3",
		Messages: []unified.Message{
			{Role: unified.RoleUser, Parts: []unified.Part{{Type: unified.PartTypeText, Text: "hi"}}},
			{Role: unified.RoleAssistant, Parts: []unified.Part{{Type: unified.PartTypeToolCall, ToolCall: &unified.ToolCall{ID: "call_1", Name: "get_weather", Args: map[string]any{"city": "Berlin"}}}}},
			{Role: unified.RoleTool, Parts: []unified.Part{{Type: unified.PartTypeToolResult, ToolResult: &unified.ToolResult{ToolCallID: "call_1", ToolOutput: "18C"}}}},
		},
	})
	if err != nil { t.Fatalf("BuildOllamaRequest() error = %v", err) }
	if got := wire.Messages[len(wire.Messages)-1].ToolName; got != "get_weather" {
		t.Fatalf("expected resolved tool name get_weather, got %q", got)
	}
}

func TestBuildOllamaRequestSupportsOllamaExtras(t *testing.T) {
	t.Parallel()
	wire, err := BuildOllamaRequest(unified.Request{
		Model:    "gpt-oss",
		Messages: []unified.Message{{Role: unified.RoleUser, Parts: []unified.Part{{Type: unified.PartTypeText, Text: "hi"}}}},
		Extras: unified.RequestExtras{Ollama: &unified.OllamaExtras{ThinkLevel: "high", KeepAlive: "10m", Options: map[string]any{"seed": 42}, LogProbs: true, TopLogProbs: 3}},
	})
	if err != nil { t.Fatalf("BuildOllamaRequest() error = %v", err) }
	if wire.Think != "high" || wire.KeepAlive != "10m" || !wire.LogProbs || wire.TopLogProbs != 3 { t.Fatalf("extras not mapped: %#v", wire) }
	if got := wire.Options["seed"]; got != 42 { t.Fatalf("expected seed option 42, got %#v", got) }
}

func TestRequestFromOllamaPreservesOllamaExtras(t *testing.T) {
	t.Parallel()
	round, err := RequestFromOllama(ollama.Request{
		Model:       "gpt-oss",
		Messages:    []ollama.Message{{Role: "user", Content: "hi"}},
		Think:       "high",
		KeepAlive:   "10m",
		Options:     map[string]any{"seed": 42, "num_predict": 9},
		LogProbs:    true,
		TopLogProbs: 3,
	})
	if err != nil { t.Fatalf("RequestFromOllama() error = %v", err) }
	if round.Extras.Ollama == nil { t.Fatalf("expected ollama extras") }
	if round.Extras.Ollama.ThinkLevel != "high" || round.Extras.Ollama.KeepAlive != "10m" || !round.Extras.Ollama.LogProbs || round.Extras.Ollama.TopLogProbs != 3 {
		t.Fatalf("unexpected ollama extras: %#v", round.Extras.Ollama)
	}
	if got := round.Extras.Ollama.Options["seed"]; got != 42 { t.Fatalf("expected seed=42, got %#v", got) }
	if round.MaxTokens != 9 { t.Fatalf("expected MaxTokens from num_predict, got %d", round.MaxTokens) }
}
