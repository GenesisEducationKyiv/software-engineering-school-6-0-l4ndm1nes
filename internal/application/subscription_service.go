package application

import (
	"context"
	"fmt"
	"log/slog"
	"net/mail"
	"strings"

	"github.com/user/github-release-notification-api/internal/domain"
	"github.com/user/github-release-notification-api/internal/domain/port"
)

type subscriptionDeps interface {
	port.RepositoryWriter
	port.SubscriptionReader
	port.SubscriptionWriter
}

type SubscriptionService struct {
	repo   subscriptionDeps
	github port.GitHubClient
	mailer port.Mailer
	tokens TokenGenerator
	urls   URLBuilder
	logger *slog.Logger
}

func NewSubscriptionService(
	repo subscriptionDeps,
	github port.GitHubClient,
	mailer port.Mailer,
	tokens TokenGenerator,
	urls URLBuilder,
	logger *slog.Logger,
) *SubscriptionService {
	return &SubscriptionService{
		repo:   repo,
		github: github,
		mailer: mailer,
		tokens: tokens,
		urls:   urls,
		logger: logger,
	}
}

func (s *SubscriptionService) Subscribe(ctx context.Context, email, repoFullName string) error {
	if err := validateEmail(email); err != nil {
		return err
	}

	owner, repo, err := parseRepoFullName(repoFullName)
	if err != nil {
		return err
	}

	exists, err := s.github.RepoExists(ctx, owner, repo)
	if err != nil {
		return fmt.Errorf("checking repository: %w", err)
	}
	if !exists {
		return domain.ErrRepoNotFound
	}

	dbRepo, err := s.repo.CreateRepository(ctx, owner, repo)
	if err != nil {
		return fmt.Errorf("creating repository record: %w", err)
	}

	existing, err := s.repo.GetSubscriptionByEmailAndRepo(ctx, email, dbRepo.ID)
	if err != nil {
		return fmt.Errorf("checking existing subscription: %w", err)
	}
	if existing != nil {
		return domain.ErrAlreadySubscribed
	}

	confirmToken, err := s.tokens.Generate()
	if err != nil {
		return fmt.Errorf("generating confirm token: %w", err)
	}
	unsubToken, err := s.tokens.Generate()
	if err != nil {
		return fmt.Errorf("generating unsubscribe token: %w", err)
	}

	sub := &domain.Subscription{
		Email:            email,
		RepositoryID:     dbRepo.ID,
		Confirmed:        false,
		ConfirmToken:     confirmToken,
		UnsubscribeToken: unsubToken,
	}

	if err := s.repo.CreateSubscription(ctx, sub); err != nil {
		return fmt.Errorf("creating subscription: %w", err)
	}

	if err := s.mailer.SendConfirmation(ctx, email, repoFullName, s.urls.Confirm(confirmToken)); err != nil {
		s.logger.Error("failed to send confirmation email", "email", email, "error", err)
		if delErr := s.repo.DeleteSubscription(ctx, sub.ID); delErr != nil {
			s.logger.Error("failed to rollback subscription", "id", sub.ID, "error", delErr)
		}
		return fmt.Errorf("sending confirmation email: %w", err)
	}

	return nil
}

func (s *SubscriptionService) Confirm(ctx context.Context, token string) error {
	if token == "" {
		return domain.ErrInvalidToken
	}

	sub, err := s.repo.GetSubscriptionByConfirmToken(ctx, token)
	if err != nil {
		return fmt.Errorf("finding subscription by token: %w", err)
	}
	if sub == nil {
		return domain.ErrTokenNotFound
	}

	if err := s.repo.ConfirmSubscription(ctx, sub.ID); err != nil {
		return fmt.Errorf("confirming subscription: %w", err)
	}

	return nil
}

func (s *SubscriptionService) Unsubscribe(ctx context.Context, token string) error {
	if token == "" {
		return domain.ErrInvalidToken
	}

	sub, err := s.repo.GetSubscriptionByUnsubscribeToken(ctx, token)
	if err != nil {
		return fmt.Errorf("finding subscription by token: %w", err)
	}
	if sub == nil {
		return domain.ErrTokenNotFound
	}

	if err := s.repo.DeleteSubscription(ctx, sub.ID); err != nil {
		return fmt.Errorf("deleting subscription: %w", err)
	}

	return nil
}

func (s *SubscriptionService) GetSubscriptions(ctx context.Context, email string) ([]domain.Subscription, error) {
	if err := validateEmail(email); err != nil {
		return nil, err
	}

	subs, err := s.repo.GetSubscriptionsByEmail(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("getting subscriptions: %w", err)
	}

	return subs, nil
}

func parseRepoFullName(fullName string) (owner, repo string, err error) {
	fullName = strings.TrimSpace(fullName)
	parts := strings.Split(fullName, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", domain.ErrInvalidRepoFormat
	}
	return parts[0], parts[1], nil
}

func validateEmail(email string) error {
	if email == "" {
		return domain.ErrInvalidEmail
	}
	_, err := mail.ParseAddress(email)
	if err != nil {
		return domain.ErrInvalidEmail
	}
	return nil
}
