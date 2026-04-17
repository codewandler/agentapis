package completions

import (
	"context"

	"github.com/codewandler/agentapis/internal/protocolcore"
)

type RequestMeta = protocolcore.RequestMeta[Request]
type ResponseMeta = protocolcore.ResponseMeta[Request]

type CallOptions struct {
	OnRequest  func(ctx context.Context, meta RequestMeta) error
	OnResponse func(ctx context.Context, meta ResponseMeta) error
}
