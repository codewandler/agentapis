package messages

const (
	HeaderAPIKey           = "x-api-key"
	HeaderAnthropicVersion = "anthropic-version"
	HeaderAnthropicBeta    = "anthropic-beta"
)

const APIVersion = "2023-06-01"

const (
	BetaInterleavedThinking = "interleaved-thinking-2025-05-14"
)

const (
	HeaderRateLimitReqLimit        = "anthropic-ratelimit-requests-limit"
	HeaderRateLimitReqRemaining    = "anthropic-ratelimit-requests-remaining"
	HeaderRateLimitReqReset        = "anthropic-ratelimit-requests-reset"
	HeaderRateLimitTokLimit        = "anthropic-ratelimit-tokens-limit"
	HeaderRateLimitTokRemaining    = "anthropic-ratelimit-tokens-remaining"
	HeaderRateLimitTokReset        = "anthropic-ratelimit-tokens-reset"
	HeaderRateLimitInTokLimit      = "anthropic-ratelimit-input-tokens-limit"
	HeaderRateLimitInTokRemaining  = "anthropic-ratelimit-input-tokens-remaining"
	HeaderRateLimitInTokReset      = "anthropic-ratelimit-input-tokens-reset"
	HeaderRateLimitOutTokLimit     = "anthropic-ratelimit-output-tokens-limit"
	HeaderRateLimitOutTokRemaining = "anthropic-ratelimit-output-tokens-remaining"
	HeaderRateLimitOutTokReset     = "anthropic-ratelimit-output-tokens-reset"
	HeaderRequestID                = "request-id"
)

const (
	EventMessageStart      = "message_start"
	EventContentBlockStart = "content_block_start"
	EventContentBlockDelta = "content_block_delta"
	EventContentBlockStop  = "content_block_stop"
	EventMessageDelta      = "message_delta"
	EventMessageStop       = "message_stop"
	EventError             = "error"
	EventPing              = "ping"
)

const (
	BlockTypeText                = "text"
	BlockTypeToolUse             = "tool_use"
	BlockTypeThinking            = "thinking"
	BlockTypeRedactedThinking    = "redacted_thinking"
	BlockTypeServerToolUse       = "server_tool_use"
	BlockTypeWebSearchToolResult = "web_search_tool_result"
)

const (
	DeltaTypeText      = "text_delta"
	DeltaTypeInputJSON = "input_json_delta"
	DeltaTypeThinking  = "thinking_delta"
	DeltaTypeSignature = "signature_delta"
)

const (
	StopReasonEndTurn = "end_turn"
	StopReasonToolUse = "tool_use"
	StopReasonMaxTok  = "max_tokens"
)

const DefaultPath = "/v1/messages"

const (
	ThinkingDisplaySummarized = "summarized"
	ThinkingDisplayOmitted    = "omitted"
)
