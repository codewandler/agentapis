package messages

import (
	"errors"
	"testing"
)

func TestAPIErrorError(t *testing.T) {
	tests := []struct {
		name string
		err  *APIError
		want string
	}{
		{
			name: "with type",
			err:  &APIError{Type: ErrTypeRateLimit, Message: "Too many requests", StatusCode: 429},
			want: "rate_limit_error: Too many requests (HTTP 429)",
		},
		{
			name: "without type",
			err:  &APIError{Message: "Something went wrong", StatusCode: 500},
			want: "Something went wrong (HTTP 500)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.want {
				t.Errorf("APIError.Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAPIErrorIs(t *testing.T) {
	rateLimitErr := &APIError{Type: ErrTypeRateLimit, Message: "Too many requests", StatusCode: 429}

	if !errors.Is(rateLimitErr, ErrRateLimit) {
		t.Error("expected rate limit error to match ErrRateLimit sentinel")
	}
	if errors.Is(rateLimitErr, ErrAuth) {
		t.Error("rate limit error should not match ErrAuth sentinel")
	}
	if errors.Is(rateLimitErr, nil) {
		t.Error("error should not match nil")
	}
}

func TestAPIErrorTypeChecks(t *testing.T) {
	tests := []struct {
		name      string
		err       *APIError
		checkFunc func(*APIError) bool
		want      bool
	}{
		{"rate limit", &APIError{Type: ErrTypeRateLimit}, (*APIError).IsRateLimit, true},
		{"not rate limit", &APIError{Type: ErrTypeAuthentication}, (*APIError).IsRateLimit, false},
		{"auth", &APIError{Type: ErrTypeAuthentication}, (*APIError).IsAuthentication, true},
		{"overloaded", &APIError{Type: ErrTypeOverloaded}, (*APIError).IsOverloaded, true},
		{"invalid request", &APIError{Type: ErrTypeInvalidRequest}, (*APIError).IsInvalidRequest, true},
		{"insufficient quota", &APIError{Type: ErrTypeInsufficientQuota}, (*APIError).IsInsufficientQuota, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.checkFunc(tt.err); got != tt.want {
				t.Errorf("check function returned %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAPIErrorIsRetryable(t *testing.T) {
	tests := []struct {
		name string
		err  *APIError
		want bool
	}{
		{"rate limit", &APIError{Type: ErrTypeRateLimit, StatusCode: 429}, true},
		{"overloaded", &APIError{Type: ErrTypeOverloaded, StatusCode: 529}, true},
		{"api error", &APIError{Type: ErrTypeAPI, StatusCode: 500}, true},
		{"auth error", &APIError{Type: ErrTypeAuthentication, StatusCode: 401}, false},
		{"invalid request", &APIError{Type: ErrTypeInvalidRequest, StatusCode: 400}, false},
		{"5xx without type", &APIError{StatusCode: 503}, true},
		{"4xx without type", &APIError{StatusCode: 400}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.IsRetryable(); got != tt.want {
				t.Errorf("IsRetryable() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseAPIError(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		wantType   string
		wantMsg    string
		wantNil    bool
	}{
		{
			name:       "valid error",
			statusCode: 429,
			body:       `{"type":"error","error":{"type":"rate_limit_error","message":"Too many requests"}}`,
			wantType:   ErrTypeRateLimit,
			wantMsg:    "Too many requests",
		},
		{
			name:       "auth error",
			statusCode: 401,
			body:       `{"type":"error","error":{"type":"authentication_error","message":"Invalid API key"}}`,
			wantType:   ErrTypeAuthentication,
			wantMsg:    "Invalid API key",
		},
		{
			name:       "invalid json",
			statusCode: 500,
			body:       `not json`,
			wantNil:    true,
		},
		{
			name:       "empty message",
			statusCode: 500,
			body:       `{"type":"error","error":{"type":"api_error","message":""}}`,
			wantNil:    true,
		},
		{
			name:       "no error field",
			statusCode: 500,
			body:       `{"type":"error"}`,
			wantNil:    true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseAPIError(tt.statusCode, []byte(tt.body))
			if tt.wantNil {
				if got != nil {
					t.Errorf("ParseAPIError() = %v, want nil", got)
				}
				return
			}
			if got == nil {
				t.Fatal("ParseAPIError() = nil, want non-nil")
			}
			if got.Type != tt.wantType {
				t.Errorf("Type = %q, want %q", got.Type, tt.wantType)
			}
			if got.Message != tt.wantMsg {
				t.Errorf("Message = %q, want %q", got.Message, tt.wantMsg)
			}
			if got.StatusCode != tt.statusCode {
				t.Errorf("StatusCode = %d, want %d", got.StatusCode, tt.statusCode)
			}
		})
	}
}

func TestAsAPIError(t *testing.T) {
	apiErr := &APIError{Type: ErrTypeRateLimit, Message: "test", StatusCode: 429}

	// Should extract from direct APIError
	if got := AsAPIError(apiErr); got != apiErr {
		t.Errorf("AsAPIError() = %v, want %v", got, apiErr)
	}

	// Should return nil for other errors
	otherErr := errors.New("some other error")
	if got := AsAPIError(otherErr); got != nil {
		t.Errorf("AsAPIError() = %v, want nil", got)
	}

	// Should return nil for nil
	if got := AsAPIError(nil); got != nil {
		t.Errorf("AsAPIError(nil) = %v, want nil", got)
	}
}
