package conversation

import (
	"context"
	"errors"
	"testing"

	"github.com/codewandler/agentapis/api/unified"
	"github.com/codewandler/agentapis/client"
)

type fakeStreamer struct {
	requests []unified.Request
	streams  [][]client.StreamResult
	err      error
}

func (f *fakeStreamer) Stream(_ context.Context, req unified.Request) (<-chan client.StreamResult, error) {
	f.requests = append(f.requests, req)
	if f.err != nil {
		return nil, f.err
	}
	idx := len(f.requests) - 1
	ch := make(chan client.StreamResult, len(f.streams[idx]))
	for _, item := range f.streams[idx] {
		ch <- item
	}
	close(ch)
	return ch, nil
}

func completedText(responseID, text string) []client.StreamResult {
	return []client.StreamResult{
		{Event: unified.StreamEvent{Type: unified.StreamEventContentDelta, ContentDelta: &unified.ContentDelta{ContentBase: unified.ContentBase{Ref: unified.StreamRef{ResponseID: responseID}, Kind: unified.ContentKindText, Variant: unified.ContentVariantPrimary, Encoding: unified.ContentEncodingUTF8, Data: text}}}},
		{Event: unified.StreamEvent{Type: unified.StreamEventCompleted, Lifecycle: &unified.Lifecycle{Scope: unified.LifecycleScopeResponse, State: unified.LifecycleStateDone, Ref: unified.StreamRef{ResponseID: responseID}}, Completed: &unified.Completed{StopReason: unified.StopReasonEndTurn}}},
	}
}

func completedToolCall(responseID, id, name string) []client.StreamResult {
	return []client.StreamResult{
		{Event: unified.StreamEvent{Type: unified.StreamEventToolCall, StreamToolCall: &unified.StreamToolCall{Ref: unified.StreamRef{ResponseID: responseID}, ID: id, Name: name, Args: map[string]any{"city": "Berlin"}}, ToolCall: &unified.ToolCall{ID: id, Name: name, Args: map[string]any{"city": "Berlin"}}}},
		{Event: unified.StreamEvent{Type: unified.StreamEventCompleted, Lifecycle: &unified.Lifecycle{Scope: unified.LifecycleScopeResponse, State: unified.LifecycleStateDone, Ref: unified.StreamRef{ResponseID: responseID}}, Completed: &unified.Completed{StopReason: unified.StopReasonToolUse}}},
	}
}

func completedReasoning(responseID, raw, summary string) []client.StreamResult {
	out := []client.StreamResult{}
	if raw != "" {
		out = append(out, client.StreamResult{Event: unified.StreamEvent{Type: unified.StreamEventContentDelta, ContentDelta: &unified.ContentDelta{ContentBase: unified.ContentBase{Ref: unified.StreamRef{ResponseID: responseID}, Kind: unified.ContentKindReasoning, Variant: unified.ContentVariantRaw, Encoding: unified.ContentEncodingUTF8, Data: raw}}}})
	}
	if summary != "" {
		out = append(out, client.StreamResult{Event: unified.StreamEvent{Type: unified.StreamEventContentDelta, ContentDelta: &unified.ContentDelta{ContentBase: unified.ContentBase{Ref: unified.StreamRef{ResponseID: responseID}, Kind: unified.ContentKindReasoning, Variant: unified.ContentVariantSummary, Encoding: unified.ContentEncodingUTF8, Data: summary}}}})
	}
	out = append(out, client.StreamResult{Event: unified.StreamEvent{Type: unified.StreamEventCompleted, Lifecycle: &unified.Lifecycle{Scope: unified.LifecycleScopeResponse, State: unified.LifecycleStateDone, Ref: unified.StreamRef{ResponseID: responseID}}, Completed: &unified.Completed{StopReason: unified.StopReasonEndTurn}}})
	return out
}

func drain(stream <-chan client.StreamResult) {
	for range stream {
	}
}

func TestSessionRequestRequiresModel(t *testing.T) {
	t.Parallel()
	s := New(&fakeStreamer{})
	_, err := s.RequestUnified(context.Background(), Request{Inputs: []Input{{Role: unified.RoleUser, Text: "hi"}}})
	if !errors.Is(err, ErrModelRequired) {
		t.Fatalf("expected ErrModelRequired, got %v", err)
	}
}

func TestSessionReplayFirstTurnCommitsUserAndAssistant(t *testing.T) {
	t.Parallel()
	fs := &fakeStreamer{streams: [][]client.StreamResult{completedText("resp_1", "pong")}}
	s := New(fs, WithModel("gpt-4o-mini"))
	stream, err := s.RequestUnified(context.Background(), Request{Inputs: []Input{{Role: unified.RoleUser, Text: "ping"}}})
	if err != nil {
		t.Fatalf("Request() error = %v", err)
	}
	drain(stream)
	h := s.History()
	if len(h) != 2 || h[0].Role != unified.RoleUser || h[1].Role != unified.RoleAssistant || h[1].Parts[0].Text != "pong" {
		t.Fatalf("unexpected history: %#v", h)
	}
	if len(fs.requests) != 1 || len(fs.requests[0].Messages) != 1 {
		t.Fatalf("unexpected outbound requests: %#v", fs.requests)
	}
}

func TestSessionReplaySecondTurnReplaysFullHistory(t *testing.T) {
	t.Parallel()
	fs := &fakeStreamer{streams: [][]client.StreamResult{completedText("resp_1", "pong"), completedText("resp_2", "still pong")}}
	s := New(fs, WithModel("gpt-4o-mini"))
	stream, _ := s.RequestUnified(context.Background(), Request{Inputs: []Input{{Role: unified.RoleUser, Text: "ping"}}})
	drain(stream)
	stream, _ = s.RequestUnified(context.Background(), Request{Inputs: []Input{{Role: unified.RoleUser, Text: "again"}}})
	drain(stream)
	if len(fs.requests) != 2 || len(fs.requests[1].Messages) != 3 {
		t.Fatalf("expected replayed history on second request, got %#v", fs.requests)
	}
}

func TestSessionNativeStrategySecondTurnUsesPreviousResponseID(t *testing.T) {
	t.Parallel()
	fs := &fakeStreamer{streams: [][]client.StreamResult{completedText("resp_1", "pong"), completedText("resp_2", "again")}}
	s := New(fs, WithModel("gpt-4o-mini"), WithCapabilities(Capabilities{SupportsResponsesPreviousResponseID: true}))
	stream, _ := s.RequestUnified(context.Background(), Request{Inputs: []Input{{Role: unified.RoleUser, Text: "ping"}}})
	drain(stream)
	stream, _ = s.RequestUnified(context.Background(), Request{Inputs: []Input{{Role: unified.RoleUser, Text: "again"}}})
	drain(stream)
	if fs.requests[1].Extras.Responses == nil || fs.requests[1].Extras.Responses.PreviousResponseID != "resp_1" {
		t.Fatalf("expected previous response id on second request, got %#v", fs.requests[1].Extras.Responses)
	}
	if len(fs.requests[1].Messages) != 1 {
		t.Fatalf("expected native incremental request, got %#v", fs.requests[1].Messages)
	}
}

func TestSessionFailedTurnDoesNotCommit(t *testing.T) {
	t.Parallel()
	fs := &fakeStreamer{streams: [][]client.StreamResult{{{Err: errors.New("boom")}}}}
	s := New(fs, WithModel("gpt-4o-mini"))
	stream, _ := s.RequestUnified(context.Background(), Request{Inputs: []Input{{Role: unified.RoleUser, Text: "ping"}}})
	drain(stream)
	if len(s.History()) != 0 {
		t.Fatalf("expected no history on failed turn, got %#v", s.History())
	}
}

func TestSessionResetClearsHistoryAndNativeStateButKeepsDefaults(t *testing.T) {
	t.Parallel()
	fs := &fakeStreamer{streams: [][]client.StreamResult{completedText("resp_1", "pong")}}
	s := New(fs, WithModel("gpt-4o-mini"), WithCapabilities(Capabilities{SupportsResponsesPreviousResponseID: true}))
	stream, _ := s.RequestUnified(context.Background(), Request{Inputs: []Input{{Role: unified.RoleUser, Text: "ping"}}})
	drain(stream)
	s.Reset()
	if len(s.History()) != 0 || s.native.lastResponseID != "" || s.defaults.model != "gpt-4o-mini" {
		t.Fatalf("unexpected reset state: history=%#v native=%#v defaults=%#v", s.History(), s.native, s.defaults)
	}
}

type blockingStreamer struct {
	started chan struct{}
	release chan struct{}
	request unified.Request
}

func (b *blockingStreamer) Stream(_ context.Context, req unified.Request) (<-chan client.StreamResult, error) {
	b.request = req
	close(b.started)
	ch := make(chan client.StreamResult)
	go func() {
		<-b.release
		ch <- client.StreamResult{Event: unified.StreamEvent{Type: unified.StreamEventCompleted, Lifecycle: &unified.Lifecycle{Scope: unified.LifecycleScopeResponse, State: unified.LifecycleStateDone, Ref: unified.StreamRef{ResponseID: "resp_block"}}, Completed: &unified.Completed{StopReason: unified.StopReasonEndTurn}}}
		close(ch)
	}()
	return ch, nil
}

func TestSessionRejectsOverlappingTurns(t *testing.T) {
	t.Parallel()
	bs := &blockingStreamer{started: make(chan struct{}), release: make(chan struct{})}
	sess := New(bs, WithModel("gpt-4o-mini"))
	stream, err := sess.RequestUnified(context.Background(), Request{Inputs: []Input{{Role: unified.RoleUser, Text: "first"}}})
	if err != nil {
		t.Fatalf("first Request() error = %v", err)
	}
	<-bs.started
	_, err = sess.RequestUnified(context.Background(), Request{Inputs: []Input{{Role: unified.RoleUser, Text: "second"}}})
	if !errors.Is(err, ErrTurnInProgress) {
		t.Fatalf("expected ErrTurnInProgress, got %v", err)
	}
	close(bs.release)
	drain(stream)
}

func TestSessionCommitsToolCallsAsAssistantParts(t *testing.T) {
	t.Parallel()
	fs := &fakeStreamer{streams: [][]client.StreamResult{completedToolCall("resp_tool", "call_1", "weather")}}
	s := New(fs, WithModel("gpt-4o-mini"))
	stream, err := s.RequestUnified(context.Background(), Request{Inputs: []Input{{Role: unified.RoleUser, Text: "use the weather tool"}}})
	if err != nil {
		t.Fatalf("Request() error = %v", err)
	}
	drain(stream)
	h := s.History()
	if len(h) != 2 || len(h[1].Parts) != 1 || h[1].Parts[0].ToolCall == nil || h[1].Parts[0].ToolCall.Name != "weather" {
		t.Fatalf("unexpected history with tool call: %#v", h)
	}
}

func TestSessionRecordsReasoningHistory(t *testing.T) {
	t.Parallel()
	fs := &fakeStreamer{streams: [][]client.StreamResult{completedReasoning("resp_reason", "raw-thought", "short-summary")}}
	s := New(fs, WithModel("gpt-4o-mini"))
	stream, err := s.RequestUnified(context.Background(), Request{Inputs: []Input{{Role: unified.RoleUser, Text: "think"}}})
	if err != nil {
		t.Fatalf("Request() error = %v", err)
	}
	drain(stream)
	r := s.ReasoningHistory()
	if len(r) != 1 || r[0].Raw != "raw-thought" || r[0].Summary != "short-summary" {
		t.Fatalf("unexpected reasoning history: %#v", r)
	}
}

func TestSessionReasoningReplayPreservesThinkingPartsInHistoryAndRequests(t *testing.T) {
	t.Parallel()
	fs := &fakeStreamer{streams: [][]client.StreamResult{
		completedReasoning("resp_reason_1", "raw-thought", "short-summary"),
		completedText("resp_reason_2", "answer"),
	}}
	s := New(fs, WithModel("gpt-4o-mini"), WithStrategy(StrategyReplay))
	stream, err := s.RequestUnified(context.Background(), Request{Inputs: []Input{{Role: unified.RoleUser, Text: "think first"}}})
	if err != nil {
		t.Fatalf("first Request() error = %v", err)
	}
	drain(stream)
	h := s.History()
	if len(h) != 2 || len(h[1].Parts) < 2 {
		t.Fatalf("expected assistant reasoning parts in history, got %#v", h)
	}
	if h[1].Parts[0].Type != unified.PartTypeThinking || h[1].Parts[0].Thinking == nil || h[1].Parts[0].Thinking.Text != "raw-thought" {
		t.Fatalf("expected raw thinking part, got %#v", h[1].Parts)
	}
	if h[1].Parts[1].Type != unified.PartTypeThinking || h[1].Parts[1].Thinking == nil || h[1].Parts[1].Thinking.Text != "short-summary" {
		t.Fatalf("expected summary thinking part, got %#v", h[1].Parts)
	}
	stream, err = s.RequestUnified(context.Background(), Request{Inputs: []Input{{Role: unified.RoleUser, Text: "continue"}}})
	if err != nil {
		t.Fatalf("second Request() error = %v", err)
	}
	drain(stream)
	if len(fs.requests) < 2 || len(fs.requests[1].Messages) < 3 {
		t.Fatalf("expected replay request to include prior assistant reasoning, got %#v", fs.requests)
	}
	assistant := fs.requests[1].Messages[1]
	if assistant.Role != unified.RoleAssistant || len(assistant.Parts) < 2 || assistant.Parts[0].Type != unified.PartTypeThinking {
		t.Fatalf("expected replayed assistant reasoning message, got %#v", assistant)
	}
}

func completedMultipleToolCalls(responseID string, calls ...unified.ToolCall) []client.StreamResult {
	out := make([]client.StreamResult, 0, len(calls)+1)
	for _, call := range calls {
		call := call
		out = append(out, client.StreamResult{Event: unified.StreamEvent{
			Type: unified.StreamEventToolCall,
			StreamToolCall: &unified.StreamToolCall{
				Ref:      unified.StreamRef{ResponseID: responseID},
				ID:       call.ID,
				Name:     call.Name,
				Args:     cloneAnyMap(call.Args),
				RawInput: "{}",
			},
			ToolCall: &unified.ToolCall{ID: call.ID, Name: call.Name, Args: cloneAnyMap(call.Args)},
		}})
	}
	out = append(out, client.StreamResult{Event: unified.StreamEvent{Type: unified.StreamEventCompleted, Lifecycle: &unified.Lifecycle{Scope: unified.LifecycleScopeResponse, State: unified.LifecycleStateDone, Ref: unified.StreamRef{ResponseID: responseID}}, Completed: &unified.Completed{StopReason: unified.StopReasonToolUse}}})
	return out
}

func TestSessionCommitsMultipleToolCallsInAssistantTurn(t *testing.T) {
	t.Parallel()
	fs := &fakeStreamer{streams: [][]client.StreamResult{completedMultipleToolCalls("resp_multi",
		unified.ToolCall{ID: "call_1", Name: "weather", Args: map[string]any{"city": "Berlin"}},
		unified.ToolCall{ID: "call_2", Name: "time", Args: map[string]any{"zone": "UTC"}},
	)}}
	s := New(fs, WithModel("gpt-4o-mini"))
	stream, err := s.RequestUnified(context.Background(), Request{Inputs: []Input{{Role: unified.RoleUser, Text: "use both tools"}}})
	if err != nil {
		t.Fatalf("Request() error = %v", err)
	}
	drain(stream)
	h := s.History()
	if len(h) != 2 || len(h[1].Parts) != 2 {
		t.Fatalf("expected assistant message with two tool calls, got %#v", h)
	}
	if h[1].Parts[0].ToolCall == nil || h[1].Parts[0].ToolCall.Name != "weather" || h[1].Parts[1].ToolCall == nil || h[1].Parts[1].ToolCall.Name != "time" {
		t.Fatalf("unexpected tool call ordering/history: %#v", h[1].Parts)
	}
}

func TestSessionReplayIncludesPriorAssistantToolCalls(t *testing.T) {
	t.Parallel()
	fs := &fakeStreamer{streams: [][]client.StreamResult{
		completedMultipleToolCalls("resp_multi",
			unified.ToolCall{ID: "call_1", Name: "weather", Args: map[string]any{"city": "Berlin"}},
			unified.ToolCall{ID: "call_2", Name: "time", Args: map[string]any{"zone": "UTC"}},
		),
		completedText("resp_followup", "done"),
	}}
	s := New(fs, WithModel("gpt-4o-mini"), WithStrategy(StrategyReplay))
	stream, err := s.RequestUnified(context.Background(), Request{Inputs: []Input{{Role: unified.RoleUser, Text: "use both tools"}}})
	if err != nil {
		t.Fatalf("first Request() error = %v", err)
	}
	drain(stream)
	stream, err = s.RequestUnified(context.Background(), Request{Inputs: []Input{{Role: unified.RoleTool, ToolResult: &ToolResult{ToolCallID: "call_1", Output: `{"temp":"20C"}`}}, {Role: unified.RoleTool, ToolResult: &ToolResult{ToolCallID: "call_2", Output: `{"time":"12:00"}`}}}})
	if err != nil {
		t.Fatalf("second Request() error = %v", err)
	}
	drain(stream)
	if len(fs.requests) < 2 {
		t.Fatalf("expected two outbound requests, got %#v", fs.requests)
	}
	msgs := fs.requests[1].Messages
	if len(msgs) != 4 {
		t.Fatalf("expected replay request with user, assistant tool calls, and two tool results, got %#v", msgs)
	}
	if msgs[1].Role != unified.RoleAssistant || len(msgs[1].Parts) != 2 || msgs[1].Parts[0].ToolCall == nil || msgs[1].Parts[1].ToolCall == nil {
		t.Fatalf("expected replayed assistant tool calls, got %#v", msgs[1])
	}
	if msgs[2].Role != unified.RoleTool || msgs[2].Parts[0].ToolResult == nil || msgs[2].Parts[0].ToolResult.ToolCallID != "call_1" {
		t.Fatalf("expected first replayed tool result, got %#v", msgs[2])
	}
	if msgs[3].Role != unified.RoleTool || msgs[3].Parts[0].ToolResult == nil || msgs[3].Parts[0].ToolResult.ToolCallID != "call_2" {
		t.Fatalf("expected second replayed tool result, got %#v", msgs[3])
	}
}

func TestSessionRequestNormalizesInstructionsToolResultsAndUserInputsInOrder(t *testing.T) {
	t.Parallel()
	fs := &fakeStreamer{streams: [][]client.StreamResult{completedText("resp_order", "ok")}}
	s := New(fs, WithModel("gpt-4o-mini"), WithStrategy(StrategyReplay))
	stream, err := s.RequestUnified(context.Background(), Request{
		Instructions: []string{"be brief", "be precise"},
		Inputs: []Input{
			{Role: unified.RoleTool, ToolResult: &ToolResult{ToolCallID: "call_1", Output: `{"ok":true}`}},
			{Role: unified.RoleUser, Text: "continue"},
		},
	})
	if err != nil {
		t.Fatalf("Request() error = %v", err)
	}
	drain(stream)
	msgs := fs.requests[0].Messages
	if len(msgs) != 4 {
		t.Fatalf("expected four normalized messages, got %#v", msgs)
	}
	if msgs[0].Role != unified.RoleDeveloper || msgs[0].Parts[0].Text != "be brief" {
		t.Fatalf("unexpected first normalized message: %#v", msgs[0])
	}
	if msgs[1].Role != unified.RoleDeveloper || msgs[1].Parts[0].Text != "be precise" {
		t.Fatalf("unexpected second normalized message: %#v", msgs[1])
	}
	if msgs[2].Role != unified.RoleTool || msgs[2].Parts[0].ToolResult == nil || msgs[2].Parts[0].ToolResult.ToolCallID != "call_1" {
		t.Fatalf("unexpected third normalized message: %#v", msgs[2])
	}
	if msgs[3].Role != unified.RoleUser || msgs[3].Parts[0].Text != "continue" {
		t.Fatalf("unexpected fourth normalized message: %#v", msgs[3])
	}
}


func mixedAssistantTurn(responseID string) []client.StreamResult {
	return []client.StreamResult{
		{Event: unified.StreamEvent{Type: unified.StreamEventContentDelta, ContentDelta: &unified.ContentDelta{ContentBase: unified.ContentBase{Ref: unified.StreamRef{ResponseID: responseID}, Kind: unified.ContentKindReasoning, Variant: unified.ContentVariantRaw, Encoding: unified.ContentEncodingUTF8, Data: "reason-1 "}}}},
		{Event: unified.StreamEvent{Type: unified.StreamEventToolCall, StreamToolCall: &unified.StreamToolCall{Ref: unified.StreamRef{ResponseID: responseID}, ID: "call_1", Name: "weather", Args: map[string]any{"city": "Berlin"}}, ToolCall: &unified.ToolCall{ID: "call_1", Name: "weather", Args: map[string]any{"city": "Berlin"}}}},
		{Event: unified.StreamEvent{Type: unified.StreamEventContentDelta, ContentDelta: &unified.ContentDelta{ContentBase: unified.ContentBase{Ref: unified.StreamRef{ResponseID: responseID}, Kind: unified.ContentKindText, Variant: unified.ContentVariantPrimary, Encoding: unified.ContentEncodingUTF8, Data: "text-1 "}}}},
		{Event: unified.StreamEvent{Type: unified.StreamEventContentDelta, ContentDelta: &unified.ContentDelta{ContentBase: unified.ContentBase{Ref: unified.StreamRef{ResponseID: responseID}, Kind: unified.ContentKindReasoning, Variant: unified.ContentVariantSummary, Encoding: unified.ContentEncodingUTF8, Data: "summary-1 "}}}},
		{Event: unified.StreamEvent{Type: unified.StreamEventToolCall, StreamToolCall: &unified.StreamToolCall{Ref: unified.StreamRef{ResponseID: responseID}, ID: "call_2", Name: "time", Args: map[string]any{"zone": "UTC"}}, ToolCall: &unified.ToolCall{ID: "call_2", Name: "time", Args: map[string]any{"zone": "UTC"}}}},
		{Event: unified.StreamEvent{Type: unified.StreamEventContentDelta, ContentDelta: &unified.ContentDelta{ContentBase: unified.ContentBase{Ref: unified.StreamRef{ResponseID: responseID}, Kind: unified.ContentKindText, Variant: unified.ContentVariantPrimary, Encoding: unified.ContentEncodingUTF8, Data: "text-2"}}}},
		{Event: unified.StreamEvent{Type: unified.StreamEventCompleted, Lifecycle: &unified.Lifecycle{Scope: unified.LifecycleScopeResponse, State: unified.LifecycleStateDone, Ref: unified.StreamRef{ResponseID: responseID}}, Completed: &unified.Completed{StopReason: unified.StopReasonToolUse}}},
	}
}

func TestSessionPreservesExactAssistantPartOrderForMixedTurn(t *testing.T) {
	t.Parallel()
	fs := &fakeStreamer{streams: [][]client.StreamResult{mixedAssistantTurn("resp_mixed")}}
	s := New(fs, WithModel("gpt-4o-mini"), WithStrategy(StrategyReplay))
	stream, err := s.RequestUnified(context.Background(), Request{Inputs: []Input{{Role: unified.RoleUser, Text: "mixed turn"}}})
	if err != nil {
		t.Fatalf("Request() error = %v", err)
	}
	drain(stream)
	h := s.History()
	if len(h) != 2 {
		t.Fatalf("expected user and assistant history, got %#v", h)
	}
	parts := h[1].Parts
	if len(parts) != 6 {
		t.Fatalf("expected six assistant parts preserving emission order, got %#v", parts)
	}
	if parts[0].Type != unified.PartTypeThinking || parts[0].Thinking == nil || parts[0].Thinking.Provider != "conversation.reasoning.raw" || parts[0].Thinking.Text != "reason-1 " {
		t.Fatalf("unexpected part 0: %#v", parts[0])
	}
	if parts[1].Type != unified.PartTypeToolCall || parts[1].ToolCall == nil || parts[1].ToolCall.ID != "call_1" {
		t.Fatalf("unexpected part 1: %#v", parts[1])
	}
	if parts[2].Type != unified.PartTypeText || parts[2].Text != "text-1 " {
		t.Fatalf("unexpected part 2: %#v", parts[2])
	}
	if parts[3].Type != unified.PartTypeThinking || parts[3].Thinking == nil || parts[3].Thinking.Provider != "conversation.reasoning.summary" || parts[3].Thinking.Text != "summary-1 " {
		t.Fatalf("unexpected part 3: %#v", parts[3])
	}
	if parts[4].Type != unified.PartTypeToolCall || parts[4].ToolCall == nil || parts[4].ToolCall.ID != "call_2" {
		t.Fatalf("unexpected part 4: %#v", parts[4])
	}
	if parts[5].Type != unified.PartTypeText || parts[5].Text != "text-2" {
		t.Fatalf("unexpected part 5: %#v", parts[5])
	}
}

func TestSessionReplayIterativeToolLoopAcrossMultipleTurns(t *testing.T) {
	t.Parallel()
	fs := &fakeStreamer{streams: [][]client.StreamResult{
		completedToolCall("resp_1", "call_1", "weather"),
		completedToolCall("resp_2", "call_2", "time"),
		completedText("resp_3", "final answer"),
	}}
	s := New(fs, WithModel("gpt-4o-mini"), WithStrategy(StrategyReplay))
	stream, err := s.RequestUnified(context.Background(), Request{Inputs: []Input{{Role: unified.RoleUser, Text: "start"}}})
	if err != nil {
		t.Fatalf("first Request() error = %v", err)
	}
	drain(stream)
	stream, err = s.RequestUnified(context.Background(), Request{Inputs: []Input{{Role: unified.RoleTool, ToolResult: &ToolResult{ToolCallID: "call_1", Output: `{"temp":"20C"}`}}}})
	if err != nil {
		t.Fatalf("second Request() error = %v", err)
	}
	drain(stream)
	stream, err = s.RequestUnified(context.Background(), Request{Inputs: []Input{{Role: unified.RoleTool, ToolResult: &ToolResult{ToolCallID: "call_2", Output: `{"time":"12:00"}`}}}})
	if err != nil {
		t.Fatalf("third Request() error = %v", err)
	}
	drain(stream)
	if len(fs.requests) != 3 {
		t.Fatalf("expected three outbound requests, got %#v", fs.requests)
	}
	if len(fs.requests[1].Messages) != 3 {
		t.Fatalf("expected second replay request to include prior user, assistant tool call, and tool result, got %#v", fs.requests[1].Messages)
	}
	if len(fs.requests[2].Messages) != 5 {
		t.Fatalf("expected third replay request to include full iterative tool chain, got %#v", fs.requests[2].Messages)
	}
	msgs := fs.requests[2].Messages
	if msgs[0].Role != unified.RoleUser || msgs[1].Role != unified.RoleAssistant || msgs[1].Parts[0].ToolCall == nil || msgs[1].Parts[0].ToolCall.ID != "call_1" || msgs[2].Role != unified.RoleTool || msgs[2].Parts[0].ToolResult == nil || msgs[2].Parts[0].ToolResult.ToolCallID != "call_1" || msgs[3].Role != unified.RoleAssistant || msgs[3].Parts[0].ToolCall == nil || msgs[3].Parts[0].ToolCall.ID != "call_2" || msgs[4].Role != unified.RoleTool || msgs[4].Parts[0].ToolResult == nil || msgs[4].Parts[0].ToolResult.ToolCallID != "call_2" {
		t.Fatalf("unexpected iterative replay chain: %#v", msgs)
	}
	h := s.History()
	if len(h) != 6 {
		t.Fatalf("expected six committed messages after iterative loop, got %#v", h)
	}
	if h[5].Role != unified.RoleAssistant || len(h[5].Parts) != 1 || h[5].Parts[0].Text != "final answer" {
		t.Fatalf("expected final assistant answer committed, got %#v", h[5])
	}
}

func TestSessionNativeStrategyToolLoopUsesLatestPreviousResponseID(t *testing.T) {
	t.Parallel()
	fs := &fakeStreamer{streams: [][]client.StreamResult{
		completedToolCall("resp_1", "call_1", "weather"),
		completedToolCall("resp_2", "call_2", "time"),
		completedText("resp_3", "done"),
	}}
	s := New(fs, WithModel("gpt-4o-mini"), WithCapabilities(Capabilities{SupportsResponsesPreviousResponseID: true}))
	stream, err := s.RequestUnified(context.Background(), Request{Inputs: []Input{{Role: unified.RoleUser, Text: "start"}}})
	if err != nil {
		t.Fatalf("first Request() error = %v", err)
	}
	drain(stream)
	stream, err = s.RequestUnified(context.Background(), Request{Inputs: []Input{{Role: unified.RoleTool, ToolResult: &ToolResult{ToolCallID: "call_1", Output: `{"temp":"20C"}`}}}})
	if err != nil {
		t.Fatalf("second Request() error = %v", err)
	}
	drain(stream)
	stream, err = s.RequestUnified(context.Background(), Request{Inputs: []Input{{Role: unified.RoleTool, ToolResult: &ToolResult{ToolCallID: "call_2", Output: `{"time":"12:00"}`}}}})
	if err != nil {
		t.Fatalf("third Request() error = %v", err)
	}
	drain(stream)
	if len(fs.requests) != 3 {
		t.Fatalf("expected three outbound requests, got %#v", fs.requests)
	}
	if fs.requests[1].Extras.Responses == nil || fs.requests[1].Extras.Responses.PreviousResponseID != "resp_1" {
		t.Fatalf("expected second request previous_response_id resp_1, got %#v", fs.requests[1].Extras.Responses)
	}
	if fs.requests[2].Extras.Responses == nil || fs.requests[2].Extras.Responses.PreviousResponseID != "resp_2" {
		t.Fatalf("expected third request previous_response_id resp_2, got %#v", fs.requests[2].Extras.Responses)
	}
	if len(fs.requests[1].Messages) != 1 || fs.requests[1].Messages[0].Role != unified.RoleTool || len(fs.requests[2].Messages) != 1 || fs.requests[2].Messages[0].Role != unified.RoleTool {
		t.Fatalf("expected native incremental tool-result requests, got %#v", fs.requests)
	}
	h := s.History()
	if len(h) != 6 || h[1].Role != unified.RoleAssistant || h[3].Role != unified.RoleAssistant || h[5].Role != unified.RoleAssistant {
		t.Fatalf("expected full local history retained in native mode, got %#v", h)
	}
}


func incompleteMixedAssistantTurn(responseID string) []client.StreamResult {
	return []client.StreamResult{
		{Event: unified.StreamEvent{Type: unified.StreamEventContentDelta, ContentDelta: &unified.ContentDelta{ContentBase: unified.ContentBase{Ref: unified.StreamRef{ResponseID: responseID}, Kind: unified.ContentKindReasoning, Variant: unified.ContentVariantRaw, Encoding: unified.ContentEncodingUTF8, Data: "thinking "}}}},
		{Event: unified.StreamEvent{Type: unified.StreamEventToolCall, StreamToolCall: &unified.StreamToolCall{Ref: unified.StreamRef{ResponseID: responseID}, ID: "call_partial", Name: "weather", Args: map[string]any{"city": "Berlin"}}, ToolCall: &unified.ToolCall{ID: "call_partial", Name: "weather", Args: map[string]any{"city": "Berlin"}}}},
		{Err: errors.New("stream interrupted")},
	}
}

func TestSessionIncompleteMixedAssistantTurnDoesNotCommit(t *testing.T) {
	t.Parallel()
	fs := &fakeStreamer{streams: [][]client.StreamResult{incompleteMixedAssistantTurn("resp_partial")}}
	s := New(fs, WithModel("gpt-4o-mini"), WithStrategy(StrategyReplay))
	stream, err := s.RequestUnified(context.Background(), Request{Inputs: []Input{{Role: unified.RoleUser, Text: "start"}}})
	if err != nil {
		t.Fatalf("Request() error = %v", err)
	}
	drain(stream)
	if len(s.History()) != 0 {
		t.Fatalf("expected no history after incomplete mixed turn, got %#v", s.History())
	}
	if s.native.lastResponseID != "" {
		t.Fatalf("expected no native response state after incomplete mixed turn, got %#v", s.native)
	}
}

func TestSessionFailedToolResultFollowupDoesNotCorruptPriorState(t *testing.T) {
	t.Parallel()
	fs := &fakeStreamer{streams: [][]client.StreamResult{
		completedToolCall("resp_1", "call_1", "weather"),
		{{Err: errors.New("tool follow-up failed")}},
		completedText("resp_2", "recovered"),
	}}
	s := New(fs, WithModel("gpt-4o-mini"), WithStrategy(StrategyReplay))
	stream, err := s.RequestUnified(context.Background(), Request{Inputs: []Input{{Role: unified.RoleUser, Text: "start"}}})
	if err != nil {
		t.Fatalf("first Request() error = %v", err)
	}
	drain(stream)
	before := s.History()
	if len(before) != 2 || before[1].Role != unified.RoleAssistant || len(before[1].Parts) != 1 || before[1].Parts[0].ToolCall == nil || before[1].Parts[0].ToolCall.ID != "call_1" {
		t.Fatalf("unexpected committed history before failed follow-up: %#v", before)
	}
	stream, err = s.RequestUnified(context.Background(), Request{Inputs: []Input{{Role: unified.RoleTool, ToolResult: &ToolResult{ToolCallID: "call_1", Output: `{"temp":"20C"}`}}}})
	if err != nil {
		t.Fatalf("second Request() error = %v", err)
	}
	drain(stream)
	afterFailed := s.History()
	if len(afterFailed) != 2 {
		t.Fatalf("expected failed follow-up not to commit, got %#v", afterFailed)
	}
	stream, err = s.RequestUnified(context.Background(), Request{Inputs: []Input{{Role: unified.RoleTool, ToolResult: &ToolResult{ToolCallID: "call_1", Output: `{"temp":"21C"}`}}}})
	if err != nil {
		t.Fatalf("third Request() error = %v", err)
	}
	drain(stream)
	if len(fs.requests) != 3 {
		t.Fatalf("expected three outbound requests, got %#v", fs.requests)
	}
	msgs := fs.requests[2].Messages
	if len(msgs) != 3 || msgs[0].Role != unified.RoleUser || msgs[1].Role != unified.RoleAssistant || msgs[1].Parts[0].ToolCall == nil || msgs[1].Parts[0].ToolCall.ID != "call_1" || msgs[2].Role != unified.RoleTool || msgs[2].Parts[0].ToolResult == nil || msgs[2].Parts[0].ToolResult.ToolCallID != "call_1" {
		t.Fatalf("expected retry to replay last committed state only, got %#v", msgs)
	}
	afterRecovery := s.History()
	if len(afterRecovery) != 4 || afterRecovery[2].Role != unified.RoleTool || afterRecovery[3].Role != unified.RoleAssistant || afterRecovery[3].Parts[0].Text != "recovered" {
		t.Fatalf("unexpected recovered history: %#v", afterRecovery)
	}
}

func TestSessionReplayIterativeMultiToolLoopAcrossTurns(t *testing.T) {
	t.Parallel()
	fs := &fakeStreamer{streams: [][]client.StreamResult{
		completedMultipleToolCalls("resp_1",
			unified.ToolCall{ID: "call_1", Name: "weather", Args: map[string]any{"city": "Berlin"}},
			unified.ToolCall{ID: "call_2", Name: "time", Args: map[string]any{"zone": "UTC"}},
		),
		completedToolCall("resp_2", "call_3", "calendar"),
		completedText("resp_3", "all done"),
	}}
	s := New(fs, WithModel("gpt-4o-mini"), WithStrategy(StrategyReplay))
	stream, err := s.RequestUnified(context.Background(), Request{Inputs: []Input{{Role: unified.RoleUser, Text: "start"}}})
	if err != nil {
		t.Fatalf("first Request() error = %v", err)
	}
	drain(stream)
	stream, err = s.RequestUnified(context.Background(), Request{Inputs: []Input{
		{Role: unified.RoleTool, ToolResult: &ToolResult{ToolCallID: "call_1", Output: `{"temp":"20C"}`}},
		{Role: unified.RoleTool, ToolResult: &ToolResult{ToolCallID: "call_2", Output: `{"time":"12:00"}`}},
	}})
	if err != nil {
		t.Fatalf("second Request() error = %v", err)
	}
	drain(stream)
	stream, err = s.RequestUnified(context.Background(), Request{Inputs: []Input{{Role: unified.RoleTool, ToolResult: &ToolResult{ToolCallID: "call_3", Output: `{"slot":"13:00"}`}}}})
	if err != nil {
		t.Fatalf("third Request() error = %v", err)
	}
	drain(stream)
	if len(fs.requests) != 3 {
		t.Fatalf("expected three outbound requests, got %#v", fs.requests)
	}
	msgs := fs.requests[2].Messages
	if len(msgs) != 6 {
		t.Fatalf("expected full multi-tool iterative chain in third replay request, got %#v", msgs)
	}
	if msgs[0].Role != unified.RoleUser || msgs[1].Role != unified.RoleAssistant || len(msgs[1].Parts) != 2 || msgs[1].Parts[0].ToolCall == nil || msgs[1].Parts[0].ToolCall.ID != "call_1" || msgs[1].Parts[1].ToolCall == nil || msgs[1].Parts[1].ToolCall.ID != "call_2" || msgs[2].Role != unified.RoleTool || msgs[2].Parts[0].ToolResult == nil || msgs[2].Parts[0].ToolResult.ToolCallID != "call_1" || msgs[3].Role != unified.RoleTool || msgs[3].Parts[0].ToolResult == nil || msgs[3].Parts[0].ToolResult.ToolCallID != "call_2" || msgs[4].Role != unified.RoleAssistant || msgs[4].Parts[0].ToolCall == nil || msgs[4].Parts[0].ToolCall.ID != "call_3" || msgs[5].Role != unified.RoleTool || msgs[5].Parts[0].ToolResult == nil || msgs[5].Parts[0].ToolResult.ToolCallID != "call_3" {
		t.Fatalf("unexpected multi-tool iterative replay chain: %#v", msgs)
	}
	h := s.History()
	if len(h) != 7 || h[6].Role != unified.RoleAssistant || h[6].Parts[0].Text != "all done" {
		t.Fatalf("unexpected final history for multi-tool iterative loop: %#v", h)
	}
}

func TestSessionNativeMixedContentFailedFollowupDoesNotAdvancePreviousResponseID(t *testing.T) {
	t.Parallel()
	fs := &fakeStreamer{streams: [][]client.StreamResult{
		mixedAssistantTurn("resp_1"),
		{{Err: errors.New("native follow-up failed")}},
		completedText("resp_2", "done"),
	}}
	s := New(fs, WithModel("gpt-4o-mini"), WithCapabilities(Capabilities{SupportsResponsesPreviousResponseID: true}))
	stream, err := s.RequestUnified(context.Background(), Request{Inputs: []Input{{Role: unified.RoleUser, Text: "start"}}})
	if err != nil {
		t.Fatalf("first Request() error = %v", err)
	}
	drain(stream)
	stream, err = s.RequestUnified(context.Background(), Request{Inputs: []Input{{Role: unified.RoleTool, ToolResult: &ToolResult{ToolCallID: "call_1", Output: `{"ok":true}`}}}})
	if err != nil {
		t.Fatalf("second Request() error = %v", err)
	}
	drain(stream)
	if len(s.History()) != 2 {
		t.Fatalf("expected failed native follow-up not to commit, got %#v", s.History())
	}
	stream, err = s.RequestUnified(context.Background(), Request{Inputs: []Input{{Role: unified.RoleTool, ToolResult: &ToolResult{ToolCallID: "call_1", Output: `{"ok":true}`}}}})
	if err != nil {
		t.Fatalf("third Request() error = %v", err)
	}
	drain(stream)
	if len(fs.requests) != 3 {
		t.Fatalf("expected three outbound requests, got %#v", fs.requests)
	}
	if fs.requests[1].Extras.Responses == nil || fs.requests[1].Extras.Responses.PreviousResponseID != "resp_1" {
		t.Fatalf("expected second request previous_response_id resp_1, got %#v", fs.requests[1].Extras.Responses)
	}
	if fs.requests[2].Extras.Responses == nil || fs.requests[2].Extras.Responses.PreviousResponseID != "resp_1" {
		t.Fatalf("expected failed follow-up not to advance previous_response_id, got %#v", fs.requests[2].Extras.Responses)
	}
	h := s.History()
	if len(h) != 4 || h[1].Role != unified.RoleAssistant || len(h[1].Parts) != 6 || h[3].Role != unified.RoleAssistant || h[3].Parts[0].Text != "done" {
		t.Fatalf("unexpected native mixed-content history: %#v", h)
	}
}


type captureProjector struct {
	seen []MessageProjectionState
	out  []unified.Message
	err  error
}

func (c *captureProjector) ProjectMessages(state MessageProjectionState) ([]unified.Message, error) {
	c.seen = append(c.seen, MessageProjectionState{
		Defaults: ProjectionDefaults{
			Model:      state.Defaults.Model,
			Tools:      cloneTools(state.Defaults.Tools),
			ToolChoice: state.Defaults.ToolChoice,
			System:     append([]string(nil), state.Defaults.System...),
			Developer:  append([]string(nil), state.Defaults.Developer...),
		},
		History:        cloneMessages(state.History),
		Pending:        cloneMessages(state.Pending),
		Strategy:       state.Strategy,
		Capabilities:   state.Capabilities,
		LastResponseID: state.LastResponseID,
	})
	if c.err != nil {
		return nil, c.err
	}
	return cloneMessages(c.out), nil
}

func TestSessionProjectMessagesUsesDefaultProjector(t *testing.T) {
	t.Parallel()
	fs := &fakeStreamer{}
	s := New(fs, WithModel("gpt-4o-mini"), WithSystem("sys"), WithDeveloper("dev"), WithStrategy(StrategyReplay))
	s.history = []unified.Message{{Role: unified.RoleAssistant, Parts: []unified.Part{{Type: unified.PartTypeText, Text: "prior"}}}}
	msgs, err := s.ProjectMessages(Request{Inputs: []Input{{Role: unified.RoleUser, Text: "next"}}})
	if err != nil {
		t.Fatalf("ProjectMessages() error = %v", err)
	}
	if len(msgs) != 4 || msgs[0].Role != unified.RoleSystem || msgs[1].Role != unified.RoleDeveloper || msgs[2].Role != unified.RoleAssistant || msgs[3].Role != unified.RoleUser {
		t.Fatalf("unexpected projected messages: %#v", msgs)
	}
}

func TestSessionBuildRequestUsesCustomMessageProjector(t *testing.T) {
	t.Parallel()
	proj := &captureProjector{out: []unified.Message{{Role: unified.RoleDeveloper, Parts: []unified.Part{{Type: unified.PartTypeText, Text: "projected"}}}}}
	s := New(&fakeStreamer{}, WithModel("gpt-4o-mini"), WithSystem("sys"), WithDeveloper("dev"), WithMessageProjector(proj), WithCapabilities(Capabilities{SupportsResponsesPreviousResponseID: true}))
	s.history = []unified.Message{{Role: unified.RoleAssistant, Parts: []unified.Part{{Type: unified.PartTypeText, Text: "prior"}}}}
	s.native.lastResponseID = "resp_1"
	req, err := s.BuildRequest(Request{Inputs: []Input{{Role: unified.RoleUser, Text: "next"}}})
	if err != nil {
		t.Fatalf("BuildRequest() error = %v", err)
	}
	if len(proj.seen) != 1 {
		t.Fatalf("expected projector to be called once, got %d", len(proj.seen))
	}
	seen := proj.seen[0]
	if seen.Strategy != StrategyResponsesPreviousResponseID || seen.LastResponseID != "resp_1" || len(seen.Pending) != 1 || seen.Pending[0].Role != unified.RoleUser {
		t.Fatalf("unexpected projector state: %#v", seen)
	}
	if len(req.Messages) != 1 || req.Messages[0].Role != unified.RoleDeveloper || req.Messages[0].Parts[0].Text != "projected" {
		t.Fatalf("unexpected built request messages: %#v", req.Messages)
	}
	if req.Extras.Responses == nil || req.Extras.Responses.PreviousResponseID != "resp_1" {
		t.Fatalf("expected previous response id to be set, got %#v", req.Extras.Responses)
	}
}

func TestSessionRequestUnifiedUsesBuildRequestPath(t *testing.T) {
	t.Parallel()
	proj := &captureProjector{out: []unified.Message{{Role: unified.RoleUser, Parts: []unified.Part{{Type: unified.PartTypeText, Text: "projected-user"}}}}}
	fs := &fakeStreamer{streams: [][]client.StreamResult{completedText("resp_1", "ok")}}
	s := New(fs, WithModel("gpt-4o-mini"), WithMessageProjector(proj))
	stream, err := s.RequestUnified(context.Background(), Request{Inputs: []Input{{Role: unified.RoleUser, Text: "next"}}})
	if err != nil {
		t.Fatalf("Request() error = %v", err)
	}
	drain(stream)
	if len(fs.requests) != 1 || len(fs.requests[0].Messages) != 1 || fs.requests[0].Messages[0].Parts[0].Text != "projected-user" {
		t.Fatalf("expected streamer request to use projected messages, got %#v", fs.requests)
	}
}




func TestSessionBuildRequestWithCustomRejectingProjectorFailsEarlyOnUnsupportedReplayShape(t *testing.T) {
	t.Parallel()
	proj := &captureProjector{err: errors.New("custom projector rejected replay shape")}
	s := New(&fakeStreamer{}, WithModel("gpt-4o-mini"), WithStrategy(StrategyReplay), WithMessageProjector(proj))
	_, err := s.BuildRequest(Request{Inputs: []Input{{Role: unified.RoleUser, Text: "next"}}})
	if err == nil || err.Error() != "custom projector rejected replay shape" {
		t.Fatalf("expected custom projector error, got %v", err)
	}
}


func completedUsage(responseID string) []client.StreamResult {
	return []client.StreamResult{{Event: unified.StreamEvent{Type: unified.StreamEventUsage, Usage: &unified.StreamUsage{RequestID: responseID, Input: unified.InputTokens{Total: 3, New: 3}, Output: unified.OutputTokens{Total: 2}, Tokens: unified.TokenItems{{Kind: unified.TokenKindInputNew, Count: 3}, {Kind: unified.TokenKindOutput, Count: 2}}}}}}
}

func TestEventMapperMapsMinimalConcreteEvents(t *testing.T) {
	t.Parallel()
	m := eventMapper{}
	items := []streamItem{
		{client.StreamResult{Event: unified.StreamEvent{Type: unified.StreamEventContentDelta, ContentDelta: &unified.ContentDelta{ContentBase: unified.ContentBase{Ref: unified.StreamRef{ResponseID: "resp_1"}, Kind: unified.ContentKindText, Variant: unified.ContentVariantPrimary, Encoding: unified.ContentEncodingUTF8, Data: "hello"}}}}},
		{client.StreamResult{Event: unified.StreamEvent{Type: unified.StreamEventToolCall, ToolCall: &unified.ToolCall{ID: "call_1", Name: "weather", Args: map[string]any{"city": "Berlin"}}}}},
		{client.StreamResult{Event: unified.StreamEvent{Type: unified.StreamEventUsage, Usage: &unified.StreamUsage{RequestID: "resp_1", Input: unified.InputTokens{Total: 1, New: 1}, Output: unified.OutputTokens{Total: 2}}}}},
		{client.StreamResult{Event: unified.StreamEvent{Type: unified.StreamEventCompleted, Lifecycle: &unified.Lifecycle{Scope: unified.LifecycleScopeResponse, State: unified.LifecycleStateDone, Ref: unified.StreamRef{ResponseID: "resp_1"}}, Completed: &unified.Completed{StopReason: unified.StopReasonEndTurn}}}},
	}
	var out []Event
	for _, item := range items {
		out = append(out, m.Map(item)...)
	}
	if len(out) != 4 {
		t.Fatalf("expected four minimal events, got %#v", out)
	}
	if ev, ok := out[0].(TextDeltaEvent); !ok || ev.Text != "hello" || ev.ResponseID != "resp_1" {
		t.Fatalf("unexpected text event: %#v", out[0])
	}
	if ev, ok := out[1].(ToolCallEvent); !ok || ev.ToolCall.ID != "call_1" {
		t.Fatalf("unexpected tool call event: %#v", out[1])
	}
	if ev, ok := out[2].(UsageEvent); !ok || ev.Usage.Output.Total != 2 {
		t.Fatalf("unexpected usage event: %#v", out[2])
	}
	if ev, ok := out[3].(CompletedEvent); !ok || ev.StopReason != unified.StopReasonEndTurn {
		t.Fatalf("unexpected completed event: %#v", out[3])
	}
}

func TestSessionEventsEmitsConcreteTypes(t *testing.T) {
	t.Parallel()
	fs := &fakeStreamer{streams: [][]client.StreamResult{{
		completedText("resp_1", "pong")[0],
		completedUsage("resp_1")[0],
		completedText("resp_1", "pong")[1],
	}}}
	s := New(fs, WithModel("gpt-4o-mini"))
	stream, err := s.Request(context.Background(), Request{Inputs: []Input{{Role: unified.RoleUser, Text: "ping"}}})
	if err != nil {
		t.Fatalf("Events() error = %v", err)
	}
	var got []Event
	for ev := range stream {
		got = append(got, ev)
	}
	if len(got) != 3 {
		t.Fatalf("expected three events, got %#v", got)
	}
	if _, ok := got[0].(TextDeltaEvent); !ok {
		t.Fatalf("expected first event to be TextDeltaEvent, got %#v", got[0])
	}
	if _, ok := got[1].(UsageEvent); !ok {
		t.Fatalf("expected second event to be UsageEvent, got %#v", got[1])
	}
	if _, ok := got[2].(CompletedEvent); !ok {
		t.Fatalf("expected third event to be CompletedEvent, got %#v", got[2])
	}
}

func TestSessionEventsConvertsStreamErrorsToErrorEvent(t *testing.T) {
	t.Parallel()
	fs := &fakeStreamer{streams: [][]client.StreamResult{{{Err: errors.New("boom")}}}}
	s := New(fs, WithModel("gpt-4o-mini"))
	stream, err := s.Request(context.Background(), Request{Inputs: []Input{{Role: unified.RoleUser, Text: "ping"}}})
	if err != nil {
		t.Fatalf("Events() error = %v", err)
	}
	var got []Event
	for ev := range stream {
		got = append(got, ev)
	}
	if len(got) != 1 {
		t.Fatalf("expected one error event, got %#v", got)
	}
	if ev, ok := got[0].(ErrorEvent); !ok || ev.Err == nil || ev.Err.Error() != "boom" {
		t.Fatalf("unexpected error event: %#v", got[0])
	}
}

func TestSessionBuildRequestCarriesTopLevelCacheHint(t *testing.T) {
	t.Parallel()
	s := New(&fakeStreamer{}, WithModel("gpt-4o-mini"))
	req, err := s.BuildRequest(Request{CacheHint: &unified.CacheHint{Enabled: true, TTL: "1h"}, Inputs: []Input{{Role: unified.RoleUser, Text: "ping"}}})
	if err != nil {
		t.Fatalf("BuildRequest() error = %v", err)
	}
	if req.CacheHint == nil || !req.CacheHint.Enabled || req.CacheHint.TTL != "1h" {
		t.Fatalf("expected propagated top-level cache hint, got %#v", req.CacheHint)
	}
}

func TestSessionBuildRequestCarriesMaxTokensAndTemperature(t *testing.T) {
	t.Parallel()
	s := New(&fakeStreamer{}, WithModel("gpt-4o-mini"))
	req, err := s.BuildRequest(Request{MaxTokens: 321, Temperature: 0.7, Inputs: []Input{{Role: unified.RoleUser, Text: "ping"}}})
	if err != nil {
		t.Fatalf("BuildRequest() error = %v", err)
	}
	if req.MaxTokens != 321 || req.Temperature != 0.7 {
		t.Fatalf("expected propagated max_tokens/temperature, got %#v", req)
	}
}

func TestSessionBuildRequestUsesDefaultMaxTokensAndTemperature(t *testing.T) {
	t.Parallel()
	s := New(&fakeStreamer{}, WithModel("gpt-4o-mini"), WithMaxTokens(111), WithTemperature(0.3))
	req, err := s.BuildRequest(Request{Inputs: []Input{{Role: unified.RoleUser, Text: "ping"}}})
	if err != nil {
		t.Fatalf("BuildRequest() error = %v", err)
	}
	if req.MaxTokens != 111 || req.Temperature != 0.3 {
		t.Fatalf("expected default max_tokens/temperature, got %#v", req)
	}
}

func TestSessionBuildRequestRequestValuesOverrideDefaultMaxTokensAndTemperature(t *testing.T) {
	t.Parallel()
	s := New(&fakeStreamer{}, WithModel("gpt-4o-mini"), WithMaxTokens(111), WithTemperature(0.3))
	req, err := s.BuildRequest(Request{MaxTokens: 222, Temperature: 0.8, Inputs: []Input{{Role: unified.RoleUser, Text: "ping"}}})
	if err != nil {
		t.Fatalf("BuildRequest() error = %v", err)
	}
	if req.MaxTokens != 222 || req.Temperature != 0.8 {
		t.Fatalf("expected request override max_tokens/temperature, got %#v", req)
	}
}

func TestSessionBuildRequestCarriesEffortAndThinking(t *testing.T) {
	t.Parallel()
	s := New(&fakeStreamer{}, WithModel("gpt-4o-mini"))
	req, err := s.BuildRequest(Request{Effort: unified.EffortMedium, Thinking: unified.ThinkingModeOn, Inputs: []Input{{Role: unified.RoleUser, Text: "ping"}}})
	if err != nil {
		t.Fatalf("BuildRequest() error = %v", err)
	}
	if req.Effort != unified.EffortMedium || req.Thinking != unified.ThinkingModeOn {
		t.Fatalf("expected propagated effort/thinking, got %#v", req)
	}
}

func TestSessionBuildRequestUsesDefaultEffortAndThinking(t *testing.T) {
	t.Parallel()
	s := New(&fakeStreamer{}, WithModel("gpt-4o-mini"), WithEffort(unified.EffortLow), WithThinking(unified.ThinkingModeOff))
	req, err := s.BuildRequest(Request{Inputs: []Input{{Role: unified.RoleUser, Text: "ping"}}})
	if err != nil {
		t.Fatalf("BuildRequest() error = %v", err)
	}
	if req.Effort != unified.EffortLow || req.Thinking != unified.ThinkingModeOff {
		t.Fatalf("expected default effort/thinking, got %#v", req)
	}
}

func TestSessionBuildRequestRequestValuesOverrideDefaultEffortAndThinking(t *testing.T) {
	t.Parallel()
	s := New(&fakeStreamer{}, WithModel("gpt-4o-mini"), WithEffort(unified.EffortLow), WithThinking(unified.ThinkingModeOff))
	req, err := s.BuildRequest(Request{Effort: unified.EffortHigh, Thinking: unified.ThinkingModeOn, Inputs: []Input{{Role: unified.RoleUser, Text: "ping"}}})
	if err != nil {
		t.Fatalf("BuildRequest() error = %v", err)
	}
	if req.Effort != unified.EffortHigh || req.Thinking != unified.ThinkingModeOn {
		t.Fatalf("expected request override effort/thinking, got %#v", req)
	}
}

func TestSessionBuildRequestUsesDefaultCacheHint(t *testing.T) {
	t.Parallel()
	s := New(&fakeStreamer{}, WithModel("gpt-4o-mini"), WithCacheHint(&unified.CacheHint{Enabled: true, TTL: "1h"}))
	req, err := s.BuildRequest(Request{Inputs: []Input{{Role: unified.RoleUser, Text: "ping"}}})
	if err != nil {
		t.Fatalf("BuildRequest() error = %v", err)
	}
	if req.CacheHint == nil || !req.CacheHint.Enabled || req.CacheHint.TTL != "1h" {
		t.Fatalf("expected default cache hint, got %#v", req.CacheHint)
	}
}

func TestSessionBuildRequestRequestCacheHintOverridesDefault(t *testing.T) {
	t.Parallel()
	s := New(&fakeStreamer{}, WithModel("gpt-4o-mini"), WithCacheHint(&unified.CacheHint{Enabled: true, TTL: "1h"}))
	req, err := s.BuildRequest(Request{CacheHint: &unified.CacheHint{Enabled: true, TTL: "5m"}, Inputs: []Input{{Role: unified.RoleUser, Text: "ping"}}})
	if err != nil {
		t.Fatalf("BuildRequest() error = %v", err)
	}
	if req.CacheHint == nil || req.CacheHint.TTL != "5m" {
		t.Fatalf("expected request cache hint override, got %#v", req.CacheHint)
	}
}

func TestSessionBuildRequestCachePolicyOffSuppressesDefaultCacheHint(t *testing.T) {
	t.Parallel()
	s := New(&fakeStreamer{}, WithModel("gpt-4o-mini"), WithCacheHint(&unified.CacheHint{Enabled: true, TTL: "1h"}), WithCachePolicy(CachePolicyOn))
	req, err := s.BuildRequest(Request{CachePolicy: CachePolicyOff, Inputs: []Input{{Role: unified.RoleUser, Text: "ping"}}})
	if err != nil {
		t.Fatalf("BuildRequest() error = %v", err)
	}
	if req.CacheHint != nil {
		t.Fatalf("expected cache policy off to suppress cache hint, got %#v", req.CacheHint)
	}
}

func TestSessionBuildRequestCachePolicyOnDerivesTopLevelHint(t *testing.T) {
	t.Parallel()
	s := New(&fakeStreamer{}, WithModel("gpt-4o-mini"), WithCachePolicy(CachePolicyOn))
	req, err := s.BuildRequest(Request{Inputs: []Input{{Role: unified.RoleUser, Text: "ping"}}})
	if err != nil {
		t.Fatalf("BuildRequest() error = %v", err)
	}
	if req.CacheHint == nil || !req.CacheHint.Enabled || req.CacheHint.TTL != "1h" {
		t.Fatalf("expected derived cache hint, got %#v", req.CacheHint)
	}
}

func TestSessionBuildRequestCachePolicyProgressiveMarksStableReplayMessages(t *testing.T) {
	t.Parallel()
	fs := &fakeStreamer{streams: [][]client.StreamResult{completedText("resp_1", "pong"), completedText("resp_2", "again")}}
	s := New(fs, WithModel("gpt-4o-mini"), WithStrategy(StrategyReplay), WithCachePolicy(CachePolicyProgressive))
	stream, _ := s.RequestUnified(context.Background(), Request{Inputs: []Input{{Role: unified.RoleUser, Text: "first"}}})
	drain(stream)
	req, err := s.BuildRequest(Request{Inputs: []Input{{Role: unified.RoleUser, Text: "second"}}})
	if err != nil {
		t.Fatalf("BuildRequest() error = %v", err)
	}
	if len(req.Messages) < 3 {
		t.Fatalf("expected replay messages, got %#v", req.Messages)
	}
	if req.Messages[0].CacheHint == nil || !req.Messages[0].CacheHint.Enabled {
		t.Fatalf("expected stable first replay message to be cacheable, got %#v", req.Messages[0])
	}
	if req.Messages[len(req.Messages)-1].CacheHint != nil {
		t.Fatalf("expected newest pending message not to be cacheable under progressive policy, got %#v", req.Messages[len(req.Messages)-1])
	}
}

func TestSessionBuildRequestExplicitCacheHintWinsOverPolicyOff(t *testing.T) {
	t.Parallel()
	s := New(&fakeStreamer{}, WithModel("gpt-4o-mini"), WithCacheHint(&unified.CacheHint{Enabled: true, TTL: "1h"}), WithCachePolicy(CachePolicyOn))
	req, err := s.BuildRequest(Request{CacheHint: &unified.CacheHint{Enabled: true, TTL: "5m"}, CachePolicy: CachePolicyOff, Inputs: []Input{{Role: unified.RoleUser, Text: "ping"}}})
	if err != nil {
		t.Fatalf("BuildRequest() error = %v", err)
	}
	if req.CacheHint == nil || req.CacheHint.TTL != "5m" {
		t.Fatalf("expected explicit request cache hint to win over policy off, got %#v", req.CacheHint)
	}
}
