package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

const defaultGRPCPort = "50051"

type Config struct {
	DatabaseURL      string
	GRPCPort         string
	ServiceName      string
	LogLevel         string
	CancelOddOpsCron string
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
