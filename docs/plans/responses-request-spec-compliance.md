# Plan: `responses.Request` — 100% OpenAPI Spec Compliance

**Scope:** `api/responses/types.go` (`Request` struct and its dependency types), the `adapt`
bridge layer, and the `api/unified` types that feed into it. Events are out of scope for this
pass.

**Goal:** The Go `responses.Request` struct must exactly model the `CreateResponse` OpenAPI
schema — every field present, correctly typed, correctly constrained, nothing spurious. All
jsonschema constraints are encoded as struct tags at the time each field is written, so a JSON
Schema and a validator can be derived from the Go type automatically.

**Reference:** `docs/specs/openai-responses.yaml` — `CreateResponse` schema (`allOf` of
`CreateModelResponseProperties`, `ResponseProperties`, and inline fields).

---

## Guiding Principles

1. **`api/responses` = wire format only.** Types in this package model exactly what the OpenAI
   API sends and receives — nothing more, nothing less. No internal bridge state lives here.
2. **`adapt` = the mismatch layer.** The bridge translates `unified.Request` ↔ wire types. All
   field-name and capability mismatches are absorbed there.
3. **Pointer semantics for nullability.** Every `field | null` in the schema becomes `*T` in Go.
   Required for correct three-state JSON serialization: absent (omit), null (explicit), real value.
4. **Enum string aliases.** Every `enum` field gets a typed `string` alias and `const` block. No
   raw strings scattered through call sites.
5. **Union types get custom marshal/unmarshal.** Where the schema says `oneOf`, Go uses a wrapper
   struct with `MarshalJSON`/`UnmarshalJSON` and named constructor functions — never `any`.
6. **Constructors own the discriminator.** For union types whose variants have a fixed `type`
   string (`x-stainless-const: true` in the schema), the constructor sets `Type` automatically.
   Callers never write the discriminator string.
7. **jsonschema tags from day one.** Every field with schema constraints (`minimum`, `maximum`,
   `maxLength`, `enum`, `default`, `description`, `const`) gets a `jsonschema:"..."` tag at the
   same time the field is written, not as a follow-up pass.
8. **Internal state never touches the wire.** `Extra map[string]any \`json:"-"\`` on `Request`
   is the escape hatch for `RequestTransform` hooks. Nothing internal goes through `Metadata`.

---

## What Is Wrong Today

### Spurious fields (not in `CreateResponse` schema — must be removed)

| Field | JSON key | Why wrong |
|---|---|---|
| `MaxTokens int` | `max_tokens` | Chat Completions field. `CreateResponse` only has `max_output_tokens`. |
| `TopK int` | `top_k` | Not in the Responses API schema at all. |
| `ResponseFormat *ResponseFormat` | `response_format` | Chat Completions field. Replaced by `text.format` in the Responses API. |

### Fields completely missing from the Go struct

| Schema field | Type in schema | Notes |
|---|---|---|
| `text` | `ResponseTextParam` | `{format, verbosity}` — the replacement for `response_format`. |
| `truncation` | `"auto"\|"disabled"\|null` | What to do when input exceeds the context window. |
| `include` | `[]IncludeEnum\|null` | Additional output data to include (logprobs, encrypted reasoning, etc.). |
| `stream_options` | `{include_obfuscation: bool}\|null` | Only relevant when `stream=true`. |
| `background` | `bool\|null` | Run response in the background. |
| `max_tool_calls` | `int\|null` | Max total built-in tool calls per response, across all tools. |
| `conversation` | `string\|{id:string}\|null` | Link to a conversation. Mutually exclusive with `previous_response_id`. |
| `context_management` | `[]ContextManagementParam\|null` | Auto-compaction config. Min 1 item. |
| `prompt` | `{id,version?,variables?}\|null` | Reference to a stored prompt template. |
| `safety_identifier` | `string` | Hashed user ID for abuse detection. Max 64 chars. Replaces `user`. |
| `service_tier` | `"auto"\|"default"\|"flex"\|"scale"\|"priority"\|null` | Processing tier. |
| `top_logprobs` | `int` | 0–20. Most likely tokens to return at each position. |

### Fields present but with wrong or imprecise types/constraints

| Field | Current | Schema | Problem |
|---|---|---|---|
| `Input []Input` | `[]Input` | `string \| []InputItem` | No string shorthand; `Input` is a flat catch-all, not the `InputItem` discriminated union. |
| `Instructions string` | `string` | `string\|null` | Needs `*string`. Cannot distinguish empty from null. |
| `Store bool` | `bool` | `bool\|null`, default `true` | Needs `*bool`. Cannot send null or distinguish false from unset. |
| `ParallelToolCalls bool` | `bool` | `bool\|null`, default `true` | Same issue. Needs `*bool`. |
| `Stream bool` | `bool` (no omitempty) | `bool\|null`, default `false` | Always serializes. Needs `*bool`. |
| `PreviousResponseID string` | `string` | `string\|null` | Needs `*string`. |
| `Temperature float64` | `float64` | `number\|null`, min 0, max 2, default 1 | Needs `*float64`. Missing constraints. |
| `TopP float64` | `float64` | `number\|null`, min 0, max 1, default 1 | Needs `*float64`. Missing constraints. |
| `MaxOutputTokens int` | `int` | `int\|null`, min 16 | Needs `*int`. Missing minimum. |
| `PromptCacheRetention string` | `string` | `"in-memory"\|"24h"\|null` | Needs typed alias and pointer. |
| `Tools []Tool` | `[]Tool` | `ToolsArray` — discriminated union of 15 types | Current `Tool` struct only models function tools. |
| `ToolChoice any` | `any` | Complex `oneOf` union | No type safety, no schema derivation. |
| `Reasoning.Effort string` | `string` | Enum: `none/minimal/low/medium/high/xhigh`, nullable | Needs typed alias, pointer. |
| `Reasoning.Summary string` | `string` | Enum: `auto/concise/detailed`, nullable | Needs typed alias, pointer. |
| `Metadata map[string]any` | `map[string]any` | `map[string]string\|null`, max 16 entries | Wrong value type; also conflated with internal adapter state (see below). |
| `User string` | `string` | Deprecated — replaced by `safety_identifier` and `prompt_cache_key` | Should be tagged deprecated. |

### Metadata-specific problem: two concepts conflated

`responses.Request.Metadata` (type `map[string]any`) is populated by `metadataToOpenAI()`, which
merges `unified.RequestMetadata.Metadata` (internal adapter roundtrip state) with
`ResponsesExtras.ExtraMetadata` (caller-supplied extras). Both end up on the OpenAI wire,
meaning internal adapter state silently leaks to OpenAI.

| Concept | Goes to wire? | Current home | Correct home |
|---|---|---|---|
| OpenAI `metadata` API feature — up to 16 `string→string` pairs stored with the response | **Yes** | `responses.Request.Metadata map[string]any` (wrong type) | `responses.Request.Metadata map[string]string` |
| Internal bridge/adapter state | **Never** | Merged into `Metadata` via `metadataToOpenAI()` | `responses.Request.Extra map[string]any \`json:"-"\`` |
| Explicit caller-supplied OpenAI metadata | **Yes** | `ResponsesExtras.ExtraMetadata map[string]any` (wrong name/type) | `ResponsesExtras.OpenAIMetadata map[string]string` |
| User identity for routing/caching | Wire as `user` field | `unified.RequestMetadata.User` | Stays, struct renamed to `RequestIdentity` |

---

## Execution Plan

Each phase leaves the codebase in a **compilable, green-test state**. Bridge changes are always
committed in the same phase as the type change that requires them.

---

### Phase 0 — Dependencies

**File:** `go.mod`, `go.sum`

Add two libraries:

- **`github.com/invopop/jsonschema`** — reflects Go structs into `*jsonschema.Schema` using
  `jsonschema:"..."` struct tags. Relevant tag keys: `minimum=N`, `maximum=N`, `minLength=N`,
  `maxLength=N`, `maxProperties=N`, `minItems=N`, `default=val`, `description=...`, `required`,
  `deprecated=true`, `const=val`. For union types where tags are insufficient, the wrapper type
  implements `JSONSchema() *jsonschema.Schema`.

- **`github.com/santhosh-tekuri/jsonschema/v6`** — validates a JSON document against a
  `*jsonschema.Schema`. Used by the `Validate()` function added in Phase 14.

---

### Phase 1 — Enum type aliases and constants

**File:** `api/responses/request_enums.go` *(new file)*

Define typed `string` aliases for every enum field in `CreateResponse` and its referenced
schemas. Pattern:

```go
type ReasoningEffort string

const (
    ReasoningEffortNone    ReasoningEffort = "none"
    ReasoningEffortMinimal ReasoningEffort = "minimal"
    ReasoningEffortLow     ReasoningEffort = "low"
    ReasoningEffortMedium  ReasoningEffort = "medium"
    ReasoningEffortHigh    ReasoningEffort = "high"
    ReasoningEffortXHigh   ReasoningEffort = "xhigh"
)
```

Full set of types to define:

| Type | Enum values | Source schema |
|---|---|---|
| `ReasoningEffort` | `none`, `minimal`, `low`, `medium`, `high`, `xhigh` | `ReasoningEffort` |
| `ReasoningSummary` | `auto`, `concise`, `detailed` | `Reasoning.summary` |
| `PromptCacheRetention` | `in-memory`, `24h` | `ModelResponseProperties.prompt_cache_retention` |
| `ServiceTier` | `auto`, `default`, `flex`, `scale`, `priority` | `ServiceTier` |
| `Truncation` | `auto`, `disabled` | `ResponseProperties.truncation` |
| `IncludeItem` | `file_search_call.results`, `web_search_call.results`, `web_search_call.action.sources`, `message.input_image.image_url`, `computer_call_output.output.image_url`, `code_interpreter_call.outputs`, `reasoning.encrypted_content`, `message.output_text.logprobs` | `IncludeEnum` |
| `Verbosity` | `low`, `medium`, `high` | `Verbosity` |

Because `invopop/jsonschema` does not propagate struct tags from type aliases, each enum type
implements `JSONSchema() *jsonschema.Schema` returning a schema with `Enum` populated from the
constant values.

---

### Phase 2 — Fix existing fields on `Request` and `Reasoning`

**File:** `api/responses/types.go`

This phase fixes all existing fields that have wrong types or missing constraints. No fields are
added or removed yet.

**`Reasoning` struct** — both fields are `anyOf: [string, null]`; drop `omitempty` because the
API echoes explicit `null` and callers depend on the distinction:

```go
type Reasoning struct {
    Effort  *ReasoningEffort  `json:"effort"`
    Summary *ReasoningSummary `json:"summary"`
}
```

**Scalar fields on `Request`** — change to pointer types and add constraint tags:

| Field | Change | Tags to add |
|---|---|---|
| `Instructions string` | → `*string` | `description=System or developer message inserted into model context` |
| `Store bool` | → `*bool` | `default=true` |
| `ParallelToolCalls bool` | → `*bool` | `default=true` |
| `Stream bool` (no omitempty) | → `*bool` | `default=false` |
| `PreviousResponseID string` | → `*string` | `description=ID of previous response for multi-turn conversations` |
| `Temperature float64` | → `*float64` | `minimum=0,maximum=2,default=1` |
| `TopP float64` | → `*float64` | `minimum=0,maximum=1,default=1` |
| `MaxOutputTokens int` | → `*int` | `minimum=16` |
| `PromptCacheRetention string` | → `*PromptCacheRetention` | (enum via type's `JSONSchema()`) |

`User string` keeps `omitempty` but gains
`jsonschema:"deprecated=true,description=Replaced by safety_identifier and prompt_cache_key"`.

**Bridge impact (`adapt/responses_request_bridge.go`):** The `Store` and `ParallelToolCalls`
reads in `RequestFromResponses` currently guard on `r.Store || r.ParallelToolCalls` — update to
pointer-nil checks. The `BuildResponsesRequest` writes also change: assign the pointer directly
rather than copying the bool value.

---

### Phase 3 — Remove spurious fields

**File:** `api/responses/types.go`, `Request` struct

Remove these three fields:
- `MaxTokens int` (`json:"max_tokens"`)
- `TopK int` (`json:"top_k"`)
- `ResponseFormat *ResponseFormat` (`json:"response_format"`)

The `ResponseFormat` struct definition is **not** removed here — it is still referenced in the
bridge temporarily. It is deleted in Phase 10 when `TextResponseFormat` fully replaces it.

**Bridge fixes in `adapt/responses_request_bridge.go`** (must compile after this phase):

`BuildResponsesRequest`:
- Remove the `usedMaxField` logic entirely. Always write `out.MaxOutputTokens`.
- Remove `out.TopK = r.TopK`.
- The `OutputModeJSONObject` case previously wrote `out.ResponseFormat = ...`. Replace with a
  returned error: `return nil, fmt.Errorf("json_object output mode: use Text.Format via RequestTransform until Phase 10")`.
  This is a temporary stub — the proper implementation comes in Phase 10 when `TextResponseFormat`
  exists. It is acceptable because `OutputModeJSONObject` was already broken for Responses API
  callers (the field never worked correctly against the spec).

`RequestFromResponses`:
- Remove `r.MaxTokens`, `r.TopK`, and `r.ResponseFormat` reads.
- The `r.ResponseFormat` read will be reinstated as `r.Text` in Phase 10.

**`adapt/completions_request_bridge.go`:**
- Remove the `UsedMaxTokenField` logic in `RequestFromCompletions` that references `max_tokens`
  on a responses request.

**`api/unified/types.go`:**
- Remove `UsedMaxTokenField string` from `ResponsesExtras`.

---

### Phase 4 — Metadata separation

This phase eliminates the leak of internal adapter state onto the wire. All steps are applied in
order and must all compile before committing.

#### Phase 4a — `api/unified/types.go`: `RequestMetadata` → `RequestIdentity`

Drop the `Metadata map[string]any` bag from the struct. Rename the type and the field on
`unified.Request`:

```go
// RequestIdentity carries end-user identification for safety and caching.
// Maps to the wire `user` or `safety_identifier` fields depending on the backend.
type RequestIdentity struct {
    User string `json:"user,omitempty"`
}
```

```go
// On unified.Request:
// Before: Metadata *RequestMetadata `json:"metadata,omitempty"`
// After:
Identity *RequestIdentity `json:"identity,omitempty"`
```

`unified.Request` is an internal type never serialized to any external wire protocol, so the
JSON tag rename is safe.

#### Phase 4b — `api/unified/types.go`: Fix `ResponsesExtras` and `CompletionsExtras`

```go
type ResponsesExtras struct {
    PromptCacheRetention string            `json:"prompt_cache_retention,omitempty"`
    PromptCacheKey       string            `json:"prompt_cache_key,omitempty"`
    PreviousResponseID   string            `json:"previous_response_id,omitempty"`
    ReasoningSummary     string            `json:"reasoning_summary,omitempty"`
    Store                *bool             `json:"store,omitempty"`
    ParallelToolCalls    *bool             `json:"parallel_tool_calls,omitempty"`
    UseInstructions      *bool             `json:"use_instructions,omitempty"`
    OpenAIMetadata       map[string]string `json:"openai_metadata,omitempty"`
    // UsedMaxTokenField removed in Phase 3
}
```

`Store` and `ParallelToolCalls` change from `bool` to `*bool` here (consistent with Phase 2
changes on the wire type). `ExtraMetadata map[string]any` is renamed to
`OpenAIMetadata map[string]string` — callers who want data stored in the OpenAI response object
set this field explicitly; nothing internal is merged here automatically.

Same rename in `CompletionsExtras`: `ExtraMetadata map[string]any` → `OpenAIMetadata map[string]string`.

#### Phase 4c — `adapt/helpers.go`: Replace metadata helpers

Delete `metadataToOpenAI` and `metadataFromOpenAI`. Add three focused replacements:

```go
// wireUser returns the user identifier for the wire `user` field.
func wireUser(id *unified.RequestIdentity) string {
    if id == nil { return "" }
    return id.User
}

// wireOpenAIMetadata clones explicitly declared OpenAI metadata for the wire.
// Never contains internal adapter state.
func wireOpenAIMetadata(m map[string]string) map[string]string {
    if len(m) == 0 { return nil }
    out := make(map[string]string, len(m))
    for k, v := range m { out[k] = v }
    return out
}

// identityFromWire reconstructs RequestIdentity from the wire user field.
func identityFromWire(user string) *unified.RequestIdentity {
    if user == "" { return nil }
    return &unified.RequestIdentity{User: user}
}
```

#### Phase 4d — `adapt/responses_request_bridge.go`: Update both directions

`BuildResponsesRequest` metadata block:
```go
// Before (leaking internal state to wire):
out.User, out.Metadata = metadataToOpenAI(r.Metadata, nil)
if rextras != nil {
    _, out.Metadata = metadataToOpenAI(r.Metadata, rextras.ExtraMetadata)
    out.User, _ = metadataToOpenAI(r.Metadata, nil)
}

// After (explicit, clean):
out.User = wireUser(r.Identity)
if rextras != nil {
    out.Metadata = wireOpenAIMetadata(rextras.OpenAIMetadata)
}
```

`Store` and `ParallelToolCalls` bridge logic (now `*bool` on both sides):
```go
if rextras.Store != nil           { out.Store = rextras.Store }
if rextras.ParallelToolCalls != nil { out.ParallelToolCalls = rextras.ParallelToolCalls }
```

`RequestFromResponses` metadata block:
```go
// Before:
if meta, extra := metadataFromOpenAI(r.User, r.Metadata); meta != nil { ... }

// After:
u.Identity = identityFromWire(r.User)
if len(r.Metadata) > 0 {
    ensureResponsesExtras(&u).OpenAIMetadata = maps.Clone(r.Metadata)
}
```

#### Phase 4e — `adapt/completions_request_bridge.go`: Mirror the same cleanup

Replace `metadataToOpenAI`/`metadataFromOpenAI` calls, use `wireUser(r.Identity)` and
`wireOpenAIMetadata(rextras.OpenAIMetadata)`.

#### Phase 4f — `adapt/messages_request_bridge.go`: Update identity reference

`applyMessagesMetadata` currently takes `*unified.RequestMetadata`. Update to
`*unified.RequestIdentity` — access `.User` directly. The removed `Metadata map[string]any`
field is no longer present.

#### Phase 4g — `api/responses/types.go` and `api/completions/types.go`: Fix wire Metadata field

`responses.Request`:
```go
// Metadata stores up to 16 string key-value pairs alongside the response.
// Use this to tag responses for auditing or later retrieval via the OpenAI API.
// Keys: max 64 chars. Values: max 512 chars.
Metadata map[string]string `json:"metadata,omitempty" jsonschema:"maxProperties=16"`

// Extra holds non-serialized adapter state for RequestTransform hooks.
// This field is never sent to OpenAI.
Extra map[string]any `json:"-"`
```

`completions.Request`: same `Metadata` type fix (`map[string]any` → `map[string]string`).

---

### Phase 5 — Add missing top-level fields and their supporting structs

**Files:** `api/responses/types.go`, `api/responses/request_types.go` *(new file for
supporting structs)*

Add the 12 missing fields to `Request`. Union types (`TextResponseFormat`, `ConversationParam`)
are declared as **opaque stubs** in this phase — their wire shape is correct but constructors and
`JSONSchema()` methods come in Phases 9 and 10. This avoids a dependency on later phases while
keeping the code compilable.

```go
// On Request struct:
Text              *ResponseTextParam       `json:"text,omitempty"`
Truncation        *Truncation              `json:"truncation,omitempty"             jsonschema:"description=auto truncates to fit context window; disabled returns 400. Default: disabled"`
Include           []IncludeItem            `json:"include,omitempty"                jsonschema:"description=Additional output data to include in the response"`
StreamOptions     *StreamOptions           `json:"stream_options,omitempty"         jsonschema:"description=Streaming options. Only valid when stream=true"`
Background        *bool                    `json:"background,omitempty"             jsonschema:"default=false,description=Run the response in the background"`
MaxToolCalls      *int                     `json:"max_tool_calls,omitempty"         jsonschema:"description=Maximum total built-in tool calls across the response"`
Conversation      *ConversationParam       `json:"conversation,omitempty"           jsonschema:"description=Links response to a conversation. Cannot combine with previous_response_id"`
ContextManagement []ContextManagementParam `json:"context_management,omitempty"     jsonschema:"minItems=1"`
Prompt            *Prompt                  `json:"prompt,omitempty"                 jsonschema:"description=Reference to a stored prompt template"`
SafetyIdentifier  string                   `json:"safety_identifier,omitempty"      jsonschema:"maxLength=64,description=Hashed stable user ID for abuse detection"`
ServiceTier       *ServiceTier             `json:"service_tier,omitempty"           jsonschema:"description=Processing tier. Default: auto"`
TopLogprobs       *int                     `json:"top_logprobs,omitempty"           jsonschema:"minimum=0,maximum=20"`
```

**Supporting structs in `request_types.go`:**

```go
// ResponseTextParam configures text output format and verbosity.
type ResponseTextParam struct {
    Format    *TextResponseFormat `json:"format,omitempty"`
    Verbosity *Verbosity          `json:"verbosity,omitempty"`
}

// TextResponseFormat is an opaque stub here; constructors added in Phase 9.
type TextResponseFormat struct{ raw json.RawMessage }

func (f TextResponseFormat) MarshalJSON() ([]byte, error)  { return f.raw, nil }
func (f *TextResponseFormat) UnmarshalJSON(b []byte) error { f.raw = append([]byte(nil), b...); return nil }

// ConversationParam is an opaque stub here; constructors added in Phase 10.
type ConversationParam struct{ raw json.RawMessage }

func (c ConversationParam) MarshalJSON() ([]byte, error)  { return c.raw, nil }
func (c *ConversationParam) UnmarshalJSON(b []byte) error { c.raw = append([]byte(nil), b...); return nil }

// StreamOptions configures streaming behaviour.
type StreamOptions struct {
    IncludeObfuscation bool `json:"include_obfuscation,omitempty" jsonschema:"description=Add obfuscation fields to mitigate side-channel attacks"`
}

// ContextManagementParam configures automatic context compaction.
type ContextManagementParam struct {
    Type             string `json:"type"                        jsonschema:"required,description=Entry type. Currently only compaction is supported"`
    CompactThreshold *int   `json:"compact_threshold,omitempty" jsonschema:"minimum=1000,description=Token threshold at which compaction triggers"`
}

// Prompt references a stored prompt template by ID.
type Prompt struct {
    ID        string         `json:"id"                  jsonschema:"required"`
    Version   *string        `json:"version,omitempty"`
    Variables map[string]any `json:"variables,omitempty"`
}
```

---

### Phase 6 — Redesign `InputParam` (the `Input` field)

**Schema:** `InputParam = string | []InputItem`

**File:** `api/responses/request_input.go` *(new file)*

#### 6a — `InputParam` union wrapper

`Input` is a required field, so there is no "unset" state to model. The wrapper disambiguates
string vs. array form:

```go
// InputParam is the input to the model: either a plain text string or a list of
// structured input items.
type InputParam struct {
    text  string
    items []InputItem
}

// InputText creates a plain-text input, equivalent to a single user message.
func InputText(s string) InputParam { return InputParam{text: s} }

// InputItems creates a structured input list.
func InputItems(items []InputItem) InputParam { return InputParam{items: items} }

func (p InputParam) MarshalJSON() ([]byte, error) {
    if p.items != nil {
        return json.Marshal(p.items)
    }
    return json.Marshal(p.text)
}

func (p *InputParam) UnmarshalJSON(b []byte) error {
    // First non-space byte: '"' → string variant, '[' → array variant.
    ...
}

func (InputParam) JSONSchema() *jsonschema.Schema {
    // oneOf: {type: string}, {type: array, items: InputItem schema}
    ...
}
```

No `set bool` flag — `Input` is required and must always have a value. The zero value (empty
`InputParam{}`) marshals as `""` (empty string), which is a valid wire representation of an
empty text input.

#### 6b — Concrete input item types

The types needed to cover the current bridge usage and the most common patterns:

```go
// EasyInputMessage is the standard message form for most conversation turns.
// The Type field is always "message" — set automatically by the constructor.
type EasyInputMessage struct {
    Type    string           `json:"type"            jsonschema:"const=message"`
    Role    string           `json:"role"            jsonschema:"required,enum=user,enum=assistant,enum=system,enum=developer"`
    Content EasyInputContent `json:"content"         jsonschema:"required"`
    Phase   *string          `json:"phase,omitempty"`
}

// NewEasyInputMessage constructs an EasyInputMessage, setting Type automatically.
func NewEasyInputMessage(role string, content EasyInputContent) EasyInputMessage {
    return EasyInputMessage{Type: "message", Role: role, Content: content}
}

// EasyInputContent is string | []InputContentPart — same wrapper pattern as InputParam.
type EasyInputContent struct{ ... }

func EasyInputContentText(s string) EasyInputContent  { ... }
func EasyInputContentParts(p []InputContentPart) EasyInputContent { ... }

// FunctionCallOutput sends a function tool result back to the model.
// The Type field is always "function_call_output" — set automatically by the constructor.
type FunctionCallOutput struct {
    Type   string `json:"type"    jsonschema:"const=function_call_output"`
    CallID string `json:"call_id" jsonschema:"required"`
    Output string `json:"output"  jsonschema:"required"`
}

func NewFunctionCallOutput(callID, output string) FunctionCallOutput {
    return FunctionCallOutput{Type: "function_call_output", CallID: callID, Output: output}
}

// FunctionCallInput represents a model-generated function call, used when
// replaying conversation history as input.
// The Type field is always "function_call" — set automatically by the constructor.
type FunctionCallInput struct {
    Type      string `json:"type"      jsonschema:"const=function_call"`
    CallID    string `json:"call_id"   jsonschema:"required"`
    Name      string `json:"name"      jsonschema:"required"`
    Arguments string `json:"arguments" jsonschema:"required"`
    Phase     string `json:"phase,omitempty"`
}

func NewFunctionCallInput(callID, name, arguments string) FunctionCallInput {
    return FunctionCallInput{Type: "function_call", CallID: callID, Name: name, Arguments: arguments}
}
```

#### 6c — `InputItem` discriminated wrapper

```go
// InputItem wraps any valid input item variant.
type InputItem struct{ raw json.RawMessage }

func InputItemFromMessage(m EasyInputMessage) InputItem          { return mustMarshalItem(m) }
func InputItemFromFunctionOutput(f FunctionCallOutput) InputItem { return mustMarshalItem(f) }
func InputItemFromFunctionCall(f FunctionCallInput) InputItem    { return mustMarshalItem(f) }
// InputItemRaw accepts pre-serialised JSON for uncommon item types.
func InputItemRaw(raw json.RawMessage) InputItem                 { return InputItem{raw: raw} }

func (i InputItem) MarshalJSON() ([]byte, error)  { return i.raw, nil }
func (i *InputItem) UnmarshalJSON(b []byte) error { i.raw = append([]byte(nil), b...); return nil }
// Raw returns the underlying JSON bytes for decoding by the bridge.
func (i InputItem) Raw() json.RawMessage           { return i.raw }
func (InputItem) JSONSchema() *jsonschema.Schema    { /* oneOf of all InputItem types */ }

// mustMarshalItem is a package-private helper that panics if marshalling fails.
// It is only called with known-good concrete types where failure is a programming error.
func mustMarshalItem(v any) InputItem { ... }
```

#### 6d — Update `Request.Input`

```go
// Before:
Input []Input `json:"input"`

// After:
Input InputParam `json:"input" jsonschema:"required"`
```

The flat `Input` struct (the old catch-all) is **deleted**. All construction sites are in the
bridge.

#### 6e — Update bridge

`BuildResponsesRequest` input construction:
```go
// System message → instructions field (unchanged)
// Developer/user messages:
items = append(items, responses.InputItemFromMessage(
    responses.NewEasyInputMessage("user", responses.EasyInputContentText(partsText(m.Parts))),
))

// Function call output:
items = append(items, responses.InputItemFromFunctionOutput(
    responses.NewFunctionCallOutput(p.ToolResult.ToolCallID, p.ToolResult.ToolOutput),
))

// Function call (assistant turn):
items = append(items, responses.InputItemFromFunctionCall(
    responses.NewFunctionCallInput(p.ToolCall.ID, p.ToolCall.Name, string(argRaw)),
))

out.Input = responses.InputItems(items)
```

`RequestFromResponses` input decoding: unmarshal `item.Raw()` into a `map[string]any`, read the
`type` and `role` discriminators, then route to the appropriate concrete type for field
extraction.

---

### Phase 7 — Redesign `Tool` as a discriminated union

**Schema:** `Tool = oneOf FunctionTool | FileSearchTool | WebSearchTool | MCPTool |
CodeInterpreterTool | ImageGenTool | ComputerTool | ...`

**File:** `api/responses/request_tools.go` *(new file)*

#### 7a — Concrete tool structs

Each tool struct has its `Type` field declared as `const=<value>`. Constructors set it
automatically — callers never write the string.

```go
type FunctionTool struct {
    Type         string  `json:"type"                  jsonschema:"const=function"`
    Name         string  `json:"name"                  jsonschema:"required"`
    Description  *string `json:"description,omitempty"`
    Parameters   any     `json:"parameters,omitempty"  jsonschema:"description=JSON Schema describing the function parameters"`
    Strict       *bool   `json:"strict,omitempty"      jsonschema:"description=Enforce strict parameter validation. Default true"`
    DeferLoading bool    `json:"defer_loading,omitempty"`
}

type FileSearchTool struct {
    Type           string   `json:"type"                      jsonschema:"const=file_search"`
    VectorStoreIDs []string `json:"vector_store_ids,omitempty"`
    MaxNumResults  *int     `json:"max_num_results,omitempty"  jsonschema:"minimum=1,maximum=50"`
}

type WebSearchTool struct {
    Type              string  `json:"type"                          jsonschema:"const=web_search_preview"`
    SearchContextSize *string `json:"search_context_size,omitempty" jsonschema:"enum=low,enum=medium,enum=high"`
}

type MCPTool struct {
    Type         string   `json:"type"         jsonschema:"const=mcp"`
    ServerLabel  string   `json:"server_label" jsonschema:"required"`
    ServerURL    string   `json:"server_url,omitempty"`
    AllowedTools []string `json:"allowed_tools,omitempty"`
}

type CodeInterpreterTool struct {
    Type      string  `json:"type"              jsonschema:"const=code_interpreter"`
    Container *string `json:"container,omitempty"`
}

type ImageGenTool struct {
    Type string `json:"type" jsonschema:"const=image_generation"`
}
```

Constructors for each (example pattern):
```go
func NewFunctionTool(name string, description *string, parameters any, strict *bool) FunctionTool {
    return FunctionTool{Type: "function", Name: name, Description: description,
        Parameters: parameters, Strict: strict}
}
```

Additional tool types (`ComputerTool`, `LocalShellTool`, `ApplyPatchTool`, etc.) are supported
via `ToolRaw` for now — concrete structs can be added incrementally.

#### 7b — `ToolParam` wrapper

```go
// ToolParam wraps any Tool variant for use in a request.
type ToolParam struct{ raw json.RawMessage }

func ToolFromFunction(t FunctionTool) ToolParam           { return mustMarshalTool(t) }
func ToolFromFileSearch(t FileSearchTool) ToolParam       { return mustMarshalTool(t) }
func ToolFromWebSearch(t WebSearchTool) ToolParam         { return mustMarshalTool(t) }
func ToolFromMCP(t MCPTool) ToolParam                     { return mustMarshalTool(t) }
func ToolFromCodeInterpreter(t CodeInterpreterTool) ToolParam { return mustMarshalTool(t) }
func ToolFromImageGen(t ImageGenTool) ToolParam           { return mustMarshalTool(t) }
// ToolRaw accepts pre-serialised JSON for tool types without a concrete struct.
func ToolRaw(raw json.RawMessage) ToolParam               { return ToolParam{raw: raw} }

func (t ToolParam) MarshalJSON() ([]byte, error)  { return t.raw, nil }
func (t *ToolParam) UnmarshalJSON(b []byte) error { t.raw = append([]byte(nil), b...); return nil }
// Raw returns the underlying JSON bytes for use by the bridge decoder.
func (t ToolParam) Raw() json.RawMessage           { return t.raw }
// Type returns the value of the discriminator `type` field.
func (t ToolParam) Type() string                   { /* unmarshal just the type field */ }
func (ToolParam) JSONSchema() *jsonschema.Schema    { /* oneOf of all tool types */ }

// mustMarshalTool is a package-private helper that panics on marshal error.
// It is only called with known-good concrete types.
func mustMarshalTool(v any) ToolParam { ... }
```

#### 7c — Update `Request.Tools`

```go
// Before:
Tools []Tool `json:"tools,omitempty"`

// After:
Tools []ToolParam `json:"tools,omitempty"`
```

The old flat `Tool` struct is **deleted**.

#### 7d — Update bridge

`BuildResponsesRequest`:
```go
out.Tools = append(out.Tools, responses.ToolFromFunction(
    responses.NewFunctionTool(t.Name, ptrOrNil(t.Description),
        sortmap.NewSortedMap(t.Parameters), ptrOrNil(t.Strict)),
))
```

`RequestFromResponses` — decode by discriminator:
```go
for _, tp := range r.Tools {
    switch tp.Type() {
    case "function":
        var ft responses.FunctionTool
        _ = json.Unmarshal(tp.Raw(), &ft)
        u.Tools = append(u.Tools, unified.Tool{
            Name:        ft.Name,
            Description: deref(ft.Description),
            Parameters:  toMap(ft.Parameters),
            Strict:      deref(ft.Strict),
        })
    // Non-function tools have no unified equivalent — silently skip for now.
    }
}
```

---

### Phase 8 — Redesign `ToolChoiceParam`

**Schema:** `ToolChoiceParam = "none"|"auto"|"required" | {type:function,name} |
{type:mcp,server_label,name?} | {type:allowed_tools,mode,tools} | ...`

**File:** `api/responses/request_tools.go`

```go
// ToolChoiceParam controls which tool(s) the model calls.
type ToolChoiceParam struct{ raw json.RawMessage }

func ToolChoiceAuto() ToolChoiceParam                              { /* raw: `"auto"` */ }
func ToolChoiceRequired() ToolChoiceParam                          { /* raw: `"required"` */ }
func ToolChoiceNone() ToolChoiceParam                              { /* raw: `"none"` */ }
func ToolChoiceForFunction(name string) ToolChoiceParam            { /* raw: `{"type":"function","name":name}` */ }
func ToolChoiceForMCP(serverLabel string, name *string) ToolChoiceParam { ... }
func ToolChoiceAllowed(mode string, tools []any) ToolChoiceParam   { ... }

// AsString returns the string value if this is a string-form choice ("auto", "required", "none").
func (tc ToolChoiceParam) AsString() (string, bool) { ... }
// AsObject returns the raw map if this is an object-form choice.
func (tc ToolChoiceParam) AsObject() (map[string]any, bool) { ... }

func (tc ToolChoiceParam) MarshalJSON() ([]byte, error)  { return tc.raw, nil }
func (tc *ToolChoiceParam) UnmarshalJSON(b []byte) error { tc.raw = append([]byte(nil), b...); return nil }
func (ToolChoiceParam) JSONSchema() *jsonschema.Schema    { /* oneOf: string enum + object variants */ }
```

Update `Request.ToolChoice`:
```go
// Before:
ToolChoice any `json:"tool_choice,omitempty"`

// After:
ToolChoice *ToolChoiceParam `json:"tool_choice,omitempty"`
```

Update bridge construction:
```go
case nil, unified.ToolChoiceAuto{}:   out.ToolChoice = ptr(responses.ToolChoiceAuto())
case unified.ToolChoiceRequired{}:    out.ToolChoice = ptr(responses.ToolChoiceRequired())
case unified.ToolChoiceNone{}:        out.ToolChoice = ptr(responses.ToolChoiceNone())
case unified.ToolChoiceTool{} as tc: out.ToolChoice = ptr(responses.ToolChoiceForFunction(tc.Name))
```

Update `toolChoiceFromResponses` to use `AsString()` / `AsObject()` instead of type-switching
on `any`.

---

### Phase 9 — `TextResponseFormat`: full constructors and schema

**File:** `api/responses/request_format.go` *(new file)*

`TextResponseFormat` was declared as an opaque stub in Phase 5. This phase adds the full API.

**Schema:** `TextResponseFormatConfiguration = oneOf ResponseFormatText |
TextResponseFormatJsonSchema | ResponseFormatJsonObject`

```go
// FormatText returns a plain-text output format (the default).
func FormatText() TextResponseFormat { /* raw: `{"type":"text"}` */ }

// FormatJSONObject returns a JSON-mode format (legacy; prefer FormatJSONSchema).
func FormatJSONObject() TextResponseFormat { /* raw: `{"type":"json_object"}` */ }

// FormatJSONSchema returns a structured-output format with a specific schema.
// name is required; strict defaults to true.
func FormatJSONSchema(name string, schema map[string]any, strict *bool, description *string) TextResponseFormat { ... }

// Type returns the value of the discriminator `type` field ("text", "json_object", "json_schema").
func (f TextResponseFormat) Type() string { ... }

func (TextResponseFormat) JSONSchema() *jsonschema.Schema { /* oneOf: text | json_schema | json_object */ }
```

Delete the now-redundant `ResponseFormat` struct (the old Chat Completions type that was kept as
a stub in Phase 3).

Update bridge — replace the Phase 3 stub error with the real implementation:
```go
case unified.OutputModeJSONObject:
    out.Text = &responses.ResponseTextParam{Format: ptr(responses.FormatJSONObject())}
case unified.OutputModeJSONSchema:
    out.Text = &responses.ResponseTextParam{
        Format: ptr(responses.FormatJSONSchema(r.Output.SchemaName, r.Output.Schema, ptr(true), nil)),
    }
```

`RequestFromResponses` — read `r.Text.Format.Type()` to reconstruct `u.Output`.

---

### Phase 10 — `ConversationParam`: full constructors and schema

**File:** `api/responses/request_types.go`

`ConversationParam` was declared as an opaque stub in Phase 5. This phase adds the full API.

**Schema:** `ConversationParam = string | {id: string} | null`

```go
// ConversationByID creates a ConversationParam from a bare conversation ID string.
func ConversationByID(id string) ConversationParam { /* raw: `"<id>"` */ }

// ConversationObject creates a ConversationParam from the object form.
func ConversationObject(id string) ConversationParam { /* raw: `{"id":"<id>"}` */ }

func (ConversationParam) JSONSchema() *jsonschema.Schema { /* oneOf: string, {id: string} */ }
```

No bridge changes needed — `Conversation` has no unified equivalent and is only reachable via
`ResponsesExtras.ConversationID` (surfaced in Phase 11) or a direct `RequestTransform`.

---

### Phase 11 — `ResponsesExtras` expansion and full bridge coverage

**Files:** `api/unified/types.go`, `adapt/responses_request_bridge.go`

Surface all new Responses API capabilities through `ResponsesExtras` so callers using the
`unified.Request` path can access them without writing a `RequestTransform`:

```go
type ResponsesExtras struct {
    // Existing fields (cleaned up in Phases 3 and 4):
    PromptCacheRetention string            `json:"prompt_cache_retention,omitempty"`
    PromptCacheKey       string            `json:"prompt_cache_key,omitempty"`
    PreviousResponseID   string            `json:"previous_response_id,omitempty"`
    ReasoningSummary     string            `json:"reasoning_summary,omitempty"`
    Store                *bool             `json:"store,omitempty"`
    ParallelToolCalls    *bool             `json:"parallel_tool_calls,omitempty"`
    UseInstructions      *bool             `json:"use_instructions,omitempty"`
    OpenAIMetadata       map[string]string `json:"openai_metadata,omitempty"`

    // New — surfaces Responses API fields added in Phase 5:
    ServiceTier          string            `json:"service_tier,omitempty"`
    Truncation           string            `json:"truncation,omitempty"`
    Include              []string          `json:"include,omitempty"`
    Background           *bool             `json:"background,omitempty"`
    MaxToolCalls         *int              `json:"max_tool_calls,omitempty"`
    TopLogprobs          *int              `json:"top_logprobs,omitempty"`
    ConversationID       string            `json:"conversation_id,omitempty"`
}
```

**`BuildResponsesRequest`** — add forwarding for each new extra:
```go
if rextras.ServiceTier != ""      { out.ServiceTier = ptr(responses.ServiceTier(rextras.ServiceTier)) }
if rextras.Truncation != ""       { out.Truncation   = ptr(responses.Truncation(rextras.Truncation)) }
if len(rextras.Include) > 0       { out.Include = toIncludeItems(rextras.Include) }
if rextras.Background != nil      { out.Background = rextras.Background }
if rextras.MaxToolCalls != nil    { out.MaxToolCalls = rextras.MaxToolCalls }
if rextras.TopLogprobs != nil     { out.TopLogprobs  = rextras.TopLogprobs }
if rextras.ConversationID != ""   { out.Conversation = ptr(responses.ConversationByID(rextras.ConversationID)) }
```

**`RequestFromResponses`** — read each new wire field back:
```go
if r.ServiceTier != nil  { ensureResponsesExtras(&u).ServiceTier = string(*r.ServiceTier) }
if r.Truncation != nil   { ensureResponsesExtras(&u).Truncation  = string(*r.Truncation) }
if len(r.Include) > 0    { ensureResponsesExtras(&u).Include = fromIncludeItems(r.Include) }
if r.Background != nil   { ensureResponsesExtras(&u).Background = r.Background }
if r.MaxToolCalls != nil { ensureResponsesExtras(&u).MaxToolCalls = r.MaxToolCalls }
if r.TopLogprobs != nil  { ensureResponsesExtras(&u).TopLogprobs  = r.TopLogprobs }
// Conversation is read-back as ConversationID if the wire value is a string form.
```

Fields that are Responses API-specific with no meaningful unified equivalent
(`Prompt`, `ContextManagement`, `StreamOptions`, `SafetyIdentifier`) are **intentionally not
bridged** — callers reach them only via a `RequestTransform` on `responses.Request`. Document
this explicitly in a comment block at the top of `BuildResponsesRequest`.

---

### Phase 12 — Field-coverage documentation and round-trip tests

**Files:** `adapt/responses_request_bridge.go`, `adapt/responses_request_bridge_test.go`

This phase hardens correctness rather than adding new functionality.

**In `BuildResponsesRequest`:** Add a comment block immediately before the function listing
every field on `responses.Request` and its source:
```
// Field coverage for responses.Request:
//   model                  ← r.Model
//   input                  ← r.Messages (via item constructors)
//   instructions           ← first system message (if UseInstructions=true)
//   stream                 ← always true (streaming only)
//   tools                  ← r.Tools
//   tool_choice            ← r.ToolChoice
//   reasoning              ← r.Effort + rextras.ReasoningSummary
//   max_output_tokens      ← r.MaxTokens
//   temperature            ← r.Temperature
//   top_p                  ← r.TopP
//   metadata               ← rextras.OpenAIMetadata
//   user                   ← r.Identity.User
//   store                  ← rextras.Store
//   parallel_tool_calls    ← rextras.ParallelToolCalls
//   previous_response_id   ← rextras.PreviousResponseID
//   prompt_cache_retention ← rextras.PromptCacheRetention or r.CacheHint
//   prompt_cache_key       ← rextras.PromptCacheKey
//   text                   ← r.Output (format only)
//   service_tier           ← rextras.ServiceTier
//   truncation             ← rextras.Truncation
//   include                ← rextras.Include
//   background             ← rextras.Background
//   max_tool_calls         ← rextras.MaxToolCalls
//   top_logprobs           ← rextras.TopLogprobs
//   conversation           ← rextras.ConversationID
//   safety_identifier      NOT BRIDGED — use RequestTransform
//   prompt                 NOT BRIDGED — use RequestTransform
//   context_management     NOT BRIDGED — use RequestTransform
//   stream_options         NOT BRIDGED — use RequestTransform
```

**Round-trip test additions** in `adapt/responses_request_bridge_test.go`:
- `TestRoundTrip_AllBridgedFields` — constructs a `unified.Request` with every bridged
  `ResponsesExtras` field populated, calls `BuildResponsesRequest`, then `RequestFromResponses`,
  and asserts the result equals the original.
- `TestBuildResponsesRequest_ToolTypes` — verifies `ToolFromFunction` is correctly constructed.
- `TestBuildResponsesRequest_ToolChoiceVariants` — verifies all four `ToolChoice` variants round-
  trip.
- `TestBuildResponsesRequest_TextFormat` — verifies `OutputModeJSONObject` and
  `OutputModeJSONSchema` produce correct `Text.Format` values.

---

### Phase 13 — `Validate()` function

**File:** `api/responses/validate.go` *(new file)*

```go
// Validate validates a Request against the CreateResponse JSON Schema derived from
// the Go struct tags. Returns nil if the request is valid.
// Field-level errors are returned as a ValidationErrors slice.
func Validate(r *Request) error { ... }

type ValidationError struct {
    Field   string
    Message string
}

type ValidationErrors []ValidationError

func (e ValidationErrors) Error() string { ... }
```

Implementation:
1. Reflect `Request` into `*jsonschema.Schema` via `invopop/jsonschema` reflector — cached via
   `sync.Once` so reflection only runs once per process.
2. Marshal `r` to JSON.
3. Validate JSON against the schema using `santhosh-tekuri/jsonschema/v6`.
4. Map the validator's error output to `ValidationErrors`.

This produces zero hand-written validation rules. Every constraint comes directly from the struct
tags applied in the preceding phases.

---

## Execution Order Summary

```
Phase 0   go.mod: add invopop/jsonschema and santhosh-tekuri/jsonschema/v6                          ✅ DONE
Phase 1   api/responses/request_enums.go: all enum types + constants + JSONSchema() methods          ✅ DONE
Phase 2   api/responses/types.go: Reasoning fields → typed pointers; scalar Request fields          ✅ DONE
Phase 3   api/responses/types.go: remove MaxTokens, TopK, ResponseFormat; stub bridge               ✅ DONE
Phase 4a  api/unified/types.go: RequestMetadata → RequestIdentity, drop .Metadata bag               ✅ DONE
Phase 4b  api/unified/types.go: ResponsesExtras + CompletionsExtras: ExtraMetadata → OpenAIMetadata  ✅ DONE
Phase 4c  adapt/helpers.go: delete metadataToOpenAI/metadataFromOpenAI; add wireUser etc.            ✅ DONE
Phase 4d  adapt/responses_request_bridge.go: update both directions for identity + metadata          ✅ DONE
Phase 4e  adapt/completions_request_bridge.go: same metadata update                                 ✅ DONE
Phase 4f  adapt/messages_request_bridge.go: update to RequestIdentity                               ✅ DONE
Phase 4g  api/responses/types.go: Metadata map[string]string + Extra json:"-"; completions same     ✅ DONE
Phase 5   api/responses/types.go + request_types.go: 12 missing fields + supporting structs         ✅ DONE
Phase 6   api/responses/request_input.go: InputParam + InputItem + EasyInputMessage etc.; bridge     ✅ DONE
Phase 7   api/responses/request_tools.go: ToolParam + concrete tool structs; bridge                  ✅ DONE
Phase 8   api/responses/request_tools.go: ToolChoiceParam; bridge                                   ✅ DONE
Phase 9   api/responses/request_format.go: TextResponseFormat constructors; bridge                   ✅ DONE
Phase 10  api/responses/request_types.go: ConversationParam constructors                             ✅ DONE
Phase 11  api/unified/types.go: ResponsesExtras expansion; bridge forward/reverse coverage           ✅ DONE
Phase 12  adapt/responses_request_bridge.go: field-coverage comment; round-trip tests                ✅ DONE
Phase 13  api/responses/decode.go: DecodeRequest + RequestSchema + validation                        ✅ DONE
```

**Sequential critical path:** 0 → 1 → 2 → 3 → 4a–4g → 5 → 6 → 7 → 8 → 9 → 10 → 11 → 12 → 13

**Parallelisable after Phase 1:** Phases 2 and 3 touch different field sets in the same file —
both can be done as a single atomic edit.

**Parallelisable after Phase 5:** Phases 6, 7, 8, 9, 10 each touch a different new type and a
different bridge code path. They can be done concurrently on separate branches and merged before
Phase 11.

---

## What Does Not Change in This Pass

- All event types (`ResponseCreatedEvent`, `OutputTextDeltaEvent`, etc.) — out of scope, separate
  plan.
- `ResponsePayload`, `ResponseOutputItem`, `ResponseContentPart` — response-side types.
- `api/messages`, `api/completions`, `api/ollama` packages — untouched except where the bridge
  passes through them.
- The `conversation/` package — consumes `unified.Request`, fully insulated from wire changes.
- The `client/` package — uses `*responses.Client` and `unified.Request`; insulated. Test files
  that construct `responses.Request` literals directly will need minor field-name updates but no
  logic changes.
