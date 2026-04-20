package integration

import (
	"context"
	"os"
	"strings"
	"testing"

	ollamaapi "github.com/codewandler/agentapis/api/ollama"
	responsesapi "github.com/codewandler/agentapis/api/responses"
	"github.com/codewandler/agentapis/api/unified"
	"github.com/codewandler/agentapis/client"
	"github.com/codewandler/agentapis/conversation"
	"github.com/stretchr/testify/require"
)

func TestUnifiedConversationToolLoop(t *testing.T) {
	skipIntegrationIfNotEnabled(t)

	cases := []struct {
		name    string
		setup   func(t *testing.T) (conversation.Streamer, string)
		native  bool
	}{
		{
			name: "ollama",
			setup: func(t *testing.T) (conversation.Streamer, string) {
				ctx, model := ollamaIntegrationContext(t)
				_ = ctx
				protocol := ollamaapi.NewClient(ollamaapi.WithBaseURL(ollamaBaseURL()))
				return client.NewOllamaClient(protocol), model
			},
		},
		{
			name: "openai",
			setup: func(t *testing.T) (conversation.Streamer, string) {
				apiKey := os.Getenv("OPENAI_KEY")
				if apiKey == "" {
					t.Skip("set OPENAI_KEY to run unified conversation tool tests against OpenAI")
				}
				model := os.Getenv("OPENAI_MODEL")
				if model == "" {
					model = defaultOpenAIConversationModel
				}
				protocol := responsesapi.NewClient(responsesapi.WithAPIKey(apiKey))
				return client.NewResponsesClient(protocol), model
			},
			native: true,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			streamer, model := tc.setup(t)
			runUnifiedConversationToolLoopCase(t, streamer, model, tc.native)
		})
	}
}

func runUnifiedConversationToolLoopCase(t *testing.T, streamer conversation.Streamer, model string, native bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), openRouterSmokeTimeout)
	defer cancel()

	sess := conversation.New(
		streamer,
		conversation.WithModel(model),
		conversation.WithTools([]unified.Tool{weatherTool()}),
		conversation.WithCapabilities(conversation.Capabilities{SupportsResponsesPreviousResponseID: native}),
	)

	stream, err := sess.Request(ctx, conversation.NewRequest().User(ollamaToolSmokePrompt).Build())
	require.NoError(t, err)
	name, args, raw, completed := collectToolCall(stream, t)
	require.True(t, completed)
	require.Equal(t, ollamaToolName, name)
	require.Truef(t, toolPayloadContains(raw, args, "berlin"), "expected Berlin in tool payload, raw=%q args=%v", raw, args)

	callID := firstToolCallID(sess.History())
	require.NotEmpty(t, callID)

	toolOutput := `{"location":"Berlin","forecast":"Sunny","temperature_c":21}`
	stream, err = sess.Request(ctx, conversation.NewRequest().ToolChoice(unified.ToolChoiceNone{}).ToolResult(callID, toolOutput).Build())
	require.NoError(t, err)
	text, rawEvents := collectUnifiedConversationText(t, stream)
	require.Truef(t, strings.Contains(strings.ToLower(text), "sunny") || strings.Contains(text, "21"), "expected follow-up answer to reflect tool output, got %q (raw events: %v)", text, rawEvents)

	history := sess.History()
	require.GreaterOrEqual(t, len(history), 4)
	require.Equal(t, unified.RoleTool, history[len(history)-2].Role)
	require.Equal(t, unified.RoleAssistant, history[len(history)-1].Role)
}

func firstToolCallID(history []unified.Message) string {
	for _, msg := range history {
		if msg.Role != unified.RoleAssistant {
			continue
		}
		for _, part := range msg.Parts {
			if part.ToolCall != nil && part.ToolCall.ID != "" {
				return part.ToolCall.ID
			}
		}
	}
	return ""
}
