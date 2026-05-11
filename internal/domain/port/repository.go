package port

import (
	"context"

	"github.com/user/github-release-notification-api/internal/domain"
)

type RepositoryReader interface {
	GetRepositoryByOwnerRepo(ctx context.Context, owner, repo string) (*domain.Repository, error)
	GetRepositoriesWithConfirmedSubs(ctx context.Context) ([]domain.Repository, error)
}

type RepositoryWriter interface {
	CreateRepository(ctx context.Context, owner, repo string) (*domain.Repository, error)
	UpdateLastSeenTag(ctx context.Context, repoID int, tag string) error
}

type SubscriptionReader interface {
	GetSubscriptionByEmailAndRepo(ctx context.Context, email string, repoID int) (*domain.Subscription, error)
	GetSubscriptionsByEmail(ctx context.Context, email string) ([]domain.Subscription, error)
	GetSubscriptionByConfirmToken(ctx context.Context, token string) (*domain.Subscription, error)
	GetSubscriptionByUnsubscribeToken(ctx context.Context, token string) (*domain.Subscription, error)
	GetConfirmedSubscriptionsByRepoID(ctx context.Context, repoID int) ([]domain.Subscription, error)
}

type SubscriptionWriter interface {
	CreateSubscription(ctx context.Context, sub *domain.Subscription) error
	ConfirmSubscription(ctx context.Context, id int) error
	DeleteSubscription(ctx context.Context, id int) error
}

type SubscriptionRepository interface {
	RepositoryReader
	RepositoryWriter
	SubscriptionReader
	SubscriptionWriter
}
