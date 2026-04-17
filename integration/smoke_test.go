//go:build integration

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
)

func TestSmokeOpenRouterResponses(t *testing.T) {
	if os.Getenv("RUN_INTEGRATION") != "1" {
		t.Skip("set RUN_INTEGRATION=1 to run integration smoke tests")
	}
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		t.Skip("set OPENROUTER_API_KEY to run OpenRouter smoke tests")
	}

	baseURL := os.Getenv("OPENROUTER_BASE_URL")
	if baseURL == "" {
		baseURL = "https://openrouter.ai/api"
	}
	model := os.Getenv("OPENROUTER_MODEL")
	if model == "" {
		model = "openai/gpt-4o-mini"
	}

	protocol := responsesapi.NewClient(
		responsesapi.WithAPIKey(apiKey),
		responsesapi.WithBaseURL(baseURL),
	)
	uclient := client.NewResponsesClient(protocol)

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	stream, err := uclient.Stream(ctx, unified.Request{
		Model:     model,
		MaxTokens: 32,
		Messages: []unified.Message{{
			Role:  unified.RoleUser,
			Parts: []unified.Part{{Type: unified.PartTypeText, Text: "Reply with exactly the word pong."}},
		}},
	})
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}

	var (
		sawStarted   bool
		sawContent   bool
		sawCompleted bool
		text         strings.Builder
		rawEvents    []string
	)

	for item := range stream {
		if item.Err != nil {
			t.Fatalf("unexpected stream error: %v (raw events: %v)", item.Err, rawEvents)
		}
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

	if !sawStarted {
		t.Fatalf("expected a started event, raw events: %v", rawEvents)
	}
	if !sawContent {
		t.Fatalf("expected content-bearing events, raw events: %v", rawEvents)
	}
	if !sawCompleted {
		t.Fatalf("expected a completed event, raw events: %v", rawEvents)
	}
	if !strings.Contains(strings.ToLower(text.String()), "pong") {
		t.Fatalf("expected streamed text to contain pong, got %q (raw events: %v)", text.String(), rawEvents)
	}
}
