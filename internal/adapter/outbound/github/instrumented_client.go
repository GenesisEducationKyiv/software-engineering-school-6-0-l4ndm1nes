package github

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/user/github-release-notification-api/internal/domain"
	"github.com/user/github-release-notification-api/internal/domain/port"
)

type InstrumentedClient struct {
	inner   port.GitHubClient
	counter *prometheus.CounterVec
}

func NewInstrumentedClient(inner port.GitHubClient, counter *prometheus.CounterVec) *InstrumentedClient {
	return &InstrumentedClient{inner: inner, counter: counter}
}

func (c *InstrumentedClient) RepoExists(ctx context.Context, owner, repo string) (bool, error) {
	exists, err := c.inner.RepoExists(ctx, owner, repo)
	status := "success"
	if err != nil {
		status = "error"
	}
	c.counter.WithLabelValues("repo_exists", status).Inc()
	return exists, err
}

func (c *InstrumentedClient) GetLatestRelease(ctx context.Context, owner, repo string) (*domain.Release, error) {
	release, err := c.inner.GetLatestRelease(ctx, owner, repo)
	status := "success"
	if err != nil {
		status = "error"
	}
	c.counter.WithLabelValues("get_latest_release", status).Inc()
	return release, err
}
