package adapt

import "strings"

type ModelCaps struct {
	SupportsAdaptiveThinking bool
	SupportsEffort           bool
	SupportsMaxEffort        bool
	DefaultThinkingDisplay   string
}

type ModelCapsFunc func(model string) ModelCaps

var DefaultAnthropicMessagesModelCaps ModelCapsFunc = func(model string) ModelCaps {
	isNew := strings.Contains(model, "claude-sonnet-4-6") ||
		strings.Contains(model, "claude-opus-4-6")

	effortOnly := strings.Contains(model, "claude-opus-4-5")

	return ModelCaps{
		SupportsAdaptiveThinking: isNew,
		SupportsEffort:           isNew || effortOnly,
		SupportsMaxEffort:        isNew,
	}
}
