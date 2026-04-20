# Architecture

## Overview

This repository has six layers:

1. Typed protocol clients in `api/messages`, `api/completions`, `api/responses`, `api/ollama`
2. Canonical request and stream event types in `api/unified`
3. Translation bridges in `adapt`
4. Unified wrapper clients and runtime backend selection in `client`
5. Stateful canonical session management and replay/native strategy handling in `conversation`
6. Provider/service-specific policy helpers such as `api/openrouter`

Shared HTTP, retry, and SSE execution lives in `internal/protocolcore`.

## Protocol Layer

Each protocol package exposes:

- typed request and event types
- a typed streaming client
- typed hooks for request transforms, HTTP mutation, event transforms, and request or response observation

The protocol layer knows about provider wire formats and event shapes. It does not know about the canonical unified model or session semantics.

## Canonical Unified Layer

`api/unified` defines:

- a canonical `Request`
- canonical enums for roles, parts, thinking, output, and stop reasons
- canonical `StreamEvent` envelopes for lifecycle, content deltas, tool deltas, usage, completion, and errors

The unified model is designed to be stable across multiple upstream protocols.

## Adapt Layer

`adapt` connects the protocol-specific and unified layers.

Outbound bridges:

- `BuildMessagesRequest`
- `BuildCompletionsRequest`
- `BuildResponsesRequest`
- `BuildOllamaRequest`

Inbound bridges:

- `MapMessagesEvent`
- `MapCompletionsEvent`
- `MapResponsesEvent`
- `OllamaMapper.MapEvent`

`adapt` should remain translation-focused. It should not own transport execution, session state, or service-level conversation policy.

## Unified Wrapper Layer

`client` exposes unified wrappers around concrete protocol clients.

Examples:

- `client.MessagesClient`
- `client.CompletionsClient`
- `client.ResponsesClient`
- `client.OllamaClient`
- `client.MuxClient`

These wrappers:

1. accept `unified.Request`
2. apply unified request transforms
3. build a protocol request via `adapt`
4. stream via a typed protocol client
5. map protocol events back to canonical `unified.StreamEvent`
6. apply unified event transforms

`client.MuxClient` adds runtime backend selection on top of the same flow.

## Conversation Layer

`conversation` owns stateful conversation semantics above the unified streaming clients.

It is responsible for:

- session defaults (`model`, tools, system/developer defaults)
- canonical committed history as `[]unified.Message`
- exact assistant-part ordering in committed history
- turn commit / rollback behavior
- replay vs native continuation strategy selection
- outbound message projection via `MessageProjector`
- request inspection helpers via `Session.ProjectMessages(...)` and `Session.BuildRequest(...)`

Important distinction:

- canonical session history is the local source of truth
- outbound replay messages are a projection derived from that history

This lets the library preserve exact history while still allowing service-specific replay policy.

## Provider/Service Policy Layer

Some quirks are not purely protocol-level. Multiple services may expose a similar API kind but differ in replay expectations or tolerated assistant message shapes.

That policy belongs outside the generic `conversation` package.

Example:

- `api/openrouter/conversation.go` exposes `openrouter.ConversationProjector`, which validates OpenRouter Responses replay constraints early without mutating canonical history.

## Shared Runtime

`internal/protocolcore` provides shared support code for:

- HTTP request execution
- response status and typed error parsing
- SSE scanning
- retry transport behavior
- typed request and response metadata passed to protocol hooks

It is intentionally internal so the public API stays typed and package-specific.

## Request Flow

### Unified streaming flow

1. Caller creates `unified.Request`
2. `client` applies unified request transforms
3. `adapt` builds a typed protocol request
4. Typed protocol client applies protocol request transforms and header resolution
5. Shared runtime sends HTTP request and streams SSE/NDJSON events
6. Typed protocol parser yields typed protocol events
7. `adapt` maps those events to canonical `unified.StreamEvent`
8. `client` applies unified event transforms and emits final stream items

### Stateful conversation flow

1. Caller creates `conversation.Request`
2. `conversation` normalizes it into pending unified messages
3. `conversation` resolves replay vs native continuation strategy
4. `conversation.MessageProjector` derives the outbound message list
5. `conversation` assembles the final `unified.Request`
6. a unified `client` streams the request
7. `conversation` accumulates streamed assistant output into exact canonical history
8. successful turns are committed; failed/incomplete turns are not committed

## Why Protocol Hooks Are Typed

Protocol-specific customization often needs real wire request types and real protocol event types.

Examples:

- request field normalization after bridging
- provider-specific header mutation based on typed request content
- filtering or patching a specific protocol event shape

Typed hooks make these operations explicit and safe without collapsing the public API into `any`-based extension points.

## Testing Strategy

- Unit tests cover bridges, parsers, typed protocol clients, unified wrappers, conversation sessions, projection logic, and mux logic
- Integration smoke tests live in `integration/`
- Integration tests are runtime-gated so `go test ./...` stays deterministic by default
