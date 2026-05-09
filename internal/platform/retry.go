package platform

import (
	"context"
	"errors"
	"math"
	"math/rand/v2"
	"time"
)

type RetryConfig struct {
	MaxAttempts int
	InitialWait time.Duration
	MaxWait     time.Duration
	Multiplier  float64
}

func DefaultMailRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts: 3,
		InitialWait: 500 * time.Millisecond,
		MaxWait:     5 * time.Second,
		Multiplier:  2.0,
	}
}

// IsRetryable is a function that decides whether an error should be retried.
type IsRetryable func(err error) bool

// Do executes fn with exponential backoff + jitter.
// It retries only if isRetryable returns true for the error.
// Returns the result of the last attempt.
func Do[T any](ctx context.Context, cfg RetryConfig, isRetryable IsRetryable, fn func(ctx context.Context) (T, error)) (T, error) {
	var lastErr error
	var zero T

	for attempt := range cfg.MaxAttempts {
		result, err := fn(ctx)
		if err == nil {
			return result, nil
		}
		lastErr = err

		if !isRetryable(err) {
			return zero, err
		}

		if attempt == cfg.MaxAttempts-1 {
			break
		}

		wait := calcBackoff(cfg.InitialWait, cfg.MaxWait, cfg.Multiplier, attempt)

		select {
		case <-ctx.Done():
			return zero, errors.Join(lastErr, ctx.Err())
		case <-time.After(wait):
		}
	}

	return zero, lastErr
}

// DoVoid is Do without a return value.
func DoVoid(ctx context.Context, cfg RetryConfig, isRetryable IsRetryable, fn func(ctx context.Context) error) error {
	_, err := Do(ctx, cfg, isRetryable, func(ctx context.Context) (struct{}, error) {
		return struct{}{}, fn(ctx)
	})
	return err
}

func calcBackoff(initial, maxWait time.Duration, multiplier float64, attempt int) time.Duration {
	wait := time.Duration(float64(initial) * math.Pow(multiplier, float64(attempt)))
	if wait > maxWait {
		wait = maxWait
	}
	jitter := time.Duration(float64(wait) * (0.75 + rand.Float64()*0.5)) //nolint:gosec // G404: jitter only, not crypto
	return jitter
}
