package integration

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	ollamaapi "github.com/codewandler/agentapis/api/ollama"
	responsesapi "github.com/codewandler/agentapis/api/responses"
	"github.com/codewandler/agentapis/api/unified"
	"github.com/codewandler/agentapis/client"
	"github.com/codewandler/agentapis/conversation"
	"github.com/stretchr/testify/require"
)

const defaultOpenAIConversationModel = "gpt-4o-mini"

func TestUnifiedConversation(t *testing.T) {
	skipIntegrationIfNotEnabled(t)

	cases := []struct {
		name    string
		setup   func(t *testing.T) (conversation.Streamer, string)
		expects string
	}{
		{
			name: "ollama",
			setup: func(t *testing.T) (conversation.Streamer, string) {
				ctx, model := ollamaIntegrationContext(t)
				_ = ctx
				protocol := ollamaapi.NewClient(ollamaapi.WithBaseURL(ollamaBaseURL()))
				return client.NewOllamaClient(protocol), model
			},
			expects: "replay",
		},
		{
			name: "openai",
			setup: func(t *testing.T) (conversation.Streamer, string) {
				apiKey := os.Getenv("OPENAI_KEY")
				if apiKey == "" {
					t.Skip("set OPENAI_KEY to run unified conversation tests against OpenAI")
				}
				model := os.Getenv("OPENAI_MODEL")
				if model == "" {
					model = defaultOpenAIConversationModel
				}
				protocol := responsesapi.NewClient(responsesapi.WithAPIKey(apiKey))
				return client.NewResponsesClient(protocol), model
			},
			expects: "native",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			streamer, model := tc.setup(t)
			runUnifiedConversationCase(t, streamer, model, tc.expects)
		})
	}
}

func runUnifiedConversationCase(t *testing.T, streamer conversation.Streamer, model, mode string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), openRouterSmokeTimeout)
	defer cancel()

	token := fmt.Sprintf("unified-conv-token-%d", time.Now().UnixNano())
	sess := conversation.New(
		streamer,
		conversation.WithModel(model),
		conversation.WithCapabilities(conversation.Capabilities{SupportsResponsesPreviousResponseID: mode == "native"}),
	)

	firstReq := conversation.NewRequest().
		User(fmt.Sprintf("For this short conversation test, the reference codeword is %s. Reply with exactly: stored", token)).
		Build()
	firstStream, err := sess.Request(ctx, firstReq)
	require.NoError(t, err)
	firstText, firstRawEvents := collectUnifiedConversationText(t, firstStream)
	require.Containsf(t, strings.ToLower(firstText), "stored", "expected first response to acknowledge storage, got %q (raw events: %v)", firstText, firstRawEvents)

	secondReq := conversation.NewRequest().
		User("What was the exact reference codeword from the previous turn? Reply with only that exact codeword.").
		Build()
	secondStream, err := sess.Request(ctx, secondReq)
	require.NoError(t, err)
	secondText, secondRawEvents := collectUnifiedConversationText(t, secondStream)
	require.Containsf(t, secondText, token, "expected second response to contain remembered token %q, got %q (raw events: %v)", token, secondText, secondRawEvents)

	history := sess.History()
	require.GreaterOrEqualf(t, len(history), 4, "expected conversation history to contain two turns, got %#v", history)
	require.Equal(t, unified.RoleAssistant, history[len(history)-1].Role)
	require.Equal(t, unified.RoleUser, history[len(history)-2].Role)
}

func collectUnifiedConversationText(t *testing.T, stream <-chan client.StreamResult) (string, []string) {
	t.Helper()
	var text strings.Builder
	var fallback strings.Builder
	var rawEvents []string
	var sawCompleted bool

	for item := range stream {
		require.NoErrorf(t, item.Err, "raw events: %v", rawEvents)
		if item.RawEventName != "" {
			rawEvents = append(rawEvents, item.RawEventName)
		}
		if item.Event.ContentDelta != nil && item.Event.ContentDelta.Kind == unified.ContentKindText {
			text.WriteString(item.Event.ContentDelta.Data)
		}
		if item.Event.StreamContent != nil && item.Event.StreamContent.Kind == unified.ContentKindText {
			fallback.WriteString(item.Event.StreamContent.Data)
		}
		if item.Event.Type == unified.StreamEventCompleted && item.Event.Completed != nil {
			sawCompleted = true
		}
	}

	require.Truef(t, sawCompleted, "expected completed event, raw events: %v", rawEvents)
	if text.Len() > 0 {
		return text.String(), rawEvents
	}
	return fallback.String(), rawEvents
}
