package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/golang-migrate/migrate/v4"
	pgMigrate "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus"
	goRedis "github.com/redis/go-redis/v9"

	"github.com/user/github-release-notification-api/internal/adapter/inbound/grpc"
	httpAdapter "github.com/user/github-release-notification-api/internal/adapter/inbound/http"
	"github.com/user/github-release-notification-api/internal/adapter/outbound/email"
	ghClient "github.com/user/github-release-notification-api/internal/adapter/outbound/github"
	pgRepo "github.com/user/github-release-notification-api/internal/adapter/outbound/postgres"
	redisCache "github.com/user/github-release-notification-api/internal/adapter/outbound/redis"
	"github.com/user/github-release-notification-api/internal/application"
	"github.com/user/github-release-notification-api/internal/config"
	"github.com/user/github-release-notification-api/internal/domain/port"
	"github.com/user/github-release-notification-api/internal/metrics"
	"github.com/user/github-release-notification-api/internal/platform"
)

func main() {
	cfg := config.Load()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.Log.Level}))
	slog.SetDefault(logger)

	db, err := sql.Open("postgres", cfg.DB.DSN())
	if err != nil {
		logger.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}

	db.SetMaxOpenConns(cfg.DB.MaxOpenConns)
	db.SetMaxIdleConns(cfg.DB.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.DB.ConnMaxLifetime)

	if err := db.Ping(); err != nil {
		logger.Error("failed to ping database", "error", err)
		_ = db.Close()
		os.Exit(1)
	}
	defer func() { _ = db.Close() }()
	logger.Info("connected to database")

	if err := runMigrations(db); err != nil {
		logger.Error("failed to run migrations", "error", err)
		_ = db.Close()
		os.Exit(1) //nolint:gocritic // DB closed above; Exit skips defer
	}
	logger.Info("migrations applied successfully")

	rdb := goRedis.NewClient(&goRedis.Options{
		Addr:         cfg.Redis.Addr,
		Password:     cfg.Redis.Password,
		DB:           cfg.Redis.DB,
		DialTimeout:  cfg.Redis.DialTimeout,
		ReadTimeout:  cfg.Redis.ReadTimeout,
		WriteTimeout: cfg.Redis.WriteTimeout,
		MaxRetries:   cfg.Redis.MaxRetries,
	})
	defer func() { _ = rdb.Close() }()

	appMetrics := metrics.New(prometheus.DefaultRegisterer)

	subscriptionRepo := pgRepo.NewSubscriptionRepo(db, cfg.DB.QueryTimeout)

	rawGHClient := ghClient.NewClient(cfg.GitHub.Token, cfg.GitHub.BaseURL, cfg.GitHub.Timeout, cfg.GitHub.MaxRetries, logger)

	var githubClient port.GitHubClient = rawGHClient
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		logger.Warn("redis not available, caching disabled", "error", err)
	} else {
		logger.Info("connected to redis")
		cache := redisCache.NewCache(rdb)
		githubClient = ghClient.NewCachedClient(rawGHClient, cache, cfg.GitHub.CacheTTL, logger)
	}
	githubClient = ghClient.NewInstrumentedClient(githubClient, appMetrics.GitHubAPICalls)

	mailer := email.NewSender(
		cfg.SMTP.Host, cfg.SMTP.Port,
		cfg.SMTP.User, cfg.SMTP.Password,
		cfg.SMTP.From, cfg.SMTP.Timeout, logger,
	)

	urls := application.NewURLBuilder(cfg.BaseURL)
	tokens := application.NewCryptoTokenGenerator(0)

	subService := application.NewSubscriptionService(
		subscriptionRepo, githubClient, mailer, tokens, urls, logger,
	)
	notifierService := application.NewNotifierService(
		subscriptionRepo, mailer, urls,
		platform.DefaultMailRetryConfig(), platform.IsTransientMailError,
		logger, appMetrics.EmailsSent,
	)
	scannerService := application.NewScannerService(
		subscriptionRepo, githubClient, notifierService, logger,
		application.ScannerConfig{
			Interval:     cfg.Scanner.Interval,
			CycleTimeout: cfg.Scanner.CycleTimeout,
			RepoTimeout:  cfg.Scanner.RepoTimeout,
		},
		appMetrics.ScanCycles,
	)

	httpHandler := httpAdapter.NewHandler(subService)
	router := httpAdapter.NewRouter(httpAdapter.RouterConfig{
		Handler:         httpHandler,
		APIKey:          cfg.APIKey,
		RequestsTotal:   appMetrics.HTTPRequestsTotal,
		RequestDuration: appMetrics.HTTPRequestDuration,
	})

	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	grpcHandler := grpc.NewHandler(subService)
	grpcServer := grpc.NewServer(grpcHandler, cfg.GRPC.Port, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go scannerService.Start(ctx)

	go func() {
		if err := grpcServer.Start(); err != nil {
			logger.Error("gRPC server error", "error", err)
		}
	}()

	go func() {
		logger.Info("HTTP server starting", "port", cfg.Server.Port)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("HTTP server error", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	logger.Info("shutting down", "signal", sig.String())

	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer shutdownCancel()

	grpcServer.Stop()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("HTTP server shutdown error", "error", err)
	}

	logger.Info("server stopped gracefully")
}

func runMigrations(db *sql.DB) error {
	driver, err := pgMigrate.WithInstance(db, &pgMigrate.Config{})
	if err != nil {
		return fmt.Errorf("creating migration driver: %w", err)
	}

	m, err := migrate.NewWithDatabaseInstance("file://migrations", "postgres", driver)
	if err != nil {
		return fmt.Errorf("creating migration instance: %w", err)
	}

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("running migrations: %w", err)
	}

	return nil
}
