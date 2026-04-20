# AGENTS

## Scope

This repository is a standalone library for typed model API clients, canonical unified request and event types, adapters between protocol-specific and unified layers, and a stateful conversation/session abstraction built on top of the unified client layer.

## Package Boundaries

- `api/messages`, `api/completions`, `api/responses`, `api/ollama`: public typed protocol clients and protocol-native types
- `api/unified`: public canonical request and stream event model
- `adapt`: translation between typed protocol types and canonical unified types
- `client`: public unified wrapper clients and mux routing
- `conversation`: public stateful session abstraction, canonical history, replay/native strategy selection, and message projection hooks
- `api/openrouter`: OpenRouter-specific helpers and service-level projection policy helpers
- `internal/protocolcore`: shared non-public runtime for HTTP, retry, and SSE execution

## Working Rules

- Keep protocol hooks typed at the protocol layer
- Keep unified request and event transforms in `client`
- Keep `adapt` focused on translation, not transport logic
- Keep `conversation` focused on canonical state, session lifecycle, and generic projection interfaces
- Keep provider/service-specific replay policy outside `conversation` when it is truly vendor-specific
- Prefer the smallest correct change and preserve current package boundaries
- Avoid magic values when there is already a provider/package-level constant that should be reused

## Testing

- Unit tests must pass with `go test ./...`
- Integration tests must be opt-in via runtime gating, not build tags
- Shared integration gating lives in `integration/testing.go`
- Integration tests must skip in `-short` mode
- Integration tests must skip unless `TEST_INTEGRATION=1`
- Provider-specific smoke tests may add additional runtime reachability checks
- Put long-running or external-network smoke coverage under `integration/`
- Prefer `require.*` assertions in integration tests so failures stop immediately at the first invalid assumption

## Docs

- Repository-facing documentation lives in `README.md`, `docs/`, and this file
- Keep docs focused on this repository and its public API
- Update `README.md` when public examples, supported providers, or user-facing session APIs change
- Update `docs/architecture.md` and `docs/conversation.md` when layering or conversation semantics change
- Update `CHANGELOG.md` for user-visible behavior and API changes
