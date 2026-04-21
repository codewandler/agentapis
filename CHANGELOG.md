# Changelog

## v0.10.0 - 2026-04-21

### Added

- add custom default HTTP client with transparent decompression for brotli, zstd, gzip, and deflate responses
- add `DefaultHTTPClient()` and `DefaultHTTPTransport()` to `protocolcore` for shared HTTP client with decompression support
- add `Accept-Encoding: br, zstd, gzip, deflate` header on outgoing requests when not already set
- add automatic retry with exponential backoff for empty SSE event streams (up to 2 retries)
- add `ErrEmptyEventStream` sentinel error when stream ends without any events after retries
- add `CloneRequestForRetry` helper for safely replaying HTTP requests on retry
- add structured logging at all stream lifecycle points (request, response, events, retries, errors)
- add comprehensive test coverage for empty-stream retry, decompression, and transport wiring

### Changed

- all API clients (`messages`, `completions`, `responses`, `ollama`) now default to `protocolcore.DefaultHTTPClient()` instead of `http.DefaultClient`
- `NewRetryTransport` now defaults to `DefaultHTTPTransport()` (with decompression) instead of `http.DefaultTransport`
- `Content-Length` header is only stripped from responses that are actually decompressed, preserving it for uncompressed responses

### Dependencies

- add `github.com/andybalholm/brotli` v1.2.1
- add `github.com/klauspost/compress` v1.18.5

## v0.9.1 - 2026-04-21

### Fixed

- emit the unified completed event for Anthropic Messages streams on `message_stop` instead of on `message_delta`
- preserve response lifecycle completion signaling on `message_delta` while avoiding duplicate completion emission
- add regression coverage for Messages stream completion ordering in both adapter and client tests

## v0.9.0 - 2026-04-20

### Added

- add `PromptCacheKey` field to Responses API request for server-side prompt caching
- add `PromptCacheKey` to `ResponsesExtras` in unified request types
- add automatic session ID generation in conversation layer for prompt caching
- add `WithSessionID(id)` option for custom session IDs in conversation sessions
- add `Session.SessionID()` getter to retrieve the session's cache key

### Changed

- conversation sessions now automatically set `PromptCacheKey` to the session ID for all Responses API requests

## v0.8.0 - 2026-04-20

### Added

- add structured API error types in `api/messages` with `APIError`, error type constants, and sentinel errors
- add `AsAPIError` helper for unwrapping API errors with type-safe checking
- add rate limit parsing from response headers via `ParseRateLimits`
- add `RateLimits` struct with request/token limits and reset times
- add auto system cache control transform with configurable TTL via `AutoSystemCacheControlWithTTL`
- add `WithRateLimitCallback` session option for observing rate limits per response
- add `SignatureEvent` to conversation events for thinking signature verification

### Fixed

- fix thinking signature tracking in `turnAccumulator` to use wire indices instead of filtered indices
- fix signature verification for interleaved thinking blocks in multi-part responses

## v0.7.1 - 2026-04-20

### Fixed

- make explicit per-request `CacheHint` override always win over policy-derived top-level cache behavior
- clarify cache control precedence and replay-oriented cache policy semantics in README and conversation docs

## v0.6.0 - 2026-04-20

### Added

- add per-request `MaxTokens` and `Temperature` to `conversation.Request`
- add request-builder helpers `Builder.MaxTokens(...)` and `Builder.Temperature(...)`
- add session defaults `WithMaxTokens(...)` and `WithTemperature(...)` for conversation sessions
- add top-level cache hint support to `conversation.Request` and request building

### Changed

- make conversation request building propagate `max_tokens`, `temperature`, and `cache_hint` into the final unified request
- make session defaults apply to conversation request building unless overridden per request
- improve the example REPL with live text/reasoning streaming, detailed usage display, top-level cache hint wiring, and OpenAI model selection via `-m`

## v0.5.0 - 2026-04-20

### Added

- add a compact `conversation.Event` stream surface with concrete event types such as `TextDeltaEvent`, `ReasoningDeltaEvent`, `ToolCallEvent`, `UsageEvent`, `CompletedEvent`, and `ErrorEvent`
- add `Session.RequestUnified(...)` as the explicit richer unified-event escape hatch alongside the agent-facing `Session.Request(...)`
- add top-level cache hint support to `conversation.Request` and the request builder via `Builder.CacheHint(...)`
- add richer REPL behavior with live text and reasoning streaming, usage breakdown display, OpenAI model selection via `-m`, and optional top-level caching

### Changed

- make `Session.Request(...)` the primary smaller agent-facing stream API while reserving `Session.RequestUnified(...)` for rich normalized unified events
- keep a single `UsageEvent` in the conversation event stream carrying the provider-reported usage item for the current request/response
- simplify the conversation API surface to `Request(...)` plus `RequestUnified(...)` for streaming entry points
- update docs and examples to reflect the new conversation event model and REPL behavior

## v0.4.0 - 2026-04-20

### Added

- add a new public `conversation` package with stateful sessions built on unified streaming clients
- add `conversation.Request`, request builder helpers, canonical history tracking, reasoning history, and tool-result round-tripping
- add replay and native `previous_response_id` continuation strategies for conversation sessions
- add exact canonical assistant-part ordering preservation in committed conversation history
- add `conversation.MessageProjector`, `DefaultMessageProjector`, `Session.ProjectMessages(...)`, and `Session.BuildRequest(...)`
- add `api/openrouter.ConversationProjector` for early validation of OpenRouter Responses replay constraints
- add conversation-focused unit and integration coverage for multi-step tool loops, mixed-content ordering, and failure semantics

### Changed

- add release-facing docs and examples for conversation sessions, projection helpers, and OpenRouter-specific replay policy
- standardize response identity propagation across unified stream bridges so conversation/native continuation can rely on `StreamRef.ResponseID`
- make provider-specific conversation replay policy explicit at the service layer instead of burying it in generic session logic
- update repository documentation to cover the conversation package, replay projection, and OpenRouter-specific replay policy

## v0.3.2 - 2026-04-20

- make unified output token accounting non-overlapping: output excludes reasoning and total includes both
- align Responses and Chat Completions adapters with output-minus-reasoning semantics
- keep canonical input invariant as total = cache_read + cache_write + new
- extend invariant tests to cover non-overlapping output token semantics

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
