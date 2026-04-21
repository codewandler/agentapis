package responses

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	ijsonschema "github.com/invopop/jsonschema"
	jschema "github.com/santhosh-tekuri/jsonschema/v6"
)

var (
	compiledSchema     *jschema.Schema
	compiledSchemaOnce sync.Once
	compiledSchemaErr  error
)

func compileRequestSchema() (*jschema.Schema, error) {
	compiledSchemaOnce.Do(func() {
		// Reflect the Go Request struct into a JSON Schema document.
		r := &ijsonschema.Reflector{
			Anonymous:                 true,
			DoNotReference:            true,
			AllowAdditionalProperties: true,
		}
		goSchema := r.Reflect(&Request{})

		// Serialize to JSON and back to map[string]any for the validator.
		schemaBytes, err := json.Marshal(goSchema)
		if err != nil {
			compiledSchemaErr = fmt.Errorf("marshal reflected schema: %w", err)
			return
		}
		var schemaDoc any
		if err := json.Unmarshal(schemaBytes, &schemaDoc); err != nil {
			compiledSchemaErr = fmt.Errorf("unmarshal reflected schema: %w", err)
			return
		}

		// Compile with santhosh-tekuri validator.
		c := jschema.NewCompiler()
		if err := c.AddResource("request.json", schemaDoc); err != nil {
			compiledSchemaErr = fmt.Errorf("add schema resource: %w", err)
			return
		}
		compiledSchema, compiledSchemaErr = c.Compile("request.json")
	})
	return compiledSchema, compiledSchemaErr
}

// DecodeRequest unmarshals JSON bytes into a Request, validates the result
// against the JSON Schema derived from the Request struct, and returns the
// decoded request or a validation error.
//
// The JSON Schema is compiled once on first call and cached for the process
// lifetime.
func DecodeRequest(data []byte) (Request, error) {
	// First unmarshal to any for schema validation.
	var doc any
	if err := json.Unmarshal(data, &doc); err != nil {
		return Request{}, fmt.Errorf("decode request JSON: %w", err)
	}

	sch, err := compileRequestSchema()
	if err != nil {
		return Request{}, fmt.Errorf("compile request schema: %w", err)
	}

	if err := sch.Validate(doc); err != nil {
		var verr *jschema.ValidationError
		if ok := asValidationError(err, &verr); ok {
			return Request{}, &RequestValidationError{Causes: flattenValidationErrors(verr)}
		}
		return Request{}, fmt.Errorf("validate request: %w", err)
	}

	// Schema is valid — unmarshal into the typed struct.
	var req Request
	if err := json.Unmarshal(data, &req); err != nil {
		return Request{}, fmt.Errorf("decode request: %w", err)
	}
	return req, nil
}

// RequestSchema returns the JSON Schema for the Request struct as raw JSON.
// Useful for documentation, code generation, or external validators.
func RequestSchema() (json.RawMessage, error) {
	r := &ijsonschema.Reflector{
		Anonymous:                 true,
		DoNotReference:            true,
		AllowAdditionalProperties: true,
	}
	goSchema := r.Reflect(&Request{})
	b, err := json.MarshalIndent(goSchema, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal request schema: %w", err)
	}
	return b, nil
}

// RequestValidationError is returned when a request fails JSON Schema validation.
type RequestValidationError struct {
	Causes []ValidationCause
}

// ValidationCause describes a single schema validation failure.
type ValidationCause struct {
	Path    string // JSON Pointer path to the failing field (e.g. "/temperature").
	Message string // Human-readable validation message.
}

func (e *RequestValidationError) Error() string {
	if len(e.Causes) == 0 {
		return "request validation failed"
	}
	var b strings.Builder
	b.WriteString("request validation failed:")
	for _, c := range e.Causes {
		b.WriteString("\n  ")
		if c.Path != "" {
			b.WriteString(c.Path)
			b.WriteString(": ")
		}
		b.WriteString(c.Message)
	}
	return b.String()
}

// asValidationError attempts to unwrap a santhosh-tekuri ValidationError.
func asValidationError(err error, target **jschema.ValidationError) bool {
	if ve, ok := err.(*jschema.ValidationError); ok {
		*target = ve
		return true
	}
	return false
}

// flattenValidationErrors recursively flattens nested validation errors into a
// flat list of causes with JSON pointer paths.
func flattenValidationErrors(ve *jschema.ValidationError) []ValidationCause {
	var causes []ValidationCause
	flattenVE(ve, &causes)
	return causes
}

func flattenVE(ve *jschema.ValidationError, out *[]ValidationCause) {
	if len(ve.Causes) == 0 {
		loc := "/" + strings.Join(ve.InstanceLocation, "/")
		if loc == "/" {
			loc = ""
		}
		var msg string
		if ve.ErrorKind != nil {
			msg = ve.Error()
		}
		*out = append(*out, ValidationCause{Path: loc, Message: msg})
		return
	}
	for _, child := range ve.Causes {
		flattenVE(child, out)
	}
}
