package adapt

import (
	"fmt"

	"github.com/codewandler/agentapis/api/unified"
)

func Validate(r unified.Request) error {
	if r.Model == "" {
		return fmt.Errorf("model is required")
	}
	if len(r.Messages) == 0 {
		return fmt.Errorf("messages are required")
	}
	if err := validateOutput(r.Output); err != nil {
		return err
	}
	if err := validateCacheHint("request cache hint", r.CacheHint); err != nil {
		return err
	}
	for i, m := range r.Messages {
		if err := validateMessage(fmt.Sprintf("messages[%d]", i), m); err != nil {
			return err
		}
		if err := validateCacheHint(fmt.Sprintf("messages[%d].cache_hint", i), m.CacheHint); err != nil {
			return err
		}
	}
	return nil
}

func validateOutput(out *unified.OutputSpec) error {
	if out == nil {
		return nil
	}
	switch out.Mode {
	case unified.OutputModeText, unified.OutputModeJSONObject:
		if out.Schema != nil {
			return fmt.Errorf("output schema requires mode json_schema")
		}
		return nil
	case unified.OutputModeJSONSchema:
		if out.Schema == nil {
			return fmt.Errorf("output json_schema mode requires schema")
		}
		return nil
	default:
		return fmt.Errorf("invalid output mode %q", out.Mode)
	}
}

func validateCacheHint(field string, h *unified.CacheHint) error {
	if h == nil {
		return nil
	}
	switch h.TTL {
	case "", "5m", "1h":
		return nil
	default:
		return fmt.Errorf("%s ttl must be one of: %q, %q, %q", field, "", "5m", "1h")
	}
}

func validateMessage(field string, m unified.Message) error {
	if !m.Phase.Valid() {
		return fmt.Errorf("%s phase must be one of: %q, %q, %q", field, "", "commentary", "final_answer")
	}
	if !m.Phase.IsEmpty() && m.Role != unified.RoleAssistant {
		return fmt.Errorf("%s phase is only valid for assistant role", field)
	}
	return nil
}
