# DESIGN: conversation package

## Goal

Add a dedicated `conversation` package that provides a **stateful conversation abstraction** on top of the unified client layer.

Users of `conversation` should get:

- stateful conversation ergonomics
- unified primitives (`unified.Request`, `unified.Message`, streamed unified events)
- portability across upstream API kinds:
  - responses
  - messages
  - completions
  - ollama
- best-effort transport optimization per backend

The most important semantic rule:

> A conversation is always stateful from the caller's perspective.
>
> Provider-native state is only an optimization.

---

## Non-goals

The initial `conversation` package should **not**:

- add hidden mutable state into low-level `api/*` protocol clients
- depend on raw HTTP/provider details
- require provider-specific wire request handling in the public API
- solve full persistence/serialization in the first iteration
- solve multimodal replay or compaction in the first iteration
- solve backend failover inside a live session in the first iteration

---

## Design principles

### 1. Canonical state is local and unified

The source of truth for a conversation should be a local transcript in unified form.

That transcript should be represented using unified primitives, primarily:

- `[]unified.Message`
- session defaults like model/system/tools
- optional provider-native optimization state such as `previous_response_id`

This gives us:

- backend independence
- replay fallback
- portability
- better debuggability
- the ability to switch or recover transport strategies later

### 2. Native provider state is an optimization, not semantics

If a provider supports server-side continuation, we should use it when beneficial.

Example:

- OpenAI Responses: use `previous_response_id`

If a provider does not support native state:

- replay full transcript

Example:

- OpenRouter Responses: replay transcript
- messages/completions/ollama: replay transcript

### 3. The package belongs above unified streaming clients

The `conversation` package should depend on a unified streaming interface, not protocol-specific clients.

This keeps layering clean:

- `api/*`: protocol wires and parsers
- `client/*`: unified transport adapters
- `conversation/*`: stateful session semantics

### 4. Replay is the correctness path

Replay should be the baseline implementation that always works.

Native continuation should be layered on top as an optimization when all prerequisites hold.

That means:

- replay path must be fully tested and production-usable
- native path should always be able to fall back to replay if state is missing or invalid

### 5. Reasoning is part of conversation state

Reasoning must not be treated as entirely out-of-scope if we want the conversation abstraction to preserve the effective state of a turn.

For the design of `conversation`, reasoning should be treated in two layers:

- **MVP semantic requirement**: reasoning-related state emitted by the backend must not be silently discarded if it is needed for coherent continuation
- **MVP representation requirement**: the initial public API should expose a place for reasoning/instruction-like turn data without forcing callers to construct full `unified.Request` objects

Concretely:

- native responses continuation may preserve provider-side reasoning implicitly
- replay mode should preserve the conversation-visible reasoning/instruction state that can be represented in unified primitives
- encrypted/provider-private reasoning payloads may still remain backend-specific, but the conversation abstraction should be designed so that reasoning can be carried forward later without reshaping the public API

So the refined rule is:

> reasoning is in-scope for the conversation model, even if the first implementation only supports a subset of reasoning carriers.

---

## Proposed package scope

The `conversation` package should own:

- session abstraction
- local transcript/history
- transport strategy selection
- turn request construction
- stream accumulation
- turn commit / rollback behavior
- provider-native continuation hints

The `conversation` package should not own:

- raw provider request serialization
- low-level HTTP behavior
- protocol event parsing
- provider-specific auth/base URL concerns

---

## Public API: refined MVP

The package should expose **one primary execution method** for session-level interaction:

```go
func (s *Session) Request(ctx context.Context, req Request) (<-chan client.StreamResult, error)
```

Ergonomics should come from a builder, not from multiple overlapping execution helpers.

### Why not `Send` / `Stream` / `RequestText`

If multiple methods all:

- submit the next turn
- return the same streamed result type
- operate on the same session state

then the extra methods create confusion rather than clarity.

So the refined rule is:

- **`Request(...)` is the one execution method**
- builders provide ergonomic construction of request values
- convenience execution helpers like `SendText` / `RequestText` are unnecessary if the builder is good

### Backend dependency

The package should depend on a narrow interface over unified streaming:

```go
type Streamer interface {
    Stream(ctx context.Context, req unified.Request) (<-chan client.StreamResult, error)
}
```

### Core public types

```go
type Session struct { ... }

type Request struct {
    Model        string
    Instructions []string
    Tools        []unified.Tool
    ToolChoice   unified.ToolChoice
    Inputs       []Input
}

type Input struct {
    Role       unified.Role
    Text       string
    ToolResult *ToolResult
}

type ToolResult struct {
    ToolCallID string
    Output     string
}
```

This `conversation.Request` is intentionally narrower than `unified.Request`.

It models the caller-facing input for the next conversation step, while the session remains responsible for:

- carrying history
- choosing replay vs native continuation
- constructing the final `unified.Request`

### Constructor

```go
func New(streamer Streamer, opts ...Option) *Session
```

### Methods: refined MVP set

```go
func (s *Session) Request(ctx context.Context, req Request) (<-chan client.StreamResult, error)
func (s *Session) History() []unified.Message
func (s *Session) Reset()
```

---

## Builder API

### Why a builder belongs here

The builder should provide the ergonomic layer that earlier drafts tried to provide with helper execution methods.

That keeps execution API minimal while still making common request construction pleasant.

### Recommendation

Keep the request struct public and directly usable, but also provide a fluent builder.

That means:

- raw struct literals remain valid and idiomatic
- builder is convenience, not mandatory ceremony

### Builder sketch

```go
type Builder struct { ... }

func NewRequest() *Builder
```

### Builder methods: likely MVP set

```go
func (b *Builder) Model(string) *Builder
func (b *Builder) Instructions(...string) *Builder
func (b *Builder) Tools([]unified.Tool) *Builder
func (b *Builder) ToolChoice(unified.ToolChoice) *Builder
func (b *Builder) User(text string) *Builder
func (b *Builder) ToolResult(callID, output string) *Builder
func (b *Builder) Build() Request
```

Optional shorthand later:

```go
func R() *Builder
```

### Example usage

```go
req := conversation.NewRequest().
    Model("gpt-4o-mini").
    Instructions("Answer tersely.", "Prefer bullet points.").
    User("Summarize the latest result.").
    Build()

stream, err := sess.Request(ctx, req)
```

And tool result feedback:

```go
req := conversation.NewRequest().
    ToolResult("call_1", `{"status":"ok"}`).
    User("Continue").
    Build()
```

---

## `conversation.Request`: intent and semantics

This type models the caller-side payload for one new step in an ongoing conversation.

### Why not expose `unified.Request` directly

Because `unified.Request` mixes:

- transport-wide configuration
- provider-targetable options
- full message history
- per-turn content

The `conversation` package should expose a smaller abstraction that represents only what a caller usually wants to add or override for the next turn.

### Request fields

```go
type Request struct {
    Model        string
    Instructions []string
    Tools        []unified.Tool
    ToolChoice   unified.ToolChoice
    Inputs       []Input
}
```

### Field semantics

#### `Model`

Optional per-request override. If empty, session default model is used.

#### `Instructions`

Per-request instruction overlay.

- ordered
- prepend-only relative to `Inputs`
- may contain multiple strings

These are not session-global defaults; they apply to this request/turn only.

#### `Tools`

Optional per-request tool override.

For MVP, recommended rule:

- if non-empty, request tools replace session default tools
- if empty, session default tools are used

#### `ToolChoice`

Optional per-request tool choice override using the unified tool-choice abstraction.

#### `Inputs`

Ordered request-local conversational inputs for the next step.

---

## `conversation.Input`: intent and semantics

```go
type Input struct {
    Role       unified.Role
    Text       string
    ToolResult *ToolResult
}
```

### MVP supported shapes

Recommended valid combinations:

- `Role=user` with `Text`
- `Role=developer` with `Text`
- `Role=system` with `Text` if needed
- `Role=tool` with `ToolResult`

Recommended invalid combinations in MVP:

- empty input
- `Text` and `ToolResult` set simultaneously
- `Role=tool` with plain `Text` only

### Why `Text` instead of `Content`

`Text` is clearer for MVP because the initial conversation package is text-centric.

If multimodal inputs are added later, the API can evolve intentionally rather than pretending to support arbitrary content from day one.

---

## Reasoning: refined scope

Reasoning should no longer be treated as a blanket non-goal.

### Refined MVP position

- reasoning is **in-scope semantically**
- full provider-specific reasoning replay is **not required** for the first implementation
- the public API and internal transcript model should leave room for reasoning-preserving evolution

### What this means concretely

#### For native continuation backends

If the backend preserves reasoning via native state, the conversation package should not interfere with that. This is one reason native continuation is valuable.

#### For replay backends

The conversation package should preserve whatever reasoning/instruction state is representable in unified transcript form.

At minimum, MVP should preserve:

- system defaults
- developer defaults
- per-request `Instructions`
- assistant visible outputs

If the unified layer already exposes reasoning summary/text in a reusable form, we should strongly consider recording it in transcript-adjacent state rather than discarding it.

### Design consequence

The transcript model may need to evolve from:

- only `[]unified.Message`

to:

- `[]unified.Message` plus turn metadata
- or a richer internal turn record

Recommendation:

- keep exported `History()` as `[]unified.Message` for now
- allow internal turn state to carry additional reasoning/instruction metadata if needed

## Session configuration

The session should support defaults configured at creation time.

### MVP options

```go
WithModel(string)
WithSystem(string)
WithDeveloper(string)
WithTools([]unified.Tool)
WithCapabilities(Capabilities)
WithStrategy(Strategy)
```

### Likely later options

```go
WithInitialHistory([]unified.Message)
WithMaxHistoryMessages(int)
WithSessionID(string)
WithPersistence(Store)
WithCompactionPolicy(Policy)
```

### Option interaction rules

#### `WithStrategy(...)`

Explicit strategy should override auto-resolution.

#### `WithCapabilities(...)`

Capabilities should be used only when strategy is `StrategyAuto`.

#### `WithSystem(...)` and `WithDeveloper(...)`

These should become default leading messages for outbound request construction, but should **not** appear in `History()` unless they were explicitly sent as turns later.

#### `WithModel(...)`

MVP should require a model either:

- from `WithModel(...)`, or
- from a later richer per-turn API

Since `Request(...)` does not require callers to provide a full `unified.Request`, MVP should require a model either from session defaults or from the conversation request itself.

Recommendation:

- if neither session defaults nor request override provide a model, `Request(...)` should return a clear error

---

## Internal model

### Session shape

```go
type Session struct {
    streamer Streamer

    defaults sessionDefaults

    mu      sync.Mutex
    history []unified.Message

    strategy Strategy
    caps     Capabilities

    native nativeState
}
```

### Session defaults

```go
type sessionDefaults struct {
    model     string
    tools     []unified.Tool
    system    *unified.Message
    developer *unified.Message
}
```

### Provider-native optimization state

```go
type nativeState struct {
    lastResponseID string
}
```

### Concurrency rule

`Session` should be safe for concurrent **reads**, but MVP should document that concurrent overlapping `Send` calls on the same session are not supported.

Recommendation:

- serialize turns per session with the internal mutex
- if a new send starts while another send is active, either block or error

Preferred MVP behavior:

- block only around state mutation setup/commit
- do not hold mutex for the entire remote stream lifetime if avoidable
- instead use an internal per-turn guard (`inFlight bool`) to reject overlapping sends with a deterministic error

This keeps semantics clear: a session is single-threaded by turn.

---

## Strategy model

### Strategy enum

```go
type Strategy int

const (
    StrategyAuto Strategy = iota
    StrategyReplay
    StrategyResponsesPreviousResponseID
)
```

### Capabilities

```go
type Capabilities struct {
    SupportsResponsesPreviousResponseID bool
}
```

This is enough for the first useful optimization.

Later we can grow capabilities for:

- tool continuation behavior
- compaction support
- reasoning continuity
- backend failover compatibility

---

## Strategy resolution

### MVP recommendation

Start with explicit configuration via `WithCapabilities(...)` or `WithStrategy(...)`.

Avoid automatic backend inspection in the first version.

This keeps `conversation` decoupled from concrete client implementations.

### Resolution function

Internally, define something like:

```go
func resolveStrategy(explicit Strategy, caps Capabilities) Strategy
```

Recommended logic:

- if explicit == `StrategyReplay` -> replay
- if explicit == `StrategyResponsesPreviousResponseID` -> native responses previous-id strategy
- if explicit == `StrategyAuto`:
  - if `caps.SupportsResponsesPreviousResponseID` -> native responses previous-id strategy
  - else -> replay

### Strategy examples

- OpenAI Responses client -> `StrategyResponsesPreviousResponseID`
- OpenRouter Responses client -> `StrategyReplay`
- Messages client -> `StrategyReplay`
- Completions client -> `StrategyReplay`
- Ollama client -> `StrategyReplay`

### Future enhancement

Allow the underlying streamer to optionally expose capabilities:

```go
type CapabilityProvider interface {
    ConversationCapabilities() conversation.Capabilities
}
```

If implemented, `conversation.New(...)` can auto-resolve defaults.

---

## Conversation state model

### Internal representation vs exported history

The exported history API can remain:

- `[]unified.Message`

But internally, the session may need slightly richer state per turn, especially once reasoning support grows.

Recommended approach:

- MVP internal state can remain mostly message-based
- but code structure should allow later introduction of an internal `turnRecord` carrying extra metadata without breaking the public API

For example:

```go
type turnRecord struct {
    Messages       []unified.Message
    LastResponseID string
    // later: reasoning summaries, provider-native metadata, etc.
}
```

This is a design guardrail more than an immediate implementation requirement.


### Canonical transcript

Canonical session state should be represented as:

- defaults (system/developer/model/tools)
- committed conversation history as `[]unified.Message`

### Why keep defaults separate from history

This is recommended for MVP.

Benefits:

- `History()` returns actual conversation turns only
- defaults remain configuration, not transcript noise
- request construction is explicit

### First supported turn kinds

For MVP, support these reliably:

- user text turns
- assistant text turns
- system/developer defaults

Tool support can come immediately after, but should not block the first session implementation.

---

## Request construction rules

### Mapping `conversation.Request` into pending unified turn state

Before building the outbound request, the session should normalize `conversation.Request` into one or more pending unified messages.

Recommended MVP mapping:

- each `Instructions[i]` -> a pending `developer` message for this request
- each `Inputs[i]` -> one pending unified message based on role and payload

This means one `conversation.Request` may normalize into multiple unified messages internally.

Example:

```go
Request{
    Instructions: []string{"Answer tersely."},
    Inputs: []Input{{Role: unified.RoleUser, Text: "Summarize this."}},
}
```

normalizes into:

1. developer message: `Answer tersely.`
2. user message: `Summarize this.`

### Why map `Instructions` to developer message

For the unified layer, a per-request instruction overlay is most naturally represented as one or more `developer` messages rather than by mutating session-global defaults.

This preserves the distinction between:

- persistent defaults configured at session creation
- transient request-local instructions supplied by the caller

### Commit rule for normalized pending input

If a turn succeeds, all normalized pending messages derived from the input `conversation.Request` should be committed to history in order.

So a request with both instructions and user inputs commits those pending messages before the assistant reply is appended.


### Outbound request builder

Define an internal helper roughly like:

```go
func (s *Session) buildRequest(pending unified.Message) (unified.Request, Strategy, error)
```

This should:

- resolve transport strategy
- merge defaults
- merge committed history as needed
- include pending turn
- inject native continuation state when appropriate

### Replay strategy

For `StrategyReplay`, outbound request should contain:

- session defaults
- full committed history
- pending user turn

Suggested construction order:

1. optional system default
2. optional developer default
3. committed history
4. pending turn

### Native responses continuation strategy

For `StrategyResponsesPreviousResponseID`, outbound request should contain:

- model from defaults
- tools from defaults
- only the new incremental turn in `Messages`
- `unified.Request.Extras.Responses.PreviousResponseID = lastResponseID`

Important:

- do **not** duplicate prior transcript in the outbound message list when using native continuation
- still keep the full local transcript internally

### Native strategy fallback rule

If native continuation is configured but unavailable for a specific turn, fall back to replay.

Fallback conditions:

- no known `lastResponseID`
- invalid or empty model/defaults
- unsupported request shape for native path
- later: provider-native continuation error if retry policy is introduced

For MVP, fallback should happen at request-build time only, not via automatic network retry.

---

## Turn execution lifecycle

### Request flow

1. Build pending user turn
2. Resolve strategy
3. Build outbound unified request
4. Start upstream stream
5. Proxy stream items to caller while accumulating turn state internally
6. If stream completes successfully:
   - commit pending user turn
   - commit assistant/tool outputs
   - update provider-native optimization state
7. If stream fails or is aborted before successful completion:
   - do not commit the turn

### Commit rule

This is important:

> Only completed successful turns should mutate conversation history.

This avoids corrupting session state when a stream errors midway.

### Error rule

If `streamer.Stream(...)` returns an error before a stream is created:

- nothing is committed
- in-flight state is cleared
- error is returned directly

If a stream item contains an error:

- pass it through to caller
- do not commit at end of stream

---

## Stream accumulation

The package needs an internal accumulator that can reconstruct the assistant side of a turn from unified streaming events.

### Proposed internal type: MVP

```go
type turnAccumulator struct {
    assistantText  strings.Builder
    lastResponseID string
    sawCompleted   bool
}
```

### Accumulator responsibilities

While consuming `client.StreamResult`, capture:

- assistant text deltas/content
- a stable response id when present
- completion state

### Preferred event sources

MVP should accept text from whichever unified event forms are already stable and emitted across backends, likely in this order:

1. `StreamEventContentDelta` / streamed text payload
2. `StreamEventContent` / final content payload
3. any completed event metadata carrying response id

Implementation should avoid double-appending duplicated final text when both delta and final content are emitted.

### Response id extraction

Accumulator should capture response id from the most stable unified location available.

Recommendation:

- prefer `item.Event` fields that include `StreamRef.ResponseID`
- if present in lifecycle/completed/started metadata, capture there too
- keep the first non-empty stable response id seen unless a stronger canonical source is later defined

### Commit output

At the end of a successful turn, accumulator should be able to produce:

- assistant `unified.Message` if there was assistant output
- updated native state (`lastResponseID`)

### Empty assistant turn rule

If a successful stream completes without assistant text:

- do not append an assistant text message in MVP
- still update `lastResponseID` if available

This leaves room for tool-only turns later.

---

## Unified event assumptions and preparatory checks

The conversation layer depends on unified events exposing enough information for:

- text reconstruction
- completion detection
- response id extraction

Before implementation, verify the unified stream surface for:

1. stable text delta event type(s)
2. stable final completion event type
3. stable response id carrier(s)

If needed, perform a small preparatory change in the unified client layer so that response identity is preserved consistently in one place.

Recommendation:

- standardize on `StreamRef.ResponseID` or another single canonical field before relying on it heavily in `conversation`

---

## MVP scope

### Included

- new `conversation` package
- in-memory `Session`
- `SendText`
- `Send`
- `History`
- `Reset`
- replay strategy
- optional native responses continuation optimization
- text-only assistant accumulation

### Explicitly excluded initially

- persistence/store interface
- multimodal replay
- annotations replay
- reasoning replay/encrypted reasoning handling
- compaction/truncation policy
- backend switching within a live session
- automatic retry across strategies after network send begins
- sophisticated branch/merge management

---

## Tool support plan

Tool support should be Phase 2, immediately after MVP.

### Why not block MVP on tools

A text-only MVP gives immediate value and validates:

- package shape
- state model
- strategy model
- turn commit semantics
- replay/native optimization split

### Phase 2 tool support should add

- assistant tool call accumulation
- tool result turns in history
- replay of tool call/result chain in transcript

This may require a richer accumulator and commit model, for example:

```go
type turnAccumulator struct {
    assistantParts []unified.Part
    lastResponseID string
    sawCompleted   bool
}
```

---

## Package structure proposal

```text
conversation/
  session.go
  options.go
  strategy.go
  accumulator.go
  types.go
  session_test.go
  strategy_test.go
```

Possible later additions:

```text
  store.go
  compact.go
  integration_test_helpers.go
```

### File responsibilities

- `types.go`: exported interfaces/types/enums
- `options.go`: options/config application
- `strategy.go`: strategy resolution logic
- `session.go`: session lifecycle, request building, stream orchestration
- `accumulator.go`: stream-to-turn reconstruction

---

## Testing plan

### 1. Unit tests

Focus:

- strategy resolution
- replay request construction
- native request construction
- history commit on success
- no commit on failure
- reset behavior
- history copying / immutability guarantees
- missing-model error behavior
- native fallback to replay when no `lastResponseID`

### 2. Fake-stream integration tests

Use a fake streamer to assert transport behavior.

Recommended fake streamer behavior:

- record every outbound `unified.Request`
- emit a configurable stream of `client.StreamResult`

Scenarios:

- replay mode sends full history on turn 2
- native mode sends only incremental turn with previous response id
- response id updates after completed turn
- failed turn does not mutate history
- empty assistant output still preserves last response id if present

### 3. Real provider integration tests

Once package exists, add conversation-level real integration tests:

- OpenAI backend:
  - session should use native continuation path
  - user experiences stateful continuation
- OpenRouter backend:
  - session should use replay path
  - user still experiences stateful continuation

This is one of the biggest value proofs of the package.

---

## Real-world behavior expectations

### OpenAI + Responses

- internal strategy: native continuation
- transport efficiency: incremental turn + `previous_response_id`
- user experience: stateful conversation

### OpenRouter + Responses

- internal strategy: replay
- transport efficiency: full history replay
- user experience: stateful conversation

### Messages / Completions / Ollama

- internal strategy: replay
- user experience: stateful conversation

---

## Implementation milestones

### Milestone 0: prerequisite check

Before implementing `conversation`, verify unified stream consistency for:

- text delta/content events
- completed events
- response id propagation

If needed, land a small normalization patch first.

### Milestone 1: Replay-only text session

Deliver:

- `conversation.New(...)`
- in-memory `Session`
- `SendText`
- `Send`
- `History`
- `Reset`
- replay strategy only
- assistant text accumulation from unified stream
- unit tests + fake-stream tests

This already delivers stateful UX across all backends.

### Milestone 2: Native responses optimization

Deliver:

- `Capabilities`
- `Strategy`
- `StrategyAuto`
- `PreviousResponseID` injection via unified request extras
- response id capture from stream
- replay fallback retained
- request-construction tests

### Milestone 3: Real provider validation

Deliver conversation-level integration tests proving same semantics with different strategies:

- OpenAI: native continuation path
- OpenRouter: replay path

### Milestone 4: Tool support

Deliver:

- tool call accumulation
- tool result replay
- transcript fidelity improvements

### Milestone 5: Persistence / compaction

Optional later.

---

## Open questions

These should be answered before or during implementation:

### 1. Request API shape

Resolved.

MVP API is:

- `Session.Request(ctx, conversation.Request)`
- builder-based ergonomics on top

No separate `Send`, `Stream(message)`, or `RequestText` helper is needed.

### 2. How should capabilities be provided?

Decision for MVP:

- use `WithCapabilities(...)`

Do not require backend auto-detection in the first implementation.

### 3. What unified event field should carry response identity for a completed turn?

This is the main unresolved technical item.

Current design assumption:

- the conversation layer needs a stable response id carrier
- especially for native responses continuation via `previous_response_id`

What we know from the current code:

- `unified.StreamRef` already has `ResponseID string`
- many streamed content/lifecycle events already populate `StreamRef.ResponseID`
- the Responses adapter already emits `Lifecycle.Ref.ResponseID` on `response.*` lifecycle/completed paths
- however, `unified.Completed` itself does not carry response identity
- `unified.Started` uses `RequestID`, and for some backends that currently contains a response-like identifier, but that is not a sufficiently explicit contract for conversation state

Refined design decision:

> The canonical response identity carrier for streaming conversation state should be `StreamRef.ResponseID`.

### Required contract

For any backend that can provide response identity, the unified stream should ensure:

- the final response/turn completion path exposes `Lifecycle.Ref.ResponseID`
- response-scoped lifecycle events expose `Lifecycle.Ref.ResponseID`
- content/tool events may also expose `Ref.ResponseID`, but conversation code should not depend solely on those

### Why not put response id on `Completed`

`Completed` models stop/completion semantics, not object identity.

Identity is already modeled elsewhere via `StreamRef`, so the cleaner design is:

- keep `Completed` focused on stop reason
- require the corresponding response-scoped `Lifecycle.Ref.ResponseID` to be present alongside it on completion events

This matches the current Responses adapter shape and avoids introducing redundant identity fields.

### Conversation accumulator rule

The conversation accumulator should capture response identity in this order of preference:

1. `item.Event.Lifecycle.Ref.ResponseID` on response-scoped lifecycle/completed events
2. `item.Event.ContentDelta.Ref.ResponseID` or `item.Event.StreamContent.Ref.ResponseID` when present
3. other unified event refs carrying `ResponseID`

For completion of a turn, the canonical source should be:

- `StreamEventCompleted` with `Lifecycle.Scope == LifecycleScopeResponse`
- and `Lifecycle.Ref.ResponseID != ""`

### Required action

Before or during `conversation` implementation:

- audit all unified adapters and ensure response-completed events consistently carry `Lifecycle.Ref.ResponseID`
- stop relying on implicit semantics of `Started.RequestID` for response continuation purposes
- document `Lifecycle.Ref.ResponseID` as the canonical response-id contract used by the conversation package

This should be treated as a prerequisite or part of PR 0.

### 4. Should empty assistant turns be committed?

Decision for MVP:

- no assistant message if stream produced no assistant text
- still update native state if response id is known

This is acceptable for the text-first MVP and leaves room for tool-only turns in Phase 2.

### 5. Should concurrent requests on one session be allowed?

Decision for MVP:

- no
- guard the session with a mutex and reject or serialize overlapping turn state transitions in the simplest correct way

Implementation preference:

- keep session turn execution single-threaded
- use a mutex-backed in-flight guard
- do not complicate MVP with concurrent turn semantics

## Recommended first PR breakdown

### PR 0 (if needed)

- normalize unified response id propagation / event consistency

### PR 1

- scaffold `conversation` package
- define `Session`, `Streamer`, options, strategy/capability types
- implement replay-only text MVP
- add unit tests and fake-stream tests

### PR 2

- add native responses optimization
- add response id capture
- add replay/native request selection tests

### PR 3

- add real integration tests for conversation package:
  - OpenAI stateful via native continuation
  - OpenRouter stateful via replay

### PR 4

- add tool support

---

## Bottom line

The `conversation` package should provide a backend-agnostic, stateful session abstraction using unified primitives.

Implementation strategy:

- keep canonical state locally as unified transcript
- use replay as the universal correctness path
- use provider-native continuation only as an optimization
- start with a text-only in-memory MVP
- then add `responses` native continuation and real provider validation

That gives the repo a strong, ergonomic, and portable conversation layer without leaking provider quirks into the user-facing API.


---

## Implementation notes: concrete internal API sketch

This section is intentionally more concrete than the earlier design sections. It is not final public API commitment, but it should be close enough to guide implementation.

### Exported types: first-pass sketch

```go
type Streamer interface {
    Stream(ctx context.Context, req unified.Request) (<-chan client.StreamResult, error)
}

type Strategy int

const (
    StrategyAuto Strategy = iota
    StrategyReplay
    StrategyResponsesPreviousResponseID
)

type Capabilities struct {
    SupportsResponsesPreviousResponseID bool
}

type Session struct {
    streamer Streamer

    defaults sessionDefaults
    strategy Strategy
    caps     Capabilities

    mu       sync.Mutex
    history  []unified.Message
    native   nativeState
    inFlight bool
}

type Request struct {
    Model        string
    Instructions []string
    Tools        []unified.Tool
    ToolChoice   unified.ToolChoice
    Inputs       []Input
}

type Input struct {
    Role       unified.Role
    Text       string
    ToolResult *ToolResult
}

type ToolResult struct {
    ToolCallID string
    Output     string
}
```

### Internal types: first-pass sketch

```go
type sessionDefaults struct {
    model     string
    tools     []unified.Tool
    system    *unified.Message
    developer *unified.Message
}

type nativeState struct {
    lastResponseID string
}

type turnPlan struct {
    strategy Strategy
    pending  []unified.Message
    request  unified.Request
}

type turnResult struct {
    assistant      *unified.Message
    lastResponseID string
    committed      bool
}
```

### Recommended internal helper split

Keep `session.go` relatively thin by splitting the lifecycle into a few focused helpers.

```go
func (s *Session) Request(ctx context.Context, req Request) (<-chan client.StreamResult, error)

func (s *Session) beginTurn(req Request) (turnPlan, error)
func (s *Session) buildTurnPlanLocked(req Request) (turnPlan, error)
func (s *Session) applyTurnResultLocked(plan turnPlan, result turnResult)
func (s *Session) endTurnLocked()
```

This avoids one giant method mixing validation, request building, stream forwarding, accumulation, commit, and cleanup.

---

## Request builder: more exact rules

### Request fields controlled by the session in MVP

The session should control these fields directly:

- `Model`
- `Tools`
- `Messages`
- `Extras.Responses.PreviousResponseID` when native continuation is used

The session should not invent unrelated extras in MVP.

### Replay request construction pseudocode

```go
req := unified.Request{
    Model: defaults.model,
    Tools: copyTools(defaults.tools),
}
req.Messages = append(req.Messages, optionalSystem(defaults.system)...) 
req.Messages = append(req.Messages, optionalDeveloper(defaults.developer)...) 
req.Messages = append(req.Messages, copyMessages(history)...) 
req.Messages = append(req.Messages, copyMessage(pending))
```

### Native continuation request construction pseudocode

```go
req := unified.Request{
    Model: defaults.model,
    Tools: copyTools(defaults.tools),
    Messages: []unified.Message{copyMessage(pending)},
}
ensureResponsesExtras(&req).PreviousResponseID = native.lastResponseID
```

### Why defaults still go out in native mode

For MVP, we should keep defaults such as model and tools explicit on every request even in native mode.

For `system` / `developer` defaults there is one important decision:

#### Recommended MVP behavior

When using native `previous_response_id`, do **not** resend system/developer defaults on later turns if they are already part of the logical conversation context.

Reason:

- replay mode needs them every time because it rebuilds full context
- native mode should avoid duplicating instruction context once the provider has it

That means:

- first native turn with no `lastResponseID` will naturally go through replay or incremental-first-turn path
- later native turns send only pending turn + previous response id

This should be explicitly covered by tests.

---

## First-turn behavior in native strategy

This needs to be explicit, because `StrategyResponsesPreviousResponseID` cannot work until a prior response exists.

### Rule

If strategy resolves to `StrategyResponsesPreviousResponseID` but `lastResponseID == ""`, the session should:

- build the request using replay/full-context semantics for the first turn
- still label the effective strategy for that request as replay
- if the turn completes successfully and yields a response id, subsequent turns may use native continuation

In other words:

- native continuation is never used on the very first turn
- the session transitions into native continuation after the first successful turn with a response id

---

## Commit semantics: exact rules

### On successful completed stream

Commit in this order under lock:

1. append all normalized pending messages to history in order
2. if accumulator produced assistant output, append assistant message
3. if accumulator produced `lastResponseID`, update native state
4. clear in-flight marker

### On failed stream

Under lock:

- do not append pending message
- do not append assistant message
- do not update native state from partial stream output
- clear in-flight marker

### On context cancellation

Treat the turn as failed unless a completed event was already observed and the result is clearly complete.

Recommended MVP simplification:

- if any stream item carries error, or stream ends without completed marker, do not commit

This is conservative and easier to reason about.

---

## In-flight lifecycle: exact behavior

### Why explicit in-flight state is needed

Without it, overlapping sends on the same session can race and corrupt history ordering.

### Recommended rule

A session is single-turn-at-a-time.

If `Request(...)` is called while another turn is active:

- return a deterministic exported error, e.g. `ErrTurnInProgress`

This is better than blocking indefinitely and easier for callers to reason about.

### Exported error sketch

```go
var ErrTurnInProgress = errors.New("conversation: turn already in progress")
var ErrModelRequired = errors.New("conversation: model is required")
```

Potentially also:

```go
var ErrUnsupportedStrategy = errors.New("conversation: unsupported strategy")
```

But avoid exporting too many errors unless truly needed.

---

## Accumulator refinement: exact event mapping goal

The accumulator should be designed around unified events, not provider-specific events.

### MVP event mapping target

The accumulator should understand at least:

- text delta events
- text content/final content events
- completed events
- lifecycle/started/content events that may carry response id in refs

### Safe accumulation policy

A practical approach for MVP:

1. Prefer deltas for assistant text assembly
2. If no deltas were seen, fall back to final content payloads
3. If both deltas and final content are seen, avoid appending duplicate final text

This keeps behavior robust across heterogeneous clients.

### Suggested internal accumulator fields

```go
type turnAccumulator struct {
    textDeltaSeen   bool
    assistantText   strings.Builder
    fallbackText    strings.Builder
    lastResponseID  string
    sawCompleted    bool
}
```

Then final assistant text can be resolved like:

- if `textDeltaSeen`, use `assistantText`
- else use `fallbackText`

### Response id capture policy

Whenever an event exposes a non-empty `ResponseID`, call:

```go
func (a *turnAccumulator) rememberResponseID(id string)
```

Rule:

- first non-empty id wins in MVP
- if a later different id appears, ignore it for now and possibly log/debug later

This avoids instability from provider-specific duplicate carriers.

---

## Assistant message reconstruction rules

### MVP reconstructed assistant message

When accumulator has final text, build:

```go
unified.Message{
    Role: unified.RoleAssistant,
    Parts: []unified.Part{{
        Type: unified.PartTypeText,
        Text: finalText,
    }},
}
```

### Empty text case

If final text is empty:

- no assistant message is appended in MVP

This is acceptable for text-only MVP and keeps room for tool-only turns later.

---

## Copying and immutability rules

The session should never expose internal slices directly.

### Required helper behavior

Introduce internal copy helpers:

```go
func cloneMessage(m unified.Message) unified.Message
func cloneMessages(in []unified.Message) []unified.Message
func cloneTools(in []unified.Tool) []unified.Tool
```

### MVP copy depth expectation

At minimum clone:

- message slice
- parts slice
- tools slice

If a part contains maps, full deep-copy may not be strictly required on day one, but this should be noted as technical debt if deferred.

Recommendation:

- deep-copy `Tool.Args` and tool parameter maps only when tool support is added

---

## Stream wrapper behavior

`Request(...)` should return a wrapped stream rather than consuming the stream eagerly before returning.

### Wrapper responsibilities

The wrapper goroutine should:

- forward every upstream item to the caller unchanged
- feed items into accumulator
- track whether any stream error occurred
- after upstream channel closes:
  - decide commit vs rollback
  - update session state under lock
  - clear in-flight flag
  - close output channel

### Important invariant

The caller should observe the same streamed events regardless of whether the session later commits or rolls back the turn.

The session’s commit logic is internal bookkeeping only.

---

## Session state transition examples

### Replay-only backend example

Initial state:

- history = []
- lastResponseID = ""

Turn 1:

- outbound request = defaults + user msg
- stream completes with assistant text
- commit user + assistant
- response id may be ignored unless useful later

Turn 2:

- outbound request = defaults + prior history + new user msg
- stream completes
- commit new user + assistant

### Native responses backend example

Initial state:

- history = []
- lastResponseID = ""

Turn 1:

- strategy resolves native, but no prior response id exists
- effective request build falls back to replay/full-context first turn
- stream completes with response id `resp_1`
- commit user + assistant
- lastResponseID = `resp_1`

Turn 2:

- outbound request = pending user msg + previous_response_id=`resp_1`
- stream completes with response id `resp_2`
- commit user + assistant
- lastResponseID = `resp_2`

---

## Proposed tests: more exact inventory

### `strategy_test.go`

- `TestResolveStrategy_AutoFallsBackToReplay`
- `TestResolveStrategy_AutoUsesResponsesPreviousIDWhenSupported`
- `TestResolveStrategy_ExplicitReplayWins`
- `TestResolveStrategy_ExplicitNativeWins`

### `session_test.go`

- `TestSessionRequestRequiresModel`
- `TestSessionReplayFirstTurnCommitsUserAndAssistant`
- `TestSessionReplaySecondTurnReplaysFullHistory`
- `TestSessionNativeStrategyFirstTurnFallsBackUntilResponseIDExists`
- `TestSessionNativeStrategySecondTurnUsesPreviousResponseID`
- `TestSessionFailedTurnDoesNotCommit`
- `TestSessionStreamWithoutCompletedEventDoesNotCommit`
- `TestSessionResetClearsHistoryAndNativeStateButKeepsDefaults`
- `TestSessionHistoryReturnsCopy`
- `TestSessionRejectsOverlappingTurns`
- `TestSessionEmptyAssistantOutputDoesNotAppendAssistantMessage`

### Fake-stream test fixtures

Build a fake streamer with:

- recorded requests
- per-call scripted output stream

This will make strategy/request-shape assertions very direct.

---

## Suggested preparatory code audit before implementation

Before writing `conversation`, inspect these existing surfaces:

1. unified stream event types carrying text deltas/content
2. unified stream event types carrying response ids
3. how responses/openai/openrouter currently populate those fields
4. whether tool call events already produce enough unified structure for Phase 2

This audit should answer one concrete question:

> Which exact unified event fields should `turnAccumulator` consume in MVP?

If the answer is currently ambiguous, fix that first.

---

## Refined first-PR goal

The very first `conversation` PR should aim for something extremely narrow but complete:

### Deliverable

- `conversation.Session`
- in-memory state only
- `Request(Request)`
- request builder
- replay strategy only
- assistant text accumulation
- strict no-overlap turns
- no persistence
- no tool support

### Why this cut is strong

It proves:

- package placement is right
- session semantics are right
- state model is right
- stream wrapping/commit logic is right

Then native optimization becomes a contained follow-up rather than part of the initial complexity spike.

---

## Refined second-PR goal

The second PR should focus only on the optimization layer:

- capabilities
- strategy auto-resolution
- previous_response_id injection
- response id capture
- native first-turn fallback behavior
- tests proving request shape differences

This separation will likely keep review much simpler.

---

## Unified stream audit: accumulator inputs and response-id normalization

This section records the current audit of unified stream adapters and turns the earlier response-id discussion into concrete implementation guidance.

### Audit summary

#### Responses adapter

Current status: **good baseline**

Observed behavior:

- response lifecycle events already carry `Lifecycle.Ref.ResponseID`
- completed response events already carry:
  - `Type == StreamEventCompleted`
  - `Lifecycle.Scope == LifecycleScopeResponse`
  - `Lifecycle.Ref.ResponseID`
- content delta/content events also carry response-scoped refs in many cases
- reasoning text/summary events are already mapped into unified content events

Conclusion:

- `adapt/responses_stream_bridge.go` is already close to the contract needed by `conversation`
- this adapter should be treated as the canonical target shape for response-id propagation

#### Ollama adapter

Current status: **partially sufficient, needs normalization**

Observed behavior:

- text delta events carry `ContentDelta.Ref.ResponseID`
- a stable synthetic response id is generated by the mapper
- `Started.RequestID` is set to that synthetic response id
- **completed events currently do not carry `Lifecycle.Ref.ResponseID`**

Conclusion:

- conversation code could recover a response id from content events for Ollama today
- but this is not the desired canonical contract
- `adapt/ollama_stream_bridge.go` should be normalized so completed events also carry response-scoped lifecycle with `Ref.ResponseID`

#### Messages adapter

Current status: **needs normalization**

Observed behavior:

- started event uses `Started.RequestID = e.Message.ID`
- text/reasoning/tool events mostly use segment/item refs only
- completed events currently set `Completed.StopReason` and usage
- **completed events do not currently carry response-scoped lifecycle or response id**

Conclusion:

- the messages adapter currently lacks the canonical response-id carrier needed by `conversation`
- we need to decide whether `e.Message.ID` should map to unified response identity
- if yes, completed message-level events should carry:
  - `Lifecycle.Scope == LifecycleScopeResponse`
  - `Lifecycle.State == LifecycleStateDone`
  - `Lifecycle.Ref.ResponseID = e.Message.ID`

#### Completions adapter

Current status: **needs normalization**

Observed behavior:

- started event uses `Started.RequestID = chunk.ID`
- content delta events do not carry `StreamRef.ResponseID`
- completed events currently set only `Completed.StopReason`
- **completed events do not carry response-scoped lifecycle or response id**

Conclusion:

- the completions adapter currently lacks the canonical response-id carrier needed by `conversation`
- if `chunk.ID` is the best available response identity, completed events should carry it via response-scoped lifecycle ref

---

## Concrete contract for `conversation` MVP

The `conversation` accumulator should consume unified events with the following expectations.

### Completion detection

A turn is considered successfully complete only when the stream contains:

- `Type == StreamEventCompleted`
- `Completed != nil`

For native responses continuation, the preferred completed event also has:

- `Lifecycle != nil`
- `Lifecycle.Scope == LifecycleScopeResponse`

### Canonical response-id source

For `conversation`, the canonical response-id source is:

- `StreamEventCompleted`
- `Lifecycle.Scope == LifecycleScopeResponse`
- `Lifecycle.Ref.ResponseID != ""`

This is the primary source used to update session-native continuation state.

### Fallback response-id sources

Until all adapters are normalized, the accumulator may temporarily fall back to:

1. `Lifecycle.Ref.ResponseID` on other response-scoped lifecycle events
2. `ContentDelta.Ref.ResponseID`
3. `StreamContent.Ref.ResponseID`
4. `Started.RequestID` only where adapter-specific behavior is known and explicitly documented

But these are transitional fallbacks, not the target contract.

### Text accumulation sources

For MVP assistant text reconstruction, the accumulator should consume:

1. `StreamEventContentDelta` for text content (`ContentKindText`, UTF-8 payloads)
2. `StreamEventContent` for text content as fallback/finalized content

Recommended policy:

- prefer delta accumulation when present
- use content/final payload only if no text deltas were seen
- avoid appending final text twice when both delta and content are emitted

### Reasoning accumulation sources

For MVP reasoning preservation, the accumulator or adjacent state recorder should observe:

1. `StreamEventContentDelta` with `ContentKindReasoning`
2. `StreamEventContent` with `ContentKindReasoning`

Even if the first implementation does not fully replay all reasoning state, it should be structured so this information is not silently lost.

---

## Required normalization work before or during PR 0

### 1. Standardize response-completed events across adapters

Target behavior for all streaming adapters that can provide a stable response/message/chunk id:

- completed events should include `Lifecycle`
- `Lifecycle.Scope` should be `LifecycleScopeResponse`
- `Lifecycle.State` should be `LifecycleStateDone` (or failed/incomplete where appropriate)
- `Lifecycle.Ref.ResponseID` should carry the stable response identity

### 2. Stop depending on `Started.RequestID` as the continuation key

`Started.RequestID` may still be useful for observability, but `conversation` should not rely on it as the canonical continuation id.

### 3. Preserve reasoning event shape

Responses and Messages already emit reasoning through unified content events. That shape should be kept stable so the conversation layer can record it later.

---

## Proposed adapter normalization targets

### Responses

No major semantic change required.

Keep current behavior as the reference shape.

### Ollama

Adjust completed event emission from something like:

```go
unified.StreamEvent{
    Type: unified.StreamEventCompleted,
    Completed: &unified.Completed{...},
}
```

to include response lifecycle ref:

```go
unified.StreamEvent{
    Type: unified.StreamEventCompleted,
    Lifecycle: &unified.Lifecycle{
        Scope: unified.LifecycleScopeResponse,
        State: unified.LifecycleStateDone,
        Ref:   unified.StreamRef{ResponseID: ref.ResponseID},
    },
    Completed: &unified.Completed{...},
}
```

### Messages

On the message-complete event, emit completed response lifecycle using the message id as unified response identity.

Target shape:

```go
unified.StreamEvent{
    Type: unified.StreamEventCompleted,
    Lifecycle: &unified.Lifecycle{
        Scope: unified.LifecycleScopeResponse,
        State: unified.LifecycleStateDone,
        Ref:   unified.StreamRef{ResponseID: e.Message.ID},
    },
    Completed: &unified.Completed{...},
}
```

If the stop/delta event does not itself carry the message id, the mapper may need to retain the last started message id in mapper state.

### Completions

On finish-reason completion, emit completed response lifecycle using `chunk.ID` as unified response identity.

Target shape:

```go
unified.StreamEvent{
    Type: unified.StreamEventCompleted,
    Lifecycle: &unified.Lifecycle{
        Scope: unified.LifecycleScopeResponse,
        State: unified.LifecycleStateDone,
        Ref:   unified.StreamRef{ResponseID: chunk.ID},
    },
    Completed: &unified.Completed{...},
}
```

If content delta events can also cheaply include `Ref.ResponseID`, that is beneficial but secondary.

---

## Recommended PR 0 deliverable

Before `conversation` implementation, land a small normalization pass that ensures:

- Responses completed events already satisfy the contract
- Ollama completed events carry `Lifecycle.Ref.ResponseID`
- Messages completed events carry `Lifecycle.Ref.ResponseID`
- Completions completed events carry `Lifecycle.Ref.ResponseID`
- tests are added or updated to lock this in

After that, `conversation` can safely implement:

- native `previous_response_id` updates from a single canonical completed-event path
- replay fallback without backend-specific response-id guessing


---

## Status update

This section tracks what has already been implemented relative to the design.

### Implemented

#### Unified response-id groundwork

Completed:

- normalized response-id carriage on completed-event paths across adapters used by the conversation layer
- `responses` already matched the intended shape
- `ollama`, `messages`, and `completions` were adjusted so completed events carry response-scoped lifecycle refs with `StreamRef.ResponseID`

This means the conversation layer can now rely primarily on:

- `StreamEventCompleted`
- `Lifecycle.Scope == LifecycleScopeResponse`
- `Lifecycle.Ref.ResponseID`

#### `conversation` package scaffolding

Completed:

- package created at `conversation/`
- basic exported structs/options added
- request builder added
- `unified.ToolChoice` now used in `conversation.Request`

#### Session MVP

Completed:

- `Session.Request(ctx, conversation.Request)` implemented
- in-memory history implemented
- replay-first request building implemented
- native responses continuation optimization scaffold implemented
- first-turn fallback from native to replay implemented
- mutex/in-flight guard implemented
- `History()` and `Reset()` implemented

#### Text accumulation and commit semantics

Completed:

- assistant text deltas/content accumulated from unified stream events
- turn commit happens only after successful completed stream
- failed/incomplete turns do not mutate conversation history

#### Tool-call accumulation

Completed:

- assistant-emitted tool calls are captured from unified stream events
- committed into assistant history as `unified.PartTypeToolCall`

#### Reasoning capture and replay

Completed:

- reasoning raw/summary content is captured from unified reasoning stream events
- reasoning is exposed via `ReasoningHistory()`
- reasoning is now also committed into assistant history as thinking parts
- replay strategy therefore carries reasoning forward automatically in transcript form

#### Integration validation

Completed:

- real provider integration coverage exists for unified conversation behavior
- same conversation test runs successfully against:
  - OpenAI (native continuation path)
  - Ollama (replay path)
- debugging REPL example added at `examples/agentrepl/main.go`

### Partially implemented / next focus

#### Tool-loop fidelity

Implemented:

- assistant tool calls are preserved in committed assistant history
- caller tool results are accepted as input
- multi-tool turns are accumulated and committed in one assistant turn
- replay preserves assistant tool-call messages and subsequent tool-result messages in order
- unit coverage exists for multi-tool accumulation and tool-result replay ordering
- integration coverage exists for a basic tool loop (assistant tool call -> tool result -> assistant follow-up)

Still to improve:

- provider-specific/native continuation integration gaps for mixed-content tool loops (partly clarified: Responses projection rejects assistant text after tool calls)
- any remaining real-provider edge cases that are not yet covered by unit tests

#### Reasoning semantics

Partially implemented:

- reasoning is captured and replayed

Still open:

- whether provider/source metadata for reasoning should be preserved more faithfully
- whether reasoning summary vs raw should later become first-class transcript metadata rather than only thinking parts

#### Public API evolution

Still open:

- whether the current `conversation.Request` shape is final
- whether more request-local fields should be added after tool-loop work settles

### Current practical status

The conversation package is already usable for:

- stateful text conversations
- replay-backed continuation across stateless backends
- native `previous_response_id` continuation when available
- reasoning capture plus canonical replay through committed history
- tool-call / tool-result loops including multi-step and multi-tool replay coverage
- exact canonical assistant-part ordering in local session history
- configurable outbound replay-message projection via `MessageProjector`
- provider-specific OpenRouter conversation projector at `api/openrouter/conversation.go` for early validation of known replay-shape constraints
- explicit `Session.ProjectMessages(...)` and `Session.BuildRequest(...)` helpers for inspection and custom request assembly
- release-facing docs covering session semantics, projection, service-specific replay policy, and inspection helpers
- example-level release prep for projection/custom-projector usage

### Status summary

Implemented:

- canonical local conversation history as the source of truth
- replay/native strategy selection with native Responses continuation fallback to replay
- response-id propagation needed for native continuation state tracking
- canonical accumulation of text, reasoning, and tool calls into committed assistant turns
- failure semantics that avoid committing incomplete turns
- public projection hooks for replay messages and full unified request construction
- provider-specific replay validation for OpenRouter outside the generic `conversation` package

Remaining likely focus after this release:

- additional provider-specific projectors only where real service quirks justify them
- optional persistence/import-export story for canonical session history
- any further UX refinement once real users exercise `ProjectMessages(...)`, `BuildRequest(...)`, and custom projector composition
