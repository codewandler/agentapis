package responses

import "encoding/json"

// FormatText returns a plain-text output format (the default).
func FormatText() TextResponseFormat {
	return TextResponseFormat{raw: []byte(`{"type":"text"}`)}
}

// FormatJSONObject returns a JSON-mode format (legacy; prefer FormatJSONSchema).
func FormatJSONObject() TextResponseFormat {
	return TextResponseFormat{raw: []byte(`{"type":"json_object"}`)}
}

// FormatJSONSchema returns a structured-output format with a specific schema.
// name is required; strict defaults to true.
func FormatJSONSchema(name string, schema map[string]any, strict *bool, description *string) TextResponseFormat {
	m := map[string]any{
		"type": "json_schema",
		"json_schema": map[string]any{
			"name":   name,
			"schema": schema,
		},
	}
	js := m["json_schema"].(map[string]any)
	if strict != nil {
		js["strict"] = *strict
	}
	if description != nil {
		js["description"] = *description
	}
	b, _ := json.Marshal(m)
	return TextResponseFormat{raw: b}
}

// Type returns the value of the discriminator `type` field ("text", "json_object", "json_schema").
func (f TextResponseFormat) Type() string {
	var probe struct {
		Type string `json:"type"`
	}
	if f.raw != nil {
		_ = json.Unmarshal(f.raw, &probe)
	}
	return probe.Type
}
