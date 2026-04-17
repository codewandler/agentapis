package client

import "github.com/codewandler/agentapis/api/unified"

// StreamResult is one item from a unified streaming response pipeline.
type StreamResult struct {
	Event        unified.StreamEvent
	Err          error
	RawEventName string
	RawJSON      []byte
}
