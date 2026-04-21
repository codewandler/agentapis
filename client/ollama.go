package client

import (
	"context"
	"fmt"

	"github.com/codewandler/agentapis/adapt"
	ollamaapi "github.com/codewandler/agentapis/api/ollama"
	"github.com/codewandler/agentapis/api/unified"
)

type ollamaStreamer interface {
	StreamWithOptions(ctx context.Context, req ollamaapi.Request, opts ollamaapi.CallOptions) (<-chan ollamaapi.StreamResult, error)
	ListModels(ctx context.Context) (*ollamaapi.TagsResponse, error)
}

type OllamaClient struct {
	protocol          ollamaStreamer
	requestTransforms []RequestTransform
	eventTransforms   []EventTransform
	costCalculator    CostCalculator
}

func NewOllamaClient(protocol ollamaStreamer, opts ...Option) *OllamaClient {
	cfg := applyOptions(opts)
	if protocol == nil {
		protocol = ollamaapi.NewClient()
	}
	return &OllamaClient{protocol: protocol, requestTransforms: append([]RequestTransform(nil), cfg.requestTransforms...), eventTransforms: append([]EventTransform(nil), cfg.eventTransforms...), costCalculator: cfg.costCalculator}
}

func (c *OllamaClient) ListModels(ctx context.Context) (*ollamaapi.TagsResponse, error) {
	if c.protocol == nil {
		return nil, fmt.Errorf("ollama protocol is not configured")
	}
	return c.protocol.ListModels(ctx)
}

func (c *OllamaClient) Stream(ctx context.Context, req unified.Request) (<-chan StreamResult, error) {
	return c.StreamWithOptions(ctx, req, StreamOptions{})
}

func (c *OllamaClient) StreamWithOptions(ctx context.Context, req unified.Request, opts StreamOptions) (<-chan StreamResult, error) {
	working := req
	if err := applyRequestTransforms(ctx, &working, c.requestTransforms); err != nil {
		return nil, fmt.Errorf("transform request: %w", err)
	}
	wire, err := adapt.BuildOllamaRequest(working)
	if err != nil {
		return nil, fmt.Errorf("build ollama request: %w", err)
	}
	upstream, err := c.protocol.StreamWithOptions(ctx, *wire, ollamaapi.CallOptions{
		OnRequest: func(ctx context.Context, meta ollamaapi.RequestMeta) error {
			if opts.OnRequest == nil {
				return nil
			}
			return opts.OnRequest(ctx, RequestMeta{Target: TargetOllama, Wire: meta.Wire, HTTP: meta.HTTP, Body: append([]byte(nil), meta.Body...)})
		},
		OnResponse: func(ctx context.Context, meta ollamaapi.ResponseMeta) error {
			if opts.OnResponse == nil {
				return nil
			}
			return opts.OnResponse(ctx, ResponseMeta{Target: TargetOllama, Wire: meta.Wire, StatusCode: meta.StatusCode, Headers: meta.Headers.Clone()})
		},
	})
	if err != nil {
		return nil, err
	}
	mapper := adapt.NewOllamaMapper()
	out := make(chan StreamResult, 16)
	go func() {
		defer close(out)
		for item := range upstream {
			if item.Err != nil {
				out <- StreamResult{Err: item.Err, RawJSON: append([]byte(nil), item.RawJSON...)}
				continue
			}
			if item.Event == nil {
				continue
			}
			ev, ignored, err := mapper.MapEvent(item.Event)
			if err != nil {
				out <- StreamResult{Err: err, RawJSON: append([]byte(nil), item.RawJSON...)}
				continue
			}
			if ignored {
				continue
			}
			ev, ignored, err = applyEventTransforms(ctx, ev, c.eventTransforms)
			if err != nil {
				out <- StreamResult{Err: err, RawJSON: append([]byte(nil), item.RawJSON...)}
				continue
			}
			if ignored {
				continue
			}
			applyCostCalculator(&ev, c.costCalculator)
			out <- StreamResult{Event: ev, RawJSON: append([]byte(nil), item.RawJSON...)}
		}
	}()
	return out, nil
}
