package billing

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	grpcserver "github.com/rexemtoxa/gepard_billing/internal/grpcServer"
	"github.com/rexemtoxa/gepard_billing/internal/repository"
	billingv1 "github.com/rexemtoxa/gepard_billing/proto/billing/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/test/bufconn"
)

const (
	defaultIntegrationDatabaseURL = "postgres://postgres:postgres@localhost:5432/gepard_billing?sslmode=disable"
	integrationBufconnSize        = 1024 * 1024
	integrationTimeout            = 10 * time.Second
)

var integrationHarnessState struct {
	once    sync.Once
	harness *billingIntegrationHarness
	err     error
}

type billingIntegrationHarness struct {
	adminDB   *sql.DB
	db        *sql.DB
	queries   *repository.Queries
	dbName    string
	listener  *bufconn.Listener
	server    *grpcserver.Server
	serverErr chan error
	conn      *grpc.ClientConn
	client    billingv1.BillingServiceClient
	mu        sync.Mutex
}

type billingIntegrationSession struct {
	t       *testing.T
	harness *billingIntegrationHarness
}

type billingDBState struct {
	OperationRequests int
	LedgerEntries     int
	Balance           string
}

func TestMain(m *testing.M) {
	code := m.Run()

	if integrationHarnessState.harness != nil {
		if err := integrationHarnessState.harness.close(); err != nil {
			fmt.Fprintf(os.Stderr, "close billing integration harness: %v\n", err)
			if code == 0 {
				code = 1
			}
		}
	}

	os.Exit(code)
}

func billingIntegrationSessionForTest(t *testing.T) *billingIntegrationSession {
	t.Helper()

	integrationHarnessState.once.Do(func() {
		integrationHarnessState.harness, integrationHarnessState.err = newBillingIntegrationHarness()
	})
	if integrationHarnessState.err != nil {
		t.Fatalf("create billing integration harness: %v", integrationHarnessState.err)
	}

	return integrationHarnessState.harness.beginTest(t)
}

func newBillingIntegrationHarness() (_ *billingIntegrationHarness, err error) {
	baseDSN := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if baseDSN == "" {
		baseDSN = defaultIntegrationDatabaseURL
	}

	adminDSN, err := replaceDatabaseInDSN(baseDSN, "postgres")
	if err != nil {
		return nil, fmt.Errorf("derive admin database URL: %w", err)
	}

	dbName, err := newIntegrationDatabaseName()
	if err != nil {
		return nil, fmt.Errorf("create integration database name: %w", err)
	}
	testDSN, err := replaceDatabaseInDSN(baseDSN, dbName)
	if err != nil {
		return nil, fmt.Errorf("derive test database URL: %w", err)
	}

	harness := &billingIntegrationHarness{dbName: dbName}
	defer func() {
		if err != nil {
			_ = harness.close()
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), integrationTimeout)
	defer cancel()

	harness.adminDB, err = sql.Open("pgx", adminDSN)
	if err != nil {
		return nil, fmt.Errorf("open admin database: %w", err)
	}
	if err := harness.adminDB.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("ping admin database: %w", err)
	}

	if _, err := harness.adminDB.ExecContext(ctx, "CREATE DATABASE "+quoteIdentifier(dbName)); err != nil {
		return nil, fmt.Errorf("create test database %q: %w", dbName, err)
	}

	harness.db, err = sql.Open("pgx", testDSN)
	if err != nil {
		return nil, fmt.Errorf("open test database: %w", err)
	}
	if err := harness.db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("ping test database: %w", err)
	}

	if err := applyTestMigrations(ctx, harness.db); err != nil {
		return nil, err
	}

	harness.queries = repository.New(harness.db)
	harness.listener = bufconn.Listen(integrationBufconnSize)
	harness.server = grpcserver.NewServer(
		"gepard-billing",
		0,
		log.New(io.Discard, "", 0),
		func(server *grpc.Server) {
			billingv1.RegisterBillingServiceServer(server, NewGRPCServer(NewService(harness.db)))
		},
	)
	harness.server.SetServingStatus(healthpb.HealthCheckResponse_SERVING)
	harness.serverErr = make(chan error, 1)

	go func() {
		harness.serverErr <- harness.server.Serve(harness.listener)
	}()

	harness.conn, err = grpc.DialContext(
		ctx,
		"bufnet",
		grpc.WithBlock(),
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return harness.listener.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("dial bufconn server: %w", err)
	}

	harness.client = billingv1.NewBillingServiceClient(harness.conn)

	return harness, nil
}

func (h *billingIntegrationHarness) beginTest(t *testing.T) *billingIntegrationSession {
	t.Helper()

	h.mu.Lock()
	t.Cleanup(h.mu.Unlock)

	ctx, cancel := context.WithTimeout(context.Background(), integrationTimeout)
	defer cancel()

	if err := h.reset(ctx); err != nil {
		t.Fatalf("reset billing integration database: %v", err)
	}

	return &billingIntegrationSession{
		t:       t,
		harness: h,
	}
}

func (s *billingIntegrationSession) applyOperation(request *billingv1.ApplyOperationRequest) (*billingv1.ApplyOperationResponse, error) {
	s.t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), integrationTimeout)
	defer cancel()

	return s.harness.client.ApplyOperation(ctx, request)
}

func (s *billingIntegrationSession) dbState() billingDBState {
	s.t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), integrationTimeout)
	defer cancel()

	var state billingDBState
	if err := s.harness.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM operation_requests").Scan(&state.OperationRequests); err != nil {
		s.t.Fatalf("count operation_requests: %v", err)
	}
	if err := s.harness.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM ledger_entries").Scan(&state.LedgerEntries); err != nil {
		s.t.Fatalf("count ledger_entries: %v", err)
	}

	head, err := s.harness.queries.GetLedgerHead(ctx)
	if err != nil {
		s.t.Fatalf("get ledger head: %v", err)
	}
	state.Balance = head.BalanceAfterText

	return state
}

func (h *billingIntegrationHarness) reset(ctx context.Context) error {
	_, err := h.db.ExecContext(ctx, "TRUNCATE TABLE ledger_entries, operation_requests RESTART IDENTITY CASCADE")
	return err
}

func (h *billingIntegrationHarness) close() error {
	var errs []error

	if h.conn != nil {
		if err := h.conn.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close gRPC client connection: %w", err))
		}
		h.conn = nil
	}

	if h.server != nil {
		h.server.Stop()
		h.server = nil
	}

	if h.listener != nil {
		if err := h.listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			errs = append(errs, fmt.Errorf("close bufconn listener: %w", err))
		}
		h.listener = nil
	}

	if h.serverErr != nil {
		select {
		case err := <-h.serverErr:
			if err != nil && !errors.Is(err, grpc.ErrServerStopped) {
				errs = append(errs, fmt.Errorf("serve bufconn gRPC server: %w", err))
			}
		case <-time.After(integrationTimeout):
			errs = append(errs, fmt.Errorf("wait for bufconn gRPC server shutdown: timed out"))
		}
		h.serverErr = nil
	}

	if h.db != nil {
		if err := h.db.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close test database: %w", err))
		}
		h.db = nil
	}

	if h.adminDB != nil {
		ctx, cancel := context.WithTimeout(context.Background(), integrationTimeout)
		if _, err := h.adminDB.ExecContext(
			ctx,
			`SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = $1 AND pid <> pg_backend_pid()`,
			h.dbName,
		); err != nil {
			errs = append(errs, fmt.Errorf("terminate active connections for %q: %w", h.dbName, err))
		}
		if _, err := h.adminDB.ExecContext(ctx, "DROP DATABASE IF EXISTS "+quoteIdentifier(h.dbName)); err != nil {
			errs = append(errs, fmt.Errorf("drop test database %q: %w", h.dbName, err))
		}
		cancel()

		if err := h.adminDB.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close admin database: %w", err))
		}
		h.adminDB = nil
	}

	if len(errs) == 0 {
		return nil
	}

	return errors.Join(errs...)
}

func applyTestMigrations(ctx context.Context, db *sql.DB) error {
	repoRoot, err := repoRootFromRuntime()
	if err != nil {
		return err
	}

	migrationsDir := filepath.Join(repoRoot, "db", "migrations")
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		return fmt.Errorf("read migrations directory: %w", err)
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".sql" {
			continue
		}
		names = append(names, entry.Name())
	}
	sort.Strings(names)

	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (version varchar PRIMARY KEY)`); err != nil {
		return fmt.Errorf("create schema_migrations table: %w", err)
	}

	for _, name := range names {
		path := filepath.Join(migrationsDir, name)
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}

		upSQL, err := migrationUpSQL(string(content))
		if err != nil {
			return fmt.Errorf("parse migration %s: %w", name, err)
		}
		if strings.TrimSpace(upSQL) == "" {
			continue
		}

		if _, err := db.ExecContext(ctx, upSQL); err != nil {
			return fmt.Errorf("apply migration %s: %w", name, err)
		}

		version := migrationVersionFromName(name)
		if _, err := db.ExecContext(
			ctx,
			`INSERT INTO schema_migrations (version) VALUES ($1) ON CONFLICT (version) DO NOTHING`,
			version,
		); err != nil {
			return fmt.Errorf("record migration %s: %w", name, err)
		}
	}

	return nil
}

func repoRootFromRuntime() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("resolve runtime caller path")
	}

	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..")), nil
}

func replaceDatabaseInDSN(dsn, dbName string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(dsn))
	if err != nil {
		return "", fmt.Errorf("parse database URL: %w", err)
	}
	if parsed.Scheme == "" {
		return "", fmt.Errorf("database URL must include a scheme")
	}

	parsed.Path = "/" + dbName
	return parsed.String(), nil
}

func newIntegrationDatabaseName() (string, error) {
	randomBytes := make([]byte, 6)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", err
	}

	return fmt.Sprintf("gepard_billing_test_%x", randomBytes), nil
}

func quoteIdentifier(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}

func migrationUpSQL(content string) (string, error) {
	const (
		upMarker   = "-- migrate:up"
		downMarker = "-- migrate:down"
	)

	upIndex := strings.Index(content, upMarker)
	if upIndex == -1 {
		return "", fmt.Errorf("missing %q marker", upMarker)
	}

	upSection := content[upIndex+len(upMarker):]
	if downIndex := strings.Index(upSection, downMarker); downIndex >= 0 {
		upSection = upSection[:downIndex]
	}

	return strings.TrimSpace(upSection), nil
}

func migrationVersionFromName(name string) string {
	parts := strings.SplitN(name, "_", 2)
	return parts[0]
}
