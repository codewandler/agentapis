# agentapis

`agentapis` provides typed streaming clients for multiple model APIs, a canonical unified request and event model, and adapters between the protocol-specific and unified layers.

## Packages

- `api/messages`, `api/completions`, `api/responses`: typed protocol clients, request types, parsers, and typed hooks
- `adapt`: request and stream bridges between protocol-specific types and canonical unified types
- `api/unified`: canonical request and stream event model
- `client`: unified wrapper clients and a mux for runtime backend selection
- `internal/protocolcore`: shared HTTP, retry, and SSE execution runtime

## Layering

1. Build or receive a `unified.Request`
2. Optionally apply unified request transforms
3. Bridge to a protocol request in `adapt`
4. Stream through a typed protocol client in `api/...`
5. Parse typed protocol events
6. Bridge typed events back to `unified.StreamEvent`
7. Optionally apply unified event transforms

See `docs/architecture.md` for the detailed flow.

## Typed Protocol Example

```go
package main

import (
	"context"
	"fmt"

	"github.com/codewandler/agentapis/api/responses"
)

func main() {
	client := responses.NewClient(
		responses.WithAPIKey("token"),
		responses.WithBaseURL("https://openrouter.ai/api"),
	)

	stream, err := client.Stream(context.Background(), responses.Request{
		Model:  "openai/gpt-4o-mini",
		Stream: true,
		Input:  []responses.Input{{Role: "user", Content: "Reply with pong."}},
	})
	if err != nil {
		panic(err)
	}

	for item := range stream {
		if item.Err != nil {
			panic(item.Err)
		}
		fmt.Printf("event=%s\n", item.Event.EventType())
	}
}
```

## Unified Wrapper Example

```go
package main

import (
	"context"
	"fmt"

	responsesapi "github.com/codewandler/agentapis/api/responses"
	"github.com/codewandler/agentapis/api/unified"
	"github.com/codewandler/agentapis/client"
)

func main() {
	protocol := responsesapi.NewClient(
		responsesapi.WithAPIKey("token"),
		responsesapi.WithBaseURL("https://openrouter.ai/api"),
	)

	uclient := client.NewResponsesClient(protocol)
	stream, err := uclient.Stream(context.Background(), unified.Request{
		Model:     "openai/gpt-4o-mini",
		MaxTokens: 32,
		Messages: []unified.Message{
			{Role: unified.RoleUser, Parts: []unified.Part{{Type: unified.PartTypeText, Text: "Reply with pong."}}},
		},
	})
	if err != nil {
		panic(err)
	}

	for item := range stream {
		if item.Err != nil {
			panic(item.Err)
		}
		fmt.Printf("type=%s\n", item.Event.Type)
	}
}
```

## Testing

- Unit tests: `go test ./...`
- Integration smoke tests are opt-in and use build tags and env gates

Example:

```bash
RUN_INTEGRATION=1 OPENROUTER_API_KEY=... go test -tags=integration ./integration -run TestSmokeOpenRouterResponses -v
```
