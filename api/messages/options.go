package messages

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/codewandler/agentapis/internal/protocolcore"
)

type HeaderFunc func(ctx context.Context, req *Request) (http.Header, error)
type RequestTransform func(ctx context.Context, req *Request) error
type HTTPRequestMutator func(ctx context.Context, httpReq *http.Request, req *Request) error
type EventTransform func(ctx context.Context, ev StreamEvent) (StreamEvent, bool, error)
type RequestHook func(ctx context.Context, meta RequestMeta)
type ResponseHook func(ctx context.Context, meta ResponseMeta)
type ErrorParser = protocolcore.ErrorParser
type Option func(*config)

type config struct {
	apiKey              string
	baseURL             string
	path                string
	errorParser         ErrorParser
	httpClient          *http.Client
	headers             http.Header
	headerFuncs         []HeaderFunc
	requestTransforms   []RequestTransform
	httpRequestMutators []HTTPRequestMutator
	eventTransforms     []EventTransform
	requestHooks        []RequestHook
	responseHooks       []ResponseHook
	logger              *slog.Logger
}

func defaultConfig() config {
	return config{httpClient: protocolcore.DefaultHTTPClient(), headers: make(http.Header)}
}

func applyOptions(opts []Option) config {
	cfg := defaultConfig()
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	return cfg
}

func WithAPIKey(key string) Option {
	return func(c *config) { c.apiKey = key }
}

func WithBaseURL(url string) Option {
	return func(c *config) { c.baseURL = url }
}

func WithHTTPClient(client *http.Client) Option {
	return func(c *config) {
		if client != nil {
			c.httpClient = client
		}
	}
}

func WithPath(path string) Option {
	return func(c *config) { c.path = path }
}

func WithErrorParser(fn ErrorParser) Option {
	return func(c *config) { c.errorParser = fn }
}

func WithHeader(key, value string) Option {
	return func(c *config) { c.headers.Add(key, value) }
}

func WithHeaderFunc(fn HeaderFunc) Option {
	return func(c *config) {
		if fn != nil {
			c.headerFuncs = append(c.headerFuncs, fn)
		}
	}
}

func WithRequestTransform(fn RequestTransform) Option {
	return func(c *config) {
		if fn != nil {
			c.requestTransforms = append(c.requestTransforms, fn)
		}
	}
}

func WithHTTPRequestMutator(fn HTTPRequestMutator) Option {
	return func(c *config) {
		if fn != nil {
			c.httpRequestMutators = append(c.httpRequestMutators, fn)
		}
	}
}

func WithEventTransform(fn EventTransform) Option {
	return func(c *config) {
		if fn != nil {
			c.eventTransforms = append(c.eventTransforms, fn)
		}
	}
}

func WithRequestHook(fn RequestHook) Option {
	return func(c *config) {
		if fn != nil {
			c.requestHooks = append(c.requestHooks, fn)
		}
	}
}

func WithResponseHook(fn ResponseHook) Option {
	return func(c *config) {
		if fn != nil {
			c.responseHooks = append(c.responseHooks, fn)
		}
	}
}

func WithLogger(logger *slog.Logger) Option {
	return func(c *config) { c.logger = logger }
}

func resolveHeaders(ctx context.Context, static http.Header, funcs []HeaderFunc, req *Request) (http.Header, error) {
	out := protocolcore.CloneHeaders(static)
	for _, fn := range funcs {
		dynamic, err := fn(ctx, req)
		if err != nil {
			return nil, err
		}
		for key, values := range dynamic {
			out.Del(key)
			for _, value := range values {
				out.Add(key, value)
			}
		}
	}
	return out, nil
}

func appendBuiltInHeaderFunc(apiKey string, funcs []HeaderFunc) []HeaderFunc {
	if apiKey == "" {
		return append([]HeaderFunc(nil), funcs...)
	}
	builtIn := func(_ context.Context, _ *Request) (http.Header, error) {
		return http.Header{HeaderAPIKey: {apiKey}}, nil
	}
	out := make([]HeaderFunc, 0, len(funcs)+1)
	out = append(out, builtIn)
	out = append(out, funcs...)
	return out
}
