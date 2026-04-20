# Conversation Package

## Overview

`conversation` provides a stateful session abstraction on top of unified streaming clients.

The guiding rule is:

> A conversation is always stateful from the caller's perspective.
> Provider-native state is only an optimization.

## Canonical History vs Projected Messages

A `conversation.Session` keeps canonical committed history locally as `[]unified.Message`.

Properties of canonical history:

- assistant output ordering is preserved exactly as observed
- reasoning, text, tool calls, and tool results are committed in canonical unified form
- failed or incomplete turns do not mutate committed history

Outbound messages for the next request are not the same thing as canonical history. They are a **projection** derived from:

- session defaults
- committed history
- pending request-local messages
- selected transport strategy

This distinction is important because some services impose replay constraints that are stricter than the canonical local transcript.

## Strategies

`conversation` currently supports:

- `StrategyReplay`
- `StrategyResponsesPreviousResponseID`
- `StrategyAuto`

`StrategyAuto` prefers native `previous_response_id` continuation when capabilities indicate it is available; otherwise it falls back to replay.

## MessageProjector

`MessageProjector` customizes how outbound replay messages are derived from canonical session state.

```go
type MessageProjector interface {
    ProjectMessages(state MessageProjectionState) ([]unified.Message, error)
}
```

The default projector:

- replays defaults + history + pending messages in replay mode
- projects only pending messages in native Responses continuation mode

Use a custom projector when a service has replay quirks that should be handled earlier than the protocol bridge layer.

## Inspection Helpers

Two helpers are available for debugging, testing, and advanced integration:

```go
msgs, err := sess.ProjectMessages(req)
out, err := sess.BuildRequest(req)
```

Use `ProjectMessages(...)` when you want to inspect only the outbound message replay projection.

Use `BuildRequest(...)` when you want the full final `unified.Request` after strategy selection and projection.

## OpenRouter-specific Replay Policy

OpenRouter-specific conversation replay policy lives in `api/openrouter/conversation.go`.

Use:

```go
conversation.WithMessageProjector(openrouter.ConversationProjector{})
```

This projector validates known OpenRouter Responses replay constraints early. Today it rejects assistant replay shapes where non-empty text appears after a tool call in the same assistant message.

That policy does **not** mutate canonical history. It only validates the outbound replay projection.

## Example

```go
sess := conversation.New(
    client.NewResponsesClient(protocol),
    conversation.WithModel("openai/gpt-4o-mini"),
    conversation.WithMessageProjector(openrouter.ConversationProjector{}),
)

msgs, err := sess.ProjectMessages(conversation.NewRequest().User("hello").Build())
req, err := sess.BuildRequest(conversation.NewRequest().User("hello").Build())
stream, err := sess.Request(ctx, conversation.NewRequest().User("hello").Build())
```

## Current Practical Status

The package is currently suitable for:

- stateful text conversations
- replay-backed continuation across stateless backends
- native `previous_response_id` continuation when available
- reasoning capture and replay in canonical history
- tool-call / tool-result loops with stronger ordering and failure-path coverage
- service-specific replay validation through custom message projectors

## Error Layering

Replay validation can happen at multiple layers.

- `MessageProjector` is the preferred place for service-specific replay validation and early, user-facing errors
- protocol bridges in `adapt` may still reject lower-level request shapes that are not representable for a specific protocol or provider

This means projector validation is an early guardrail, not a guarantee that no deeper bridge error is possible.
