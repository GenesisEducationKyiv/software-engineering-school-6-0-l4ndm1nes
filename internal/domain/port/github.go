package port

import (
	"context"

	"github.com/user/github-release-notification-api/internal/domain"
)

type GitHubClient interface {
	RepoExists(ctx context.Context, owner, repo string) (bool, error)
	GetLatestRelease(ctx context.Context, owner, repo string) (*domain.Release, error)
}
