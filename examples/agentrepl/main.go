package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	responsesapi "github.com/codewandler/agentapis/api/responses"
	"github.com/codewandler/agentapis/api/unified"
	"github.com/codewandler/agentapis/client"
	"github.com/codewandler/agentapis/conversation"
)

func main() {
	apiKey := os.Getenv("OPENAI_KEY")
	if apiKey == "" {
		log.Fatal("set OPENAI_KEY")
	}
	model := os.Getenv("OPENAI_MODEL")
	if model == "" {
		model = "gpt-4o-mini"
	}

	protocol := responsesapi.NewClient(responsesapi.WithAPIKey(apiKey))
	streamer := client.NewResponsesClient(protocol)
	sess := conversation.New(
		streamer,
		conversation.WithModel(model),
		conversation.WithCapabilities(conversation.Capabilities{SupportsResponsesPreviousResponseID: true}),
	)

	scanner := bufio.NewScanner(os.Stdin)
	fmt.Printf("agentrepl using model %s\n", model)
	fmt.Println("type messages and press enter; type /exit to quit")

	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				log.Fatal(err)
			}
			return
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if line == "/exit" || line == "/quit" {
			return
		}

		stream, err := sess.Request(context.Background(), conversation.NewRequest().User(line).Build())
		if err != nil {
			log.Printf("request error: %v", err)
			continue
		}

		var text strings.Builder
		for item := range stream {
			if item.Err != nil {
				log.Printf("stream error: %v", item.Err)
				continue
			}
			if captureText(&text, item.Event) {
				continue
			}
			if shouldSkipEvent(item.Event) {
				continue
			}
			printEvent(item.Event)
		}
		if text.Len() > 0 {
			fmt.Printf("assistant: %s\n", text.String())
		}
		fmt.Println()
	}
}

func captureText(buf *strings.Builder, ev unified.StreamEvent) bool {
	if ev.Type == unified.StreamEventContentDelta && ev.ContentDelta != nil && ev.ContentDelta.Kind == unified.ContentKindText {
		buf.WriteString(ev.ContentDelta.Data)
		return true
	}
	if ev.Type == unified.StreamEventContent && ev.StreamContent != nil && ev.StreamContent.Kind == unified.ContentKindText {
		if buf.Len() == 0 {
			buf.WriteString(ev.StreamContent.Data)
		}
		return true
	}
	return false
}

func shouldSkipEvent(ev unified.StreamEvent) bool {
	if ev.Type == unified.StreamEventContentDelta && ev.ContentDelta != nil {
		switch ev.ContentDelta.Kind {
		case unified.ContentKindText, unified.ContentKindReasoning:
			return true
		}
	}
	if ev.Type == unified.StreamEventContent && ev.StreamContent != nil {
		if ev.StreamContent.Kind == unified.ContentKindReasoning {
			return true
		}
	}
	if ev.Type == unified.StreamEventToolDelta {
		return true
	}
	if ev.Delta != nil {
		switch ev.Delta.Kind {
		case unified.DeltaKindText, unified.DeltaKindThinking, unified.DeltaKindTool:
			return true
		}
	}
	return false
}

func printEvent(ev unified.StreamEvent) {
	b, err := json.MarshalIndent(ev, "", "  ")
	if err != nil {
		fmt.Printf("%#v\n", ev)
		return
	}
	fmt.Printf("[event]\n%s\n", b)
}
