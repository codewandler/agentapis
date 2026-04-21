package client

import (
	"testing"

	"github.com/codewandler/agentapis/api/unified"
)

func TestApplyCostCalculatorEnrichesUsage(t *testing.T) {
	t.Parallel()

	calc := CostCalculator(func(u unified.StreamUsage) unified.CostItems {
		return unified.CostItems{
			{Kind: unified.CostKindInput, Amount: 0.003},
			{Kind: unified.CostKindOutput, Amount: 0.015},
		}
	})

	ev := unified.StreamEvent{
		Type: unified.StreamEventUsage,
		Usage: &unified.StreamUsage{
			Tokens: unified.TokenItems{
				{Kind: unified.TokenKindInputNew, Count: 100},
				{Kind: unified.TokenKindOutput, Count: 50},
			},
		},
	}

	applyCostCalculator(&ev, calc)

	if len(ev.Usage.Costs) != 2 {
		t.Fatalf("expected 2 cost items, got %d", len(ev.Usage.Costs))
	}
	if ev.Usage.Costs.Total() != 0.018 {
		t.Fatalf("expected total cost 0.018, got %g", ev.Usage.Costs.Total())
	}
}

func TestApplyCostCalculatorSkipsNonUsageEvents(t *testing.T) {
	t.Parallel()

	called := false
	calc := CostCalculator(func(u unified.StreamUsage) unified.CostItems {
		called = true
		return unified.CostItems{{Kind: unified.CostKindInput, Amount: 1.0}}
	})

	ev := unified.StreamEvent{Type: unified.StreamEventCompleted}
	applyCostCalculator(&ev, calc)

	if called {
		t.Fatal("cost calculator should not be called for non-usage events")
	}
}

func TestApplyCostCalculatorNilCalculatorIsNoOp(t *testing.T) {
	t.Parallel()

	ev := unified.StreamEvent{
		Type:  unified.StreamEventUsage,
		Usage: &unified.StreamUsage{Tokens: unified.TokenItems{{Kind: unified.TokenKindInputNew, Count: 10}}},
	}

	applyCostCalculator(&ev, nil)

	if len(ev.Usage.Costs) != 0 {
		t.Fatalf("expected no costs with nil calculator, got %v", ev.Usage.Costs)
	}
}

func TestApplyCostCalculatorEmptyReturnPreservesCosts(t *testing.T) {
	t.Parallel()

	calc := CostCalculator(func(u unified.StreamUsage) unified.CostItems {
		return nil
	})

	ev := unified.StreamEvent{
		Type:  unified.StreamEventUsage,
		Usage: &unified.StreamUsage{Tokens: unified.TokenItems{{Kind: unified.TokenKindInputNew, Count: 10}}},
	}

	applyCostCalculator(&ev, calc)

	if len(ev.Usage.Costs) != 0 {
		t.Fatalf("expected no costs when calculator returns nil, got %v", ev.Usage.Costs)
	}
}
