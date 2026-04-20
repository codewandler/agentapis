package conversation

import (
	"context"
	"errors"

	"github.com/codewandler/agentapis/api/unified"
	"github.com/codewandler/agentapis/client"
)

var (
	ErrTurnInProgress = errors.New("conversation: turn already in progress")
	ErrModelRequired  = errors.New("conversation: model is required")
	ErrEmptyRequest   = errors.New("conversation: request must contain instructions or inputs")
)

// New creates a new conversation session with static defaults.
func New(streamer Streamer, opts ...Option) *Session {
	cfg := applyOptions(opts)
	return &Session{
		streamer: streamer,
		defaults: sessionDefaults{
			model:       cfg.model,
			maxTokens:   cfg.maxTokens,
			temperature: cfg.temperature,
			effort:      cfg.effort,
			thinking:    cfg.thinking,
			cacheHint:   func() *unified.CacheHint { if cfg.cacheHint == nil { return nil }; cp := *cfg.cacheHint; return &cp }(),
			cachePolicy: cfg.cachePolicy,
			tools:       append([]unified.Tool(nil), cfg.tools...),
			toolChoice:  cfg.toolChoice,
			system:      append([]string(nil), cfg.system...),
			developer:   append([]string(nil), cfg.developer...),
		},
		strategy:  cfg.strategy,
		caps:      cfg.caps,
		projector: cfg.projector,
	}
}

// RequestUnified advances the conversation by one logical request and streams the richer unified event surface.
// Most agent-facing callers should prefer Request, which emits a smaller conversation.Event stream.
func (s *Session) RequestUnified(ctx context.Context, req Request) (<-chan client.StreamResult, error) {
	plan, err := s.beginTurn(req)
	if err != nil {
		return nil, err
	}
	upstream, err := s.streamer.Stream(ctx, plan.request)
	if err != nil {
		s.mu.Lock()
		s.inFlight = false
		s.mu.Unlock()
		return nil, err
	}
	out := make(chan client.StreamResult, 16)
	go s.forwardTurn(upstream, out, plan)
	return out, nil
}

// Request advances the conversation by one logical request and emits the smaller, agent-facing conversation event stream.
func (s *Session) Request(ctx context.Context, req Request) (<-chan Event, error) {
	return s.eventStream(ctx, req)
}

// History returns a safe copy of the committed conversation history.
func (s *Session) History() []unified.Message {
	s.mu.Lock()
	defer s.mu.Unlock()
	return cloneMessages(s.history)
}

// ReasoningHistory returns collected reasoning content per committed turn.
func (s *Session) ReasoningHistory() []ReasoningRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]ReasoningRecord, len(s.reasoning))
	copy(out, s.reasoning)
	return out
}

// Reset clears conversation turn state while preserving static defaults.
func (s *Session) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.history = nil
	s.reasoning = nil
	s.native = nativeState{}
}

// ProjectMessages returns the outbound message projection for the next turn without starting a stream or mutating session state.
func (s *Session) ProjectMessages(req Request) ([]unified.Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx, err := s.buildProjectionContextLocked(req)
	if err != nil {
		return nil, err
	}
	return cloneMessages(ctx.messages), nil
}

// BuildRequest returns the outbound unified request for the next turn without starting a stream or mutating session state.
func (s *Session) BuildRequest(req Request) (unified.Request, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx, err := s.buildProjectionContextLocked(req)
	if err != nil {
		return unified.Request{}, err
	}
	return ctx.request, nil
}


func (s *Session) beginTurn(req Request) (turnPlan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.inFlight {
		return turnPlan{}, ErrTurnInProgress
	}
	plan, err := s.buildTurnPlanLocked(req)
	if err != nil {
		return turnPlan{}, err
	}
	s.inFlight = true
	return plan, nil
}

func (s *Session) buildTurnPlanLocked(req Request) (turnPlan, error) {
	ctx, err := s.buildProjectionContextLocked(req)
	if err != nil {
		return turnPlan{}, err
	}
	return turnPlan{strategy: ctx.strategy, pending: ctx.pending, request: ctx.request}, nil
}

type projectionContext struct {
	strategy Strategy
	pending  []unified.Message
	messages []unified.Message
	request  unified.Request
}

func (s *Session) buildProjectionContextLocked(req Request) (projectionContext, error) {
	pending, err := normalizeRequest(req)
	if err != nil {
		return projectionContext{}, err
	}
	effectiveModel := req.Model
	if effectiveModel == "" {
		effectiveModel = s.defaults.model
	}
	if effectiveModel == "" {
		return projectionContext{}, ErrModelRequired
	}
	strategy := resolveStrategy(s.strategy, s.caps)
	if strategy == StrategyResponsesPreviousResponseID && s.native.lastResponseID == "" {
		strategy = StrategyReplay
	}
	effectiveMaxTokens := req.MaxTokens
	if effectiveMaxTokens == 0 {
		effectiveMaxTokens = s.defaults.maxTokens
	}
	effectiveTemperature := req.Temperature
	if effectiveTemperature == 0 {
		effectiveTemperature = s.defaults.temperature
	}
	effectiveEffort := req.Effort
	if effectiveEffort.IsEmpty() {
		effectiveEffort = s.defaults.effort
	}
	effectiveThinking := req.Thinking
	if effectiveThinking == "" {
		effectiveThinking = s.defaults.thinking
	}
	tools := cloneTools(s.defaults.tools)
	if len(req.Tools) > 0 {
		tools = cloneTools(req.Tools)
	}
	toolChoice := s.defaults.toolChoice
	if req.ToolChoice != nil {
		toolChoice = req.ToolChoice
	}
	projector := s.projector
	if projector == nil {
		projector = DefaultMessageProjector{}
	}
	msgs, err := projector.ProjectMessages(MessageProjectionState{
		Defaults: ProjectionDefaults{
			Model:      s.defaults.model,
			Tools:      cloneTools(s.defaults.tools),
			ToolChoice: s.defaults.toolChoice,
			System:     append([]string(nil), s.defaults.system...),
			Developer:  append([]string(nil), s.defaults.developer...),
		},
		History:        cloneMessages(s.history),
		Pending:        cloneMessages(pending),
		Strategy:       strategy,
		Capabilities:   s.caps,
		LastResponseID: s.native.lastResponseID,
	})
	if err != nil {
		return projectionContext{}, err
	}
	out := unified.Request{Model: effectiveModel, MaxTokens: effectiveMaxTokens, Temperature: effectiveTemperature, Effort: effectiveEffort, Thinking: effectiveThinking, Tools: tools, ToolChoice: toolChoice, Messages: cloneMessages(msgs)}
	effectiveCacheHint, effectiveCachePolicy := resolveCacheSettings(req, s.defaults)
	if effectiveCacheHint != nil {
		h := *effectiveCacheHint
		out.CacheHint = &h
	}
	if effectiveCachePolicy != CachePolicyOff && len(out.Messages) > 0 {
		out.Messages = applyCachePolicy(out.Messages, effectiveCachePolicy, effectiveCacheHint, strategy)
	}
	if strategy == StrategyResponsesPreviousResponseID {
		ensureResponsesExtras(&out).PreviousResponseID = s.native.lastResponseID
	}
	return projectionContext{strategy: strategy, pending: pending, messages: msgs, request: out}, nil
}

func (s *Session) forwardTurn(upstream <-chan client.StreamResult, out chan<- client.StreamResult, plan turnPlan) {
	defer close(out)
	acc := turnAccumulator{}
	failed := false
	for item := range upstream {
		if item.Err != nil {
			failed = true
			out <- item
			continue
		}
		acc.ingest(item.Event)
		out <- item
	}
	result := acc.result()
	s.mu.Lock()
	defer s.mu.Unlock()
	if !failed && result.committed {
		s.applyTurnResultLocked(plan, result)
	}
	s.endTurnLocked()
}

func (s *Session) applyTurnResultLocked(plan turnPlan, result turnResult) {
	s.history = append(s.history, cloneMessages(plan.pending)...)
	if result.assistant != nil {
		s.history = append(s.history, cloneMessage(*result.assistant))
	}
	if result.reasoning.HasContent() {
		s.reasoning = append(s.reasoning, result.reasoning)
	}
	if result.lastResponseID != "" {
		s.native.lastResponseID = result.lastResponseID
	}
}

func (s *Session) endTurnLocked() { s.inFlight = false }

// DefaultMessageProjector implements the library's standard replay/native message projection behavior.
type DefaultMessageProjector struct{}

func (DefaultMessageProjector) ProjectMessages(state MessageProjectionState) ([]unified.Message, error) {
	msgs := make([]unified.Message, 0, len(state.History)+len(state.Pending)+len(state.Defaults.System)+len(state.Defaults.Developer))
	switch state.Strategy {
	case StrategyResponsesPreviousResponseID:
		msgs = append(msgs, cloneMessages(state.Pending)...)
		return msgs, nil
	case StrategyReplay, StrategyAuto:
		for _, line := range state.Defaults.System {
			msgs = append(msgs, textMessage(unified.RoleSystem, line))
		}
		for _, line := range state.Defaults.Developer {
			msgs = append(msgs, textMessage(unified.RoleDeveloper, line))
		}
		msgs = append(msgs, cloneMessages(state.History)...)
		msgs = append(msgs, cloneMessages(state.Pending)...)
		return msgs, nil
	default:
		for _, line := range state.Defaults.System {
			msgs = append(msgs, textMessage(unified.RoleSystem, line))
		}
		for _, line := range state.Defaults.Developer {
			msgs = append(msgs, textMessage(unified.RoleDeveloper, line))
		}
		msgs = append(msgs, cloneMessages(state.History)...)
		msgs = append(msgs, cloneMessages(state.Pending)...)
		return msgs, nil
	}
}

func resolveStrategy(explicit Strategy, caps Capabilities) Strategy {
	switch explicit {
	case StrategyReplay:
		return StrategyReplay
	case StrategyResponsesPreviousResponseID:
		return StrategyResponsesPreviousResponseID
	default:
		if caps.SupportsResponsesPreviousResponseID {
			return StrategyResponsesPreviousResponseID
		}
		return StrategyReplay
	}
}

type turnAccumulator struct {
	lastResponseID    string
	sawCompleted      bool
	parts             []unified.Part
	textDeltaSeen     bool
	reasoningDeltaRaw bool
	reasoningDeltaSum bool
	fallbackText      string
	fallbackRaw       string
	fallbackSummary   string
	reasoning         ReasoningRecord
}

func (a *turnAccumulator) ingest(ev unified.StreamEvent) {
	if ev.Lifecycle != nil && ev.Lifecycle.Ref.ResponseID != "" {
		a.rememberResponseID(ev.Lifecycle.Ref.ResponseID)
	}
	if ev.ContentDelta != nil && ev.ContentDelta.Ref.ResponseID != "" {
		a.rememberResponseID(ev.ContentDelta.Ref.ResponseID)
	}
	if ev.StreamContent != nil && ev.StreamContent.Ref.ResponseID != "" {
		a.rememberResponseID(ev.StreamContent.Ref.ResponseID)
	}
	if ev.Type == unified.StreamEventContentDelta && ev.ContentDelta != nil {
		a.ingestDelta(*ev.ContentDelta)
	}
	if ev.Type == unified.StreamEventContent && ev.StreamContent != nil {
		a.ingestContent(*ev.StreamContent)
	}
	if ev.Type == unified.StreamEventToolCall && ev.ToolCall != nil {
		a.parts = append(a.parts, unified.Part{Type: unified.PartTypeToolCall, ToolCall: &unified.ToolCall{ID: ev.ToolCall.ID, Name: ev.ToolCall.Name, Args: cloneAnyMap(ev.ToolCall.Args)}})
	}
	if ev.Type == unified.StreamEventCompleted && ev.Completed != nil {
		a.sawCompleted = true
	}
}

func (a *turnAccumulator) ingestDelta(delta unified.ContentDelta) {
	if delta.Data == "" {
		return
	}
	switch delta.Kind {
	case unified.ContentKindText:
		a.textDeltaSeen = true
		a.appendText(delta.Data)
	case unified.ContentKindReasoning:
		if delta.Variant == unified.ContentVariantSummary {
			a.reasoningDeltaSum = true
			a.reasoning.Summary += delta.Data
			a.appendThinking("conversation.reasoning.summary", delta.Data)
		} else {
			a.reasoningDeltaRaw = true
			a.reasoning.Raw += delta.Data
			a.appendThinking("conversation.reasoning.raw", delta.Data)
		}
	}
}

func (a *turnAccumulator) ingestContent(content unified.StreamContent) {
	if content.Data == "" {
		return
	}
	switch content.Kind {
	case unified.ContentKindText:
		if !a.textDeltaSeen {
			a.fallbackText += content.Data
		}
	case unified.ContentKindReasoning:
		if content.Variant == unified.ContentVariantSummary {
			if !a.reasoningDeltaSum {
				a.fallbackSummary += content.Data
			}
		} else {
			if !a.reasoningDeltaRaw {
				a.fallbackRaw += content.Data
			}
		}
	}
}

func (a *turnAccumulator) appendText(text string) {
	if text == "" {
		return
	}
	if n := len(a.parts); n > 0 && a.parts[n-1].Type == unified.PartTypeText {
		a.parts[n-1].Text += text
		return
	}
	a.parts = append(a.parts, unified.Part{Type: unified.PartTypeText, Text: text})
}

func (a *turnAccumulator) appendThinking(provider, text string) {
	if text == "" {
		return
	}
	if n := len(a.parts); n > 0 && a.parts[n-1].Type == unified.PartTypeThinking && a.parts[n-1].Thinking != nil && a.parts[n-1].Thinking.Provider == provider {
		a.parts[n-1].Thinking.Text += text
		return
	}
	a.parts = append(a.parts, unified.Part{Type: unified.PartTypeThinking, Thinking: &unified.ThinkingPart{Provider: provider, Text: text}})
}

func (a *turnAccumulator) rememberResponseID(id string) {
	if a.lastResponseID == "" && id != "" {
		a.lastResponseID = id
	}
}

func (a *turnAccumulator) result() turnResult {
	if !a.sawCompleted {
		return turnResult{}
	}
	if !a.textDeltaSeen && a.fallbackText != "" {
		a.appendText(a.fallbackText)
	}
	if !a.reasoningDeltaRaw && a.fallbackRaw != "" {
		a.reasoning.Raw += a.fallbackRaw
		a.appendThinking("conversation.reasoning.raw", a.fallbackRaw)
	}
	if !a.reasoningDeltaSum && a.fallbackSummary != "" {
		a.reasoning.Summary += a.fallbackSummary
		a.appendThinking("conversation.reasoning.summary", a.fallbackSummary)
	}
	res := turnResult{lastResponseID: a.lastResponseID, committed: true, reasoning: a.reasoning}
	if len(a.parts) > 0 {
		msg := unified.Message{Role: unified.RoleAssistant, Parts: cloneParts(a.parts)}
		res.assistant = &msg
	}
	return res
}

type turnPlan struct {
	strategy Strategy
	pending  []unified.Message
	request  unified.Request
}

type turnResult struct {
	assistant      *unified.Message
	reasoning      ReasoningRecord
	lastResponseID string
	committed      bool
}

func normalizeRequest(req Request) ([]unified.Message, error) {
	if len(req.Instructions) == 0 && len(req.Inputs) == 0 {
		return nil, ErrEmptyRequest
	}
	out := make([]unified.Message, 0, len(req.Instructions)+len(req.Inputs))
	for _, line := range req.Instructions {
		if line == "" {
			continue
		}
		out = append(out, textMessage(unified.RoleDeveloper, line))
	}
	for _, in := range req.Inputs {
		switch {
		case in.ToolResult != nil:
			out = append(out, unified.Message{Role: unified.RoleTool, Parts: []unified.Part{{Type: unified.PartTypeToolResult, ToolResult: &unified.ToolResult{ToolCallID: in.ToolResult.ToolCallID, ToolOutput: in.ToolResult.Output, IsError: in.ToolResult.IsError}}}})
		case in.Text != "":
			out = append(out, textMessage(in.Role, in.Text))
		}
	}
	if len(out) == 0 {
		return nil, ErrEmptyRequest
	}
	return out, nil
}

func textMessage(role unified.Role, text string) unified.Message {
	return unified.Message{Role: role, Parts: []unified.Part{{Type: unified.PartTypeText, Text: text}}}
}

func cloneMessages(in []unified.Message) []unified.Message {
	out := make([]unified.Message, len(in))
	for i, m := range in {
		out[i] = cloneMessage(m)
	}
	return out
}

func cloneParts(in []unified.Part) []unified.Part {
	if in == nil {
		return nil
	}
	out := make([]unified.Part, len(in))
	copy(out, in)
	for i := range out {
		if out[i].ToolCall != nil {
			tc := *out[i].ToolCall
			tc.Args = cloneAnyMap(tc.Args)
			out[i].ToolCall = &tc
		}
		if out[i].ToolResult != nil {
			tr := *out[i].ToolResult
			out[i].ToolResult = &tr
		}
		if out[i].Thinking != nil {
			th := *out[i].Thinking
			out[i].Thinking = &th
		}
	}
	return out
}

func cloneMessage(m unified.Message) unified.Message {
	out := m
	out.Parts = cloneParts(m.Parts)
	return out
}

func cloneTools(in []unified.Tool) []unified.Tool {
	out := make([]unified.Tool, len(in))
	for i, t := range in {
		out[i] = t
		out[i].Parameters = cloneAnyMap(t.Parameters)
	}
	return out
}

func cloneAnyMap(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func ensureResponsesExtras(req *unified.Request) *unified.ResponsesExtras {
	if req.Extras.Responses == nil {
		req.Extras.Responses = &unified.ResponsesExtras{}
	}
	return req.Extras.Responses
}


func resolveCacheSettings(req Request, defaults sessionDefaults) (*unified.CacheHint, CachePolicy) {
	policy := req.CachePolicy
	if policy == CachePolicySessionDefault {
		policy = defaults.cachePolicy
	}
	if req.CacheHint != nil {
		h := *req.CacheHint
		return &h, policy
	}
	if policy == CachePolicyOff {
		return nil, CachePolicyOff
	}
	if defaults.cacheHint != nil {
		h := *defaults.cacheHint
		return &h, policy
	}
	if policy == CachePolicyOn || policy == CachePolicyProgressive {
		return &unified.CacheHint{Enabled: true, TTL: "1h"}, policy
	}
	return nil, policy
}

func applyCachePolicy(msgs []unified.Message, policy CachePolicy, hint *unified.CacheHint, strategy Strategy) []unified.Message {
	if policy == CachePolicyOff || hint == nil {
		return cloneMessages(msgs)
	}
	out := cloneMessages(msgs)
	if strategy == StrategyResponsesPreviousResponseID {
		return out
	}
	apply := func(m *unified.Message) {
		cp := *hint
		m.CacheHint = &cp
	}
	switch policy {
	case CachePolicyOn:
		for i := range out {
			apply(&out[i])
		}
	case CachePolicyProgressive:
		stableEnd := len(out) - 1
		if stableEnd < 0 {
			return out
		}
		for i := 0; i < stableEnd; i++ {
			apply(&out[i])
		}
	case CachePolicySessionDefault:
		return out
	}
	return out
}
