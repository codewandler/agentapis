package sse

import (
	"bufio"
	"context"
	"io"
	"strings"
)

func fieldValue(line, prefix string) string {
	v := strings.TrimPrefix(line, prefix)
	return strings.TrimLeft(v, " ")
}

type Event struct {
	Name string
	Data []byte
}

type Scanner struct {
	ctx    context.Context
	cancel context.CancelFunc
	lines  chan scanResult
}

type scanResult struct {
	line string
	err  error
}

func NewScanner(ctx context.Context, r io.Reader) *Scanner {
	ctx, cancel := context.WithCancel(ctx)
	s := &Scanner{
		ctx:    ctx,
		cancel: cancel,
		lines:  make(chan scanResult, 64),
	}
	go s.scan(r)
	return s
}

func (s *Scanner) scan(r io.Reader) {
	defer close(s.lines)
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		select {
		case <-s.ctx.Done():
			return
		case s.lines <- scanResult{line: scanner.Text()}:
		}
	}
	if scanner.Err() != nil {
		select {
		case <-s.ctx.Done():
		case s.lines <- scanResult{err: scanner.Err()}:
		}
	}
}

func (s *Scanner) Next() (Event, bool, error) {
	var name string
	var dataLines []string

	for {
		select {
		case <-s.ctx.Done():
			return Event{}, false, s.ctx.Err()
		case result, ok := <-s.lines:
			if !ok {
				if len(dataLines) > 0 {
					return Event{Name: name, Data: []byte(strings.Join(dataLines, "\n"))}, false, nil
				}
				return Event{}, false, nil
			}
			if result.err != nil {
				return Event{}, false, result.err
			}

			line := result.line

			if line == "" {
				if len(dataLines) > 0 {
					return Event{Name: name, Data: []byte(strings.Join(dataLines, "\n"))}, true, nil
				}
				continue
			}

			if strings.HasPrefix(line, "event:") {
				name = fieldValue(line, "event:")
			} else if strings.HasPrefix(line, "data:") {
				dataLines = append(dataLines, fieldValue(line, "data:"))
			} else if strings.HasPrefix(line, "id:") || strings.HasPrefix(line, "retry:") || strings.HasPrefix(line, ":") {
				// skip
			}
		}
	}
}

func (s *Scanner) Stop() {
	s.cancel()
}
