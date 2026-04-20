package messages

import (
	"net/http"
	"testing"
	"time"
)

func TestParseRateLimitsFromHeaders(t *testing.T) {
	h := http.Header{}
	h.Set(HeaderRateLimitReqLimit, "100")
	h.Set(HeaderRateLimitReqRemaining, "95")
	h.Set(HeaderRateLimitReqReset, "2024-01-15T10:00:00Z")
	h.Set(HeaderRateLimitTokLimit, "100000")
	h.Set(HeaderRateLimitTokRemaining, "80000")
	h.Set(HeaderRateLimitTokReset, "2024-01-15T10:05:00Z")
	h.Set(HeaderRateLimitInTokLimit, "50000")
	h.Set(HeaderRateLimitInTokRemaining, "40000")
	h.Set(HeaderRateLimitInTokReset, "2024-01-15T10:10:00Z")
	h.Set(HeaderRateLimitOutTokLimit, "50000")
	h.Set(HeaderRateLimitOutTokRemaining, "40000")
	h.Set(HeaderRateLimitOutTokReset, "2024-01-15T10:15:00Z")
	h.Set(HeaderRequestID, "req_12345")

	rl := ParseRateLimits(h)

	if rl.RequestLimit != 100 {
		t.Errorf("RequestLimit = %d, want 100", rl.RequestLimit)
	}
	if rl.RequestRemaining != 95 {
		t.Errorf("RequestRemaining = %d, want 95", rl.RequestRemaining)
	}
	if rl.RequestReset.IsZero() {
		t.Error("RequestReset should not be zero")
	}
	if rl.TokenLimit != 100000 {
		t.Errorf("TokenLimit = %d, want 100000", rl.TokenLimit)
	}
	if rl.TokenRemaining != 80000 {
		t.Errorf("TokenRemaining = %d, want 80000", rl.TokenRemaining)
	}
	if rl.InputTokenLimit != 50000 {
		t.Errorf("InputTokenLimit = %d, want 50000", rl.InputTokenLimit)
	}
	if rl.OutputTokenLimit != 50000 {
		t.Errorf("OutputTokenLimit = %d, want 50000", rl.OutputTokenLimit)
	}
	if rl.RequestID != "req_12345" {
		t.Errorf("RequestID = %q, want %q", rl.RequestID, "req_12345")
	}
}

func TestParseRateLimitsEmptyHeaders(t *testing.T) {
	h := http.Header{}
	rl := ParseRateLimits(h)

	if rl.RequestLimit != 0 {
		t.Errorf("RequestLimit = %d, want 0", rl.RequestLimit)
	}
	if rl.RequestRemaining != 0 {
		t.Errorf("RequestRemaining = %d, want 0", rl.RequestRemaining)
	}
	if !rl.RequestReset.IsZero() {
		t.Error("RequestReset should be zero")
	}
	if rl.RequestID != "" {
		t.Errorf("RequestID = %q, want empty", rl.RequestID)
	}
}

func TestParseRateLimitsInvalidValues(t *testing.T) {
	h := http.Header{}
	h.Set(HeaderRateLimitReqLimit, "not-a-number")
	h.Set(HeaderRateLimitReqReset, "not-a-time")

	rl := ParseRateLimits(h)

	if rl.RequestLimit != 0 {
		t.Errorf("RequestLimit = %d, want 0 for invalid input", rl.RequestLimit)
	}
	if !rl.RequestReset.IsZero() {
		t.Error("RequestReset should be zero for invalid input")
	}
}

func TestRateLimitsHasMethods(t *testing.T) {
	rl := RateLimits{}
	if rl.HasRequestLimits() {
		t.Error("HasRequestLimits should return false when limit is 0")
	}
	if rl.HasTokenLimits() {
		t.Error("HasTokenLimits should return false when limit is 0")
	}
	if rl.HasInputTokenLimits() {
		t.Error("HasInputTokenLimits should return false when limit is 0")
	}
	if rl.HasOutputTokenLimits() {
		t.Error("HasOutputTokenLimits should return false when limit is 0")
	}

	rl.RequestLimit = 100
	rl.TokenLimit = 100000
	rl.InputTokenLimit = 50000
	rl.OutputTokenLimit = 50000

	if !rl.HasRequestLimits() {
		t.Error("HasRequestLimits should return true when limit is set")
	}
	if !rl.HasTokenLimits() {
		t.Error("HasTokenLimits should return true when limit is set")
	}
	if !rl.HasInputTokenLimits() {
		t.Error("HasInputTokenLimits should return true when limit is set")
	}
	if !rl.HasOutputTokenLimits() {
		t.Error("HasOutputTokenLimits should return true when limit is set")
	}
}

func TestRateLimitsUtilization(t *testing.T) {
	rl := RateLimits{
		RequestLimit:     100,
		RequestRemaining: 80,
		TokenLimit:       100000,
		TokenRemaining:   75000,
	}

	reqUtil := rl.RequestUtilization()
	if reqUtil != 0.2 {
		t.Errorf("RequestUtilization = %f, want 0.2", reqUtil)
	}

	tokUtil := rl.TokenUtilization()
	if tokUtil != 0.25 {
		t.Errorf("TokenUtilization = %f, want 0.25", tokUtil)
	}
}

func TestRateLimitsUtilizationZeroLimit(t *testing.T) {
	rl := RateLimits{}

	if rl.RequestUtilization() != 0 {
		t.Error("RequestUtilization should return 0 when limit is 0")
	}
	if rl.TokenUtilization() != 0 {
		t.Error("TokenUtilization should return 0 when limit is 0")
	}
}

func TestParseTimeRFC3339(t *testing.T) {
	// Test valid RFC3339 time
	expected := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	result := parseTime("2024-01-15T10:30:00Z")
	if !result.Equal(expected) {
		t.Errorf("parseTime = %v, want %v", result, expected)
	}

	// Test with timezone offset
	result2 := parseTime("2024-01-15T12:30:00+02:00")
	if result2.IsZero() {
		t.Error("parseTime should parse time with timezone offset")
	}
}
