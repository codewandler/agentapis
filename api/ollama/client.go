package ollama

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/codewandler/agentapis/internal/protocolcore"
)

const scannerMaxCapacity = 8 * 1024 * 1024

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
	baseURL := cfg.baseURL
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return &Client{
		baseURL:             baseURL,
		path:                cfg.path,
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
	httpReq.GetBody = func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(body)), nil }
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
	for _, hook := range c.requestHooks {
		if hook != nil {
			hook(ctx, RequestMeta{Wire: &wire, HTTP: httpReq, Body: append([]byte(nil), finalBody...)})
		}
	}
	if opts.OnRequest != nil {
		if err := opts.OnRequest(ctx, RequestMeta{Wire: &wire, HTTP: httpReq, Body: append([]byte(nil), finalBody...)}); err != nil {
			return nil, err
		}
	}
	httpClient := c.httpClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	resp, err := httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		errBody, _ := io.ReadAll(resp.Body)
		errorParser := c.errorParser
		if errorParser == nil {
			errorParser = parseOllamaError
		}
		if parseErr := errorParser(resp.StatusCode, errBody); parseErr != nil {
			return nil, &protocolcore.StatusError{StatusCode: resp.StatusCode, Body: errBody, Err: parseErr}
		}
		return nil, &protocolcore.StatusError{StatusCode: resp.StatusCode, Body: errBody}
	}
	for _, hook := range c.responseHooks {
		if hook != nil {
			hook(ctx, ResponseMeta{Wire: &wire, StatusCode: resp.StatusCode, Headers: protocolcore.CloneHeaders(resp.Header)})
		}
	}
	if opts.OnResponse != nil {
		if err := opts.OnResponse(ctx, ResponseMeta{Wire: &wire, StatusCode: resp.StatusCode, Headers: protocolcore.CloneHeaders(resp.Header)}); err != nil {
			_ = resp.Body.Close()
			return nil, err
		}
	}
	parser := c.parser()
	out := make(chan StreamResult, 16)
	go func() {
		defer close(out)
		defer resp.Body.Close()
		scanner := bufio.NewScanner(resp.Body)
		buf := make([]byte, 0, 64*1024)
		scanner.Buffer(buf, scannerMaxCapacity)
		for scanner.Scan() {
			line := bytes.TrimSpace(scanner.Bytes())
			if len(line) == 0 {
				continue
			}
			event, err := parser.Parse(line)
			if err != nil {
				out <- StreamResult{Err: err, RawJSON: append([]byte(nil), line...)}
				continue
			}
			if event == nil {
				continue
			}
			ignored := false
			for _, transform := range c.eventTransforms {
				event, ignored, err = transform(ctx, event)
				if err != nil {
					out <- StreamResult{Err: err, RawJSON: append([]byte(nil), line...)}
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
			out <- StreamResult{Event: event, RawJSON: append([]byte(nil), line...)}
		}
		if err := scanner.Err(); err != nil && ctx.Err() == nil {
			out <- StreamResult{Err: err}
		}
	}()
	return out, nil
}

func (c *Client) ListModels(ctx context.Context) (*TagsResponse, error) {
	path := c.path
	if path == "" || path == DefaultPath {
		path = TagsPath
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, protocolcore.JoinURL(c.baseURL, path), nil)
	if err != nil {
		return nil, fmt.Errorf("build HTTP request: %w", err)
	}
	headers, err := resolveHeaders(ctx, c.headers, c.headerFuncs, nil)
	if err != nil {
		return nil, fmt.Errorf("build request headers: %w", err)
	}
	httpReq.Header = protocolcore.CloneHeaders(headers)
	httpClient := c.httpClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	resp, err := httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		errorParser := c.errorParser
		if errorParser == nil {
			errorParser = parseOllamaError
		}
		if parseErr := errorParser(resp.StatusCode, body); parseErr != nil {
			return nil, &protocolcore.StatusError{StatusCode: resp.StatusCode, Body: body, Err: parseErr}
		}
		return nil, &protocolcore.StatusError{StatusCode: resp.StatusCode, Body: body}
	}
	var out TagsResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("decode tags response: %w", err)
	}
	return &out, nil
}

func parseOllamaError(statusCode int, body []byte) error {
	var resp struct { Error string `json:"error"` }
	if err := json.Unmarshal(body, &resp); err != nil || resp.Error == "" {
		return &protocolcore.HTTPError{StatusCode: statusCode, Body: body}
	}
	return fmt.Errorf("%s (HTTP %d)", resp.Error, statusCode)
}
