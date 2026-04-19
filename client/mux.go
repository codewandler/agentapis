package client

import (
	"context"
	"fmt"
	"strings"

	"github.com/codewandler/agentapis/api/unified"
)

type Target int

const (
	TargetMessages Target = iota
	TargetCompletions
	TargetResponses
	TargetOllama
)

type TargetResolver func(ctx context.Context, req *unified.Request) (Target, error)

func (t Target) String() string {
	switch t {
	case TargetMessages:
		return "messages"
	case TargetCompletions:
		return "completions"
	case TargetResponses:
		return "responses"
	case TargetOllama:
		return "ollama"
	default:
		return fmt.Sprintf("target(%d)", int(t))
	}
}

// DefaultTargetResolver is an opt-in heuristic resolver for common provider/model hints.
// It prefers explicit provider hints in req.Extras.Provider and otherwise uses model prefixes
// where those prefixes are unambiguous.
func DefaultTargetResolver(_ context.Context, req *unified.Request) (Target, error) {
	if req == nil {
		return 0, fmt.Errorf("request is nil")
	}
	if req.Extras.Provider != nil {
		for _, key := range []string{"target", "provider", "api"} {
			if raw, ok := req.Extras.Provider[key]; ok {
				if s, ok := raw.(string); ok {
					switch strings.ToLower(strings.TrimSpace(s)) {
					case "messages", "anthropic":
						return TargetMessages, nil
					case "completions", "chat_completions":
						return TargetCompletions, nil
					case "responses", "openai":
						return TargetResponses, nil
					case "ollama":
						return TargetOllama, nil
					}
				}
			}
		}
	}
	model := strings.ToLower(strings.TrimSpace(req.Model))
	switch {
	case strings.HasPrefix(model, "anthropic/") || strings.HasPrefix(model, "claude"):
		return TargetMessages, nil
	case strings.HasPrefix(model, "ollama/"):
		return TargetOllama, nil
	default:
		return 0, fmt.Errorf("default target resolver could not infer target for model %q", req.Model)
	}
}


type MuxOption func(*muxConfig)

type muxConfig struct {
	messages          *MessagesClient
	completions       *CompletionsClient
	responses         *ResponsesClient
	ollama            *OllamaClient
	targetResolver    TargetResolver
	requestTransforms []RequestTransform
	eventTransforms   []EventTransform
}

type MuxClient struct {
	messages          *MessagesClient
	completions       *CompletionsClient
	responses         *ResponsesClient
	ollama            *OllamaClient
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
		ollama:            cfg.ollama,
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

func WithOllamaClient(client *OllamaClient) MuxOption {
	return func(c *muxConfig) { c.ollama = client }
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
	case TargetOllama:
		if c.ollama == nil {
			return nil, fmt.Errorf("ollama client is not configured")
		}
		upstream, err = c.ollama.StreamWithOptions(ctx, working, opts)
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
	if c.ollama != nil {
		configured++
		target = TargetOllama
	}
	if configured == 1 {
		return target, nil
	}
	return 0, fmt.Errorf("target resolver is required when multiple clients are configured")
}
