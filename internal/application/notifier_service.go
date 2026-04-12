package application

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/user/github-release-notification-api/internal/domain"
	"github.com/user/github-release-notification-api/internal/domain/port"
	"github.com/user/github-release-notification-api/internal/platform"
)

type NotifierService struct {
	repo       port.SubscriptionRepository
	mailer     port.Mailer
	logger     *slog.Logger
	baseURL    string
	mailRetry  platform.RetryConfig
	emailsSent *prometheus.CounterVec
}

func NewNotifierService(
	repo port.SubscriptionRepository,
	mailer port.Mailer,
	logger *slog.Logger,
	baseURL string,
	mailRetry platform.RetryConfig,
	emailsSent *prometheus.CounterVec,
) *NotifierService {
	return &NotifierService{
		repo:       repo,
		mailer:     mailer,
		logger:     logger,
		baseURL:    baseURL,
		mailRetry:  mailRetry,
		emailsSent: emailsSent,
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
		unsubURL := fmt.Sprintf("%s/api/unsubscribe/%s", n.baseURL, sub.UnsubscribeToken)

		err := platform.DoVoid(ctx, n.mailRetry, isTransientMailError, func(ctx context.Context) error {
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

func isTransientMailError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	for _, s := range []string{"dial", "timeout", "connection", "EOF", "reset", "broken pipe", "temporary"} {
		if strings.Contains(msg, s) {
			return true
		}
	}
	return false
}
