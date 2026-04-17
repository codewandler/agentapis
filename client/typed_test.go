package client

import (
	"context"
	"net/http"
	"testing"

	"github.com/codewandler/agentapis/api/unified"
)

type fakeUnifiedStreamer struct {
	lastReq  unified.Request
	lastOpts StreamOptions
	results  []StreamResult
}

func (f *fakeUnifiedStreamer) Stream(ctx context.Context, req unified.Request) (<-chan StreamResult, error) {
	return f.StreamWithOptions(ctx, req, StreamOptions{})
}

func (f *fakeUnifiedStreamer) StreamWithOptions(ctx context.Context, req unified.Request, opts StreamOptions) (<-chan StreamResult, error) {
	f.lastReq = req
	f.lastOpts = opts
	if opts.OnRequest != nil {
		if err := opts.OnRequest(ctx, RequestMeta{Target: TargetResponses, HTTP: &http.Request{Method: http.MethodPost}, Body: []byte(`{"ok":true}`)}); err != nil {
			return nil, err
		}
	}
	if opts.OnResponse != nil {
		if err := opts.OnResponse(ctx, ResponseMeta{Target: TargetResponses, StatusCode: http.StatusOK, Headers: http.Header{"X-Test": {"yes"}}}); err != nil {
			return nil, err
		}
	}
	return NewTestStream(f.results...), nil
}

type fakeBridgeBuilder struct{}

func (fakeBridgeBuilder) NewBridge() StreamBridge[string, string] { return &fakeBridge{} }

type fakeBridge struct{}

func (*fakeBridge) BuildRequest(_ context.Context, req string) (unified.Request, UpstreamHints, error) {
	target := TargetResponses
	return unified.Request{Model: req}, UpstreamHints{PreferredTarget: &target}, nil
}

func (*fakeBridge) OnRequest(_ context.Context, meta RequestMeta) ([]string, error) {
	return []string{"request:" + meta.HTTP.Method}, nil
}

func (*fakeBridge) OnResponse(_ context.Context, meta ResponseMeta) ([]string, error) {
	return []string{"response:" + meta.Headers.Get("X-Test")}, nil
}

func (*fakeBridge) OnEvent(_ context.Context, ev unified.StreamEvent) ([]string, error) {
	return []string{string(ev.Type)}, nil
}

func (*fakeBridge) OnClose(_ context.Context) ([]string, error) {
	return []string{"closed"}, nil
}

func TestTypedClientBridgesRequestMetadataEventsAndClose(t *testing.T) {
	t.Parallel()

	upstream := &fakeUnifiedStreamer{results: []StreamResult{{Event: unified.StreamEvent{Type: unified.StreamEventStarted}}}}
	client := NewTypedClient[string, string](upstream, fakeBridgeBuilder{})

	stream, err := client.Stream(context.Background(), "gpt-5")
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}

	var events []string
	for item := range stream {
		if item.Err != nil {
			t.Fatalf("unexpected bridge error: %v", item.Err)
		}
		events = append(events, item.Event)
	}

	if upstream.lastReq.Model != "gpt-5" {
		t.Fatalf("expected upstream request model gpt-5, got %q", upstream.lastReq.Model)
	}
	if upstream.lastOpts.PreferredTarget == nil || *upstream.lastOpts.PreferredTarget != TargetResponses {
		t.Fatalf("expected preferred target responses, got %#v", upstream.lastOpts.PreferredTarget)
	}
	expected := []string{"request:POST", "response:yes", string(unified.StreamEventStarted), "closed"}
	if len(events) != len(expected) {
		t.Fatalf("expected %d events, got %d: %#v", len(expected), len(events), events)
	}
	for i := range expected {
		if events[i] != expected[i] {
			t.Fatalf("event[%d] = %q, want %q", i, events[i], expected[i])
		}
	}
}
