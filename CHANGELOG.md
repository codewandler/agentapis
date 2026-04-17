# Changelog

## Unreleased

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
