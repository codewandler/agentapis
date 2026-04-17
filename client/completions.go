package client

import (
	"context"
	"fmt"

	"github.com/codewandler/agentapis/adapt"
	completionsapi "github.com/codewandler/agentapis/api/completions"
	"github.com/codewandler/agentapis/api/unified"
)

type completionsStreamer interface {
	StreamWithOptions(ctx context.Context, req completionsapi.Request, opts completionsapi.CallOptions) (<-chan completionsapi.StreamResult, error)
}

type CompletionsClient struct {
	protocol          completionsStreamer
	requestTransforms []RequestTransform
	eventTransforms   []EventTransform
}

func NewCompletionsClient(protocol completionsStreamer, opts ...Option) *CompletionsClient {
	cfg := applyOptions(opts)
	if protocol == nil {
		protocol = completionsapi.NewClient()
	}
	return &CompletionsClient{
		protocol:          protocol,
		requestTransforms: append([]RequestTransform(nil), cfg.requestTransforms...),
		eventTransforms:   append([]EventTransform(nil), cfg.eventTransforms...),
	}
}

func (c *CompletionsClient) Stream(ctx context.Context, req unified.Request) (<-chan StreamResult, error) {
	return c.StreamWithOptions(ctx, req, StreamOptions{})
}

func (c *CompletionsClient) StreamWithOptions(ctx context.Context, req unified.Request, opts StreamOptions) (<-chan StreamResult, error) {
	working := req
	if err := applyRequestTransforms(ctx, &working, c.requestTransforms); err != nil {
		return nil, fmt.Errorf("transform request: %w", err)
	}
	wire, err := adapt.BuildCompletionsRequest(working)
	if err != nil {
		return nil, fmt.Errorf("build completions request: %w", err)
	}
	upstream, err := c.protocol.StreamWithOptions(ctx, *wire, completionsapi.CallOptions{
		OnRequest: func(ctx context.Context, meta completionsapi.RequestMeta) error {
			if opts.OnRequest == nil {
				return nil
			}
			return opts.OnRequest(ctx, RequestMeta{Target: TargetCompletions, Wire: meta.Wire, HTTP: meta.HTTP, Body: append([]byte(nil), meta.Body...)})
		},
		OnResponse: func(ctx context.Context, meta completionsapi.ResponseMeta) error {
			if opts.OnResponse == nil {
				return nil
			}
			return opts.OnResponse(ctx, ResponseMeta{Target: TargetCompletions, Wire: meta.Wire, StatusCode: meta.StatusCode, Headers: meta.Headers.Clone()})
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
			ev, ignored, err := adapt.MapCompletionsEvent(item.Event)
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
			out <- StreamResult{Event: ev, RawEventName: item.RawEventName, RawJSON: append([]byte(nil), item.RawJSON...)}
		}
	}()
	return out, nil
}
