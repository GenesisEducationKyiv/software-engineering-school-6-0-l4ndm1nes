package platform

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestDo_SuccessOnFirstAttempt(t *testing.T) {
	cfg := RetryConfig{MaxAttempts: 3, InitialWait: time.Millisecond, MaxWait: time.Second, Multiplier: 2}
	calls := 0
	result, err := Do(context.Background(), cfg, func(_ error) bool { return true }, func(_ context.Context) (string, error) {
		calls++
		return "ok", nil
	})
	if err != nil || result != "ok" || calls != 1 {
		t.Fatalf("expected success on first call, got err=%v result=%s calls=%d", err, result, calls)
	}
}

func TestDo_RetriesOnTransientError(t *testing.T) {
	cfg := RetryConfig{MaxAttempts: 3, InitialWait: time.Millisecond, MaxWait: 10 * time.Millisecond, Multiplier: 2}
	calls := 0
	transient := errors.New("transient")
	result, err := Do(context.Background(), cfg, func(e error) bool { return errors.Is(e, transient) }, func(_ context.Context) (string, error) {
		calls++
		if calls < 3 {
			return "", transient
		}
		return "recovered", nil
	})
	if err != nil || result != "recovered" || calls != 3 {
		t.Fatalf("expected recovery on 3rd call, got err=%v result=%s calls=%d", err, result, calls)
	}
}

func TestDo_NoRetryOnNonRetryableError(t *testing.T) {
	cfg := RetryConfig{MaxAttempts: 3, InitialWait: time.Millisecond, MaxWait: time.Second, Multiplier: 2}
	calls := 0
	permanent := errors.New("permanent")
	_, err := Do(context.Background(), cfg, func(_ error) bool { return false }, func(_ context.Context) (string, error) {
		calls++
		return "", permanent
	})
	if !errors.Is(err, permanent) || calls != 1 {
		t.Fatalf("expected immediate failure, got err=%v calls=%d", err, calls)
	}
}

func TestDo_RespectsContextCancellation(t *testing.T) {
	cfg := RetryConfig{MaxAttempts: 5, InitialWait: 100 * time.Millisecond, MaxWait: time.Second, Multiplier: 2}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := Do(ctx, cfg, func(_ error) bool { return true }, func(_ context.Context) (string, error) {
		return "", errors.New("fail")
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected DeadlineExceeded, got %v", err)
	}
}

func TestDo_ExhaustsAllAttempts(t *testing.T) {
	cfg := RetryConfig{MaxAttempts: 3, InitialWait: time.Millisecond, MaxWait: 10 * time.Millisecond, Multiplier: 2}
	calls := 0
	persistent := errors.New("always fails")
	_, err := Do(context.Background(), cfg, func(_ error) bool { return true }, func(_ context.Context) (string, error) {
		calls++
		return "", persistent
	})
	if !errors.Is(err, persistent) || calls != 3 {
		t.Fatalf("expected 3 attempts, got err=%v calls=%d", err, calls)
	}
}
