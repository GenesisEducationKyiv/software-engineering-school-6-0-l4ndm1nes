package application

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"

	"github.com/user/github-release-notification-api/internal/domain"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestSubscribe_Success(t *testing.T) {
	repo := newMockRepo()
	gh := newMockGitHub()
	gh.existingRepos["golang/go"] = true
	mailer := newMockMailer()

	svc := NewSubscriptionService(repo, gh, mailer, testLogger(), "http://localhost:8080")

	err := svc.Subscribe(context.Background(), "test@example.com", "golang/go")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(mailer.confirmationsSent) != 1 {
		t.Fatalf("expected 1 confirmation email, got %d", len(mailer.confirmationsSent))
	}
	if mailer.confirmationsSent[0].Email != "test@example.com" {
		t.Errorf("expected email test@example.com, got %s", mailer.confirmationsSent[0].Email)
	}
}

func TestSubscribe_InvalidEmail(t *testing.T) {
	svc := NewSubscriptionService(newMockRepo(), newMockGitHub(), newMockMailer(), testLogger(), "http://localhost:8080")

	err := svc.Subscribe(context.Background(), "not-an-email", "golang/go")
	if !errors.Is(err, domain.ErrInvalidEmail) {
		t.Fatalf("expected ErrInvalidEmail, got: %v", err)
	}
}

func TestSubscribe_InvalidRepoFormat(t *testing.T) {
	svc := NewSubscriptionService(newMockRepo(), newMockGitHub(), newMockMailer(), testLogger(), "http://localhost:8080")

	tests := []string{"", "noslash", "too/many/slashes", "/empty", "empty/"}
	for _, repo := range tests {
		err := svc.Subscribe(context.Background(), "test@example.com", repo)
		if !errors.Is(err, domain.ErrInvalidRepoFormat) {
			t.Errorf("repo=%q: expected ErrInvalidRepoFormat, got: %v", repo, err)
		}
	}
}

func TestSubscribe_RepoNotFound(t *testing.T) {
	gh := newMockGitHub()
	svc := NewSubscriptionService(newMockRepo(), gh, newMockMailer(), testLogger(), "http://localhost:8080")

	err := svc.Subscribe(context.Background(), "test@example.com", "nonexistent/repo")
	if !errors.Is(err, domain.ErrRepoNotFound) {
		t.Fatalf("expected ErrRepoNotFound, got: %v", err)
	}
}

func TestSubscribe_AlreadySubscribed(t *testing.T) {
	repo := newMockRepo()
	gh := newMockGitHub()
	gh.existingRepos["golang/go"] = true
	mailer := newMockMailer()

	svc := NewSubscriptionService(repo, gh, mailer, testLogger(), "http://localhost:8080")

	_ = svc.Subscribe(context.Background(), "test@example.com", "golang/go")
	err := svc.Subscribe(context.Background(), "test@example.com", "golang/go")
	if !errors.Is(err, domain.ErrAlreadySubscribed) {
		t.Fatalf("expected ErrAlreadySubscribed, got: %v", err)
	}
}

func TestConfirm_Success(t *testing.T) {
	repo := newMockRepo()
	gh := newMockGitHub()
	gh.existingRepos["golang/go"] = true
	mailer := newMockMailer()

	svc := NewSubscriptionService(repo, gh, mailer, testLogger(), "http://localhost:8080")

	_ = svc.Subscribe(context.Background(), "test@example.com", "golang/go")

	// Get the confirm token from the mock repo
	var token string
	for _, s := range repo.subs {
		token = s.ConfirmToken
		break
	}

	err := svc.Confirm(context.Background(), token)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify subscription is confirmed
	for _, s := range repo.subs {
		if !s.Confirmed {
			t.Error("expected subscription to be confirmed")
		}
	}
}

func TestConfirm_TokenNotFound(t *testing.T) {
	svc := NewSubscriptionService(newMockRepo(), newMockGitHub(), newMockMailer(), testLogger(), "http://localhost:8080")

	err := svc.Confirm(context.Background(), "nonexistent-token")
	if !errors.Is(err, domain.ErrTokenNotFound) {
		t.Fatalf("expected ErrTokenNotFound, got: %v", err)
	}
}

func TestConfirm_EmptyToken(t *testing.T) {
	svc := NewSubscriptionService(newMockRepo(), newMockGitHub(), newMockMailer(), testLogger(), "http://localhost:8080")

	err := svc.Confirm(context.Background(), "")
	if !errors.Is(err, domain.ErrInvalidToken) {
		t.Fatalf("expected ErrInvalidToken, got: %v", err)
	}
}

func TestUnsubscribe_Success(t *testing.T) {
	repo := newMockRepo()
	gh := newMockGitHub()
	gh.existingRepos["golang/go"] = true

	svc := NewSubscriptionService(repo, gh, newMockMailer(), testLogger(), "http://localhost:8080")

	_ = svc.Subscribe(context.Background(), "test@example.com", "golang/go")

	var token string
	for _, s := range repo.subs {
		token = s.UnsubscribeToken
		break
	}

	err := svc.Unsubscribe(context.Background(), token)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(repo.subs) != 0 {
		t.Error("expected subscription to be deleted")
	}
}

func TestUnsubscribe_TokenNotFound(t *testing.T) {
	svc := NewSubscriptionService(newMockRepo(), newMockGitHub(), newMockMailer(), testLogger(), "http://localhost:8080")

	err := svc.Unsubscribe(context.Background(), "nonexistent-token")
	if !errors.Is(err, domain.ErrTokenNotFound) {
		t.Fatalf("expected ErrTokenNotFound, got: %v", err)
	}
}

func TestGetSubscriptions_Success(t *testing.T) {
	repo := newMockRepo()
	gh := newMockGitHub()
	gh.existingRepos["golang/go"] = true
	gh.existingRepos["gin-gonic/gin"] = true

	svc := NewSubscriptionService(repo, gh, newMockMailer(), testLogger(), "http://localhost:8080")

	_ = svc.Subscribe(context.Background(), "test@example.com", "golang/go")
	_ = svc.Subscribe(context.Background(), "test@example.com", "gin-gonic/gin")

	subs, err := svc.GetSubscriptions(context.Background(), "test@example.com")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(subs) != 2 {
		t.Fatalf("expected 2 subscriptions, got %d", len(subs))
	}
}

func TestGetSubscriptions_InvalidEmail(t *testing.T) {
	svc := NewSubscriptionService(newMockRepo(), newMockGitHub(), newMockMailer(), testLogger(), "http://localhost:8080")

	_, err := svc.GetSubscriptions(context.Background(), "invalid")
	if !errors.Is(err, domain.ErrInvalidEmail) {
		t.Fatalf("expected ErrInvalidEmail, got: %v", err)
	}
}

func TestSubscribe_GitHubClientError(t *testing.T) {
	gh := newMockGitHub()
	gh.repoExistsErr = errors.New("github api down")

	svc := NewSubscriptionService(newMockRepo(), gh, newMockMailer(), testLogger(), "http://localhost:8080")

	err := svc.Subscribe(context.Background(), "test@example.com", "golang/go")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if errors.Is(err, domain.ErrRepoNotFound) {
		t.Fatal("should be a wrapped error, not ErrRepoNotFound")
	}
}

func TestSubscribe_CreateRepoError(t *testing.T) {
	repo := newMockRepo()
	repo.createRepoErr = errors.New("db connection lost")
	gh := newMockGitHub()
	gh.existingRepos["golang/go"] = true

	svc := NewSubscriptionService(repo, gh, newMockMailer(), testLogger(), "http://localhost:8080")

	err := svc.Subscribe(context.Background(), "test@example.com", "golang/go")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestSubscribe_CreateSubscriptionError(t *testing.T) {
	repo := newMockRepo()
	repo.createSubErr = errors.New("unique constraint violation")
	gh := newMockGitHub()
	gh.existingRepos["golang/go"] = true

	svc := NewSubscriptionService(repo, gh, newMockMailer(), testLogger(), "http://localhost:8080")

	err := svc.Subscribe(context.Background(), "test@example.com", "golang/go")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestSubscribe_EmailFailure_RollsBackSubscription(t *testing.T) {
	repo := newMockRepo()
	gh := newMockGitHub()
	gh.existingRepos["golang/go"] = true
	mailer := newMockMailer()
	mailer.sendConfirmErr = errors.New("smtp timeout")

	svc := NewSubscriptionService(repo, gh, mailer, testLogger(), "http://localhost:8080")

	err := svc.Subscribe(context.Background(), "test@example.com", "golang/go")
	if err == nil {
		t.Fatal("expected error when email fails")
	}

	if len(repo.subs) != 0 {
		t.Error("subscription should be rolled back after email failure")
	}
}
