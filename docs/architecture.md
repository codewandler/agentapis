# Architecture

## Overview

This repository has four layers:

1. Typed protocol clients in `api/messages`, `api/completions`, and `api/responses`
2. Canonical request and stream event types in `api/unified`
3. Translation bridges in `adapt`
4. Unified wrapper clients and runtime backend selection in `client`

Shared HTTP, retry, and SSE execution lives in `internal/protocolcore`.

## Protocol Layer

Each protocol package exposes:

- typed request and event types
- a typed streaming client
- typed hooks for request transforms, HTTP mutation, event transforms, and request or response observation

The protocol layer knows about provider wire formats and event shapes. It does not know about the canonical unified model.

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

Inbound bridges:

- `MapMessagesEvent`
- `MapCompletionsEvent`
- `MapResponsesEvent`

`adapt` should remain translation-focused. It should not own transport execution or provider-specific HTTP behavior.

## Unified Wrapper Layer

`client` exposes unified wrappers around concrete protocol clients.

Examples:

- `client.MessagesClient`
- `client.CompletionsClient`
- `client.ResponsesClient`
- `client.MuxClient`

These wrappers:

1. accept `unified.Request`
2. apply unified request transforms
3. build a protocol request via `adapt`
4. stream via a typed protocol client
5. map protocol events back to canonical `unified.StreamEvent`
6. apply unified event transforms

`client.MuxClient` adds runtime backend selection on top of the same flow.

## Shared Runtime

`internal/protocolcore` provides shared support code for:

- HTTP request execution
- response status and typed error parsing
- SSE scanning
- retry transport behavior
- typed request and response metadata passed to protocol hooks

It is intentionally internal so the public API stays typed and package-specific.

## Request Flow

1. Caller creates `unified.Request`
2. `client` applies unified request transforms
3. `adapt` builds a typed protocol request
4. Typed protocol client applies protocol request transforms and header resolution
5. Shared runtime sends HTTP request and streams SSE events
6. Typed protocol parser yields typed protocol events
7. `adapt` maps those events to canonical `unified.StreamEvent`
8. `client` applies unified event transforms and emits final stream items

## Why Protocol Hooks Are Typed

Protocol-specific customization often needs real wire request types and real protocol event types.

Examples:

- request field normalization after bridging
- provider-specific header mutation based on typed request content
- filtering or patching a specific protocol event shape

Typed hooks make these operations explicit and safe without collapsing the public API into `any`-based extension points.

## Testing Strategy

- Unit tests cover bridges, parsers, typed protocol clients, unified wrappers, and mux logic
- Integration smoke tests live in `integration/`
- Integration tests are build-tagged and env-gated so `go test ./...` stays deterministic

The first smoke target is OpenRouter via the Responses API because it exercises a real upstream stream using the richest currently supported event model.
