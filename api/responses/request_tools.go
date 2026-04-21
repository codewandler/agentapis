package responses

import (
	"encoding/json"

	"github.com/invopop/jsonschema"
)

// === Concrete tool structs ===

// FunctionTool defines a function that the model can call.
type FunctionTool struct {
	Type         string  `json:"type"                  jsonschema:"const=function"`
	Name         string  `json:"name"                  jsonschema:"required"`
	Description  *string `json:"description,omitempty"`
	Parameters   any     `json:"parameters,omitempty"  jsonschema:"description=JSON Schema describing the function parameters"`
	Strict       *bool   `json:"strict,omitempty"      jsonschema:"description=Enforce strict parameter validation. Default true"`
	DeferLoading bool    `json:"defer_loading,omitempty"`
}

// NewFunctionTool constructs a FunctionTool, setting Type automatically.
func NewFunctionTool(name string, description *string, parameters any, strict *bool) FunctionTool {
	return FunctionTool{Type: "function", Name: name, Description: description, Parameters: parameters, Strict: strict}
}

// FileSearchTool enables file search over vector stores.
type FileSearchTool struct {
	Type           string   `json:"type"                      jsonschema:"const=file_search"`
	VectorStoreIDs []string `json:"vector_store_ids,omitempty"`
	MaxNumResults  *int     `json:"max_num_results,omitempty"  jsonschema:"minimum=1,maximum=50"`
}

// WebSearchTool enables web search.
type WebSearchTool struct {
	Type              string  `json:"type"                          jsonschema:"const=web_search_preview"`
	SearchContextSize *string `json:"search_context_size,omitempty" jsonschema:"enum=low,enum=medium,enum=high"`
}

// MCPTool connects to an MCP server.
type MCPTool struct {
	Type         string   `json:"type"         jsonschema:"const=mcp"`
	ServerLabel  string   `json:"server_label" jsonschema:"required"`
	ServerURL    string   `json:"server_url,omitempty"`
	AllowedTools []string `json:"allowed_tools,omitempty"`
}

// CodeInterpreterTool enables code execution.
type CodeInterpreterTool struct {
	Type      string  `json:"type"              jsonschema:"const=code_interpreter"`
	Container *string `json:"container,omitempty"`
}

// ImageGenTool enables image generation.
type ImageGenTool struct {
	Type string `json:"type" jsonschema:"const=image_generation"`
}

// === ToolParam wrapper ===

// ToolParam wraps any Tool variant for use in a request.
type ToolParam struct{ raw json.RawMessage }

// ToolFromFunction creates a ToolParam from a FunctionTool.
func ToolFromFunction(t FunctionTool) ToolParam { return mustMarshalTool(t) }

// ToolFromFileSearch creates a ToolParam from a FileSearchTool.
func ToolFromFileSearch(t FileSearchTool) ToolParam { return mustMarshalTool(t) }

// ToolFromWebSearch creates a ToolParam from a WebSearchTool.
func ToolFromWebSearch(t WebSearchTool) ToolParam { return mustMarshalTool(t) }

// ToolFromMCP creates a ToolParam from an MCPTool.
func ToolFromMCP(t MCPTool) ToolParam { return mustMarshalTool(t) }

// ToolFromCodeInterpreter creates a ToolParam from a CodeInterpreterTool.
func ToolFromCodeInterpreter(t CodeInterpreterTool) ToolParam { return mustMarshalTool(t) }

// ToolFromImageGen creates a ToolParam from an ImageGenTool.
func ToolFromImageGen(t ImageGenTool) ToolParam { return mustMarshalTool(t) }

// ToolRaw accepts pre-serialised JSON for tool types without a concrete struct.
func ToolRaw(raw json.RawMessage) ToolParam { return ToolParam{raw: raw} }

func (t ToolParam) MarshalJSON() ([]byte, error) { return t.raw, nil }

func (t *ToolParam) UnmarshalJSON(b []byte) error {
	t.raw = append([]byte(nil), b...)
	return nil
}

// Raw returns the underlying JSON bytes for use by the bridge decoder.
func (t ToolParam) Raw() json.RawMessage { return t.raw }

// Type returns the value of the discriminator `type` field.
func (t ToolParam) Type() string {
	var probe struct {
		Type string `json:"type"`
	}
	_ = json.Unmarshal(t.raw, &probe)
	return probe.Type
}

func mustMarshalTool(v any) ToolParam {
	b, err := json.Marshal(v)
	if err != nil {
		panic("responses: marshal tool: " + err.Error())
	}
	return ToolParam{raw: b}
}

// === ToolChoiceParam ===

// ToolChoiceParam controls which tool(s) the model calls.
// Use ToolChoiceAuto(), ToolChoiceRequired(), ToolChoiceNone(), or ToolChoiceForFunction() to construct.
type ToolChoiceParam struct{ raw json.RawMessage }

// ToolChoiceAuto lets the model decide whether to call tools.
func ToolChoiceAuto() ToolChoiceParam { return ToolChoiceParam{raw: []byte(`"auto"`)} }

// ToolChoiceRequired forces the model to call at least one tool.
func ToolChoiceRequired() ToolChoiceParam { return ToolChoiceParam{raw: []byte(`"required"`)} }

// ToolChoiceNone prevents the model from calling any tools.
func ToolChoiceNone() ToolChoiceParam { return ToolChoiceParam{raw: []byte(`"none"`)} }

// ToolChoiceForFunction forces the model to call a specific function.
func ToolChoiceForFunction(name string) ToolChoiceParam {
	b, _ := json.Marshal(map[string]string{"type": "function", "name": name})
	return ToolChoiceParam{raw: b}
}

// ToolChoiceForMCP forces the model to call a tool from a specific MCP server.
func ToolChoiceForMCP(serverLabel string, name *string) ToolChoiceParam {
	m := map[string]any{"type": "mcp", "server_label": serverLabel}
	if name != nil {
		m["name"] = *name
	}
	b, _ := json.Marshal(m)
	return ToolChoiceParam{raw: b}
}

// ToolChoiceAllowed restricts which tools the model can call.
func ToolChoiceAllowed(mode string, tools []any) ToolChoiceParam {
	b, _ := json.Marshal(map[string]any{"type": "allowed_tools", "mode": mode, "tools": tools})
	return ToolChoiceParam{raw: b}
}

// AsString returns the string value if this is a string-form choice ("auto", "required", "none").
func (tc ToolChoiceParam) AsString() (string, bool) {
	var s string
	if err := json.Unmarshal(tc.raw, &s); err != nil {
		return "", false
	}
	return s, true
}

// AsObject returns the raw map if this is an object-form choice.
func (tc ToolChoiceParam) AsObject() (map[string]any, bool) {
	var m map[string]any
	if err := json.Unmarshal(tc.raw, &m); err != nil {
		return nil, false
	}
	return m, true
}

func (tc ToolChoiceParam) MarshalJSON() ([]byte, error) {
	if tc.raw == nil {
		return []byte("null"), nil
	}
	return tc.raw, nil
}

func (tc *ToolChoiceParam) UnmarshalJSON(b []byte) error {
	tc.raw = append([]byte(nil), b...)
	return nil
}

// JSONSchema returns a schema allowing any tool definition (object).
func (ToolParam) JSONSchema() *jsonschema.Schema {
	return &jsonschema.Schema{Type: "object"}
}

// JSONSchema returns a schema for tool choice: string or object.
func (ToolChoiceParam) JSONSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		OneOf: []*jsonschema.Schema{
			{Type: "string", Enum: []any{"none", "auto", "required"}},
			{Type: "object"},
		},
	}
}
