package responses

import (
	"encoding/json"

	"github.com/invopop/jsonschema"
)

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
	if len(b) == 0 {
		return nil
	}
	// First non-space byte: '"' → string variant, '[' → array variant.
	for _, c := range b {
		switch c {
		case ' ', '\t', '\n', '\r':
			continue
		case '"':
			return json.Unmarshal(b, &p.text)
		case '[':
			return json.Unmarshal(b, &p.items)
		default:
			return json.Unmarshal(b, &p.text)
		}
	}
	return nil
}

// IsText returns true if this is a string-form input.
func (p InputParam) IsText() bool { return p.items == nil }

// Text returns the text if this is a string-form input.
func (p InputParam) Text() string { return p.text }

// Items returns the items if this is an array-form input.
func (p InputParam) Items() []InputItem { return p.items }

// EasyInputMessage is the standard message form for most conversation turns.
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

// NewEasyInputMessageWithPhase constructs an EasyInputMessage with a phase, setting Type automatically.
func NewEasyInputMessageWithPhase(role string, content EasyInputContent, phase string) EasyInputMessage {
	return EasyInputMessage{Type: "message", Role: role, Content: content, Phase: &phase}
}

// EasyInputContent is string | []InputContentPart — a union type for message content.
type EasyInputContent struct {
	text  string
	parts []json.RawMessage
}

// EasyInputContentText creates a text content.
func EasyInputContentText(s string) EasyInputContent { return EasyInputContent{text: s} }

// EasyInputContentParts creates a multi-part content from raw JSON parts.
func EasyInputContentParts(p []json.RawMessage) EasyInputContent { return EasyInputContent{parts: p} }

func (c EasyInputContent) MarshalJSON() ([]byte, error) {
	if c.parts != nil {
		return json.Marshal(c.parts)
	}
	return json.Marshal(c.text)
}

func (c *EasyInputContent) UnmarshalJSON(b []byte) error {
	if len(b) == 0 {
		return nil
	}
	for _, ch := range b {
		switch ch {
		case ' ', '\t', '\n', '\r':
			continue
		case '"':
			return json.Unmarshal(b, &c.text)
		case '[':
			return json.Unmarshal(b, &c.parts)
		default:
			return json.Unmarshal(b, &c.text)
		}
	}
	return nil
}

// Text returns the text if this is a string-form content.
func (c EasyInputContent) Text() string { return c.text }

// FunctionCallOutput sends a function tool result back to the model.
type FunctionCallOutput struct {
	Type   string `json:"type"    jsonschema:"const=function_call_output"`
	CallID string `json:"call_id" jsonschema:"required"`
	Output string `json:"output"  jsonschema:"required"`
}

// NewFunctionCallOutput constructs a FunctionCallOutput, setting Type automatically.
func NewFunctionCallOutput(callID, output string) FunctionCallOutput {
	return FunctionCallOutput{Type: "function_call_output", CallID: callID, Output: output}
}

// FunctionCallInput represents a model-generated function call, used when
// replaying conversation history as input.
type FunctionCallInput struct {
	Type      string `json:"type"      jsonschema:"const=function_call"`
	CallID    string `json:"call_id"   jsonschema:"required"`
	Name      string `json:"name"      jsonschema:"required"`
	Arguments string `json:"arguments" jsonschema:"required"`
	Phase     string `json:"phase,omitempty"`
}

// NewFunctionCallInput constructs a FunctionCallInput, setting Type automatically.
func NewFunctionCallInput(callID, name, arguments string) FunctionCallInput {
	return FunctionCallInput{Type: "function_call", CallID: callID, Name: name, Arguments: arguments}
}

// InputItem wraps any valid input item variant.
type InputItem struct{ raw json.RawMessage }

// InputItemFromMessage creates an InputItem from an EasyInputMessage.
func InputItemFromMessage(m EasyInputMessage) InputItem { return mustMarshalItem(m) }

// InputItemFromFunctionOutput creates an InputItem from a FunctionCallOutput.
func InputItemFromFunctionOutput(f FunctionCallOutput) InputItem { return mustMarshalItem(f) }

// InputItemFromFunctionCall creates an InputItem from a FunctionCallInput.
func InputItemFromFunctionCall(f FunctionCallInput) InputItem { return mustMarshalItem(f) }

// InputItemRaw accepts pre-serialised JSON for uncommon item types.
func InputItemRaw(raw json.RawMessage) InputItem { return InputItem{raw: raw} }

func (i InputItem) MarshalJSON() ([]byte, error) { return i.raw, nil }

func (i *InputItem) UnmarshalJSON(b []byte) error {
	i.raw = append([]byte(nil), b...)
	return nil
}

// Raw returns the underlying JSON bytes for decoding by the bridge.
func (i InputItem) Raw() json.RawMessage { return i.raw }

// mustMarshalItem is a package-private helper that panics if marshalling fails.
func mustMarshalItem(v any) InputItem {
	b, err := json.Marshal(v)
	if err != nil {
		panic("responses: marshal input item: " + err.Error())
	}
	return InputItem{raw: b}
}

// JSONSchema returns a schema allowing string or array input.
func (InputParam) JSONSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		OneOf: []*jsonschema.Schema{
			{Type: "string"},
			{Type: "array"},
		},
	}
}

// JSONSchema returns a schema allowing any input item (object).
func (InputItem) JSONSchema() *jsonschema.Schema {
	return &jsonschema.Schema{Type: "object"}
}

// JSONSchema returns a schema allowing string or object content.
func (EasyInputContent) JSONSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		OneOf: []*jsonschema.Schema{
			{Type: "string"},
			{Type: "array"},
		},
	}
}
