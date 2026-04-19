package integration

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	responsesapi "github.com/codewandler/agentapis/api/responses"
	"github.com/codewandler/agentapis/api/unified"
	"github.com/codewandler/agentapis/client"
	"github.com/stretchr/testify/require"
)

const (
	defaultOpenRouterBaseURL = "https://openrouter.ai/api"
	defaultOpenRouterModel   = "openai/gpt-4o-mini"
	openRouterSmokePrompt    = "Reply with exactly the word pong."
	openRouterSmokeTimeout   = 90 * time.Second
)

func TestSmokeOpenRouterResponses(t *testing.T) {
	skipIntegrationIfNotEnabled(t)
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		t.Skip("set OPENROUTER_API_KEY to run OpenRouter smoke tests")
	}

	baseURL := os.Getenv("OPENROUTER_BASE_URL")
	if baseURL == "" {
		baseURL = defaultOpenRouterBaseURL
	}
	model := os.Getenv("OPENROUTER_MODEL")
	if model == "" {
		model = defaultOpenRouterModel
	}

	protocol := responsesapi.NewClient(
		responsesapi.WithAPIKey(apiKey),
		responsesapi.WithBaseURL(baseURL),
	)
	uclient := client.NewResponsesClient(protocol)

	ctx, cancel := context.WithTimeout(context.Background(), openRouterSmokeTimeout)
	defer cancel()

	stream, err := uclient.Stream(ctx, unified.Request{
		Model:     model,
		MaxTokens: 32,
		Messages:  []unified.Message{{Role: unified.RoleUser, Parts: []unified.Part{{Type: unified.PartTypeText, Text: openRouterSmokePrompt}}}},
	})
	require.NoError(t, err)

	var sawStarted, sawContent, sawCompleted bool
	var text strings.Builder
	var rawEvents []string

	for item := range stream {
		require.NoErrorf(t, item.Err, "raw events: %v", rawEvents)
		if item.RawEventName != "" {
			rawEvents = append(rawEvents, item.RawEventName)
		}
		switch item.Event.Type {
		case unified.StreamEventStarted:
			sawStarted = true
		case unified.StreamEventContentDelta:
			sawContent = true
			if item.Event.ContentDelta != nil {
				text.WriteString(item.Event.ContentDelta.Data)
			}
		case unified.StreamEventContent:
			sawContent = true
			if item.Event.StreamContent != nil {
				text.WriteString(item.Event.StreamContent.Data)
			}
		case unified.StreamEventCompleted:
			sawCompleted = true
		}
	}

	require.Truef(t, sawStarted, "expected a started event, raw events: %v", rawEvents)
	require.Truef(t, sawContent, "expected content-bearing events, raw events: %v", rawEvents)
	require.Truef(t, sawCompleted, "expected a completed event, raw events: %v", rawEvents)
	require.Containsf(t, strings.ToLower(text.String()), "pong", "expected streamed text to contain pong, got %q (raw events: %v)", text.String(), rawEvents)
}
