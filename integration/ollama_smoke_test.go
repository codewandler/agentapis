package integration

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	ollamaapi "github.com/codewandler/agentapis/api/ollama"
	responsesapi "github.com/codewandler/agentapis/api/responses"
	"github.com/codewandler/agentapis/api/unified"
	"github.com/codewandler/agentapis/client"
	"github.com/stretchr/testify/require"
)

const (
	defaultOllamaSmokeModel       = "gemma4:e4b"
	ollamaSmokePrompt             = "Reply with exactly the word pong."
	ollamaToolSmokePrompt         = "Use the weather tool to get the weather in Berlin. Do not answer from memory."
	ollamaToolName                = "get_weather"
	ollamaToolDescription         = "Get the weather for a location"
	ollamaToolRequiredField       = "location"
	ollamaSmokeTimeout            = 90 * time.Second
	ollamaReachabilityProbeTimeout = 2 * time.Second
)

func TestSmokeOllamaNative(t *testing.T) {
	ctx, model := ollamaIntegrationContext(t)
	uclient := client.NewOllamaClient(nil)
	stream, err := uclient.Stream(ctx, unified.Request{
		Model:     model,
		MaxTokens: 32,
		Messages: []unified.Message{{Role: unified.RoleUser, Parts: []unified.Part{{Type: unified.PartTypeText, Text: ollamaSmokePrompt}}}},
	})
	require.NoError(t, err)

	var sawContent, sawCompleted bool
	var text strings.Builder
	for item := range stream {
		require.NoError(t, item.Err)
		if item.Event.Type == unified.StreamEventContentDelta && item.Event.ContentDelta != nil {
			sawContent = true
			text.WriteString(item.Event.ContentDelta.Data)
		}
		if item.Event.Type == unified.StreamEventCompleted {
			sawCompleted = true
		}
	}
	require.True(t, sawContent)
	require.True(t, sawCompleted)
	require.Contains(t, strings.ToLower(text.String()), "pong")
}

func TestSmokeOllamaNativeToolCalling(t *testing.T) {
	ctx, model := ollamaIntegrationContext(t)
	uclient := client.NewOllamaClient(nil)
	stream, err := uclient.Stream(ctx, unified.Request{
		Model: model,
		Tools: []unified.Tool{weatherTool()},
		Messages: []unified.Message{{Role: unified.RoleUser, Parts: []unified.Part{{Type: unified.PartTypeText, Text: ollamaToolSmokePrompt}}}},
	})
	require.NoError(t, err)
	name, args, raw, completed := collectToolCall(stream, t)
	require.True(t, completed)
	require.Equal(t, ollamaToolName, name)
	require.Truef(t, toolPayloadContains(raw, args, "berlin"), "expected Berlin in tool payload, raw=%q args=%v", raw, args)
}

func TestSmokeOllamaMessagesToolCalling(t *testing.T) {
	t.Skip("Ollama Anthropic-compatible /v1/messages tool calling is not stable enough in local smoke runs yet")
}

func TestSmokeOllamaResponsesToolCalling(t *testing.T) {
	ctx, model := ollamaIntegrationContext(t)
	protocol := responsesapi.NewClient(responsesapi.WithBaseURL(ollamaBaseURL()), responsesapi.WithAPIKey("ollama"))
	uclient := client.NewResponsesClient(protocol)
	stream, err := uclient.Stream(ctx, unified.Request{
		Model: model,
		Tools: []unified.Tool{weatherTool()},
		Messages: []unified.Message{{Role: unified.RoleUser, Parts: []unified.Part{{Type: unified.PartTypeText, Text: ollamaToolSmokePrompt}}}},
	})
	require.NoError(t, err)
	name, args, raw, completed := collectToolCall(stream, t)
	require.True(t, completed)
	require.Equal(t, ollamaToolName, name)
	require.Truef(t, toolPayloadContains(raw, args, "berlin"), "expected Berlin in tool payload, raw=%q args=%v", raw, args)
}

func ollamaIntegrationContext(t *testing.T) (context.Context, string) {
	t.Helper()
	skipIntegrationIfNotEnabled(t)
	if !ollamaReachable(t) {
		t.Skipf("skipping Ollama smoke tests because %s is not reachable", ollamaBaseURL())
	}
	model := os.Getenv("OLLAMA_MODEL")
	if model == "" {
		model = defaultOllamaSmokeModel
	}
	ctx, cancel := context.WithTimeout(context.Background(), ollamaSmokeTimeout)
	t.Cleanup(cancel)
	return ctx, model
}

func ollamaReachable(t *testing.T) bool {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), ollamaReachabilityProbeTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ollamaBaseURL()+ollamaapi.TagsPath, nil)
	if err != nil {
		return false
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusInternalServerError
}

func ollamaBaseURL() string {
	if v := os.Getenv("OLLAMA_BASE_URL"); v != "" {
		return v
	}
	return ollamaapi.DefaultBaseURL
}

func weatherTool() unified.Tool {
	return unified.Tool{
		Name:        ollamaToolName,
		Description: ollamaToolDescription,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"location": map[string]any{"type": "string", "description": "The city and state, e.g. Berlin"},
				"city":     map[string]any{"type": "string", "description": "The city name"},
			},
			"required": []any{ollamaToolRequiredField},
		},
	}
}

func collectToolCall(stream <-chan client.StreamResult, t *testing.T) (string, map[string]any, string, bool) {
	t.Helper()
	var name, raw string
	var args map[string]any
	var completed bool
	for item := range stream {
		require.NoError(t, item.Err)
		if item.Event.StreamToolCall != nil {
			name = item.Event.StreamToolCall.Name
			args = item.Event.StreamToolCall.Args
			raw = item.Event.StreamToolCall.RawInput
		}
		if item.Event.ToolCall != nil && name == "" {
			name = item.Event.ToolCall.Name
			args = item.Event.ToolCall.Args
		}
		if item.Event.Type == unified.StreamEventCompleted {
			completed = true
		}
	}
	return name, args, raw, completed
}

func toolPayloadContains(raw string, args map[string]any, needle string) bool {
	needle = strings.ToLower(needle)
	if strings.Contains(strings.ToLower(raw), needle) {
		return true
	}
	for _, v := range args {
		if strings.Contains(strings.ToLower(fmt.Sprint(v)), needle) {
			return true
		}
	}
	return false
}
