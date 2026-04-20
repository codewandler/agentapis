package integration

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	responsesapi "github.com/codewandler/agentapis/api/responses"
	"github.com/stretchr/testify/require"
)

const defaultOpenAIResponsesModel = "gpt-4o-mini"

type responsesConversationProvider struct {
	name           string
	apiKeyEnv      string
	baseURL        string
	model          string
	expectStateful bool
}

func TestResponsesConversationChaining(t *testing.T) {
	skipIntegrationIfNotEnabled(t)

	providers := []responsesConversationProvider{
		{
			name:           "openrouter",
			apiKeyEnv:      "OPENROUTER_API_KEY",
			baseURL:        envOrDefault("OPENROUTER_BASE_URL", defaultOpenRouterBaseURL),
			model:          envOrDefault("OPENROUTER_MODEL", defaultOpenRouterModel),
			expectStateful: false,
		},
		{
			name:           "openai",
			apiKeyEnv:      "OPENAI_KEY",
			baseURL:        "",
			model:          envOrDefault("OPENAI_MODEL", defaultOpenAIResponsesModel),
			expectStateful: true,
		},
	}

	for _, provider := range providers {
		provider := provider
		t.Run(provider.name, func(t *testing.T) {
			apiKey := os.Getenv(provider.apiKeyEnv)
			if apiKey == "" {
				t.Skipf("set %s to run %s responses conversation tests", provider.apiKeyEnv, provider.name)
			}
			runResponsesConversationChainingTest(t, provider, apiKey)
		})
	}
}

func runResponsesConversationChainingTest(t *testing.T, provider responsesConversationProvider, apiKey string) {
	t.Helper()

	opts := []responsesapi.Option{responsesapi.WithAPIKey(apiKey)}
	if provider.baseURL != "" {
		opts = append(opts, responsesapi.WithBaseURL(provider.baseURL))
	}
	protocol := responsesapi.NewClient(opts...)

	ctx, cancel := context.WithTimeout(context.Background(), openRouterSmokeTimeout)
	defer cancel()

	token := fmt.Sprintf("stateful-token-%d", time.Now().UnixNano())

	firstStream, err := protocol.Stream(ctx, responsesapi.Request{
		Model: provider.model,
		Input: []responsesapi.Input{{
			Role:    "user",
			Content: fmt.Sprintf("For a short conversation-state test, treat this as non-sensitive test data. The reference codeword is %s. Reply with exactly: stored", token),
		}},
	})
	require.NoError(t, err)

	firstResponseID, firstText, firstRawEvents := collectResponsesConversationResult(t, firstStream)
	assertResponsesConversationTurn(t, provider.name, 1, firstResponseID, firstText, firstRawEvents)
	require.Containsf(t, strings.ToLower(firstText), "stored", "%s: expected first response to acknowledge storage, got %q (raw events: %v)", provider.name, firstText, firstRawEvents)

	secondStream, err := protocol.Stream(ctx, responsesapi.Request{
		Model:              provider.model,
		PreviousResponseID: firstResponseID,
		Input: []responsesapi.Input{{
			Role:    "user",
			Content: "What was the reference codeword from the previous turn? Reply with only that exact codeword.",
		}},
	})
	require.NoError(t, err)

	secondResponseID, secondText, secondRawEvents := collectResponsesConversationResult(t, secondStream)
	assertResponsesConversationTurn(t, provider.name, 2, secondResponseID, secondText, secondRawEvents)
	assertResponsesConversationStateBehavior(t, provider, token, secondText, secondRawEvents)
}

func assertResponsesConversationTurn(t *testing.T, provider string, turn int, responseID, text string, rawEvents []string) {
	t.Helper()
	require.NotEmptyf(t, responseID, "%s turn %d: expected response id, raw events: %v", provider, turn, rawEvents)
	require.NotEmptyf(t, strings.TrimSpace(text), "%s turn %d: expected response text, raw events: %v", provider, turn, rawEvents)
}

func assertResponsesConversationStateBehavior(t *testing.T, provider responsesConversationProvider, token, secondText string, rawEvents []string) {
	t.Helper()
	if provider.expectStateful {
		require.Containsf(t, secondText, token, "%s: expected second response to contain remembered token %q, got %q (raw events: %v)", provider.name, token, secondText, rawEvents)
		return
	}
	require.NotContainsf(t, secondText, token, "%s: expected stateless provider not to recover token %q, got %q (raw events: %v)", provider.name, token, secondText, rawEvents)
}

func collectResponsesConversationResult(t *testing.T, stream <-chan responsesapi.StreamResult) (string, string, []string) {
	t.Helper()

	var responseID string
	var text strings.Builder
	var rawEvents []string
	var sawCompleted bool

	for item := range stream {
		require.NoErrorf(t, item.Err, "raw events: %v", rawEvents)
		if item.RawEventName != "" {
			rawEvents = append(rawEvents, item.RawEventName)
		}
		switch ev := item.Event.(type) {
		case *responsesapi.ResponseCreatedEvent:
			if responseID == "" {
				responseID = ev.Response.ID
			}
		case *responsesapi.ResponseCompletedEvent:
			if responseID == "" {
				responseID = ev.Response.ID
			}
			sawCompleted = true
		case *responsesapi.OutputTextDeltaEvent:
			text.WriteString(ev.Delta)
		case *responsesapi.OutputTextDoneEvent:
			if ev.Text != "" && !strings.Contains(text.String(), ev.Text) {
				text.WriteString(ev.Text)
			}
		case *responsesapi.ContentPartDoneEvent:
			if ev.Part.Text != "" && !strings.Contains(text.String(), ev.Part.Text) {
				text.WriteString(ev.Part.Text)
			}
		}
	}

	require.Truef(t, sawCompleted, "expected completed event, raw events: %v", rawEvents)
	return responseID, text.String(), rawEvents
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
