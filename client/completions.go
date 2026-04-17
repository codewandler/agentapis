package client

import (
	"context"
	"fmt"

	"github.com/codewandler/agentapis/adapt"
	completionsapi "github.com/codewandler/agentapis/api/completions"
	"github.com/codewandler/agentapis/api/unified"
)

type completionsStreamer interface {
	Stream(ctx context.Context, req completionsapi.Request) (<-chan completionsapi.StreamResult, error)
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
	working := req
	if err := applyRequestTransforms(ctx, &working, c.requestTransforms); err != nil {
		return nil, fmt.Errorf("transform request: %w", err)
	}
	wire, err := adapt.BuildCompletionsRequest(working)
	if err != nil {
		return nil, fmt.Errorf("build completions request: %w", err)
	}
	upstream, err := c.protocol.Stream(ctx, *wire)
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
