package responses

import (
	"encoding/json"

	"github.com/invopop/jsonschema"
)

// ResponseTextParam configures text output format and verbosity.
type ResponseTextParam struct {
	Format    *TextResponseFormat `json:"format,omitempty"`
	Verbosity *Verbosity          `json:"verbosity,omitempty"`
}

// TextResponseFormat is a union type for text output format configuration.
// Use FormatText(), FormatJSONObject(), or FormatJSONSchema() to construct.
type TextResponseFormat struct{ raw json.RawMessage }

func (f TextResponseFormat) MarshalJSON() ([]byte, error) {
	if f.raw == nil {
		return []byte("null"), nil
	}
	return f.raw, nil
}

func (f *TextResponseFormat) UnmarshalJSON(b []byte) error {
	f.raw = append([]byte(nil), b...)
	return nil
}

// ConversationParam links a response to a conversation.
// Use ConversationByID() or ConversationObject() to construct.
type ConversationParam struct{ raw json.RawMessage }

func (c ConversationParam) MarshalJSON() ([]byte, error) {
	if c.raw == nil {
		return []byte("null"), nil
	}
	return c.raw, nil
}

func (c *ConversationParam) UnmarshalJSON(b []byte) error {
	c.raw = append([]byte(nil), b...)
	return nil
}

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

// ConversationByID creates a ConversationParam from a bare conversation ID string.
func ConversationByID(id string) ConversationParam {
	b, _ := json.Marshal(id)
	return ConversationParam{raw: b}
}

// ConversationObject creates a ConversationParam from the object form.
func ConversationObject(id string) ConversationParam {
	b, _ := json.Marshal(map[string]string{"id": id})
	return ConversationParam{raw: b}
}

func ptrBool(b bool) *bool { return &b }

// JSONSchema returns a schema for text format configuration (object with type discriminator).
func (TextResponseFormat) JSONSchema() *jsonschema.Schema {
	return &jsonschema.Schema{Type: "object"}
}

// JSONSchema returns a schema for conversation param: string or object.
func (ConversationParam) JSONSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		OneOf: []*jsonschema.Schema{
			{Type: "string"},
			{Type: "object"},
		},
	}
}
