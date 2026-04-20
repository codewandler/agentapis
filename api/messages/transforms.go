package messages

import "context"

// AutoSystemCacheControl returns a RequestTransform that adds cache control
// to the last system block. This is useful for Anthropic's prompt caching
// feature where you want to cache the full system prompt.
//
// Usage:
//
//	client := messages.New(
//	    messages.WithRequestTransform(messages.AutoSystemCacheControl()),
//	)
func AutoSystemCacheControl() RequestTransform {
	return AutoSystemCacheControlWithTTL("")
}

// AutoSystemCacheControlWithTTL returns a RequestTransform that adds cache control
// with a specific TTL to the last system block.
//
// If ttl is empty, uses the default "ephemeral" type without TTL.
func AutoSystemCacheControlWithTTL(ttl string) RequestTransform {
	return func(_ context.Context, req *Request) error {
		if len(req.System) == 0 {
			return nil
		}
		last := req.System[len(req.System)-1]
		if last == nil {
			return nil
		}
		// Only add if not already set
		if last.CacheControl != nil {
			return nil
		}
		cc := &CacheControl{Type: "ephemeral"}
		if ttl != "" {
			cc.TTL = ttl
		}
		last.CacheControl = cc
		return nil
	}
}
