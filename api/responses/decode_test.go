package responses

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestDecodeRequest_Valid(t *testing.T) {
	t.Parallel()

	input := `{
		"model": "gpt-4o",
		"input": [
			{"type": "message", "role": "user", "content": "Hello!"}
		],
		"temperature": 0.5,
		"max_output_tokens": 1024,
		"top_p": 0.9,
		"store": true,
		"stream": true
	}`

	req, err := DecodeRequest([]byte(input))
	if err != nil {
		t.Fatalf("DecodeRequest returned error: %v", err)
	}
	if req.Model != "gpt-4o" {
		t.Errorf("model: got %q, want gpt-4o", req.Model)
	}
	if req.Temperature == nil || *req.Temperature != 0.5 {
		t.Errorf("temperature: got %v, want 0.5", req.Temperature)
	}
	if req.MaxOutputTokens == nil || *req.MaxOutputTokens != 1024 {
		t.Errorf("max_output_tokens: got %v, want 1024", req.MaxOutputTokens)
	}
	if req.TopP == nil || *req.TopP != 0.9 {
		t.Errorf("top_p: got %v, want 0.9", req.TopP)
	}
	if req.Store == nil || !*req.Store {
		t.Errorf("store: got %v, want true", req.Store)
	}
	if req.Stream == nil || !*req.Stream {
		t.Errorf("stream: got %v, want true", req.Stream)
	}
}

func TestDecodeRequest_StringInput(t *testing.T) {
	t.Parallel()

	input := `{
		"model": "gpt-4o",
		"input": "Hello, world!"
	}`

	req, err := DecodeRequest([]byte(input))
	if err != nil {
		t.Fatalf("DecodeRequest returned error: %v", err)
	}
	if !req.Input.IsText() {
		t.Error("expected text input")
	}
	if req.Input.Text() != "Hello, world!" {
		t.Errorf("input text: got %q, want 'Hello, world!'", req.Input.Text())
	}
}

func TestDecodeRequest_FullFeatured(t *testing.T) {
	t.Parallel()

	input := `{
		"model": "gpt-4o",
		"input": [
			{"type": "message", "role": "system", "content": "You are helpful."},
			{"type": "message", "role": "user", "content": "Tell me about Go."}
		],
		"instructions": "Be concise.",
		"temperature": 1.0,
		"top_p": 0.95,
		"max_output_tokens": 2048,
		"reasoning": {
			"effort": "high",
			"summary": "concise"
		},
		"tools": [
			{
				"type": "function",
				"name": "get_weather",
				"description": "Get weather for a city",
				"parameters": {"type": "object", "properties": {"city": {"type": "string"}}},
				"strict": true
			}
		],
		"tool_choice": "auto",
		"store": true,
		"parallel_tool_calls": true,
		"metadata": {"key1": "value1"},
		"user": "user-123",
		"service_tier": "auto",
		"truncation": "auto",
		"include": ["file_search_call.results"],
		"top_logprobs": 5,
		"stream": true,
		"prompt_cache_key": "session-abc",
		"previous_response_id": "resp_xyz"
	}`

	req, err := DecodeRequest([]byte(input))
	if err != nil {
		t.Fatalf("DecodeRequest returned error: %v", err)
	}
	if req.Model != "gpt-4o" {
		t.Errorf("model: got %q", req.Model)
	}
	if req.Instructions == nil || *req.Instructions != "Be concise." {
		t.Errorf("instructions: got %v", req.Instructions)
	}
	if req.Reasoning == nil || req.Reasoning.Effort == nil || *req.Reasoning.Effort != ReasoningEffortHigh {
		t.Errorf("reasoning.effort: got %v", req.Reasoning)
	}
	if req.Reasoning.Summary == nil || *req.Reasoning.Summary != ReasoningSummaryConcise {
		t.Errorf("reasoning.summary: got %v", req.Reasoning)
	}
	if len(req.Tools) != 1 {
		t.Fatalf("tools: got %d, want 1", len(req.Tools))
	}
	if req.Tools[0].Type() != "function" {
		t.Errorf("tool type: got %q", req.Tools[0].Type())
	}
	if req.ToolChoice == nil {
		t.Fatal("tool_choice is nil")
	}
	if s, ok := req.ToolChoice.AsString(); !ok || s != "auto" {
		t.Errorf("tool_choice: got %v", req.ToolChoice)
	}
	if req.ServiceTier == nil || *req.ServiceTier != ServiceTierAuto {
		t.Errorf("service_tier: got %v", req.ServiceTier)
	}
	if req.Truncation == nil || *req.Truncation != TruncationAuto {
		t.Errorf("truncation: got %v", req.Truncation)
	}
	if len(req.Include) != 1 || req.Include[0] != IncludeFileSearchCallResults {
		t.Errorf("include: got %v", req.Include)
	}
	if req.TopLogprobs == nil || *req.TopLogprobs != 5 {
		t.Errorf("top_logprobs: got %v", req.TopLogprobs)
	}
	if req.PreviousResponseID == nil || *req.PreviousResponseID != "resp_xyz" {
		t.Errorf("previous_response_id: got %v", req.PreviousResponseID)
	}
	if req.PromptCacheKey != "session-abc" {
		t.Errorf("prompt_cache_key: got %q", req.PromptCacheKey)
	}
	if len(req.Metadata) != 1 || req.Metadata["key1"] != "value1" {
		t.Errorf("metadata: got %v", req.Metadata)
	}
}

func TestDecodeRequest_InvalidJSON(t *testing.T) {
	t.Parallel()

	_, err := DecodeRequest([]byte(`{invalid`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestDecodeRequest_TemperatureOutOfRange(t *testing.T) {
	t.Parallel()

	input := `{
		"model": "gpt-4o",
		"input": "hi",
		"temperature": 5.0
	}`

	_, err := DecodeRequest([]byte(input))
	if err == nil {
		t.Fatal("expected validation error for temperature > 2")
	}
	var verr *RequestValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("expected RequestValidationError, got %T: %v", err, err)
	}
	t.Logf("validation error: %v", err)
}

func TestDecodeRequest_TopPOutOfRange(t *testing.T) {
	t.Parallel()

	input := `{
		"model": "gpt-4o",
		"input": "hi",
		"top_p": 1.5
	}`

	_, err := DecodeRequest([]byte(input))
	if err == nil {
		t.Fatal("expected validation error for top_p > 1")
	}
	var verr *RequestValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("expected RequestValidationError, got %T: %v", err, err)
	}
	t.Logf("validation error: %v", err)
}

func TestDecodeRequest_TopLogprobsOutOfRange(t *testing.T) {
	t.Parallel()

	input := `{
		"model": "gpt-4o",
		"input": "hi",
		"top_logprobs": 25
	}`

	_, err := DecodeRequest([]byte(input))
	if err == nil {
		t.Fatal("expected validation error for top_logprobs > 20")
	}
	var verr *RequestValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("expected RequestValidationError, got %T: %v", err, err)
	}
	t.Logf("validation error: %v", err)
}

func TestDecodeRequest_RoundTrip(t *testing.T) {
	t.Parallel()

	// Build a request with the Go API, marshal, decode back.
	original := Request{
		Model:           "gpt-4o",
		Input:           InputText("Hello"),
		Temperature:     ptrF64(0.7),
		MaxOutputTokens: ptrI(512),
		Store:           ptrB(true),
		Stream:          ptrB(true),
	}

	b, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	decoded, err := DecodeRequest(b)
	if err != nil {
		t.Fatalf("DecodeRequest: %v", err)
	}

	if decoded.Model != original.Model {
		t.Errorf("model: got %q, want %q", decoded.Model, original.Model)
	}
	if decoded.Temperature == nil || *decoded.Temperature != *original.Temperature {
		t.Errorf("temperature mismatch")
	}
	if decoded.MaxOutputTokens == nil || *decoded.MaxOutputTokens != *original.MaxOutputTokens {
		t.Errorf("max_output_tokens mismatch")
	}
}

func TestDecodeRequest_MinimalValid(t *testing.T) {
	t.Parallel()

	// Absolute minimal valid request.
	input := `{"model": "gpt-4o", "input": "hi"}`
	req, err := DecodeRequest([]byte(input))
	if err != nil {
		t.Fatalf("DecodeRequest: %v", err)
	}
	if req.Model != "gpt-4o" {
		t.Errorf("model: got %q", req.Model)
	}
}

func TestDecodeRequest_ConversationParam(t *testing.T) {
	t.Parallel()

	input := `{
		"model": "gpt-4o",
		"input": "hi",
		"conversation": "conv-123"
	}`

	req, err := DecodeRequest([]byte(input))
	if err != nil {
		t.Fatalf("DecodeRequest: %v", err)
	}
	if req.Conversation == nil {
		t.Fatal("conversation is nil")
	}
	raw, _ := req.Conversation.MarshalJSON()
	if string(raw) != `"conv-123"` {
		t.Errorf("conversation: got %s, want \"conv-123\"", raw)
	}
}

func TestDecodeRequest_TextFormat(t *testing.T) {
	t.Parallel()

	input := `{
		"model": "gpt-4o",
		"input": "hi",
		"text": {
			"format": {"type": "json_object"}
		}
	}`

	req, err := DecodeRequest([]byte(input))
	if err != nil {
		t.Fatalf("DecodeRequest: %v", err)
	}
	if req.Text == nil || req.Text.Format == nil {
		t.Fatal("text.format is nil")
	}
	if req.Text.Format.Type() != "json_object" {
		t.Errorf("text.format.type: got %q, want json_object", req.Text.Format.Type())
	}
}

func TestRequestSchema_ReturnsValidJSON(t *testing.T) {
	t.Parallel()

	raw, err := RequestSchema()
	if err != nil {
		t.Fatalf("RequestSchema: %v", err)
	}
	if !json.Valid(raw) {
		t.Fatal("RequestSchema returned invalid JSON")
	}
	// Should contain "model" field.
	var schema map[string]any
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("unmarshal schema: %v", err)
	}
	props, _ := schema["properties"].(map[string]any)
	if props == nil {
		t.Fatal("schema has no properties")
	}
	if _, ok := props["model"]; !ok {
		t.Error("schema missing 'model' property")
	}
	if _, ok := props["temperature"]; !ok {
		t.Error("schema missing 'temperature' property")
	}
	t.Logf("schema has %d properties", len(props))
}

// test helpers
func ptrF64(f float64) *float64 { return &f }
func ptrI(i int) *int           { return &i }
func ptrB(b bool) *bool         { return &b }
