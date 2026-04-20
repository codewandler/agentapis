// Package openrouter provides OpenRouter-specific helpers that sit above the generic
// protocol and conversation layers.
//
// This package is the home for OpenRouter service quirks that are not purely part
// of the generic Responses protocol model. In particular, it exposes a
// conversation.MessageProjector implementation for validating OpenRouter-specific
// replay constraints without mutating canonical conversation history.
package openrouter
