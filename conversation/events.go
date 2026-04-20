package conversation

import (
	"context"

	"github.com/codewandler/agentapis/api/unified"
	"github.com/codewandler/agentapis/client"
)

// Event is the minimal agent-facing event surface emitted by Session.Events.
type Event interface{ event() }

// EventMeta carries shared event metadata.
type EventMeta struct {
	ResponseID string `json:"response_id,omitempty"`
}

// TextDeltaEvent carries assistant text output.
type TextDeltaEvent struct {
	EventMeta
	Text string `json:"text"`
}

// ReasoningDeltaEvent carries assistant reasoning output.
type ReasoningDeltaEvent struct {
	EventMeta
	Text    string                 `json:"text"`
	Variant unified.ContentVariant `json:"variant,omitempty"`
}

// ToolCallEvent carries an emitted assistant tool call.
type ToolCallEvent struct {
	EventMeta
	ToolCall unified.ToolCall `json:"tool_call"`
}

// UsageEvent carries exact provider-reported usage for the current request/response.
type UsageEvent struct {
	EventMeta
	Usage unified.StreamUsage `json:"usage"`
}

// CompletedEvent marks successful logical completion of a response turn.
type CompletedEvent struct {
	EventMeta
	StopReason unified.StopReason `json:"stop_reason"`
}

// ErrorEvent carries a streaming error in the minimal event surface.
type ErrorEvent struct {
	EventMeta
	Err error `json:"-"`
}

func (TextDeltaEvent) event()      {}
func (ReasoningDeltaEvent) event() {}
func (ToolCallEvent) event()       {}
func (UsageEvent) event()          {}
func (CompletedEvent) event()      {}
func (ErrorEvent) event()          {}

func (s *Session) eventStream(ctx context.Context, req Request) (<-chan Event, error) {
	stream, err := s.RequestUnified(ctx, req)
	if err != nil {
		return nil, err
	}
	out := make(chan Event, 16)
	go func() {
		defer close(out)
		mapper := eventMapper{}
		for item := range stream {
			for _, ev := range mapper.Map(streamItem{item}) {
				out <- ev
			}
		}
	}()
	return out, nil
}

type eventMapper struct {
	textDeltaSeen         bool
	reasoningRawDeltaSeen bool
	reasoningSumDeltaSeen bool
}

// Map reduces one rich unified streaming item into zero or more minimal conversation events.
func (m *eventMapper) Map(item StreamItemLike) []Event {
	if item.StreamErr() != nil {
		return []Event{ErrorEvent{Err: item.StreamErr()}}
	}
	ev := item.StreamEvent()
	meta := EventMeta{ResponseID: eventResponseID(ev)}
	var out []Event
	if ev.ContentDelta != nil {
		switch ev.ContentDelta.Kind {
		case unified.ContentKindText:
			if ev.ContentDelta.Data != "" {
				m.textDeltaSeen = true
				out = append(out, TextDeltaEvent{EventMeta: meta, Text: ev.ContentDelta.Data})
			}
		case unified.ContentKindReasoning:
			if ev.ContentDelta.Data != "" {
				if ev.ContentDelta.Variant == unified.ContentVariantSummary {
					m.reasoningSumDeltaSeen = true
				} else {
					m.reasoningRawDeltaSeen = true
				}
				out = append(out, ReasoningDeltaEvent{EventMeta: meta, Text: ev.ContentDelta.Data, Variant: ev.ContentDelta.Variant})
			}
		}
	}
	if ev.StreamContent != nil {
		switch ev.StreamContent.Kind {
		case unified.ContentKindText:
			if ev.StreamContent.Data != "" && !m.textDeltaSeen {
				out = append(out, TextDeltaEvent{EventMeta: meta, Text: ev.StreamContent.Data})
			}
		case unified.ContentKindReasoning:
			if ev.StreamContent.Data != "" {
				if ev.StreamContent.Variant == unified.ContentVariantSummary {
					if !m.reasoningSumDeltaSeen {
						out = append(out, ReasoningDeltaEvent{EventMeta: meta, Text: ev.StreamContent.Data, Variant: ev.StreamContent.Variant})
					}
				} else if !m.reasoningRawDeltaSeen {
					out = append(out, ReasoningDeltaEvent{EventMeta: meta, Text: ev.StreamContent.Data, Variant: ev.StreamContent.Variant})
				}
			}
		}
	}
	if ev.Type == unified.StreamEventToolCall {
		if ev.ToolCall != nil {
			out = append(out, ToolCallEvent{EventMeta: meta, ToolCall: unified.ToolCall{ID: ev.ToolCall.ID, Name: ev.ToolCall.Name, Args: cloneAnyMap(ev.ToolCall.Args)}})
		} else if ev.StreamToolCall != nil {
			out = append(out, ToolCallEvent{EventMeta: meta, ToolCall: unified.ToolCall{ID: ev.StreamToolCall.ID, Name: ev.StreamToolCall.Name, Args: cloneAnyMap(ev.StreamToolCall.Args)}})
		}
	}
	if ev.Usage != nil {
		usage := *ev.Usage
		usage.Tokens = append(unified.TokenItems(nil), usage.Tokens...)
		usage.Costs = append(unified.CostItems(nil), usage.Costs...)
		out = append(out, UsageEvent{EventMeta: meta, Usage: usage})
	}
	if ev.Type == unified.StreamEventCompleted && ev.Completed != nil {
		out = append(out, CompletedEvent{EventMeta: meta, StopReason: ev.Completed.StopReason})
	}
	if ev.Type == unified.StreamEventError && ev.Error != nil && ev.Error.Err != nil {
		out = append(out, ErrorEvent{EventMeta: meta, Err: ev.Error.Err})
	}
	return out
}

type StreamItemLike interface {
	StreamEvent() unified.StreamEvent
	StreamErr() error
}

func eventResponseID(ev unified.StreamEvent) string {
	if ev.Lifecycle != nil && ev.Lifecycle.Ref.ResponseID != "" {
		return ev.Lifecycle.Ref.ResponseID
	}
	if ev.ContentDelta != nil && ev.ContentDelta.Ref.ResponseID != "" {
		return ev.ContentDelta.Ref.ResponseID
	}
	if ev.StreamContent != nil && ev.StreamContent.Ref.ResponseID != "" {
		return ev.StreamContent.Ref.ResponseID
	}
	if ev.StreamToolCall != nil && ev.StreamToolCall.Ref.ResponseID != "" {
		return ev.StreamToolCall.Ref.ResponseID
	}
	if ev.Usage != nil && ev.Usage.RequestID != "" {
		return ev.Usage.RequestID
	}
	return ""
}


type streamItem struct { client.StreamResult }

func (s streamItem) StreamEvent() unified.StreamEvent { return s.Event }
func (s streamItem) StreamErr() error               { return s.Err }


