package main

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net"
	"os/signal"
	"syscall"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/rexemtoxa/gepard_billing/internal/billing"
	"github.com/rexemtoxa/gepard_billing/internal/billingworker"
	"github.com/rexemtoxa/gepard_billing/internal/config"
	grpcserver "github.com/rexemtoxa/gepard_billing/internal/grpcServer"
	billingv1 "github.com/rexemtoxa/gepard_billing/proto/billing/v1"
	"google.golang.org/grpc"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	db, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	db.SetMaxOpenConns(cfg.DBMaxOpenConns)
	db.SetMaxIdleConns(cfg.DBMaxIdleConns)
	db.SetConnMaxIdleTime(cfg.DBConnMaxIdleTime)
	db.SetConnMaxLifetime(cfg.DBConnMaxLifetime)
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			log.Printf("close database: %v", closeErr)
		}
	}()

	if err := db.PingContext(context.Background()); err != nil {
		log.Fatalf("ping database: %v", err)
	}

	billingService := billing.NewService(db)
	billingGRPCServer := billing.NewGRPCServer(billingService)

	scheduler, err := billingworker.NewScheduler(cfg.CancelOddOpsCron, billingService)
	if err != nil {
		log.Fatalf("%v", err)
	}
	scheduler.Start()

	listener, err := net.Listen("tcp", cfg.ListenAddress())
	if err != nil {
		log.Fatalf("listen on %s: %v", cfg.ListenAddress(), err)
	}

	server := grpcserver.NewServer(cfg.ServiceName, cfg.GRPCUnaryTimeout, log.Default(), func(grpcServer *grpc.Server) {
		billingv1.RegisterBillingServiceServer(grpcServer, billingGRPCServer)
	})
	server.SetServingStatus(healthpb.HealthCheckResponse_SERVING)
	errCh := make(chan error, 1)

	go func() {
		if serveErr := server.Serve(listener); serveErr != nil && !errors.Is(serveErr, grpc.ErrServerStopped) {
			errCh <- serveErr
		}
	}()

	log.Printf("billing gRPC server listening on %s", cfg.ListenAddress())

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	select {
	case serveErr := <-errCh:
		log.Fatalf("serve grpc: %v", serveErr)
	case <-ctx.Done():
		log.Printf("shutting down billing gRPC server")
		scheduler.Stop()
		server.SetServingStatus(healthpb.HealthCheckResponse_NOT_SERVING)
		server.GracefulStop()
	}
}
