package client

import (
	"context"

	"github.com/codewandler/agentapis/api/unified"
)

// StreamResult is one item from a unified streaming response pipeline.
type StreamResult struct {
	Event        unified.StreamEvent
	Err          error
	RawEventName string
	RawJSON      []byte
}

type UnifiedStreamer interface {
	Stream(ctx context.Context, req unified.Request) (<-chan StreamResult, error)
	StreamWithOptions(ctx context.Context, req unified.Request, opts StreamOptions) (<-chan StreamResult, error)
}
