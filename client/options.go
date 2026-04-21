package client

import (
	"context"

	"github.com/codewandler/agentapis/api/unified"
)

type RequestTransform func(ctx context.Context, req *unified.Request) error
type EventTransform func(ctx context.Context, ev unified.StreamEvent) (unified.StreamEvent, bool, error)

// CostCalculator derives monetary costs from a usage snapshot.
// It is called once per usage event with the fully-populated StreamUsage.
// Returning nil means no cost data is available for this event.
type CostCalculator func(usage unified.StreamUsage) unified.CostItems

type Option func(*config)

type config struct {
	requestTransforms []RequestTransform
	eventTransforms   []EventTransform
	costCalculator    CostCalculator
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

// WithCostCalculator injects a function that derives monetary costs from usage data.
// When set, every usage event in the stream is enriched with the returned CostItems
// before being forwarded to the consumer. The calculator is called after event transforms.
func WithCostCalculator(fn CostCalculator) Option {
	return func(c *config) { c.costCalculator = fn }
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

// applyCostCalculator enriches a usage event with cost data when a calculator is configured.
func applyCostCalculator(ev *unified.StreamEvent, calc CostCalculator) {
	if calc == nil || ev.Usage == nil {
		return
	}
	if costs := calc(*ev.Usage); len(costs) > 0 {
		ev.Usage.Costs = costs
	}
}
