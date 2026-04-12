package github

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestClient_RetryOn500(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		if n < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	client := NewClient("", srv.URL, 5*time.Second, 3, testLogger())
	exists, err := client.RepoExists(context.Background(), "test", "repo")
	if err != nil {
		t.Fatalf("expected success after retries, got: %v", err)
	}
	if !exists {
		t.Error("expected repo to exist")
	}
	if calls.Load() != 3 {
		t.Errorf("expected 3 calls, got %d", calls.Load())
	}
}

func TestClient_RetryOn429(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		if n < 2 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	client := NewClient("", srv.URL, 5*time.Second, 3, testLogger())
	exists, err := client.RepoExists(context.Background(), "test", "repo")
	if err != nil {
		t.Fatalf("expected success after rate limit retry, got: %v", err)
	}
	if !exists {
		t.Error("expected repo to exist")
	}
	if calls.Load() != 2 {
		t.Errorf("expected 2 calls, got %d", calls.Load())
	}
}

func TestClient_NoRetryOn404(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := NewClient("", srv.URL, 5*time.Second, 3, testLogger())
	exists, err := client.RepoExists(context.Background(), "test", "repo")
	if err != nil {
		t.Fatalf("expected no error for 404, got: %v", err)
	}
	if exists {
		t.Error("expected repo not to exist")
	}
	if calls.Load() != 1 {
		t.Errorf("404 should not be retried, expected 1 call, got %d", calls.Load())
	}
}

func TestClient_RespectsContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	client := NewClient("", srv.URL, 5*time.Second, 10, testLogger())
	_, err := client.RepoExists(ctx, "test", "repo")
	if err == nil {
		t.Fatal("expected error due to context cancellation")
	}
	if !isContextError(err) {
		t.Fatalf("expected context error, got: %v", err)
	}
}

func TestClient_GetLatestRelease_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"tag_name":"v1.0.0","name":"Release 1.0","html_url":"https://example.com","published_at":"2024-01-01T00:00:00Z"}`))
	}))
	defer srv.Close()

	client := NewClient("", srv.URL, 5*time.Second, 3, testLogger())
	release, err := client.GetLatestRelease(context.Background(), "test", "repo")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if release == nil {
		t.Fatal("expected release, got nil")
	}
	if release.TagName != "v1.0.0" {
		t.Errorf("expected tag v1.0.0, got %s", release.TagName)
	}
}

func TestClient_GetLatestRelease_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := NewClient("", srv.URL, 5*time.Second, 3, testLogger())
	release, err := client.GetLatestRelease(context.Background(), "test", "repo")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if release != nil {
		t.Fatal("expected nil release for 404")
	}
}

func isContextError(err error) bool {
	return err == context.Canceled || err == context.DeadlineExceeded ||
		context.Cause(context.Background()) != nil
}
