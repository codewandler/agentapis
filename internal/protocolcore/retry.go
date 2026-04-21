package protocolcore

import (
	"io"
	"log/slog"
	"math"
	"math/rand"
	"net/http"
	"strconv"
	"time"
)

var DefaultRetryableStatuses = []int{429, 500, 502, 503, 529}

type RetryConfig struct {
	MaxRetries        int
	RetryableStatuses []int
	InitialBackoff    time.Duration
	MaxBackoff        time.Duration
	Logger            *slog.Logger
}

func (rc RetryConfig) withDefaults() RetryConfig {
	if rc.MaxRetries == 0 {
		rc.MaxRetries = 2
	} else if rc.MaxRetries < 0 {
		rc.MaxRetries = 0
	}
	if len(rc.RetryableStatuses) == 0 {
		rc.RetryableStatuses = DefaultRetryableStatuses
	}
	if rc.InitialBackoff <= 0 {
		rc.InitialBackoff = time.Second
	}
	if rc.MaxBackoff <= 0 {
		rc.MaxBackoff = 60 * time.Second
	}
	if rc.MaxBackoff < rc.InitialBackoff {
		rc.MaxBackoff = rc.InitialBackoff
	}
	return rc
}

type retryTransport struct {
	base http.RoundTripper
	cfg  RetryConfig
}

func (t *retryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var (
		resp *http.Response
		err  error
	)
	for attempt := 0; attempt <= t.cfg.MaxRetries; attempt++ {
		if attempt > 0 {
			backoff := t.backoff(resp, attempt)
			if t.cfg.Logger != nil {
				t.cfg.Logger.InfoContext(req.Context(), "retrying request",
					slog.Int("attempt", attempt),
					slog.Int("status", resp.StatusCode),
					slog.Duration("backoff", backoff),
				)
			}
			timer := time.NewTimer(backoff)
			select {
			case <-timer.C:
			case <-req.Context().Done():
				timer.Stop()
				return resp, req.Context().Err()
			}
			if req.GetBody != nil {
				newBody, bodyErr := req.GetBody()
				if bodyErr != nil {
					return nil, bodyErr
				}
				req.Body = newBody
			}
		}

		resp, err = t.base.RoundTrip(req)
		if err != nil {
			return nil, err
		}
		if !t.isRetryable(resp.StatusCode) || attempt == t.cfg.MaxRetries {
			return resp, nil
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}
	return resp, nil
}

func (t *retryTransport) isRetryable(status int) bool {
	for _, retryable := range t.cfg.RetryableStatuses {
		if retryable == status {
			return true
		}
	}
	return false
}

func (t *retryTransport) backoff(resp *http.Response, attempt int) time.Duration {
	if resp != nil {
		if retryAfter := resp.Header.Get(HeaderRetryAfter); retryAfter != "" {
			if secs, err := strconv.ParseFloat(retryAfter, 64); err == nil && secs > 0 {
				return time.Duration(secs * float64(time.Second))
			}
			if at, err := http.ParseTime(retryAfter); err == nil {
				if delay := time.Until(at); delay > 0 {
					return delay
				}
			}
		}
	}
	exp := math.Pow(2, float64(attempt-1))
	base := time.Duration(exp * float64(t.cfg.InitialBackoff))
	jitter := time.Duration(rand.Int63n(int64(base/2 + 1)))
	delay := base + jitter
	if delay > t.cfg.MaxBackoff {
		return t.cfg.MaxBackoff
	}
	return delay
}

func NewRetryTransport(base http.RoundTripper, cfg RetryConfig) http.RoundTripper {
	if base == nil {
		base = DefaultHTTPTransport()
	}
	return &retryTransport{base: base, cfg: cfg.withDefaults()}
}
