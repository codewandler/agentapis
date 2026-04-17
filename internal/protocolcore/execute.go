package protocolcore

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/codewandler/agentapis/internal/sse"
)

type ExecuteConfig[Req any] struct {
	HTTPClient    *http.Client
	Logger        *slog.Logger
	ErrorParser   ErrorParser
	RequestHooks  []func(context.Context, RequestMeta[Req])
	ResponseHooks []func(context.Context, ResponseMeta[Req])
}

func ExecuteStream[Req any](ctx context.Context, cfg ExecuteConfig[Req], wire *Req, req HTTPRequest) (<-chan RawEvent, error) {
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	method := req.Method
	if method == "" {
		method = http.MethodPost
	}
	httpReq, err := http.NewRequestWithContext(ctx, method, req.URL, bytes.NewReader(req.Body))
	if err != nil {
		return nil, fmt.Errorf("build HTTP request: %w", err)
	}
	httpReq.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(req.Body)), nil
	}
	httpReq.ContentLength = int64(len(req.Body))
	httpReq.Header = CloneHeaders(req.Headers)
	if httpReq.Header.Get(HeaderContentType) == "" {
		httpReq.Header.Set(HeaderContentType, ContentTypeJSON)
	}
	for _, hook := range cfg.RequestHooks {
		if hook != nil {
			hook(ctx, RequestMeta[Req]{Wire: wire, HTTP: httpReq, Body: append([]byte(nil), req.Body...)})
		}
	}

	resp, err := httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if cfg.ErrorParser != nil {
			return nil, cfg.ErrorParser(resp.StatusCode, body)
		}
		return nil, &HTTPError{StatusCode: resp.StatusCode, Body: body}
	}
	for _, hook := range cfg.ResponseHooks {
		if hook != nil {
			hook(ctx, ResponseMeta[Req]{Wire: wire, StatusCode: resp.StatusCode, Headers: CloneHeaders(resp.Header)})
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
