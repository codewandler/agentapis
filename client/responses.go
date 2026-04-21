package client

import (
	"context"
	"fmt"

	"github.com/codewandler/agentapis/adapt"
	responsesapi "github.com/codewandler/agentapis/api/responses"
	"github.com/codewandler/agentapis/api/unified"
)

type responsesStreamer interface {
	StreamWithOptions(ctx context.Context, req responsesapi.Request, opts responsesapi.CallOptions) (<-chan responsesapi.StreamResult, error)
}

type ResponsesClient struct {
	protocol          responsesStreamer
	requestTransforms []RequestTransform
	eventTransforms   []EventTransform
	costCalculator    CostCalculator
}

func NewResponsesClient(protocol responsesStreamer, opts ...Option) *ResponsesClient {
	cfg := applyOptions(opts)
	if protocol == nil {
		protocol = responsesapi.NewClient()
	}
	return &ResponsesClient{
		protocol:          protocol,
		requestTransforms: append([]RequestTransform(nil), cfg.requestTransforms...),
		eventTransforms:   append([]EventTransform(nil), cfg.eventTransforms...),
		costCalculator:    cfg.costCalculator,
	}
}

func (c *ResponsesClient) Stream(ctx context.Context, req unified.Request) (<-chan StreamResult, error) {
	return c.StreamWithOptions(ctx, req, StreamOptions{})
}

func (c *ResponsesClient) StreamWithOptions(ctx context.Context, req unified.Request, opts StreamOptions) (<-chan StreamResult, error) {
	working := req
	if err := applyRequestTransforms(ctx, &working, c.requestTransforms); err != nil {
		return nil, fmt.Errorf("transform request: %w", err)
	}
	wire, err := adapt.BuildResponsesRequest(working)
	if err != nil {
		return nil, fmt.Errorf("build responses request: %w", err)
	}
	upstream, err := c.protocol.StreamWithOptions(ctx, *wire, responsesapi.CallOptions{
		OnRequest: func(ctx context.Context, meta responsesapi.RequestMeta) error {
			if opts.OnRequest == nil {
				return nil
			}
			return opts.OnRequest(ctx, RequestMeta{Target: TargetResponses, Wire: meta.Wire, HTTP: meta.HTTP, Body: append([]byte(nil), meta.Body...)})
		},
		OnResponse: func(ctx context.Context, meta responsesapi.ResponseMeta) error {
			if opts.OnResponse == nil {
				return nil
			}
			return opts.OnResponse(ctx, ResponseMeta{Target: TargetResponses, Wire: meta.Wire, StatusCode: meta.StatusCode, Headers: meta.Headers.Clone()})
		},
	})
	if err != nil {
		return nil, err
	}
	mapper := adapt.NewResponsesMapper()
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
			ev, ignored, err := mapper.MapEvent(item.Event)
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
