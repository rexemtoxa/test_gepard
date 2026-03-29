package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
)

const (
	defaultGRPCPort          = "50051"
	defaultGRPCUnaryTimeout  = "5s"
	defaultDBMaxOpenConns    = "10"
	defaultDBMaxIdleConns    = "10"
	defaultDBConnMaxIdleTime = "5m"
	defaultDBConnMaxLifetime = "30m"
)

type Config struct {
	DatabaseURL       string
	GRPCPort          string
	GRPCUnaryTimeout  time.Duration
	ServiceName       string
	LogLevel          string
	CancelOddOpsCron  string
	DBMaxOpenConns    int
	DBMaxIdleConns    int
	DBConnMaxIdleTime time.Duration
	DBConnMaxLifetime time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		DatabaseURL:      strings.TrimSpace(os.Getenv("DATABASE_URL")),
		GRPCPort:         valueOrDefault("GRPC_PORT", defaultGRPCPort),
		ServiceName:      valueOrDefault("SERVICE_NAME", "gepard-billing"),
		LogLevel:         valueOrDefault("LOG_LEVEL", "INFO"),
		CancelOddOpsCron: valueOrDefault("CANCEL_ODD_OPS_CRON", "@every 1m"),
	}

	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}

	port, err := strconv.Atoi(cfg.GRPCPort)
	if err != nil || port < 1 || port > 65535 {
		return Config{}, fmt.Errorf("GRPC_PORT must be a valid TCP port, got %q", cfg.GRPCPort)
	}
	if cfg.CancelOddOpsCron == "" {
		return Config{}, fmt.Errorf("CANCEL_ODD_OPS_CRON is required")
	}
	if _, err := cron.ParseStandard(cfg.CancelOddOpsCron); err != nil {
		return Config{}, fmt.Errorf("CANCEL_ODD_OPS_CRON must be a valid cron expression, got %q: %w", cfg.CancelOddOpsCron, err)
	}

	cfg.GRPCUnaryTimeout, err = parseNonNegativeDuration("GRPC_UNARY_TIMEOUT", valueOrDefault("GRPC_UNARY_TIMEOUT", defaultGRPCUnaryTimeout))
	if err != nil {
		return Config{}, err
	}

	cfg.DBMaxOpenConns, err = parsePositiveInt("DB_MAX_OPEN_CONNS", valueOrDefault("DB_MAX_OPEN_CONNS", defaultDBMaxOpenConns))
	if err != nil {
		return Config{}, err
	}
	cfg.DBMaxIdleConns, err = parsePositiveInt("DB_MAX_IDLE_CONNS", valueOrDefault("DB_MAX_IDLE_CONNS", defaultDBMaxIdleConns))
	if err != nil {
		return Config{}, err
	}
	if cfg.DBMaxIdleConns > cfg.DBMaxOpenConns {
		return Config{}, fmt.Errorf("DB_MAX_IDLE_CONNS must be less than or equal to DB_MAX_OPEN_CONNS")
	}

	cfg.DBConnMaxIdleTime, err = parseNonNegativeDuration("DB_CONN_MAX_IDLE_TIME", valueOrDefault("DB_CONN_MAX_IDLE_TIME", defaultDBConnMaxIdleTime))
	if err != nil {
		return Config{}, err
	}
	cfg.DBConnMaxLifetime, err = parseNonNegativeDuration("DB_CONN_MAX_LIFETIME", valueOrDefault("DB_CONN_MAX_LIFETIME", defaultDBConnMaxLifetime))
	if err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func (c Config) ListenAddress() string {
	return ":" + c.GRPCPort
}

func valueOrDefault(name, fallback string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}

	return value
}

func parsePositiveInt(name, value string) (int, error) {
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer, got %q", name, value)
	}
	return parsed, nil
}

func parseNonNegativeDuration(name, value string) (time.Duration, error) {
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be a valid duration, got %q", name, value)
	}
	if parsed < 0 {
		return 0, fmt.Errorf("%s must be a non-negative duration, got %q", name, value)
	}
	return parsed, nil
}
