package client

import (
	"context"

	"github.com/codewandler/agentapis/api/unified"
)

type RequestTransform func(ctx context.Context, req *unified.Request) error
type EventTransform func(ctx context.Context, ev unified.StreamEvent) (unified.StreamEvent, bool, error)

type Option func(*config)

type config struct {
	requestTransforms []RequestTransform
	eventTransforms   []EventTransform
}

func applyOptions(opts []Option) config {
	var cfg config
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	return cfg
}

func WithRequestTransform(fn RequestTransform) Option {
	return func(c *config) {
		if fn != nil {
			c.requestTransforms = append(c.requestTransforms, fn)
		}
	}
}

func WithEventTransform(fn EventTransform) Option {
	return func(c *config) {
		if fn != nil {
			c.eventTransforms = append(c.eventTransforms, fn)
		}
	}
}

func applyRequestTransforms(ctx context.Context, req *unified.Request, transforms []RequestTransform) error {
	for _, transform := range transforms {
		if err := transform(ctx, req); err != nil {
			return err
		}
	}
	return nil
}

func applyEventTransforms(ctx context.Context, ev unified.StreamEvent, transforms []EventTransform) (unified.StreamEvent, bool, error) {
	for _, transform := range transforms {
		next, ignored, err := transform(ctx, ev)
		if err != nil {
			return unified.StreamEvent{}, false, err
		}
		if ignored {
			return unified.StreamEvent{}, true, nil
		}
		ev = next
	}
	return ev, false, nil
}
