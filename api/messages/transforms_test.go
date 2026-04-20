package messages

import (
	"context"
	"testing"
)

func TestAutoSystemCacheControlAddsToLastBlock(t *testing.T) {
	req := &Request{
		System: []*TextBlock{
			{Type: "text", Text: "First system block"},
			{Type: "text", Text: "Second system block"},
			{Type: "text", Text: "Third system block"},
		},
	}
	transform := AutoSystemCacheControl()
	if err := transform(context.Background(), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// First two blocks should not have cache control
	if req.System[0].CacheControl != nil {
		t.Errorf("first block should not have cache control, got %#v", req.System[0].CacheControl)
	}
	if req.System[1].CacheControl != nil {
		t.Errorf("second block should not have cache control, got %#v", req.System[1].CacheControl)
	}

	// Last block should have cache control
	if req.System[2].CacheControl == nil {
		t.Fatal("last block should have cache control")
	}
	if req.System[2].CacheControl.Type != "ephemeral" {
		t.Errorf("expected type 'ephemeral', got %q", req.System[2].CacheControl.Type)
	}
}

func TestAutoSystemCacheControlWithTTLSetsTTL(t *testing.T) {
	req := &Request{
		System: []*TextBlock{
			{Type: "text", Text: "System block"},
		},
	}
	transform := AutoSystemCacheControlWithTTL("1h")
	if err := transform(context.Background(), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if req.System[0].CacheControl == nil {
		t.Fatal("expected cache control to be set")
	}
	if req.System[0].CacheControl.Type != "ephemeral" {
		t.Errorf("expected type 'ephemeral', got %q", req.System[0].CacheControl.Type)
	}
	if req.System[0].CacheControl.TTL != "1h" {
		t.Errorf("expected TTL '1h', got %q", req.System[0].CacheControl.TTL)
	}
}

func TestAutoSystemCacheControlSkipsEmptySystem(t *testing.T) {
	req := &Request{System: nil}
	transform := AutoSystemCacheControl()
	if err := transform(context.Background(), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should not panic or error
}

func TestAutoSystemCacheControlSkipsNilBlock(t *testing.T) {
	req := &Request{
		System: []*TextBlock{nil},
	}
	transform := AutoSystemCacheControl()
	if err := transform(context.Background(), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should not panic or error
}

func TestAutoSystemCacheControlDoesNotOverwriteExisting(t *testing.T) {
	req := &Request{
		System: []*TextBlock{
			{Type: "text", Text: "System block", CacheControl: &CacheControl{Type: "custom", TTL: "30m"}},
		},
	}
	transform := AutoSystemCacheControl()
	if err := transform(context.Background(), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should preserve existing cache control
	if req.System[0].CacheControl.Type != "custom" {
		t.Errorf("expected type 'custom', got %q", req.System[0].CacheControl.Type)
	}
	if req.System[0].CacheControl.TTL != "30m" {
		t.Errorf("expected TTL '30m', got %q", req.System[0].CacheControl.TTL)
	}
}
