package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Server  ServerConfig
	DB      DBConfig
	Redis   RedisConfig
	SMTP    SMTPConfig
	GitHub  GitHubConfig
	Scanner ScannerConfig
	GRPC    GRPCConfig
	APIKey  string
	BaseURL string
	Log     LogConfig
}

type LogConfig struct {
	Level slog.Level
}

type ServerConfig struct {
	Port            string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration
}

type DBConfig struct {
	Host            string
	Port            string
	User            string
	Password        string
	Name            string
	SSLMode         string
	QueryTimeout    time.Duration
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

func (c *DBConfig) DSN() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s",
		c.User, c.Password, c.Host, c.Port, c.Name, c.SSLMode,
	)
}

type RedisConfig struct {
	Addr         string
	Password     string
	DB           int
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	MaxRetries   int
}

type SMTPConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	From     string
	Timeout  time.Duration
}

type GitHubConfig struct {
	Token      string
	BaseURL    string
	Timeout    time.Duration
	MaxRetries int
	CacheTTL   time.Duration
}

type ScannerConfig struct {
	Interval     time.Duration
	CycleTimeout time.Duration
	RepoTimeout  time.Duration
}

type GRPCConfig struct {
	Port string
}

func Load() *Config {
	return &Config{
		Server: ServerConfig{
			Port:            getEnv("SERVER_PORT", "8080"),
			ReadTimeout:     getEnvDuration("SERVER_READ_TIMEOUT", 15*time.Second),
			WriteTimeout:    getEnvDuration("SERVER_WRITE_TIMEOUT", 15*time.Second),
			IdleTimeout:     getEnvDuration("SERVER_IDLE_TIMEOUT", 60*time.Second),
			ShutdownTimeout: getEnvDuration("SERVER_SHUTDOWN_TIMEOUT", 10*time.Second),
		},
		DB: DBConfig{
			Host:            getEnv("DB_HOST", "localhost"),
			Port:            getEnv("DB_PORT", "5432"),
			User:            getEnv("DB_USER", "postgres"),
			Password:        getEnv("DB_PASSWORD", "postgres"),
			Name:            getEnv("DB_NAME", "releases"),
			SSLMode:         getEnv("DB_SSLMODE", "disable"),
			QueryTimeout:    getEnvDuration("DB_QUERY_TIMEOUT", 5*time.Second),
			MaxOpenConns:    getEnvInt("DB_MAX_OPEN_CONNS", 25),
			MaxIdleConns:    getEnvInt("DB_MAX_IDLE_CONNS", 10),
			ConnMaxLifetime: getEnvDuration("DB_CONN_MAX_LIFETIME", 5*time.Minute),
		},
		Redis: RedisConfig{
			Addr:         getEnv("REDIS_ADDR", "localhost:6379"),
			Password:     getEnv("REDIS_PASSWORD", ""),
			DB:           getEnvInt("REDIS_DB", 0),
			DialTimeout:  getEnvDuration("REDIS_DIAL_TIMEOUT", 5*time.Second),
			ReadTimeout:  getEnvDuration("REDIS_READ_TIMEOUT", 3*time.Second),
			WriteTimeout: getEnvDuration("REDIS_WRITE_TIMEOUT", 3*time.Second),
			MaxRetries:   getEnvInt("REDIS_MAX_RETRIES", 3),
		},
		SMTP: SMTPConfig{
			Host:     getEnv("SMTP_HOST", "localhost"),
			Port:     getEnv("SMTP_PORT", "1025"),
			User:     getEnv("SMTP_USER", ""),
			Password: getEnv("SMTP_PASSWORD", ""),
			From:     getEnv("SMTP_FROM", "noreply@releases-api.app"),
			Timeout:  getEnvDuration("SMTP_TIMEOUT", 10*time.Second),
		},
		GitHub: GitHubConfig{
			Token:      getEnv("GITHUB_TOKEN", ""),
			BaseURL:    getEnv("GITHUB_BASE_URL", "https://api.github.com"),
			Timeout:    getEnvDuration("GITHUB_TIMEOUT", 15*time.Second),
			MaxRetries: getEnvInt("GITHUB_MAX_RETRIES", 3),
			CacheTTL:   getEnvDuration("GITHUB_CACHE_TTL", 10*time.Minute),
		},
		Scanner: ScannerConfig{
			Interval:     getEnvDuration("SCANNER_INTERVAL", 5*time.Minute),
			CycleTimeout: getEnvDuration("SCANNER_CYCLE_TIMEOUT", 4*time.Minute),
			RepoTimeout:  getEnvDuration("SCANNER_REPO_TIMEOUT", 30*time.Second),
		},
		GRPC: GRPCConfig{
			Port: getEnv("GRPC_PORT", "50051"),
		},
		APIKey:  getEnv("API_KEY", ""),
		BaseURL: getEnv("BASE_URL", "http://localhost:8080"),
		Log: LogConfig{
			Level: parseLogLevel(getEnv("LOG_LEVEL", "info")),
		},
	}
}

func getEnv(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if val, ok := os.LookupEnv(key); ok {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return fallback
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	if val, ok := os.LookupEnv(key); ok {
		if d, err := time.ParseDuration(val); err == nil {
			return d
		}
	}
	return fallback
}

func parseLogLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
