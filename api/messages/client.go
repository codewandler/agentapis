package messages

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/codewandler/agentapis/internal/protocolcore"
)

const defaultBaseURL = "https://api.anthropic.com"

type Client struct {
	baseURL             string
	path                string
	headers             http.Header
	headerFuncs         []HeaderFunc
	requestTransforms   []RequestTransform
	httpRequestMutators []HTTPRequestMutator
	eventTransforms     []EventTransform
	requestHooks        []RequestHook
	responseHooks       []ResponseHook
	httpClient          *http.Client
	logger              *slog.Logger
	parser              func() *Parser
	errorParser         ErrorParser
}

func NewClient(opts ...Option) *Client {
	cfg := applyOptions(opts)
	headers := protocolcore.CloneHeaders(cfg.headers)
	headers.Set(HeaderAnthropicVersion, APIVersion)
	baseURL := cfg.baseURL
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return &Client{
		baseURL:             baseURL,
		path:                cfg.path,
		headers:             headers,
		headerFuncs:         appendBuiltInHeaderFunc(cfg.apiKey, cfg.headerFuncs),
		requestTransforms:   append([]RequestTransform(nil), cfg.requestTransforms...),
		httpRequestMutators: append([]HTTPRequestMutator(nil), cfg.httpRequestMutators...),
		eventTransforms:     append([]EventTransform(nil), cfg.eventTransforms...),
		requestHooks:        append([]RequestHook(nil), cfg.requestHooks...),
		responseHooks:       append([]ResponseHook(nil), cfg.responseHooks...),
		httpClient:          cfg.httpClient,
		logger:              cfg.logger,
		parser:              NewParser,
		errorParser:         cfg.errorParser,
	}
}

func (c *Client) Stream(ctx context.Context, req Request) (<-chan StreamResult, error) {
	return c.StreamWithOptions(ctx, req, CallOptions{})
}

func (c *Client) StreamWithOptions(ctx context.Context, req Request, opts CallOptions) (<-chan StreamResult, error) {
	wire := req
	wire.Stream = true
	for _, transform := range c.requestTransforms {
		if err := transform(ctx, &wire); err != nil {
			return nil, fmt.Errorf("transform request: %w", err)
		}
	}
	body, err := json.Marshal(wire)
	if err != nil {
		return nil, fmt.Errorf("serialize request: %w", err)
	}
	headers, err := resolveHeaders(ctx, c.headers, c.headerFuncs, &wire)
	if err != nil {
		return nil, fmt.Errorf("build request headers: %w", err)
	}
	path := c.path
	if path == "" {
		path = DefaultPath
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, protocolcore.JoinURL(c.baseURL, path), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build HTTP request: %w", err)
	}
	httpReq.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(body)), nil
	}
	httpReq.ContentLength = int64(len(body))
	httpReq.Header = protocolcore.CloneHeaders(headers)
	if httpReq.Header.Get(protocolcore.HeaderContentType) == "" {
		httpReq.Header.Set(protocolcore.HeaderContentType, protocolcore.ContentTypeJSON)
	}
	for _, mutate := range c.httpRequestMutators {
		if err := mutate(ctx, httpReq, &wire); err != nil {
			return nil, fmt.Errorf("mutate HTTP request: %w", err)
		}
	}
	finalBody, err := protocolcore.ReadAndRestoreBody(httpReq)
	if err != nil {
		return nil, fmt.Errorf("read HTTP request body: %w", err)
	}
	requestHooks := make([]func(context.Context, protocolcore.RequestMeta[Request]) error, 0, len(c.requestHooks)+1)
	for _, hook := range c.requestHooks {
		if hook == nil {
			continue
		}
		requestHooks = append(requestHooks, func(ctx context.Context, meta protocolcore.RequestMeta[Request]) error {
			hook(ctx, meta)
			return nil
		})
	}
	if opts.OnRequest != nil {
		requestHooks = append(requestHooks, opts.OnRequest)
	}
	responseHooks := make([]func(context.Context, protocolcore.ResponseMeta[Request]) error, 0, len(c.responseHooks)+1)
	for _, hook := range c.responseHooks {
		if hook == nil {
			continue
		}
		responseHooks = append(responseHooks, func(ctx context.Context, meta protocolcore.ResponseMeta[Request]) error {
			hook(ctx, meta)
			return nil
		})
	}
	if opts.OnResponse != nil {
		responseHooks = append(responseHooks, opts.OnResponse)
	}
	errorParser := parseAnthropicError
	if c.errorParser != nil {
		errorParser = c.errorParser
	}
	raw, err := protocolcore.ExecuteStream(ctx, protocolcore.ExecuteConfig[Request]{
		HTTPClient:    c.httpClient,
		Logger:        c.logger,
		ErrorParser:   errorParser,
		RequestHooks:  requestHooks,
		ResponseHooks: responseHooks,
	}, &wire, httpReq, finalBody)
	if err != nil {
		return nil, err
	}
	parser := c.parser()
	out := make(chan StreamResult, 16)
	go func() {
		defer close(out)
		for item := range raw {
			if item.Err != nil {
				out <- StreamResult{Err: item.Err}
				continue
			}
			event, err := parser.Parse(item.Name, item.Data)
			if err != nil {
				out <- StreamResult{Err: err, RawEventName: item.Name, RawJSON: append([]byte(nil), item.Data...)}
				continue
			}
			if event == nil {
				continue
			}
			ignored := false
			for _, transform := range c.eventTransforms {
				event, ignored, err = transform(ctx, event)
				if err != nil {
					out <- StreamResult{Err: err, RawEventName: item.Name, RawJSON: append([]byte(nil), item.Data...)}
					ignored = true
					break
				}
				if ignored || event == nil {
					break
				}
			}
			if ignored || event == nil {
				continue
			}
			out <- StreamResult{Event: event, RawEventName: item.Name, RawJSON: append([]byte(nil), item.Data...)}
		}
	}()
	return out, nil
}
func parseAnthropicError(statusCode int, body []byte) error {
	if apiErr := ParseAPIError(statusCode, body); apiErr != nil {
		return apiErr
	}
	return &protocolcore.HTTPError{StatusCode: statusCode, Body: body}
}
