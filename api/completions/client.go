package completions

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/codewandler/agentapis/internal/protocolcore"
)

const defaultBaseURL = "https://api.openai.com"

type Client struct {
	baseURL             string
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
}

func NewClient(opts ...Option) *Client {
	cfg := applyOptions(opts)
	baseURL := cfg.baseURL
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return &Client{
		baseURL:             baseURL,
		headers:             protocolcore.CloneHeaders(cfg.headers),
		headerFuncs:         appendBuiltInHeaderFunc(cfg.apiKey, cfg.headerFuncs),
		requestTransforms:   append([]RequestTransform(nil), cfg.requestTransforms...),
		httpRequestMutators: append([]HTTPRequestMutator(nil), cfg.httpRequestMutators...),
		eventTransforms:     append([]EventTransform(nil), cfg.eventTransforms...),
		requestHooks:        append([]RequestHook(nil), cfg.requestHooks...),
		responseHooks:       append([]ResponseHook(nil), cfg.responseHooks...),
		httpClient:          cfg.httpClient,
		logger:              cfg.logger,
		parser:              NewParser,
	}
}

func (c *Client) Stream(ctx context.Context, req Request) (<-chan StreamResult, error) {
	wire := req
	wire.Stream = true
	if wire.StreamOptions == nil {
		wire.StreamOptions = &StreamOptions{}
	}
	wire.StreamOptions.IncludeUsage = true
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
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, protocolcore.JoinURL(c.baseURL, DefaultPath), nil)
	if err != nil {
		return nil, fmt.Errorf("build HTTP request: %w", err)
	}
	httpReq.Header = protocolcore.CloneHeaders(headers)
	for _, mutate := range c.httpRequestMutators {
		if err := mutate(ctx, httpReq, &wire); err != nil {
			return nil, fmt.Errorf("mutate HTTP request: %w", err)
		}
	}
	requestHooks := make([]func(context.Context, protocolcore.RequestMeta[Request]), 0, len(c.requestHooks))
	for _, hook := range c.requestHooks {
		if hook == nil {
			continue
		}
		requestHooks = append(requestHooks, func(ctx context.Context, meta protocolcore.RequestMeta[Request]) { hook(ctx, meta) })
	}
	responseHooks := make([]func(context.Context, protocolcore.ResponseMeta[Request]), 0, len(c.responseHooks))
	for _, hook := range c.responseHooks {
		if hook == nil {
			continue
		}
		responseHooks = append(responseHooks, func(ctx context.Context, meta protocolcore.ResponseMeta[Request]) { hook(ctx, meta) })
	}
	raw, err := protocolcore.ExecuteStream(ctx, protocolcore.ExecuteConfig[Request]{
		HTTPClient:    c.httpClient,
		Logger:        c.logger,
		ErrorParser:   parseOpenAIError,
		RequestHooks:  requestHooks,
		ResponseHooks: responseHooks,
	}, &wire, protocolcore.HTTPRequest{Method: http.MethodPost, URL: httpReq.URL.String(), Headers: httpReq.Header, Body: body})
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

func parseOpenAIError(statusCode int, body []byte) error {
	var resp struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &resp); err != nil || resp.Error.Message == "" {
		return &protocolcore.HTTPError{StatusCode: statusCode, Body: body}
	}
	if resp.Error.Type != "" {
		return fmt.Errorf("%s: %s (HTTP %d)", resp.Error.Type, resp.Error.Message, statusCode)
	}
	return fmt.Errorf("%s (HTTP %d)", resp.Error.Message, statusCode)
}
