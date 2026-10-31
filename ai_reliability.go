package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// =========================================================================
// AI transport reliability: pacing, bounded retry with backoff, and an
// optional second provider. All of this runs before a request body is
// consumed, so a retry can never duplicate streamed text.
// =========================================================================

type aiHTTPError struct {
	Code       int
	Message    string
	Retryable  bool
	RetryAfter time.Duration
}

func (e *aiHTTPError) Error() string { return e.Message }

type aiTransientError struct {
	Message string
	Err     error
}

func (e *aiTransientError) Error() string { return e.Message }
func (e *aiTransientError) Unwrap() error { return e.Err }

func isRetryableAIError(err error) bool {
	if err == nil {
		return false
	}
	var httpErr *aiHTTPError
	if errors.As(err, &httpErr) {
		return httpErr.Retryable
	}
	var transient *aiTransientError
	return errors.As(err, &transient)
}

func aiRetryCount(cfg AIConfig) int {
	if cfg.RetryCount < 0 {
		return 0
	}
	if cfg.RetryCount > 3 {
		return 3
	}
	return cfg.RetryCount
}

func aiRetryDelay(attempt int, retryAfter time.Duration) time.Duration {
	if retryAfter > 0 {
		if retryAfter > 10*time.Second {
			return 10 * time.Second
		}
		return retryAfter
	}
	base := time.Duration(400<<uint(attempt-1)) * time.Millisecond
	if base > 4*time.Second {
		base = 4 * time.Second
	}
	return base + time.Duration(rand.Intn(120))*time.Millisecond
}

var (
	aiRateMu   sync.Mutex
	aiRateNext = map[string]time.Time{}
)

// waitAIRateLimit paces calls for one origin. The limiter is deliberately a
// simple next-slot schedule: no burst, no dropped requests.
func waitAIRateLimit(ctx context.Context, origin string, perMinute int) error {
	perMinute = clampAIRateLimit(perMinute)
	if perMinute <= 0 || strings.TrimSpace(origin) == "" {
		return nil
	}
	interval := time.Minute / time.Duration(perMinute)
	if interval <= 0 {
		interval = time.Millisecond
	}
	aiRateMu.Lock()
	now := time.Now()
	slot := aiRateNext[origin]
	if slot.Before(now) {
		slot = now
	}
	aiRateNext[origin] = slot.Add(interval)
	aiRateMu.Unlock()
	if wait := time.Until(slot); wait > 0 {
		timer := time.NewTimer(wait)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
		}
	}
	return nil
}

func clampAIRateLimit(value int) int {
	if value < 0 {
		return 0
	}
	if value > 600 {
		return 600
	}
	return value
}

func retryAfterDuration(header string) time.Duration {
	header = strings.TrimSpace(header)
	if header == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(header); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	if when, err := http.ParseTime(header); err == nil {
		if wait := time.Until(when); wait > 0 {
			return wait
		}
	}
	return 0
}

// aiExecuteAIRequest performs one logical request with retries. The returned
// cancel func must stay alive until the response body is fully consumed.
func aiExecuteAIRequest(ctx context.Context, env aiHTTPEnvelope, timeout time.Duration) (*http.Response, context.CancelFunc, error) {
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	attempts := 1 + aiRetryCount(AIConfig{RetryCount: env.RetryCount})
	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		if attempt > 0 {
			var retryAfter time.Duration
			var httpErr *aiHTTPError
			if errors.As(lastErr, &httpErr) {
				retryAfter = httpErr.RetryAfter
			}
			timer := time.NewTimer(aiRetryDelay(attempt, retryAfter))
			select {
			case <-callCtx.Done():
				timer.Stop()
				cancel()
				return nil, func() {}, aiContextError(ctx, callCtx, timeout)
			case <-timer.C:
			}
		}
		if err := waitAIRateLimit(callCtx, env.Origin, env.RateLimitPerMinute); err != nil {
			cancel()
			return nil, func() {}, aiContextError(ctx, callCtx, timeout)
		}
		req, err := http.NewRequestWithContext(callCtx, http.MethodPost, env.URL, bytes.NewReader(env.Body))
		if err != nil {
			cancel()
			return nil, func() {}, fmt.Errorf("构造 AI 请求失败: %s", redactLogSecrets(err.Error()))
		}
		for header, value := range env.Headers {
			req.Header.Set(header, value)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			if callCtx.Err() != nil {
				cancel()
				return nil, func() {}, aiContextError(ctx, callCtx, timeout)
			}
			lastErr = &aiTransientError{
				Message: fmt.Sprintf("AI 请求失败: %s", redactLogSecrets(err.Error())),
				Err:     err,
			}
			continue
		}
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return resp, cancel, nil
		}
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 300))
		_ = resp.Body.Close()
		message := fmt.Sprintf("AI 端点返回 HTTP %d: %s", resp.StatusCode, redactAIPayload(string(snippet)))
		lastErr = &aiHTTPError{
			Code:       resp.StatusCode,
			Message:    message,
			Retryable:  aiStatusRetryable(resp.StatusCode),
			RetryAfter: retryAfterDuration(resp.Header.Get("Retry-After")),
		}
	}
	cancel()
	if lastErr == nil {
		lastErr = errors.New("AI 请求失败")
	}
	return nil, func() {}, lastErr
}

func aiContextError(parent, callCtx context.Context, timeout time.Duration) error {
	if parent.Err() == context.Canceled {
		return context.Canceled
	}
	if errors.Is(callCtx.Err(), context.DeadlineExceeded) {
		return fmt.Errorf("AI 请求超时（%s）", timeout)
	}
	return callCtx.Err()
}

func aiStatusRetryable(status int) bool {
	switch status {
	case http.StatusRequestTimeout, http.StatusTooManyRequests,
		http.StatusInternalServerError, http.StatusBadGateway,
		http.StatusServiceUnavailable, http.StatusGatewayTimeout, 529:
		return true
	default:
		return false
	}
}

func aiFallbackConfig(cfg AIConfig) (AIConfig, bool) {
	base := strings.TrimRight(strings.TrimSpace(cfg.FallbackBaseURL), "/")
	model := strings.TrimSpace(cfg.FallbackModel)
	if base == "" || model == "" {
		return cfg, false
	}
	fallback := cfg
	fallback.BaseURL = base
	fallback.Model = model
	fallback.Protocol = aiProtocol(AIConfig{Protocol: cfg.FallbackProtocol})
	fallback.FallbackBaseURL = ""
	fallback.FallbackModel = ""
	fallback.FallbackProtocol = ""
	return fallback, true
}

// aiHTTPDoWithFallback posts one non-stream request and transparently retries
// on the configured second provider when the primary keeps failing.
func aiHTTPDoWithFallback(ctx context.Context, cfg AIConfig, prompt string, timeout time.Duration) ([]byte, aiHTTPEnvelope, AIConfig, error) {
	env, err := buildAIHTTPEnvelope(cfg, prompt, false)
	if err != nil {
		return nil, aiHTTPEnvelope{}, cfg, err
	}
	data, err := aiHTTPDo(ctx, env, timeout)
	if err == nil {
		return data, env, cfg, nil
	}
	if !isRetryableAIError(err) {
		return nil, env, cfg, err
	}
	fallback, ok := aiFallbackConfig(cfg)
	if !ok {
		return nil, env, cfg, err
	}
	fallbackEnv, buildErr := buildAIHTTPEnvelope(fallback, prompt, false)
	if buildErr != nil {
		return nil, env, cfg, fmt.Errorf("%s；备用提供商配置无效: %v", err, buildErr)
	}
	data, fallbackErr := aiHTTPDo(ctx, fallbackEnv, timeout)
	if fallbackErr != nil {
		return nil, fallbackEnv, fallback, fmt.Errorf("%s；备用提供商也失败: %v", err, fallbackErr)
	}
	return data, fallbackEnv, fallback, nil
}

// aiHTTPDoStreamWithFallback only switches providers when the primary failed
// before emitting any visible text, so the user never sees a spliced answer.
func aiHTTPDoStreamWithFallback(ctx context.Context, cfg AIConfig, prompt string, timeout time.Duration, onDelta, onReasoning func(string) error) (string, aiUsage, string, error) {
	env, err := buildAIHTTPEnvelope(cfg, prompt, true)
	if err != nil {
		return "", aiUsage{}, cfg.Model, err
	}
	text, usage, emitted, err := aiHTTPDoStreamReasoning(ctx, env, timeout, onDelta, onReasoning)
	if err == nil {
		return text, usage, cfg.Model, nil
	}
	if emitted || !isRetryableAIError(err) {
		return text, usage, cfg.Model, err
	}
	fallback, ok := aiFallbackConfig(cfg)
	if !ok {
		return text, usage, cfg.Model, err
	}
	fallbackEnv, buildErr := buildAIHTTPEnvelope(fallback, prompt, true)
	if buildErr != nil {
		return text, usage, cfg.Model, fmt.Errorf("%s；备用提供商配置无效: %v", err, buildErr)
	}
	text, usage, _, fallbackErr := aiHTTPDoStreamReasoning(ctx, fallbackEnv, timeout, onDelta, onReasoning)
	if fallbackErr != nil {
		return "", usage, fallback.Model, fmt.Errorf("%s；备用提供商也失败: %v", err, fallbackErr)
	}
	return text, usage, fallback.Model, nil
}
