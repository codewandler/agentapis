package messages

import (
	"encoding/json"
	"fmt"
	"strings"
)

type textAccum struct{ buf strings.Builder }

type thinkingAccum struct {
	thinking  strings.Builder
	signature strings.Builder
}

type toolAccum struct {
	id     string
	name   string
	argBuf strings.Builder
}

type Parser struct {
	activeText     map[int]*textAccum
	activeThinking map[int]*thinkingAccum
	activeTools    map[int]*toolAccum
}

func NewParser() *Parser {
	return &Parser{
		activeText:     make(map[int]*textAccum),
		activeThinking: make(map[int]*thinkingAccum),
		activeTools:    make(map[int]*toolAccum),
	}
}

func (p *Parser) Parse(name string, data []byte) (StreamEvent, error) {
	switch name {
	case EventMessageStart:
		var evt MessageStartEvent
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("parse message_start: %w", err)
		}
		return &evt, nil

	case EventContentBlockStart:
		var evt ContentBlockStartEvent
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("parse content_block_start: %w", err)
		}

		var block StartBlockView
		if err := json.Unmarshal(evt.ContentBlock, &block); err == nil {
			switch block.Type {
			case BlockTypeText:
				p.activeText[evt.Index] = &textAccum{}
			case BlockTypeThinking:
				p.activeThinking[evt.Index] = &thinkingAccum{}
			case BlockTypeRedactedThinking:
				a := &thinkingAccum{}
				a.signature.WriteString(block.Data)
				p.activeThinking[evt.Index] = a
			case BlockTypeToolUse:
				p.activeTools[evt.Index] = &toolAccum{id: block.ID, name: block.Name}
			}
		}
		return &evt, nil

	case EventContentBlockDelta:
		var evt ContentBlockDeltaEvent
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("parse content_block_delta: %w", err)
		}

		switch evt.Delta.Type {
		case DeltaTypeText:
			if a := p.activeText[evt.Index]; a != nil {
				a.buf.WriteString(evt.Delta.Text)
			}
		case DeltaTypeThinking:
			if a := p.activeThinking[evt.Index]; a != nil {
				a.thinking.WriteString(evt.Delta.Thinking)
			}
		case DeltaTypeSignature:
			if a := p.activeThinking[evt.Index]; a != nil {
				a.signature.WriteString(evt.Delta.Signature)
			}
		case DeltaTypeInputJSON:
			if a := p.activeTools[evt.Index]; a != nil {
				a.argBuf.WriteString(evt.Delta.PartialJSON)
			}
		}
		return &evt, nil

	case EventContentBlockStop:
		var evt ContentBlockStopEvent
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("parse content_block_stop: %w", err)
		}
		idx := evt.Index

		if a, ok := p.activeText[idx]; ok {
			delete(p.activeText, idx)
			return &TextCompleteEvent{Index: idx, Text: a.buf.String()}, nil
		}
		if a, ok := p.activeThinking[idx]; ok {
			delete(p.activeThinking, idx)
			return &ThinkingCompleteEvent{
				Index: idx, Thinking: a.thinking.String(), Signature: a.signature.String(),
			}, nil
		}
		if a, ok := p.activeTools[idx]; ok {
			delete(p.activeTools, idx)
			var args map[string]any
			if a.argBuf.Len() > 0 {
				_ = json.Unmarshal([]byte(a.argBuf.String()), &args)
			}
			return &ToolCompleteEvent{Index: idx, ID: a.id, Name: a.name, RawInput: a.argBuf.String(), Args: args}, nil
		}

		return &evt, nil

	case EventMessageDelta:
		var evt MessageDeltaEvent
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("parse message_delta: %w", err)
		}
		return &evt, nil

	case EventPing:
		return &PingEvent{}, nil

	case EventMessageStop:
		return &MessageStopEvent{}, nil

	case EventError:
		var evt StreamErrorEvent
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("parse error event: %w", err)
		}
		return &evt, nil

	default:
		return nil, nil
	}
}
