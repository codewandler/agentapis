package messages

type StreamEvent interface {
	messagesStreamEvent()
	EventType() string
}

type StreamResult struct {
	Event        StreamEvent
	Err          error
	RawEventName string
	RawJSON      []byte
}

func (*MessageStartEvent) messagesStreamEvent()      {}
func (*ContentBlockStartEvent) messagesStreamEvent() {}
func (*ContentBlockDeltaEvent) messagesStreamEvent() {}
func (*ContentBlockStopEvent) messagesStreamEvent()  {}
func (*TextCompleteEvent) messagesStreamEvent()      {}
func (*ThinkingCompleteEvent) messagesStreamEvent()  {}
func (*ToolCompleteEvent) messagesStreamEvent()      {}
func (*MessageDeltaEvent) messagesStreamEvent()      {}
func (*MessageStopEvent) messagesStreamEvent()       {}
func (*StreamErrorEvent) messagesStreamEvent()       {}
func (*PingEvent) messagesStreamEvent()              {}

func (*MessageStartEvent) EventType() string      { return EventMessageStart }
func (*ContentBlockStartEvent) EventType() string { return EventContentBlockStart }
func (*ContentBlockDeltaEvent) EventType() string { return EventContentBlockDelta }
func (*ContentBlockStopEvent) EventType() string  { return EventContentBlockStop }
func (*TextCompleteEvent) EventType() string      { return EventContentBlockStop }
func (*ThinkingCompleteEvent) EventType() string  { return EventContentBlockStop }
func (*ToolCompleteEvent) EventType() string      { return EventContentBlockStop }
func (*MessageDeltaEvent) EventType() string      { return EventMessageDelta }
func (*MessageStopEvent) EventType() string       { return EventMessageStop }
func (*StreamErrorEvent) EventType() string       { return EventError }
func (*PingEvent) EventType() string              { return EventPing }
