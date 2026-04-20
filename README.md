# agentapis

`agentapis` provides typed streaming clients for multiple model APIs, a canonical unified request and event model, adapters between protocol-specific and unified layers, and a stateful `conversation` package built on top of the unified client layer.

## Packages

- `api/messages`, `api/completions`, `api/responses`, `api/ollama`: typed protocol clients, request types, parsers, and typed hooks
- `api/unified`: canonical request and stream event model
- `adapt`: request and stream bridges between protocol-specific types and canonical unified types
- `client`: unified wrapper clients and a mux for runtime backend selection
- `conversation`: stateful session abstraction with replay/native continuation strategies, exact canonical history, and configurable outbound message projection
- `api/openrouter`: OpenRouter-specific helpers, including conversation replay projection policy for OpenRouter Responses quirks
- `internal/protocolcore`: shared HTTP, retry, and SSE execution runtime

## Layering

1. Build or receive a `unified.Request`
2. Optionally apply unified request transforms
3. Bridge to a protocol request in `adapt`
4. Stream through a typed protocol client in `api/...`
5. Parse typed protocol events
6. Bridge typed events back to `unified.StreamEvent`
7. Optionally apply unified event transforms
8. Optionally manage stateful sessions and replay/native continuation with `conversation`

See `docs/architecture.md` for the detailed flow and `docs/conversation.md` for the conversation/session model.

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
		fmt.Printf("event=%s
", item.Event.EventType())
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
		fmt.Printf("type=%s
", item.Event.Type)
	}
}
```

## Conversation Example

```go
package main

import (
	"context"
	"fmt"

	responsesapi "github.com/codewandler/agentapis/api/responses"
	"github.com/codewandler/agentapis/client"
	"github.com/codewandler/agentapis/conversation"
)

func main() {
	protocol := responsesapi.NewClient(responsesapi.WithAPIKey("token"))
	sess := conversation.New(
		client.NewResponsesClient(protocol),
		conversation.WithModel("gpt-4o-mini"),
		conversation.WithCapabilities(conversation.Capabilities{SupportsResponsesPreviousResponseID: true}),
	)

	stream, err := sess.Request(context.Background(), conversation.NewRequest().User("Reply with pong.").Build())
	if err != nil {
		panic(err)
	}
	for item := range stream {
		if item.Err != nil {
			panic(item.Err)
		}
		if item.Event.ContentDelta != nil {
			fmt.Print(item.Event.ContentDelta.Data)
		}
	}
}
```

## Projection and Custom Replay Policy Example

```go
package main

import (
	"fmt"

	"github.com/codewandler/agentapis/api/openrouter"
	"github.com/codewandler/agentapis/conversation"
)

func main() {
	sess := conversation.New(
		nil,
		conversation.WithModel("openai/gpt-4o-mini"),
		conversation.WithMessageProjector(openrouter.ConversationProjector{}),
	)

	msgs, err := sess.ProjectMessages(conversation.NewRequest().User("hello").Build())
	if err != nil {
		panic(err)
	}
	fmt.Println(len(msgs))

	req, err := sess.BuildRequest(conversation.NewRequest().User("hello").Build())
	if err != nil {
		panic(err)
	}
	fmt.Println(req.Model)
}
```

`Session.ProjectMessages(...)` exposes the exact outbound replay message list for the next turn. `Session.BuildRequest(...)` exposes the final unified request after strategy selection and message projection. Custom `conversation.MessageProjector` implementations can validate or shape replay messages for service-specific quirks without mutating canonical session history.

## Stability and Release Notes

The `conversation` package and `conversation.MessageProjector` are public release-facing APIs.

Important semantics:

- canonical `Session` history is the local source of truth and preserves assistant-part ordering exactly as observed
- `Session.ProjectMessages(...)` and `Session.BuildRequest(...)` are inspection helpers and do **not** mutate session state
- only successful `Session.Request(...)` turns commit history
- provider/service-specific replay constraints should live outside generic `conversation` when they are vendor-specific, for example in `api/openrouter`

`conversation.MessageProjector` is intended as the supported extension point for service-specific replay projection and validation.

## Native Ollama Example

```go
package main

import (
	"context"
	"fmt"

	"github.com/codewandler/agentapis/api/unified"
	"github.com/codewandler/agentapis/client"
)

func main() {
	uclient := client.NewOllamaClient(nil)
	stream, err := uclient.Stream(context.Background(), unified.Request{
		Model:     "qwen3",
		MaxTokens: 64,
		Thinking:  unified.ThinkingModeOn,
		Messages: []unified.Message{
			{Role: unified.RoleUser, Parts: []unified.Part{{Type: unified.PartTypeText, Text: "What is 17 * 23?"}}},
		},
	})
	if err != nil {
		panic(err)
	}

	for item := range stream {
		if item.Err != nil {
			panic(item.Err)
		}
		if item.Event.Delta != nil && item.Event.Delta.Kind == unified.DeltaKindThinking {
			fmt.Print(item.Event.Delta.Thinking)
		}
		if item.Event.Delta != nil && item.Event.Delta.Kind == unified.DeltaKindText {
			fmt.Print(item.Event.Delta.Text)
		}
	}
}
```

## Mux Routing Example

When multiple backends are configured, either provide a custom resolver or use the opt-in heuristic `client.DefaultTargetResolver`.

```go
mux := client.NewMuxClient(
	client.WithResponsesClient(client.NewResponsesClient(nil)),
	client.WithOllamaClient(client.NewOllamaClient(nil)),
	client.WithTargetResolver(client.DefaultTargetResolver),
)

req := unified.Request{
	Model: "ollama/qwen3",
	Messages: []unified.Message{
		{Role: unified.RoleUser, Parts: []unified.Part{{Type: unified.PartTypeText, Text: "Reply with pong."}}},
	},
}

stream, err := mux.Stream(context.Background(), req)
_ = stream
_ = err
```

You can also provide an explicit provider hint via `req.Extras.Provider`, for example:

```go
req.Extras.Provider = map[string]any{"target": "ollama"}
```

## Testing

### Unit tests

```bash
go test ./...
```

### Integration tests

Integration tests live in `integration/` and are runtime-gated, not build-tag gated.

Shared gate behavior:
- skipped in `-short` mode
- skipped unless `TEST_INTEGRATION=1`

Ollama smoke tests additionally skip when the configured/default Ollama endpoint is not reachable.

Examples:

```bash
TEST_INTEGRATION=1 go test ./integration -run TestSmokeOpenRouterResponses -v
```

```bash
TEST_INTEGRATION=1 go test ./integration -run TestSmokeOllamaNative -v
```

```bash
TEST_INTEGRATION=1 OLLAMA_MODEL=gemma4:e4b go test ./integration -run 'TestSmokeOllama(Native|NativeToolCalling|ResponsesToolCalling)' -v
```
