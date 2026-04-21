# Reference API Specifications

This directory contains OpenAPI and schema documents used as the **source of truth** when
auditing Go types for spec compliance. These are reference copies — not code-generated or
consumed at runtime.

| File | Source | Used by |
|---|---|---|
| `openai-responses.yaml` | [OpenAI Responses API](https://platform.openai.com/docs/api-reference/responses) | `api/responses/` types |

When adding a new spec for another protocol (e.g. Chat Completions, Anthropic Messages),
place it here and reference it from the corresponding plan in `docs/plans/`.
