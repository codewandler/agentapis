package adapt

import (
	"encoding/json"
	"testing"

	"github.com/codewandler/agentapis/api/unified"
)

// inputItemProbe is a test helper for inspecting decoded InputItem contents.
type inputItemProbe struct {
	Type      string `json:"type"`
	Role      string `json:"role"`
	Content   string `json:"content"`
	CallID    string `json:"call_id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
	Output    string `json:"output"`
}

func decodeInputItems(t *testing.T, req interface{}) []inputItemProbe {
	t.Helper()
	raw, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	var wire struct {
		Input json.RawMessage `json:"input"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		t.Fatalf("unmarshal wire: %v", err)
	}
	var items []inputItemProbe
	if err := json.Unmarshal(wire.Input, &items); err != nil {
		t.Fatalf("unmarshal input items: %v (raw: %s)", err, string(wire.Input))
	}
	return items
}

func TestBuildResponsesRequest_ThinkingPartsStripped(t *testing.T) {
	t.Parallel()

	req := unified.Request{
		Model: "codex-mini",
		Messages: []unified.Message{
			{Role: unified.RoleSystem, Parts: []unified.Part{{Type: unified.PartTypeText, Text: "You are helpful."}}},
			{Role: unified.RoleUser, Parts: []unified.Part{{Type: unified.PartTypeText, Text: "Hello"}}},
			{
				Role: unified.RoleAssistant,
				Parts: []unified.Part{
					{Type: unified.PartTypeThinking, Thinking: &unified.ThinkingPart{Text: "Let me think...", Signature: "sig123"}},
					{Type: unified.PartTypeText, Text: "Hi there!"},
				},
			},
			{Role: unified.RoleUser, Parts: []unified.Part{{Type: unified.PartTypeText, Text: "How are you?"}}},
		},
	}

	out, err := BuildResponsesRequest(req)
	if err != nil {
		t.Fatalf("BuildResponsesRequest returned error: %v", err)
	}

	items := decodeInputItems(t, out)

	// Should have 3 inputs: first user, assistant text, second user
	// (system goes to instructions, thinking is stripped)
	if len(items) != 3 {
		t.Fatalf("expected 3 inputs, got %d: %+v", len(items), items)
	}

	// Verify assistant input has text but no thinking
	assistant := items[1]
	if assistant.Role != "assistant" {
		t.Errorf("expected assistant role, got %q", assistant.Role)
	}
	if assistant.Content != "Hi there!" {
		t.Errorf("expected assistant content 'Hi there!', got %q", assistant.Content)
	}
}

func TestBuildResponsesRequest_ThinkingOnlyAssistant(t *testing.T) {
	t.Parallel()

	// Edge case: assistant message with ONLY thinking parts (no text)
	req := unified.Request{
		Model: "codex-mini",
		Messages: []unified.Message{
			{Role: unified.RoleUser, Parts: []unified.Part{{Type: unified.PartTypeText, Text: "Hello"}}},
			{
				Role: unified.RoleAssistant,
				Parts: []unified.Part{
					{Type: unified.PartTypeThinking, Thinking: &unified.ThinkingPart{Text: "thinking only"}},
				},
			},
			{Role: unified.RoleUser, Parts: []unified.Part{{Type: unified.PartTypeText, Text: "Next"}}},
		},
	}

	out, err := BuildResponsesRequest(req)
	if err != nil {
		t.Fatalf("BuildResponsesRequest returned error: %v", err)
	}

	items := decodeInputItems(t, out)

	// With thinking stripped and no text, the assistant produces 0 inputs.
	// We should have: user + user = 2 inputs
	if len(items) != 2 {
		t.Fatalf("expected 2 inputs, got %d: %+v", len(items), items)
	}
}

func TestBuildResponsesRequest_AssistantMixedContentTextBeforeToolCallsProjects(t *testing.T) {
	t.Parallel()

	req := unified.Request{
		Model: "codex-mini",
		Messages: []unified.Message{
			{Role: unified.RoleUser, Parts: []unified.Part{{Type: unified.PartTypeText, Text: "Start"}}},
			{
				Role: unified.RoleAssistant,
				Parts: []unified.Part{
					{Type: unified.PartTypeThinking, Thinking: &unified.ThinkingPart{Text: "internal"}},
					{Type: unified.PartTypeText, Text: "I will call a tool."},
					{Type: unified.PartTypeToolCall, ToolCall: &unified.ToolCall{ID: "call_1", Name: "get_weather", Args: map[string]any{"city": "Berlin"}}},
				},
			},
		},
	}

	out, err := BuildResponsesRequest(req)
	if err != nil {
		t.Fatalf("BuildResponsesRequest returned error: %v", err)
	}

	items := decodeInputItems(t, out)
	if len(items) != 3 {
		t.Fatalf("expected 3 inputs, got %d: %+v", len(items), items)
	}
	if items[1].Role != "assistant" || items[1].Content != "I will call a tool." {
		t.Fatalf("expected assistant text input first, got %+v", items[1])
	}
	if items[2].Type != "function_call" || items[2].CallID != "call_1" || items[2].Name != "get_weather" {
		t.Fatalf("expected assistant tool call input after text, got %+v", items[2])
	}
}

func TestBuildResponsesRequest_AssistantToolCallThenTextFails(t *testing.T) {
	t.Parallel()

	req := unified.Request{
		Model: "codex-mini",
		Messages: []unified.Message{
			{Role: unified.RoleUser, Parts: []unified.Part{{Type: unified.PartTypeText, Text: "Start"}}},
			{
				Role: unified.RoleAssistant,
				Parts: []unified.Part{
					{Type: unified.PartTypeToolCall, ToolCall: &unified.ToolCall{ID: "call_1", Name: "get_weather", Args: map[string]any{"city": "Berlin"}}},
					{Type: unified.PartTypeText, Text: "The weather is sunny."},
				},
			},
		},
	}

	_, err := BuildResponsesRequest(req)
	if err == nil {
		t.Fatal("expected BuildResponsesRequest to reject assistant text after tool call for Responses API")
	}
	if got := err.Error(); got != "responses assistant message cannot contain text after tool calls" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRoundTrip_AllBridgedFields(t *testing.T) {
	t.Parallel()

	req := unified.Request{
		Model: "gpt-4o",
		Messages: []unified.Message{
			{Role: unified.RoleSystem, Parts: []unified.Part{{Type: unified.PartTypeText, Text: "You are a helpful assistant."}}},
			{Role: unified.RoleUser, Parts: []unified.Part{{Type: unified.PartTypeText, Text: "Hello"}}},
		},
		MaxTokens:   1024,
		Temperature: 0.7,
		TopP:        0.9,
		Effort:      unified.EffortHigh,
		Tools: []unified.Tool{{
			Name:        "get_weather",
			Description: "Get the current weather",
			Parameters:  map[string]any{"type": "object", "properties": map[string]any{"city": map[string]any{"type": "string"}}},
			Strict:      true,
		}},
		ToolChoice: unified.ToolChoiceAuto{},
		Identity:   &unified.RequestIdentity{User: "test-user"},
		Extras: unified.RequestExtras{
			Responses: &unified.ResponsesExtras{
				PromptCacheRetention: "24h",
				PromptCacheKey:       "session-123",
				PreviousResponseID:   "resp_abc",
				ReasoningSummary:     "concise",
				Store:                true,
				ParallelToolCalls:    true,
				ServiceTier:          "auto",
				Truncation:           "auto",
				Include:              []string{"file_search_call.results"},
				Background:           ptrBool(false),
				MaxToolCalls:         ptrInt(10),
				TopLogprobs:          ptrInt(5),
				ConversationID:       "conv-456",
			},
		},
	}

	out, err := BuildResponsesRequest(req)
	if err != nil {
		t.Fatalf("BuildResponsesRequest: %v", err)
	}

	// Verify key wire fields.
	if out.Model != "gpt-4o" {
		t.Errorf("model: got %q, want gpt-4o", out.Model)
	}
	if out.MaxOutputTokens == nil || *out.MaxOutputTokens != 1024 {
		t.Errorf("max_output_tokens: got %v, want 1024", out.MaxOutputTokens)
	}
	if out.Temperature == nil || *out.Temperature != 0.7 {
		t.Errorf("temperature: got %v, want 0.7", out.Temperature)
	}
	if out.TopP == nil || *out.TopP != 0.9 {
		t.Errorf("top_p: got %v, want 0.9", out.TopP)
	}
	if out.User != "test-user" {
		t.Errorf("user: got %q, want test-user", out.User)
	}
	if out.PromptCacheKey != "session-123" {
		t.Errorf("prompt_cache_key: got %q, want session-123", out.PromptCacheKey)
	}
	if out.PreviousResponseID == nil || *out.PreviousResponseID != "resp_abc" {
		t.Errorf("previous_response_id: got %v, want resp_abc", out.PreviousResponseID)
	}
	if out.Store == nil || !*out.Store {
		t.Errorf("store: got %v, want true", out.Store)
	}
	if out.ParallelToolCalls == nil || !*out.ParallelToolCalls {
		t.Errorf("parallel_tool_calls: got %v, want true", out.ParallelToolCalls)
	}
	if out.ServiceTier == nil || string(*out.ServiceTier) != "auto" {
		t.Errorf("service_tier: got %v, want auto", out.ServiceTier)
	}
	if out.Truncation == nil || string(*out.Truncation) != "auto" {
		t.Errorf("truncation: got %v, want auto", out.Truncation)
	}
	if len(out.Include) != 1 || string(out.Include[0]) != "file_search_call.results" {
		t.Errorf("include: got %v, want [file_search_call.results]", out.Include)
	}
	if out.Background == nil || *out.Background {
		t.Errorf("background: got %v, want false", out.Background)
	}
	if out.MaxToolCalls == nil || *out.MaxToolCalls != 10 {
		t.Errorf("max_tool_calls: got %v, want 10", out.MaxToolCalls)
	}
	if out.TopLogprobs == nil || *out.TopLogprobs != 5 {
		t.Errorf("top_logprobs: got %v, want 5", out.TopLogprobs)
	}
	if out.Conversation == nil {
		t.Error("conversation: got nil, want ConversationByID(conv-456)")
	}

	// Reasoning.
	if out.Reasoning == nil {
		t.Fatal("reasoning: got nil")
	}
	if out.Reasoning.Effort == nil || string(*out.Reasoning.Effort) != "high" {
		t.Errorf("reasoning.effort: got %v, want high", out.Reasoning.Effort)
	}
	if out.Reasoning.Summary == nil || string(*out.Reasoning.Summary) != "concise" {
		t.Errorf("reasoning.summary: got %v, want concise", out.Reasoning.Summary)
	}

	// Instructions from system message.
	if out.Instructions == nil || *out.Instructions != "You are a helpful assistant." {
		t.Errorf("instructions: got %v, want 'You are a helpful assistant.'", out.Instructions)
	}

	// Tools — verify via JSON.
	if len(out.Tools) != 1 {
		t.Fatalf("tools: got %d, want 1", len(out.Tools))
	}
	if out.Tools[0].Type() != "function" {
		t.Errorf("tool type: got %q, want function", out.Tools[0].Type())
	}

	// ToolChoice.
	if out.ToolChoice == nil {
		t.Fatal("tool_choice: got nil")
	}
	if s, ok := out.ToolChoice.AsString(); !ok || s != "auto" {
		t.Errorf("tool_choice: got %v, want auto", out.ToolChoice)
	}

	// Round-trip back.
	rt, err := RequestFromResponses(*out)
	if err != nil {
		t.Fatalf("RequestFromResponses: %v", err)
	}
	if rt.Model != req.Model {
		t.Errorf("round-trip model: got %q, want %q", rt.Model, req.Model)
	}
	if rt.MaxTokens != req.MaxTokens {
		t.Errorf("round-trip max_tokens: got %d, want %d", rt.MaxTokens, req.MaxTokens)
	}
	if rt.Temperature != req.Temperature {
		t.Errorf("round-trip temperature: got %f, want %f", rt.Temperature, req.Temperature)
	}
	if rt.Effort != req.Effort {
		t.Errorf("round-trip effort: got %q, want %q", rt.Effort, req.Effort)
	}
	if len(rt.Tools) != len(req.Tools) {
		t.Errorf("round-trip tools: got %d, want %d", len(rt.Tools), len(req.Tools))
	}
	if rt.Extras.Responses == nil {
		t.Fatal("round-trip: ResponsesExtras is nil")
	}
	rte := rt.Extras.Responses
	if rte.PreviousResponseID != req.Extras.Responses.PreviousResponseID {
		t.Errorf("round-trip previous_response_id: got %q, want %q", rte.PreviousResponseID, req.Extras.Responses.PreviousResponseID)
	}
	if rte.PromptCacheKey != req.Extras.Responses.PromptCacheKey {
		t.Errorf("round-trip prompt_cache_key: got %q, want %q", rte.PromptCacheKey, req.Extras.Responses.PromptCacheKey)
	}
	if rte.ServiceTier != req.Extras.Responses.ServiceTier {
		t.Errorf("round-trip service_tier: got %q, want %q", rte.ServiceTier, req.Extras.Responses.ServiceTier)
	}
	if rte.Truncation != req.Extras.Responses.Truncation {
		t.Errorf("round-trip truncation: got %q, want %q", rte.Truncation, req.Extras.Responses.Truncation)
	}
	if len(rte.Include) != len(req.Extras.Responses.Include) {
		t.Errorf("round-trip include: got %v, want %v", rte.Include, req.Extras.Responses.Include)
	}
	if rte.MaxToolCalls == nil || *rte.MaxToolCalls != 10 {
		t.Errorf("round-trip max_tool_calls: got %v, want 10", rte.MaxToolCalls)
	}
	if rte.TopLogprobs == nil || *rte.TopLogprobs != 5 {
		t.Errorf("round-trip top_logprobs: got %v, want 5", rte.TopLogprobs)
	}
}

func TestBuildResponsesRequest_ToolChoiceVariants(t *testing.T) {
	t.Parallel()

	base := unified.Request{
		Model: "gpt-4o",
		Messages: []unified.Message{
			{Role: unified.RoleUser, Parts: []unified.Part{{Type: unified.PartTypeText, Text: "hi"}}},
		},
		Tools: []unified.Tool{{
			Name: "fn", Description: "d", Parameters: map[string]any{"type": "object"},
		}},
	}

	tests := []struct {
		name   string
		choice unified.ToolChoice
		want   string
	}{
		{"auto", unified.ToolChoiceAuto{}, `"auto"`},
		{"required", unified.ToolChoiceRequired{}, `"required"`},
		{"none", unified.ToolChoiceNone{}, `"none"`},
		{"function", unified.ToolChoiceTool{Name: "fn"}, `{"name":"fn","type":"function"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := base
			req.ToolChoice = tt.choice
			out, err := BuildResponsesRequest(req)
			if err != nil {
				t.Fatalf("BuildResponsesRequest: %v", err)
			}
			if out.ToolChoice == nil {
				t.Fatal("tool_choice is nil")
			}
			got, _ := out.ToolChoice.MarshalJSON()
			if string(got) != tt.want {
				t.Errorf("tool_choice: got %s, want %s", got, tt.want)
			}
		})
	}
}

func TestBuildResponsesRequest_TextFormat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		output   *unified.OutputSpec
		wantType string
	}{
		{"json_object", &unified.OutputSpec{Mode: unified.OutputModeJSONObject}, "json_object"},
		{"json_schema", &unified.OutputSpec{Mode: unified.OutputModeJSONSchema, Schema: map[string]any{"type": "object"}}, "json_schema"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := unified.Request{
				Model: "gpt-4o",
				Messages: []unified.Message{
					{Role: unified.RoleUser, Parts: []unified.Part{{Type: unified.PartTypeText, Text: "hi"}}},
				},
				Output: tt.output,
			}
			out, err := BuildResponsesRequest(req)
			if err != nil {
				t.Fatalf("BuildResponsesRequest: %v", err)
			}
			if out.Text == nil || out.Text.Format == nil {
				t.Fatal("text.format is nil")
			}
			if out.Text.Format.Type() != tt.wantType {
				t.Errorf("text.format.type: got %q, want %q", out.Text.Format.Type(), tt.wantType)
			}
		})
	}
}
