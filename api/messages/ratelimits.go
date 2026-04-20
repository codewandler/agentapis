package messages

import (
	"net/http"
	"strconv"
	"time"
)

// RateLimits contains parsed rate limit information from Anthropic API response headers.
type RateLimits struct {
	// Request rate limits
	RequestLimit     int
	RequestRemaining int
	RequestReset     time.Time

	// Token rate limits (combined input + output)
	TokenLimit     int
	TokenRemaining int
	TokenReset     time.Time

	// Input token rate limits
	InputTokenLimit     int
	InputTokenRemaining int
	InputTokenReset     time.Time

	// Output token rate limits
	OutputTokenLimit     int
	OutputTokenRemaining int
	OutputTokenReset     time.Time

	// Request ID for debugging
	RequestID string
}

// ParseRateLimits extracts rate limit information from HTTP response headers.
// Missing or invalid values are left as zero values.
func ParseRateLimits(h http.Header) RateLimits {
	return RateLimits{
		RequestLimit:         parseInt(h.Get(HeaderRateLimitReqLimit)),
		RequestRemaining:     parseInt(h.Get(HeaderRateLimitReqRemaining)),
		RequestReset:         parseTime(h.Get(HeaderRateLimitReqReset)),
		TokenLimit:           parseInt(h.Get(HeaderRateLimitTokLimit)),
		TokenRemaining:       parseInt(h.Get(HeaderRateLimitTokRemaining)),
		TokenReset:           parseTime(h.Get(HeaderRateLimitTokReset)),
		InputTokenLimit:      parseInt(h.Get(HeaderRateLimitInTokLimit)),
		InputTokenRemaining:  parseInt(h.Get(HeaderRateLimitInTokRemaining)),
		InputTokenReset:      parseTime(h.Get(HeaderRateLimitInTokReset)),
		OutputTokenLimit:     parseInt(h.Get(HeaderRateLimitOutTokLimit)),
		OutputTokenRemaining: parseInt(h.Get(HeaderRateLimitOutTokRemaining)),
		OutputTokenReset:     parseTime(h.Get(HeaderRateLimitOutTokReset)),
		RequestID:            h.Get(HeaderRequestID),
	}
}

// HasRequestLimits returns true if request rate limit headers were present.
func (r RateLimits) HasRequestLimits() bool {
	return r.RequestLimit > 0
}

// HasTokenLimits returns true if token rate limit headers were present.
func (r RateLimits) HasTokenLimits() bool {
	return r.TokenLimit > 0
}

// HasInputTokenLimits returns true if input token rate limit headers were present.
func (r RateLimits) HasInputTokenLimits() bool {
	return r.InputTokenLimit > 0
}

// HasOutputTokenLimits returns true if output token rate limit headers were present.
func (r RateLimits) HasOutputTokenLimits() bool {
	return r.OutputTokenLimit > 0
}

// RequestUtilization returns the percentage of request quota used (0.0-1.0).
// Returns 0 if no request limits are available.
func (r RateLimits) RequestUtilization() float64 {
	if r.RequestLimit == 0 {
		return 0
	}
	used := r.RequestLimit - r.RequestRemaining
	return float64(used) / float64(r.RequestLimit)
}

// TokenUtilization returns the percentage of token quota used (0.0-1.0).
// Returns 0 if no token limits are available.
func (r RateLimits) TokenUtilization() float64 {
	if r.TokenLimit == 0 {
		return 0
	}
	used := r.TokenLimit - r.TokenRemaining
	return float64(used) / float64(r.TokenLimit)
}

func parseInt(s string) int {
	if s == "" {
		return 0
	}
	v, _ := strconv.Atoi(s)
	return v
}

func parseTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	// Anthropic uses RFC3339 format
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return t
}
