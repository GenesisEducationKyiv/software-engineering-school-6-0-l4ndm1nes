package application

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/user/github-release-notification-api/internal/domain"
	"github.com/user/github-release-notification-api/internal/domain/port"
	"github.com/user/github-release-notification-api/internal/platform"
)

type notifierDeps interface {
	port.SubscriptionReader
}

type NotifierService struct {
	repo        notifierDeps
	mailer      port.Mailer
	urls        URLBuilder
	mailRetry   platform.RetryConfig
	isRetryable platform.IsRetryable
	logger      *slog.Logger
	emailsSent  *prometheus.CounterVec
}

func NewNotifierService(
	repo notifierDeps,
	mailer port.Mailer,
	urls URLBuilder,
	mailRetry platform.RetryConfig,
	isRetryable platform.IsRetryable,
	logger *slog.Logger,
	emailsSent *prometheus.CounterVec,
) *NotifierService {
	if isRetryable == nil {
		isRetryable = platform.IsTransientMailError
	}
	return &NotifierService{
		repo:        repo,
		mailer:      mailer,
		urls:        urls,
		mailRetry:   mailRetry,
		isRetryable: isRetryable,
		logger:      logger,
		emailsSent:  emailsSent,
	}
}

func (n *NotifierService) NotifySubscribers(ctx context.Context, repository *domain.Repository, release *domain.Release) error {
	subs, err := n.repo.GetConfirmedSubscriptionsByRepoID(ctx, repository.ID)
	if err != nil {
		return fmt.Errorf("getting subscribers: %w", err)
	}

	repoFullName := repository.FullName()
	var sendErrors int

	for i := range subs {
		sub := &subs[i]
		unsubURL := n.urls.Unsubscribe(sub.UnsubscribeToken)

		err := platform.DoVoid(ctx, n.mailRetry, n.isRetryable, func(ctx context.Context) error {
			return n.mailer.SendReleaseNotification(
				ctx, sub.Email, repoFullName, release.TagName, release.HTMLURL, unsubURL,
			)
		})
		if err != nil {
			n.logger.Error("failed to send release notification after retries",
				"email", sub.Email,
				"repo", repoFullName,
				"error", err,
			)
			if n.emailsSent != nil {
				n.emailsSent.WithLabelValues("release_notification", "error").Inc()
			}
			sendErrors++
			continue
		}
		if n.emailsSent != nil {
			n.emailsSent.WithLabelValues("release_notification", "success").Inc()
		}
	}

	if sendErrors > 0 {
		n.logger.Warn("some notifications failed",
			"repo", repoFullName,
			"total", len(subs),
			"failed", sendErrors,
		)
	}

	return nil
}
