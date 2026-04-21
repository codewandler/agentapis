package responses

import "github.com/invopop/jsonschema"

// ReasoningEffort controls how much reasoning the model performs.
type ReasoningEffort string

const (
	ReasoningEffortNone    ReasoningEffort = "none"
	ReasoningEffortMinimal ReasoningEffort = "minimal"
	ReasoningEffortLow     ReasoningEffort = "low"
	ReasoningEffortMedium  ReasoningEffort = "medium"
	ReasoningEffortHigh    ReasoningEffort = "high"
	ReasoningEffortXHigh   ReasoningEffort = "xhigh"
)

func (ReasoningEffort) JSONSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Type: "string",
		Enum: []any{
			string(ReasoningEffortNone),
			string(ReasoningEffortMinimal),
			string(ReasoningEffortLow),
			string(ReasoningEffortMedium),
			string(ReasoningEffortHigh),
			string(ReasoningEffortXHigh),
		},
	}
}

// ReasoningSummary controls the detail level of reasoning summaries.
type ReasoningSummary string

const (
	ReasoningSummaryAuto     ReasoningSummary = "auto"
	ReasoningSummaryConcise  ReasoningSummary = "concise"
	ReasoningSummaryDetailed ReasoningSummary = "detailed"
)

func (ReasoningSummary) JSONSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Type: "string",
		Enum: []any{
			string(ReasoningSummaryAuto),
			string(ReasoningSummaryConcise),
			string(ReasoningSummaryDetailed),
		},
	}
}

// PromptCacheRetention controls how long prompt cache entries are retained.
type PromptCacheRetention string

const (
	PromptCacheRetentionInMemory PromptCacheRetention = "in-memory"
	PromptCacheRetention24H      PromptCacheRetention = "24h"
)

func (PromptCacheRetention) JSONSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Type: "string",
		Enum: []any{
			string(PromptCacheRetentionInMemory),
			string(PromptCacheRetention24H),
		},
	}
}

// ServiceTier specifies the processing tier for the request.
type ServiceTier string

const (
	ServiceTierAuto     ServiceTier = "auto"
	ServiceTierDefault  ServiceTier = "default"
	ServiceTierFlex     ServiceTier = "flex"
	ServiceTierScale    ServiceTier = "scale"
	ServiceTierPriority ServiceTier = "priority"
)

func (ServiceTier) JSONSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Type: "string",
		Enum: []any{
			string(ServiceTierAuto),
			string(ServiceTierDefault),
			string(ServiceTierFlex),
			string(ServiceTierScale),
			string(ServiceTierPriority),
		},
	}
}

// Truncation controls what happens when input exceeds the context window.
type Truncation string

const (
	TruncationAuto     Truncation = "auto"
	TruncationDisabled Truncation = "disabled"
)

func (Truncation) JSONSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Type: "string",
		Enum: []any{
			string(TruncationAuto),
			string(TruncationDisabled),
		},
	}
}

// IncludeItem specifies additional output data to include in the response.
type IncludeItem string

const (
	IncludeFileSearchCallResults           IncludeItem = "file_search_call.results"
	IncludeWebSearchCallResults            IncludeItem = "web_search_call.results"
	IncludeWebSearchCallActionSources      IncludeItem = "web_search_call.action.sources"
	IncludeMessageInputImageURL            IncludeItem = "message.input_image.image_url"
	IncludeComputerCallOutputImageURL      IncludeItem = "computer_call_output.output.image_url"
	IncludeCodeInterpreterCallOutputs      IncludeItem = "code_interpreter_call.outputs"
	IncludeReasoningEncryptedContent       IncludeItem = "reasoning.encrypted_content"
	IncludeMessageOutputTextLogprobs       IncludeItem = "message.output_text.logprobs"
)

func (IncludeItem) JSONSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Type: "string",
		Enum: []any{
			string(IncludeFileSearchCallResults),
			string(IncludeWebSearchCallResults),
			string(IncludeWebSearchCallActionSources),
			string(IncludeMessageInputImageURL),
			string(IncludeComputerCallOutputImageURL),
			string(IncludeCodeInterpreterCallOutputs),
			string(IncludeReasoningEncryptedContent),
			string(IncludeMessageOutputTextLogprobs),
		},
	}
}

// Verbosity controls the verbosity level of text output.
type Verbosity string

const (
	VerbosityLow    Verbosity = "low"
	VerbosityMedium Verbosity = "medium"
	VerbosityHigh   Verbosity = "high"
)

func (Verbosity) JSONSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Type: "string",
		Enum: []any{
			string(VerbosityLow),
			string(VerbosityMedium),
			string(VerbosityHigh),
		},
	}
}
