package protocolcore

import (
	"context"
	"io"
	"log/slog"
	"net/http"

	"github.com/codewandler/agentapis/internal/sse"
)

type ExecuteConfig[Req any] struct {
	HTTPClient    *http.Client
	Logger        *slog.Logger
	ErrorParser   ErrorParser
	RequestHooks  []func(context.Context, RequestMeta[Req]) error
	ResponseHooks []func(context.Context, ResponseMeta[Req]) error
}

func ExecuteStream[Req any](ctx context.Context, cfg ExecuteConfig[Req], wire *Req, httpReq *http.Request, body []byte) (<-chan RawEvent, error) {
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	if httpReq.Header.Get(HeaderContentType) == "" {
		httpReq.Header.Set(HeaderContentType, ContentTypeJSON)
	}
	for _, hook := range cfg.RequestHooks {
		if hook != nil {
			if err := hook(ctx, RequestMeta[Req]{Wire: wire, HTTP: httpReq, Body: append([]byte(nil), body...)}); err != nil {
				return nil, err
			}
		}
	}

	resp, err := httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if cfg.ErrorParser != nil {
			parseErr := cfg.ErrorParser(resp.StatusCode, body)
			if parseErr != nil {
				return nil, &StatusError{StatusCode: resp.StatusCode, Body: body, Err: parseErr}
			}
		}
		return nil, &StatusError{StatusCode: resp.StatusCode, Body: body}
	}
	for _, hook := range cfg.ResponseHooks {
		if hook != nil {
			if err := hook(ctx, ResponseMeta[Req]{Wire: wire, StatusCode: resp.StatusCode, Headers: CloneHeaders(resp.Header)}); err != nil {
				_ = resp.Body.Close()
				return nil, err
			}
		}
	}

	out := make(chan RawEvent, 16)
	go func() {
		defer close(out)
		defer resp.Body.Close()

		scanner := sse.NewScanner(ctx, resp.Body)
		defer scanner.Stop()

		for {
			event, more, err := scanner.Next()
			if err != nil {
				if ctx.Err() == nil {
					out <- RawEvent{Err: err}
				}
				return
			}
			if len(event.Data) > 0 || event.Name != "" {
				out <- RawEvent{Name: event.Name, Data: append([]byte(nil), event.Data...)}
			}
			if !more {
				return
			}
		}
	}()
	return out, nil
}
