package config

import (
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	setBaseEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.GRPCPort != "50051" {
		t.Fatalf("GRPCPort = %q, want %q", cfg.GRPCPort, "50051")
	}
	if cfg.GRPCUnaryTimeout != 5*time.Second {
		t.Fatalf("GRPCUnaryTimeout = %s, want %s", cfg.GRPCUnaryTimeout, 5*time.Second)
	}
	if cfg.ServiceName != "gepard-billing" {
		t.Fatalf("ServiceName = %q, want %q", cfg.ServiceName, "gepard-billing")
	}
	if cfg.CancelOddOpsCron != "@every 1m" {
		t.Fatalf("CancelOddOpsCron = %q, want %q", cfg.CancelOddOpsCron, "@every 1m")
	}
	if cfg.DBMaxOpenConns != 10 {
		t.Fatalf("DBMaxOpenConns = %d, want %d", cfg.DBMaxOpenConns, 10)
	}
	if cfg.DBMaxIdleConns != 10 {
		t.Fatalf("DBMaxIdleConns = %d, want %d", cfg.DBMaxIdleConns, 10)
	}
	if cfg.DBConnMaxIdleTime != 5*time.Minute {
		t.Fatalf("DBConnMaxIdleTime = %s, want %s", cfg.DBConnMaxIdleTime, 5*time.Minute)
	}
	if cfg.DBConnMaxLifetime != 30*time.Minute {
		t.Fatalf("DBConnMaxLifetime = %s, want %s", cfg.DBConnMaxLifetime, 30*time.Minute)
	}
}

func TestLoadAcceptsValidCronExpressions(t *testing.T) {
	testCases := []string{"@every 1m", "*/5 * * * *"}
	for _, cronSpec := range testCases {
		cronSpec := cronSpec
		t.Run(cronSpec, func(t *testing.T) {
			setBaseEnv(t)
			t.Setenv("CANCEL_ODD_OPS_CRON", cronSpec)

			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load returned error: %v", err)
			}
			if cfg.CancelOddOpsCron != cronSpec {
				t.Fatalf("CancelOddOpsCron = %q, want %q", cfg.CancelOddOpsCron, cronSpec)
			}
		})
	}
}

func TestLoadRejectsInvalidCron(t *testing.T) {
	setBaseEnv(t)
	t.Setenv("CANCEL_ODD_OPS_CRON", "not-a-cron")

	_, err := Load()
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if err.Error() != `CANCEL_ODD_OPS_CRON must be a valid cron expression, got "not-a-cron": expected exactly 5 fields, found 1: [not-a-cron]` {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadRejectsInvalidPoolSettings(t *testing.T) {
	testCases := []struct {
		name         string
		envName      string
		envValue     string
		wantErr      string
		otherEnvName string
		otherEnv     string
	}{
		{
			name:     "open conns must be positive",
			envName:  "DB_MAX_OPEN_CONNS",
			envValue: "0",
			wantErr:  `DB_MAX_OPEN_CONNS must be a positive integer, got "0"`,
		},
		{
			name:     "idle conns must be positive",
			envName:  "DB_MAX_IDLE_CONNS",
			envValue: "-1",
			wantErr:  `DB_MAX_IDLE_CONNS must be a positive integer, got "-1"`,
		},
		{
			name:     "invalid idle time duration",
			envName:  "DB_CONN_MAX_IDLE_TIME",
			envValue: "five",
			wantErr:  `DB_CONN_MAX_IDLE_TIME must be a valid duration, got "five"`,
		},
		{
			name:     "invalid lifetime duration",
			envName:  "DB_CONN_MAX_LIFETIME",
			envValue: "forever",
			wantErr:  `DB_CONN_MAX_LIFETIME must be a valid duration, got "forever"`,
		},
		{
			name:         "idle exceeds open",
			envName:      "DB_MAX_OPEN_CONNS",
			envValue:     "5",
			otherEnvName: "DB_MAX_IDLE_CONNS",
			otherEnv:     "6",
			wantErr:      "DB_MAX_IDLE_CONNS must be less than or equal to DB_MAX_OPEN_CONNS",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			setBaseEnv(t)
			t.Setenv(tc.envName, tc.envValue)
			if tc.otherEnvName != "" {
				t.Setenv(tc.otherEnvName, tc.otherEnv)
			}

			_, err := Load()
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			if err.Error() != tc.wantErr {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestLoadAcceptsUnaryTimeoutOverrides(t *testing.T) {
	testCases := []struct {
		name    string
		value   string
		want    time.Duration
		wantErr string
	}{
		{
			name:  "custom timeout",
			value: "2s",
			want:  2 * time.Second,
		},
		{
			name:  "disabled timeout",
			value: "0",
			want:  0,
		},
		{
			name:    "invalid duration",
			value:   "soon",
			wantErr: `GRPC_UNARY_TIMEOUT must be a valid duration, got "soon"`,
		},
		{
			name:    "negative duration",
			value:   "-1s",
			wantErr: `GRPC_UNARY_TIMEOUT must be a non-negative duration, got "-1s"`,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			setBaseEnv(t)
			t.Setenv("GRPC_UNARY_TIMEOUT", tc.value)

			cfg, err := Load()
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if err.Error() != tc.wantErr {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Load returned error: %v", err)
			}
			if cfg.GRPCUnaryTimeout != tc.want {
				t.Fatalf("GRPCUnaryTimeout = %s, want %s", cfg.GRPCUnaryTimeout, tc.want)
			}
		})
	}
}

func setBaseEnv(t *testing.T) {
	t.Helper()

	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/gepard_billing?sslmode=disable")
	t.Setenv("GRPC_PORT", "")
	t.Setenv("GRPC_UNARY_TIMEOUT", "")
	t.Setenv("SERVICE_NAME", "")
	t.Setenv("LOG_LEVEL", "")
	t.Setenv("CANCEL_ODD_OPS_CRON", "")
	t.Setenv("DB_MAX_OPEN_CONNS", "")
	t.Setenv("DB_MAX_IDLE_CONNS", "")
	t.Setenv("DB_CONN_MAX_IDLE_TIME", "")
	t.Setenv("DB_CONN_MAX_LIFETIME", "")
}
