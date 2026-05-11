package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/user/github-release-notification-api/internal/domain"
	"github.com/user/github-release-notification-api/internal/platform"
)

func newTestNotifierService(repo *mockRepo, mailer *mockMailer) *NotifierService {
	return NewNotifierService(
		repo, mailer,
		NewURLBuilder(testBaseURL),
		platform.DefaultMailRetryConfig(),
		platform.IsTransientMailError,
		testLogger(),
		nil,
	)
}

func TestScannerService_NewRelease_NotifiesSubscribers(t *testing.T) {
	repo := newMockRepo()
	gh := newMockGitHub()
	mailer := newMockMailer()

	gh.existingRepos["golang/go"] = true
	gh.releases["golang/go"] = &domain.Release{
		TagName: "v1.22.0",
		Name:    "Go 1.22",
		HTMLURL: "https://github.com/golang/go/releases/tag/v1.22.0",
	}

	svc := newTestSubscriptionService(repo, gh, mailer)
	_ = svc.Subscribe(context.Background(), "test@example.com", "golang/go")

	for _, s := range repo.subs {
		s.Confirmed = true
	}

	for _, r := range repo.repos {
		r.LastSeenTag = "v1.21.0"
	}

	notifier := newTestNotifierService(repo, mailer)
	scanner := NewScannerService(repo, gh, notifier, testLogger(), ScannerConfig{
		Interval: time.Minute, CycleTimeout: time.Minute, RepoTimeout: 30 * time.Second,
	}, nil)

	scanner.scan(context.Background())

	if len(mailer.notificationsSent) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(mailer.notificationsSent))
	}

	notif := mailer.notificationsSent[0]
	if notif.Tag != "v1.22.0" {
		t.Errorf("expected tag v1.22.0, got %s", notif.Tag)
	}
	if notif.Email != "test@example.com" {
		t.Errorf("expected email test@example.com, got %s", notif.Email)
	}
}

func TestScannerService_SameTag_NoNotification(t *testing.T) {
	repo := newMockRepo()
	gh := newMockGitHub()
	mailer := newMockMailer()

	gh.existingRepos["golang/go"] = true
	gh.releases["golang/go"] = &domain.Release{
		TagName: "v1.21.0",
		Name:    "Go 1.21",
		HTMLURL: "https://github.com/golang/go/releases/tag/v1.21.0",
	}

	svc := newTestSubscriptionService(repo, gh, mailer)
	_ = svc.Subscribe(context.Background(), "test@example.com", "golang/go")

	for _, s := range repo.subs {
		s.Confirmed = true
	}
	for _, r := range repo.repos {
		r.LastSeenTag = "v1.21.0"
	}

	notifier := newTestNotifierService(repo, mailer)
	scanner := NewScannerService(repo, gh, notifier, testLogger(), ScannerConfig{
		Interval: time.Minute, CycleTimeout: time.Minute, RepoTimeout: 30 * time.Second,
	}, nil)

	scanner.scan(context.Background())

	if len(mailer.notificationsSent) != 0 {
		t.Fatalf("expected 0 notifications, got %d", len(mailer.notificationsSent))
	}
}

func TestScannerService_FirstRelease_SetsBaseline(t *testing.T) {
	repo := newMockRepo()
	gh := newMockGitHub()
	mailer := newMockMailer()

	gh.existingRepos["golang/go"] = true
	gh.releases["golang/go"] = &domain.Release{
		TagName: "v1.22.0",
		Name:    "Go 1.22",
		HTMLURL: "https://github.com/golang/go/releases/tag/v1.22.0",
	}

	svc := newTestSubscriptionService(repo, gh, mailer)
	_ = svc.Subscribe(context.Background(), "test@example.com", "golang/go")

	for _, s := range repo.subs {
		s.Confirmed = true
	}

	notifier := newTestNotifierService(repo, mailer)
	scanner := NewScannerService(repo, gh, notifier, testLogger(), ScannerConfig{
		Interval: time.Minute, CycleTimeout: time.Minute, RepoTimeout: 30 * time.Second,
	}, nil)

	scanner.scan(context.Background())

	if len(mailer.notificationsSent) != 0 {
		t.Fatalf("expected 0 notifications for baseline, got %d", len(mailer.notificationsSent))
	}

	for _, r := range repo.repos {
		if r.LastSeenTag != "v1.22.0" {
			t.Errorf("expected last_seen_tag v1.22.0, got %s", r.LastSeenTag)
		}
	}
}

func TestNotifierService_SendsToAllSubscribers(t *testing.T) {
	repo := newMockRepo()
	gh := newMockGitHub()
	mailer := newMockMailer()

	gh.existingRepos["golang/go"] = true

	svc := newTestSubscriptionService(repo, gh, mailer)

	_ = svc.Subscribe(context.Background(), "user1@example.com", "golang/go")
	_ = svc.Subscribe(context.Background(), "user2@example.com", "golang/go")

	for _, s := range repo.subs {
		s.Confirmed = true
	}

	notifier := newTestNotifierService(repo, mailer)

	var targetRepo *domain.Repository
	for _, r := range repo.repos {
		targetRepo = r
		break
	}

	release := &domain.Release{
		TagName: "v1.22.0",
		HTMLURL: "https://github.com/golang/go/releases/tag/v1.22.0",
	}

	err := notifier.NotifySubscribers(context.Background(), targetRepo, release)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(mailer.notificationsSent) != 2 {
		t.Fatalf("expected 2 notifications, got %d", len(mailer.notificationsSent))
	}
}

func TestScannerService_GitHubError_ContinuesOtherRepos(t *testing.T) {
	repo := newMockRepo()
	gh := newMockGitHub()
	mailer := newMockMailer()

	gh.existingRepos["golang/go"] = true
	gh.releaseErr = errors.New("github api error")

	svc := newTestSubscriptionService(repo, gh, mailer)
	_ = svc.Subscribe(context.Background(), "test@example.com", "golang/go")

	for _, s := range repo.subs {
		s.Confirmed = true
	}
	for _, r := range repo.repos {
		r.LastSeenTag = "v1.20.0"
	}

	notifier := newTestNotifierService(repo, mailer)
	scanner := NewScannerService(repo, gh, notifier, testLogger(), ScannerConfig{
		Interval: time.Minute, CycleTimeout: time.Minute, RepoTimeout: 30 * time.Second,
	}, nil)

	scanner.scan(context.Background())

	if len(mailer.notificationsSent) != 0 {
		t.Fatalf("expected 0 notifications on github error, got %d", len(mailer.notificationsSent))
	}

	for _, r := range repo.repos {
		if r.LastSeenTag != "v1.20.0" {
			t.Errorf("last_seen_tag should not change on error, got %s", r.LastSeenTag)
		}
	}
}

func TestScannerService_NilRelease_NoAction(t *testing.T) {
	repo := newMockRepo()
	gh := newMockGitHub()
	mailer := newMockMailer()

	gh.existingRepos["golang/go"] = true

	svc := newTestSubscriptionService(repo, gh, mailer)
	_ = svc.Subscribe(context.Background(), "test@example.com", "golang/go")

	for _, s := range repo.subs {
		s.Confirmed = true
	}
	for _, r := range repo.repos {
		r.LastSeenTag = "v1.20.0"
	}

	notifier := newTestNotifierService(repo, mailer)
	scanner := NewScannerService(repo, gh, notifier, testLogger(), ScannerConfig{
		Interval: time.Minute, CycleTimeout: time.Minute, RepoTimeout: 30 * time.Second,
	}, nil)

	scanner.scan(context.Background())

	if len(mailer.notificationsSent) != 0 {
		t.Fatalf("expected 0 notifications for nil release, got %d", len(mailer.notificationsSent))
	}
}
