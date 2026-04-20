# Changelog

## Unreleased

## v0.3.1 - 2026-04-20

- normalize unified stream usage into structured input/output token breakdowns
- map Anthropic Messages input usage as new + cache read + cache write with canonical total
- map OpenAI Responses and Chat Completions cached and reasoning token details into unified usage
- add invariant and edge-case tests for token usage normalization and clamping
- document optional runtime cost breakdowns alongside canonical token usage

## v0.3.0 - 2026-04-19

- Added a first-class native Ollama integration in `api/ollama` with NDJSON streaming, `/api/chat` support, and `/api/tags` model listing
- Added Ollama request and stream bridges in `adapt`, including native thinking, tool calling, JSON output modes, and Ollama-specific extras
- Added `client.OllamaClient`, mux support for `TargetOllama`, and an opt-in default target resolver heuristic for Ollama model/provider hints
- Added integration smoke coverage for native Ollama responses and tool calling, plus Ollama compatibility coverage for `/v1/responses` tool calling
- Updated integration test ergonomics to use runtime gating via `TEST_INTEGRATION=1`, shared helper logic, and Ollama reachability checks instead of build tags
- Updated README and AGENTS documentation for Ollama support, integration testing, and mux routing

## v0.2.2 - 2026-04-18

- Fixed: Handle thinking parts in responses assistant messages (skip instead of reject, as thinking is controlled via request config)

## v0.2.1 - 2026-07-19

- Fixed: Coerce temperature to 1 when adaptive thinking is enabled on Anthropic Messages API (required by API; non-zero non-1 temperatures are rejected)

## v0.2.0 - 2026-04-17

- Added per-call request and response metadata hooks across the typed protocol clients in `api/messages`, `api/completions`, and `api/responses`
- Added per-call stream options across the unified wrapper clients and mux in `client`
- Added a generic `TypedClient[Req, Ev]` bridge layer in `client` for adapting external request and event models onto the canonical unified stream pipeline
- Added structured HTTP status error helpers and final-body request capture in `internal/protocolcore`
- Added wrapper and bridge tests covering metadata propagation, target selection, and close-time event flushing

## v0.0.1 - 2026-04-17

- Added canonical unified request and stream event types in `api/unified`
- Added request bridges for Messages, Chat Completions, and Responses in `adapt`
- Added stream bridges for Messages, Chat Completions, and Responses in `adapt`
- Added typed protocol clients with typed hooks in `api/messages`, `api/completions`, and `api/responses`
- Added shared HTTP, retry, and SSE execution runtime in `internal/protocolcore`
- Added unified wrapper clients and mux routing in `client`
