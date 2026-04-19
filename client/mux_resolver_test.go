package client

import (
	"context"
	"testing"

	"github.com/codewandler/agentapis/api/unified"
)

func TestTargetString(t *testing.T) {
	t.Parallel()
	if TargetOllama.String() != "ollama" {
		t.Fatalf("unexpected target string: %q", TargetOllama.String())
	}
}

func TestDefaultTargetResolverUsesProviderHint(t *testing.T) {
	t.Parallel()
	target, err := DefaultTargetResolver(context.Background(), &unified.Request{Model: "qwen3", Extras: unified.RequestExtras{Provider: map[string]any{"target": "ollama"}}})
	if err != nil {
		t.Fatalf("DefaultTargetResolver() error = %v", err)
	}
	if target != TargetOllama {
		t.Fatalf("expected ollama target, got %v", target)
	}
}

func TestDefaultTargetResolverUsesModelPrefix(t *testing.T) {
	t.Parallel()
	target, err := DefaultTargetResolver(context.Background(), &unified.Request{Model: "ollama/qwen3"})
	if err != nil {
		t.Fatalf("DefaultTargetResolver() error = %v", err)
	}
	if target != TargetOllama {
		t.Fatalf("expected ollama target, got %v", target)
	}
}
