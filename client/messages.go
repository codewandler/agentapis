package client

import (
	"context"
	"fmt"

	"github.com/codewandler/agentapis/adapt"
	messagesapi "github.com/codewandler/agentapis/api/messages"
	"github.com/codewandler/agentapis/api/unified"
)

type messagesStreamer interface {
	StreamWithOptions(ctx context.Context, req messagesapi.Request, opts messagesapi.CallOptions) (<-chan messagesapi.StreamResult, error)
}

type MessagesClient struct {
	protocol          messagesStreamer
	requestTransforms []RequestTransform
	eventTransforms   []EventTransform
	costCalculator    CostCalculator
}

func NewMessagesClient(protocol messagesStreamer, opts ...Option) *MessagesClient {
	cfg := applyOptions(opts)
	if protocol == nil {
		protocol = messagesapi.NewClient()
	}
	return &MessagesClient{
		protocol:          protocol,
		requestTransforms: append([]RequestTransform(nil), cfg.requestTransforms...),
		eventTransforms:   append([]EventTransform(nil), cfg.eventTransforms...),
		costCalculator:    cfg.costCalculator,
	}
}

func (c *MessagesClient) Stream(ctx context.Context, req unified.Request) (<-chan StreamResult, error) {
	return c.StreamWithOptions(ctx, req, StreamOptions{})
}

func (c *MessagesClient) StreamWithOptions(ctx context.Context, req unified.Request, opts StreamOptions) (<-chan StreamResult, error) {
	working := req
	if err := applyRequestTransforms(ctx, &working, c.requestTransforms); err != nil {
		return nil, fmt.Errorf("transform request: %w", err)
	}
	wire, err := adapt.BuildMessagesRequest(working)
	if err != nil {
		return nil, fmt.Errorf("build messages request: %w", err)
	}
	upstream, err := c.protocol.StreamWithOptions(ctx, *wire, messagesapi.CallOptions{
		OnRequest: func(ctx context.Context, meta messagesapi.RequestMeta) error {
			if opts.OnRequest == nil {
				return nil
			}
			return opts.OnRequest(ctx, RequestMeta{Target: TargetMessages, Wire: meta.Wire, HTTP: meta.HTTP, Body: append([]byte(nil), meta.Body...)})
		},
		OnResponse: func(ctx context.Context, meta messagesapi.ResponseMeta) error {
			if opts.OnResponse == nil {
				return nil
			}
			return opts.OnResponse(ctx, ResponseMeta{Target: TargetMessages, Wire: meta.Wire, StatusCode: meta.StatusCode, Headers: meta.Headers.Clone()})
		},
	})
	if err != nil {
		return nil, err
	}
	out := make(chan StreamResult, 16)
	go func() {
		defer close(out)
		for item := range upstream {
			if item.Err != nil {
				out <- StreamResult{Err: item.Err, RawEventName: item.RawEventName, RawJSON: append([]byte(nil), item.RawJSON...)}
				continue
			}
			if item.Event == nil {
				continue
			}
			ev, ignored, err := adapt.MapMessagesEvent(item.Event)
			if err != nil {
				out <- StreamResult{Err: err, RawEventName: item.RawEventName, RawJSON: append([]byte(nil), item.RawJSON...)}
				continue
			}
			if ignored {
				continue
			}
			ev, ignored, err = applyEventTransforms(ctx, ev, c.eventTransforms)
			if err != nil {
				out <- StreamResult{Err: err, RawEventName: item.RawEventName, RawJSON: append([]byte(nil), item.RawJSON...)}
				continue
			}
			if ignored {
				continue
			}
			applyCostCalculator(&ev, c.costCalculator)
			out <- StreamResult{Event: ev, RawEventName: item.RawEventName, RawJSON: append([]byte(nil), item.RawJSON...)}
		}
	}()
	return out, nil
}
