package adapt

import (
	"encoding/json"

	"github.com/codewandler/agentapis/api/unified"
)

func uint32Ptr(v int) *uint32 {
	u := uint32(v)
	return &u
}

func withRawEventName(ev unified.StreamEvent, name string) unified.StreamEvent {
	ev.Extras.RawEventName = name
	return ev
}

func withProviderExtras(ev unified.StreamEvent, provider any) unified.StreamEvent {
	m := providerMap(provider)
	if len(m) > 0 {
		ev.Extras.Provider = m
	}
	return ev
}

func withRawEventPayload(ev unified.StreamEvent, payload any) unified.StreamEvent {
	if len(ev.Extras.RawJSON) == 0 && payload != nil {
		if rawJSON, err := json.Marshal(payload); err == nil {
			ev.Extras.RawJSON = append([]byte(nil), rawJSON...)
		}
	}
	return ev
}

func providerMap(v any) map[string]any {
	if v == nil {
		return nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	var out map[string]any
	if err := json.Unmarshal(b, &out); err != nil {
		return nil
	}
	return out
}
