package github

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/user/github-release-notification-api/internal/domain"
	"github.com/user/github-release-notification-api/internal/domain/port"
)

type CachedClient struct {
	inner    port.GitHubClient
	cache    port.Cache
	cacheTTL time.Duration
	logger   *slog.Logger
}

func NewCachedClient(inner port.GitHubClient, cache port.Cache, cacheTTL time.Duration, logger *slog.Logger) *CachedClient {
	return &CachedClient{
		inner:    inner,
		cache:    cache,
		cacheTTL: cacheTTL,
		logger:   logger,
	}
}

func (c *CachedClient) RepoExists(ctx context.Context, owner, repo string) (bool, error) {
	key := fmt.Sprintf("gh:repo_exists:%s/%s", owner, repo)

	data, err := c.cache.Get(ctx, key)
	if err != nil {
		c.logger.Warn("cache get error", "key", key, "error", err)
	}
	if data != nil {
		var exists bool
		if err := json.Unmarshal(data, &exists); err == nil {
			return exists, nil
		}
	}

	exists, err := c.inner.RepoExists(ctx, owner, repo)
	if err != nil {
		return false, err
	}

	if encoded, err := json.Marshal(exists); err == nil {
		if err := c.cache.Set(ctx, key, encoded, c.cacheTTL); err != nil {
			c.logger.Warn("cache set error", "key", key, "error", err)
		}
	}

	return exists, nil
}

func (c *CachedClient) GetLatestRelease(ctx context.Context, owner, repo string) (*domain.Release, error) {
	key := fmt.Sprintf("gh:latest_release:%s/%s", owner, repo)

	data, err := c.cache.Get(ctx, key)
	if err != nil {
		c.logger.Warn("cache get error", "key", key, "error", err)
	}
	if data != nil {
		var release domain.Release
		if err := json.Unmarshal(data, &release); err == nil {
			return &release, nil
		}
	}

	release, err := c.inner.GetLatestRelease(ctx, owner, repo)
	if err != nil {
		return nil, err
	}
	if release == nil {
		return nil, nil
	}

	if encoded, err := json.Marshal(release); err == nil {
		if err := c.cache.Set(ctx, key, encoded, c.cacheTTL); err != nil {
			c.logger.Warn("cache set error", "key", key, "error", err)
		}
	}

	return release, nil
}
