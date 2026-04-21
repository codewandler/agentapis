package protocolcore

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"math"
	"math/rand"
	"net/http"
	"time"

	"github.com/codewandler/agentapis/internal/sse"
)

type ExecuteConfig[Req any] struct {
	HTTPClient    *http.Client
	Logger        *slog.Logger
	ErrorParser   ErrorParser
	RequestHooks  []func(context.Context, RequestMeta[Req]) error
	ResponseHooks []func(context.Context, ResponseMeta[Req]) error
}

var ErrEmptyEventStream = errors.New("protocolcore: stream ended without any events")

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

	resp, err := executeStreamRequest(ctx, cfg, wire, httpClient, httpReq, 0)
	if err != nil {
		return nil, err
	}

	retryCfg := (RetryConfig{}).withDefaults()
	out := make(chan RawEvent, 16)
	go func() {
		defer close(out)
		for attempt := 0; attempt <= retryCfg.MaxRetries; attempt++ {
			currentReq := httpReq
			currentResp := resp
			if attempt > 0 {
				var err error
				currentReq, err = CloneRequestForRetry(httpReq)
				if err != nil {
					emitRawEventError(ctx, out, err)
					return
				}
				currentResp, err = executeStreamRequest(ctx, cfg, wire, httpClient, currentReq, attempt)
				if err != nil {
					emitRawEventError(ctx, out, err)
					return
				}
			}

			retry, err := streamSSEAttempt(ctx, cfg, currentReq, currentResp, attempt, retryCfg.MaxRetries, out)
			if err != nil {
				emitRawEventError(ctx, out, err)
				return
			}
			if retry {
				continue
			}
			return
		}
	}()
	return out, nil
}

func executeStreamRequest[Req any](ctx context.Context, cfg ExecuteConfig[Req], wire *Req, httpClient *http.Client, req *http.Request, attempt int) (*http.Response, error) {
	resp, err := httpClient.Do(req)
	if err != nil {
		if cfg.Logger != nil {
			cfg.Logger.Warn("protocolcore stream request failed", slog.String("method", req.Method), slog.String("url", req.URL.String()), slog.Any("err", err), slog.Int("attempt", attempt))
		}
		return nil, err
	}
	if cfg.Logger != nil {
		cfg.Logger.Debug("protocolcore stream response", slog.String("method", req.Method), slog.String("url", req.URL.String()), slog.Int("status", resp.StatusCode), slog.Any("headers", CloneHeaders(resp.Header)), slog.Int("attempt", attempt))
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
	return resp, nil
}

func streamSSEAttempt[Req any](ctx context.Context, cfg ExecuteConfig[Req], req *http.Request, resp *http.Response, attempt, maxRetries int, out chan<- RawEvent) (bool, error) {
	scanner := sse.NewScanner(ctx, resp.Body)
	sawEvent := false
	eventCount := 0
	for {
		event, more, err := scanner.Next()
		if err != nil {
			scanner.Stop()
			_ = resp.Body.Close()
			if cfg.Logger != nil && ctx.Err() == nil {
				cfg.Logger.Warn("protocolcore stream read error", slog.String("method", req.Method), slog.String("url", req.URL.String()), slog.Int("status", resp.StatusCode), slog.Int("event_count", eventCount), slog.Any("err", err), slog.Int("attempt", attempt))
			}
			return false, err
		}
		if len(event.Data) > 0 || event.Name != "" {
			sawEvent = true
			eventCount++
			if cfg.Logger != nil {
				cfg.Logger.Debug("protocolcore stream event", slog.String("method", req.Method), slog.String("url", req.URL.String()), slog.Int("status", resp.StatusCode), slog.Int("event_count", eventCount), slog.String("event_name", event.Name), slog.Int("data_bytes", len(event.Data)), slog.Int("attempt", attempt))
			}
			out <- RawEvent{Name: event.Name, Data: append([]byte(nil), event.Data...)}
		}
		if !more {
			return finishSSEAttempt(ctx, cfg, req, resp, attempt, maxRetries, eventCount, sawEvent)
		}
	}
}

func finishSSEAttempt[Req any](ctx context.Context, cfg ExecuteConfig[Req], req *http.Request, resp *http.Response, attempt, maxRetries, eventCount int, sawEvent bool) (bool, error) {
	_ = resp.Body.Close()
	if sawEvent {
		if cfg.Logger != nil {
			cfg.Logger.Debug("protocolcore stream completed", slog.String("method", req.Method), slog.String("url", req.URL.String()), slog.Int("status", resp.StatusCode), slog.Int("event_count", eventCount), slog.Int("attempt", attempt))
		}
		return false, nil
	}
	if ctx.Err() != nil {
		return false, nil
	}
	if attempt < maxRetries {
		backoff := emptyStreamBackoff(attempt)
		if cfg.Logger != nil {
			cfg.Logger.Warn("protocolcore empty event stream, retrying", slog.String("method", req.Method), slog.String("url", req.URL.String()), slog.Int("status", resp.StatusCode), slog.Any("headers", CloneHeaders(resp.Header)), slog.Int("attempt", attempt+1), slog.Duration("backoff", backoff))
		}
		timer := time.NewTimer(backoff)
		select {
		case <-timer.C:
		case <-ctx.Done():
			timer.Stop()
			return false, nil
		}
		return true, nil
	}
	if cfg.Logger != nil {
		cfg.Logger.Warn("protocolcore empty event stream", slog.String("method", req.Method), slog.String("url", req.URL.String()), slog.Int("status", resp.StatusCode), slog.Any("headers", CloneHeaders(resp.Header)))
	}
	return false, ErrEmptyEventStream
}

func emitRawEventError(ctx context.Context, out chan<- RawEvent, err error) {
	if ctx.Err() == nil {
		out <- RawEvent{Err: err}
	}
}

// emptyStreamBackoff returns an exponential backoff duration with jitter for
// empty-stream retries, using the same formula as retryTransport.backoff.
func emptyStreamBackoff(attempt int) time.Duration {
	const (
		initialBackoff = 500 * time.Millisecond
		maxBackoff     = 8 * time.Second
	)
	exp := math.Pow(2, float64(attempt))
	base := time.Duration(exp * float64(initialBackoff))
	jitter := time.Duration(rand.Int63n(int64(base/2 + 1)))
	delay := base + jitter
	if delay > maxBackoff {
		return maxBackoff
	}
	return delay
}
