package sse

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func assertEvent(t *testing.T, got Event, wantName, wantData string) {
	t.Helper()
	if got.Name != wantName {
		t.Fatalf("expected event name %q, got %q", wantName, got.Name)
	}
	if string(got.Data) != wantData {
		t.Fatalf("expected event data %q, got %q", wantData, string(got.Data))
	}
}

func TestScannerReturnsBlankLineDelimitedEvent(t *testing.T) {
	scanner := NewScanner(context.Background(), strings.NewReader("event: test\ndata: hello\n\n"))
	defer scanner.Stop()

	event, hasMore, err := scanner.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertEvent(t, event, "test", "hello")
	if !hasMore {
		t.Fatalf("expected hasMore=true for blank-line-delimited event")
	}

	event, hasMore, err = scanner.Next()
	if err != nil {
		t.Fatalf("unexpected error on drain: %v", err)
	}
	assertEvent(t, event, "", "")
	if hasMore {
		t.Fatalf("expected hasMore=false after stream drain")
	}
}

func TestScannerReturnsEOFTerminatedEvent(t *testing.T) {
	scanner := NewScanner(context.Background(), strings.NewReader("data: hello"))
	defer scanner.Stop()

	event, hasMore, err := scanner.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertEvent(t, event, "", "hello")
	if hasMore {
		t.Fatalf("expected hasMore=false for EOF-delimited event")
	}
}

func TestScannerJoinsDataLinesAndIgnoresNonDataFields(t *testing.T) {
	scanner := NewScanner(context.Background(), strings.NewReader("id: 123\nretry: 5000\n: comment\ndata: line1\ndata: line2"))
	defer scanner.Stop()

	event, hasMore, err := scanner.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertEvent(t, event, "", "line1\nline2")
	if hasMore {
		t.Fatalf("expected hasMore=false at EOF")
	}
}

func TestScannerAllowsMissingEventLineWhenTypeIsInData(t *testing.T) {
	payload := `{"type":"response.completed","message":"ok"}`
	scanner := NewScanner(context.Background(), strings.NewReader("data: "+payload+"\n\n"))
	defer scanner.Stop()

	event, hasMore, err := scanner.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertEvent(t, event, "", payload)
	if !hasMore {
		t.Fatalf("expected hasMore=true for blank-line-delimited event")
	}
}

func TestScannerPreservesTrailingWhitespaceInFieldValues(t *testing.T) {
	scanner := NewScanner(context.Background(), strings.NewReader("event:  test  \ndata:  padded  "))
	defer scanner.Stop()

	event, hasMore, err := scanner.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertEvent(t, event, "test  ", "padded  ")
	if hasMore {
		t.Fatalf("expected hasMore=false at EOF")
	}
}

func TestScannerTrimsAllLeadingSpacesAfterDataPrefix(t *testing.T) {
	scanner := NewScanner(context.Background(), strings.NewReader("data:   FOO"))
	defer scanner.Stop()

	event, hasMore, err := scanner.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertEvent(t, event, "", "FOO")
	if hasMore {
		t.Fatalf("expected hasMore=false at EOF")
	}
}

func TestScannerReturnsContextErrorWhenCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	scanner := &Scanner{ctx: ctx, lines: make(chan scanResult)}
	event, hasMore, err := scanner.Next()
	assertEvent(t, event, "", "")
	if hasMore {
		t.Fatalf("expected hasMore=false when context is cancelled")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestScannerReturnsReadErrors(t *testing.T) {
	wantErr := errors.New("boom")
	scanner := NewScanner(context.Background(), errorReader{err: wantErr})
	defer scanner.Stop()

	event, hasMore, err := scanner.Next()
	assertEvent(t, event, "", "")
	if hasMore {
		t.Fatalf("expected hasMore=false when reader fails")
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected %v, got %v", wantErr, err)
	}
}

type errorReader struct {
	err error
}

func (r errorReader) Read(_ []byte) (int, error) {
	return 0, r.err
}
