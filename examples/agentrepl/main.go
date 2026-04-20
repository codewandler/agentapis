package main

import (
	"bufio"
	"context"
	"flag"
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
	modelFlag := flag.String("m", "", "OpenAI model to use (overrides OPENAI_MODEL)")
	cacheFlag := flag.Bool("cache", true, "Enable top-level prompt caching hint for OpenAI-compatible requests")
	flag.Parse()

	apiKey := os.Getenv("OPENAI_KEY")
	if apiKey == "" {
		log.Fatal("set OPENAI_KEY")
	}
	model := *modelFlag
	if model == "" {
		model = os.Getenv("OPENAI_MODEL")
	}
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
	if *cacheFlag {
		fmt.Println("top-level caching: enabled")
	} else {
		fmt.Println("top-level caching: disabled")
	}
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

		b := conversation.NewRequest().User(line)
		if *cacheFlag {
			b.CacheHint(&unified.CacheHint{Enabled: true, TTL: "1h"})
		}
		req := b.Build()
		stream, err := sess.Request(context.Background(), req)
		if err != nil {
			log.Printf("request error: %v", err)
			continue
		}

		var sawText bool
		var sawReasoning bool
		for ev := range stream {
			switch e := ev.(type) {
			case conversation.TextDeltaEvent:
				if !sawText {
					if sawReasoning {
						fmt.Println()
					}
					fmt.Print("assistant: ")
					sawText = true
				}
				fmt.Print(e.Text)
			case conversation.ReasoningDeltaEvent:
				if !sawText && !sawReasoning {
					fmt.Print("thinking: ")
					sawReasoning = true
				}
				fmt.Print(e.Text)
			case conversation.ToolCallEvent:
				if sawReasoning || sawText {
					fmt.Println()
					sawReasoning = false
					sawText = false
				}
				log.Printf("tool call: %s", e.ToolCall.Name)
			case conversation.UsageEvent:
				if sawReasoning || sawText {
					fmt.Println()
					sawReasoning = false
					sawText = false
				}
				log.Printf("usage: %s", formatUsage(e.Usage))
			case conversation.ErrorEvent:
				if sawReasoning || sawText {
					fmt.Println()
					sawReasoning = false
					sawText = false
				}
				log.Printf("stream error: %v", e.Err)
			case conversation.CompletedEvent:
				if sawReasoning || sawText {
					fmt.Println()
					sawReasoning = false
					sawText = false
				}
			}
		}
		fmt.Println()
	}
}

func formatUsage(u unified.StreamUsage) string {
	parts := []string{fmt.Sprintf("in=%d", u.Input.Total)}
	var inDetails []string
	if u.Input.New > 0 {
		inDetails = append(inDetails, fmt.Sprintf("new=%d", u.Input.New))
	}
	if u.Input.CacheRead > 0 {
		inDetails = append(inDetails, fmt.Sprintf("cache_read=%d", u.Input.CacheRead))
	}
	if u.Input.CacheWrite > 0 {
		inDetails = append(inDetails, fmt.Sprintf("cache_write=%d", u.Input.CacheWrite))
	}
	if len(inDetails) > 0 {
		parts[0] += " (" + strings.Join(inDetails, " ") + ")"
	}
	out := fmt.Sprintf("out=%d", u.Output.Total)
	var outDetails []string
	if u.Output.Reasoning > 0 {
		outDetails = append(outDetails, fmt.Sprintf("reasoning=%d", u.Output.Reasoning))
	}
	if len(outDetails) > 0 {
		out += " (" + strings.Join(outDetails, " ") + ")"
	}
	parts = append(parts, out)
	if totalCost := u.Costs.Total(); totalCost != 0 {
		parts = append(parts, fmt.Sprintf("cost=%.6f", totalCost))
	}
	return strings.Join(parts, " ")
}


