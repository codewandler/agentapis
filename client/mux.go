package client

import (
	"context"
	"fmt"

	"github.com/codewandler/agentapis/api/unified"
)

type Target int

const (
	TargetMessages Target = iota
	TargetCompletions
	TargetResponses
)

type TargetResolver func(ctx context.Context, req *unified.Request) (Target, error)

type MuxOption func(*muxConfig)

type muxConfig struct {
	messages          *MessagesClient
	completions       *CompletionsClient
	responses         *ResponsesClient
	targetResolver    TargetResolver
	requestTransforms []RequestTransform
	eventTransforms   []EventTransform
}

type MuxClient struct {
	messages          *MessagesClient
	completions       *CompletionsClient
	responses         *ResponsesClient
	targetResolver    TargetResolver
	requestTransforms []RequestTransform
	eventTransforms   []EventTransform
}

func NewMuxClient(opts ...MuxOption) *MuxClient {
	var cfg muxConfig
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	return &MuxClient{
		messages:          cfg.messages,
		completions:       cfg.completions,
		responses:         cfg.responses,
		targetResolver:    cfg.targetResolver,
		requestTransforms: append([]RequestTransform(nil), cfg.requestTransforms...),
		eventTransforms:   append([]EventTransform(nil), cfg.eventTransforms...),
	}
}

func WithMessagesClient(client *MessagesClient) MuxOption {
	return func(c *muxConfig) { c.messages = client }
}

func WithCompletionsClient(client *CompletionsClient) MuxOption {
	return func(c *muxConfig) { c.completions = client }
}

func WithResponsesClient(client *ResponsesClient) MuxOption {
	return func(c *muxConfig) { c.responses = client }
}

func WithTargetResolver(resolver TargetResolver) MuxOption {
	return func(c *muxConfig) { c.targetResolver = resolver }
}

func WithMuxRequestTransform(fn RequestTransform) MuxOption {
	return func(c *muxConfig) {
		if fn != nil {
			c.requestTransforms = append(c.requestTransforms, fn)
		}
	}
}

func WithMuxEventTransform(fn EventTransform) MuxOption {
	return func(c *muxConfig) {
		if fn != nil {
			c.eventTransforms = append(c.eventTransforms, fn)
		}
	}
}

func (c *MuxClient) Stream(ctx context.Context, req unified.Request) (<-chan StreamResult, error) {
	return c.StreamWithOptions(ctx, req, StreamOptions{})
}

func (c *MuxClient) StreamWithOptions(ctx context.Context, req unified.Request, opts StreamOptions) (<-chan StreamResult, error) {
	working := req
	if err := applyRequestTransforms(ctx, &working, c.requestTransforms); err != nil {
		return nil, fmt.Errorf("transform request: %w", err)
	}
	target, err := c.resolveTarget(ctx, &working, opts.PreferredTarget)
	if err != nil {
		return nil, err
	}

	var upstream <-chan StreamResult
	switch target {
	case TargetMessages:
		if c.messages == nil {
			return nil, fmt.Errorf("messages client is not configured")
		}
		upstream, err = c.messages.StreamWithOptions(ctx, working, opts)
	case TargetCompletions:
		if c.completions == nil {
			return nil, fmt.Errorf("completions client is not configured")
		}
		upstream, err = c.completions.StreamWithOptions(ctx, working, opts)
	case TargetResponses:
		if c.responses == nil {
			return nil, fmt.Errorf("responses client is not configured")
		}
		upstream, err = c.responses.StreamWithOptions(ctx, working, opts)
	default:
		return nil, fmt.Errorf("unsupported target %d", target)
	}
	if err != nil {
		return nil, err
	}
	out := make(chan StreamResult, 16)
	go func() {
		defer close(out)
		for item := range upstream {
			if item.Err != nil {
				out <- item
				continue
			}
			ev, ignored, err := applyEventTransforms(ctx, item.Event, c.eventTransforms)
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

func (c *MuxClient) resolveTarget(ctx context.Context, req *unified.Request, preferred *Target) (Target, error) {
	if preferred != nil {
		return *preferred, nil
	}
	if c.targetResolver != nil {
		return c.targetResolver(ctx, req)
	}
	configured := 0
	var target Target
	if c.messages != nil {
		configured++
		target = TargetMessages
	}
	if c.completions != nil {
		configured++
		target = TargetCompletions
	}
	if c.responses != nil {
		configured++
		target = TargetResponses
	}
	if configured == 1 {
		return target, nil
	}
	return 0, fmt.Errorf("target resolver is required when multiple clients are configured")
}
