package application

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/user/github-release-notification-api/internal/domain"
	"github.com/user/github-release-notification-api/internal/domain/port"
)

type ScannerConfig struct {
	Interval     time.Duration
	CycleTimeout time.Duration
	RepoTimeout  time.Duration
}

type Notifier interface {
	NotifySubscribers(ctx context.Context, repository *domain.Repository, release *domain.Release) error
}

type scannerDeps interface {
	port.RepositoryReader
	port.RepositoryWriter
}

type ScannerService struct {
	repo       scannerDeps
	github     port.GitHubClient
	notifier   Notifier
	logger     *slog.Logger
	cfg        ScannerConfig
	scanCycles prometheus.Counter
}

func NewScannerService(
	repo scannerDeps,
	github port.GitHubClient,
	notifier Notifier,
	logger *slog.Logger,
	cfg ScannerConfig,
	scanCycles prometheus.Counter,
) *ScannerService {
	return &ScannerService{
		repo:       repo,
		github:     github,
		notifier:   notifier,
		logger:     logger,
		cfg:        cfg,
		scanCycles: scanCycles,
	}
}

// Start runs the scanner in a goroutine. It returns when ctx is cancelled.
func (s *ScannerService) Start(ctx context.Context) {
	s.logger.Info("scanner started", "interval", s.cfg.Interval.String())
	ticker := time.NewTicker(s.cfg.Interval)
	defer ticker.Stop()

	s.scan(ctx)

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("scanner stopped")
			return
		case <-ticker.C:
			s.scan(ctx)
		}
	}
}

func (s *ScannerService) scan(ctx context.Context) {
	cycleCtx, cycleCancel := context.WithTimeout(ctx, s.cfg.CycleTimeout)
	defer cycleCancel()

	if s.scanCycles != nil {
		s.scanCycles.Inc()
	}

	s.logger.Info("scanning for new releases...")

	repos, err := s.repo.GetRepositoriesWithConfirmedSubs(cycleCtx)
	if err != nil {
		s.logger.Error("failed to get repositories", "error", err)
		return
	}

	s.logger.Info("found repositories to check", "count", len(repos))

	for i := range repos {
		if cycleCtx.Err() != nil {
			s.logger.Warn("scan cycle timed out, skipping remaining repos")
			return
		}
		if err := s.checkRepo(cycleCtx, &repos[i]); err != nil {
			s.logger.Error("failed to check repository",
				"repo", repos[i].FullName(),
				"error", err,
			)
		}
	}
}

func (s *ScannerService) checkRepo(ctx context.Context, repo *domain.Repository) error {
	repoCtx, repoCancel := context.WithTimeout(ctx, s.cfg.RepoTimeout)
	defer repoCancel()

	release, err := s.github.GetLatestRelease(repoCtx, repo.Owner, repo.Repo)
	if err != nil {
		return fmt.Errorf("getting latest release: %w", err)
	}

	if release == nil {
		return nil
	}

	if release.TagName == repo.LastSeenTag {
		return nil
	}

	if repo.LastSeenTag == "" {
		s.logger.Info("initial release detected, setting baseline",
			"repo", repo.FullName(),
			"tag", release.TagName,
		)
		return s.repo.UpdateLastSeenTag(repoCtx, repo.ID, release.TagName)
	}

	s.logger.Info("new release detected",
		"repo", repo.FullName(),
		"old_tag", repo.LastSeenTag,
		"new_tag", release.TagName,
	)

	if err := s.notifier.NotifySubscribers(repoCtx, repo, release); err != nil {
		s.logger.Error("failed to notify subscribers",
			"repo", repo.FullName(),
			"error", err,
		)
	}

	return s.repo.UpdateLastSeenTag(repoCtx, repo.ID, release.TagName)
}
