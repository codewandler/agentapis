package protocolcore

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestExecuteStreamErrorsOnCompletelyEmptySSEBody(t *testing.T) {
	t.Parallel()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "https://example.com", nil)
	if err != nil {
		t.Fatalf("NewRequestWithContext() error = %v", err)
	}

	stream, err := ExecuteStream(context.Background(), ExecuteConfig[struct{}]{
		HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": {"text/event-stream"}},
				Body:       io.NopCloser(strings.NewReader("")),
			}, nil
		})},
	}, &struct{}{}, req, nil)
	if err != nil {
		t.Fatalf("ExecuteStream() error = %v", err)
	}

	var gotErr error
	for item := range stream {
		if item.Err != nil {
			gotErr = item.Err
			break
		}
	}
	if !errors.Is(gotErr, ErrEmptyEventStream) {
		t.Fatalf("expected ErrEmptyEventStream, got %v", gotErr)
	}
}

func TestExecuteStreamRetriesEmptySSEBody(t *testing.T) {
	t.Parallel()

	attempts := 0
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "https://example.com", strings.NewReader(`{"ok":true}`))
	if err != nil {
		t.Fatalf("NewRequestWithContext() error = %v", err)
	}
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader(`{"ok":true}`)), nil
	}

	stream, err := ExecuteStream(context.Background(), ExecuteConfig[struct{}]{
		HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			attempts++
			if attempts == 1 {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": {"text/event-stream"}},
					Body:       io.NopCloser(strings.NewReader("")),
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": {"text/event-stream"}},
				Body:       io.NopCloser(strings.NewReader("data: hello\n\n")),
			}, nil
		})},
	}, &struct{}{}, req, []byte(`{"ok":true}`))
	if err != nil {
		t.Fatalf("ExecuteStream() error = %v", err)
	}

	var events []RawEvent
	for item := range stream {
		if item.Err != nil {
			t.Fatalf("unexpected stream error: %v", item.Err)
		}
		events = append(events, item)
	}
	if attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts)
	}
	if len(events) != 1 || string(events[0].Data) != "hello" {
		t.Fatalf("unexpected events: %#v", events)
	}
}

func TestExecuteStreamErrorsOnEmptySSEBodyAfterRetries(t *testing.T) {
	t.Parallel()

	attempts := 0
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "https://example.com", strings.NewReader(`{"ok":true}`))
	if err != nil {
		t.Fatalf("NewRequestWithContext() error = %v", err)
	}
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader(`{"ok":true}`)), nil
	}

	stream, err := ExecuteStream(context.Background(), ExecuteConfig[struct{}]{
		HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			attempts++
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": {"text/event-stream"}},
				Body:       io.NopCloser(strings.NewReader("")),
			}, nil
		})},
	}, &struct{}{}, req, []byte(`{"ok":true}`))
	if err != nil {
		t.Fatalf("ExecuteStream() error = %v", err)
	}

	var gotErr error
	for item := range stream {
		if item.Err != nil {
			gotErr = item.Err
			break
		}
	}
	if !errors.Is(gotErr, ErrEmptyEventStream) {
		t.Fatalf("expected ErrEmptyEventStream, got %v", gotErr)
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
}

func TestDefaultHTTPClientUsesDecompressingTransport(t *testing.T) {
	t.Parallel()

	client := DefaultHTTPClient()
	if client == nil {
		t.Fatal("expected client, got nil")
	}
	if client.Transport == nil {
		t.Fatal("expected transport, got nil")
	}
	if _, ok := client.Transport.(*decompressingTransport); !ok {
		t.Fatalf("expected decompressing transport, got %T", client.Transport)
	}
}

func TestNewRetryTransportUsesDefaultHTTPTransport(t *testing.T) {
	t.Parallel()

	transport := NewRetryTransport(nil, RetryConfig{})
	rt, ok := transport.(*retryTransport)
	if !ok {
		t.Fatalf("expected retry transport, got %T", transport)
	}
	if _, ok := rt.base.(*decompressingTransport); !ok {
		t.Fatalf("expected decompressing base transport, got %T", rt.base)
	}
}

func TestDefaultHTTPTransportDecompressesGzip(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write([]byte("data: hello\n\n")); err != nil {
		t.Fatalf("gzip Write() error = %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("gzip Close() error = %v", err)
	}

	transport := &decompressingTransport{wrapped: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Encoding": {"gzip"}, "Content-Type": {"text/event-stream"}},
			Body:       io.NopCloser(bytes.NewReader(buf.Bytes())),
		}, nil
	})}

	req, err := http.NewRequest(http.MethodGet, "https://example.com", nil)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip() error = %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if string(body) != "data: hello\n\n" {
		t.Fatalf("expected decompressed body, got %q", string(body))
	}
}
