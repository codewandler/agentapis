# AGENTS

## Scope

This repository is a standalone library for typed model API clients, canonical unified request and event types, and adapters between protocol-specific and unified layers.

## Package Boundaries

- `api/messages`, `api/completions`, `api/responses`: public typed protocol clients and protocol-native types
- `api/unified`: public canonical request and stream event model
- `adapt`: translation between typed protocol types and canonical unified types
- `client`: public unified wrapper clients and mux routing
- `internal/protocolcore`: shared non-public runtime for HTTP, retry, and SSE execution

## Working Rules

- Keep protocol hooks typed at the protocol layer
- Keep unified request and event transforms in `client`
- Keep `adapt` focused on translation, not transport logic
- Prefer the smallest correct change and preserve current package boundaries

## Testing

- Unit tests must pass with `go test ./...`
- Integration tests must be opt-in
- Integration tests must be credential-gated and skip cleanly when env vars are missing
- Put long-running or external-network smoke coverage under `integration/`

## Docs

- Keep docs focused on this repository and its public API
- Update `docs/architecture.md` when package boundaries or stream flow change
- Update `CHANGELOG.md` for user-visible behavior and API changes
