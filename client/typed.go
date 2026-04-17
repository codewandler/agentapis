package client

import (
	"context"

	"github.com/codewandler/agentapis/api/unified"
)

type Result[Ev any] struct {
	Event Ev
	Err   error
}

type UpstreamHints struct {
	PreferredTarget *Target
}

type StreamBridge[Req any, Ev any] interface {
	BuildRequest(ctx context.Context, req Req) (unified.Request, UpstreamHints, error)
	OnRequest(ctx context.Context, meta RequestMeta) ([]Ev, error)
	OnResponse(ctx context.Context, meta ResponseMeta) ([]Ev, error)
	OnEvent(ctx context.Context, ev unified.StreamEvent) ([]Ev, error)
	OnClose(ctx context.Context) ([]Ev, error)
}

type BridgeBuilder[Req any, Ev any] interface {
	NewBridge() StreamBridge[Req, Ev]
}

type TypedClient[Req any, Ev any] struct {
	upstream UnifiedStreamer
	builder  BridgeBuilder[Req, Ev]
}

func NewTypedClient[Req any, Ev any](upstream UnifiedStreamer, builder BridgeBuilder[Req, Ev]) *TypedClient[Req, Ev] {
	return &TypedClient[Req, Ev]{upstream: upstream, builder: builder}
}

func (c *TypedClient[Req, Ev]) Stream(ctx context.Context, req Req) (<-chan Result[Ev], error) {
	bridge := c.builder.NewBridge()
	uReq, hints, err := bridge.BuildRequest(ctx, req)
	if err != nil {
		return nil, err
	}
	out := make(chan Result[Ev], 32)
	upstream, err := c.upstream.StreamWithOptions(ctx, uReq, StreamOptions{
		PreferredTarget: hints.PreferredTarget,
		OnRequest: func(ctx context.Context, meta RequestMeta) error {
			events, err := bridge.OnRequest(ctx, meta)
			if err != nil {
				return err
			}
			for _, event := range events {
				out <- Result[Ev]{Event: event}
			}
			return nil
		},
		OnResponse: func(ctx context.Context, meta ResponseMeta) error {
			events, err := bridge.OnResponse(ctx, meta)
			if err != nil {
				return err
			}
			for _, event := range events {
				out <- Result[Ev]{Event: event}
			}
			return nil
		},
	})
	if err != nil {
		close(out)
		return out, err
	}
	go func() {
		defer close(out)
		for item := range upstream {
			if item.Err != nil {
				out <- Result[Ev]{Err: item.Err}
				return
			}
			events, err := bridge.OnEvent(ctx, item.Event)
			if err != nil {
				out <- Result[Ev]{Err: err}
				return
			}
			for _, event := range events {
				out <- Result[Ev]{Event: event}
			}
		}
		events, err := bridge.OnClose(ctx)
		if err != nil {
			out <- Result[Ev]{Err: err}
			return
		}
		for _, event := range events {
			out <- Result[Ev]{Event: event}
		}
	}()
	return out, nil
}
