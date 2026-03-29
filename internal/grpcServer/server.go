package grpcserver

import (
	"context"
	"log"
	"net"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

const healthMethodPrefix = "/grpc.health.v1.Health/"

type Server struct {
	grpcServer   *grpc.Server
	healthServer *health.Server
	serviceName  string
}

func NewServer(serviceName string, unaryTimeout time.Duration, logger *log.Logger, registerFns ...func(*grpc.Server)) *Server {
	if logger == nil {
		logger = log.Default()
	}

	server := grpc.NewServer(grpc.ChainUnaryInterceptor(
		loggingUnaryInterceptor(logger),
		timeoutUnaryInterceptor(unaryTimeout),
	))
	healthServer := health.NewServer()

	runtimeServer := &Server{
		grpcServer:   server,
		healthServer: healthServer,
		serviceName:  serviceName,
	}
	runtimeServer.SetServingStatus(healthpb.HealthCheckResponse_NOT_SERVING)

	healthpb.RegisterHealthServer(server, healthServer)
	for _, registerFn := range registerFns {
		registerFn(server)
	}
	reflection.Register(server)

	return runtimeServer
}

func loggingUnaryInterceptor(logger *log.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if shouldSkipUnaryInterceptors(info.FullMethod) {
			return handler(ctx, req)
		}

		startedAt := time.Now()
		response, err := handler(ctx, req)
		logger.Printf(
			"event=grpc_request method=%s grpc_code=%s duration=%s",
			info.FullMethod,
			status.Code(err),
			time.Since(startedAt),
		)

		return response, err
	}
}

func timeoutUnaryInterceptor(unaryTimeout time.Duration) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if unaryTimeout == 0 || shouldSkipUnaryInterceptors(info.FullMethod) {
			return handler(ctx, req)
		}

		if deadline, ok := ctx.Deadline(); ok && time.Until(deadline) <= unaryTimeout {
			return handler(ctx, req)
		}

		timedCtx, cancel := context.WithTimeout(ctx, unaryTimeout)
		defer cancel()

		return handler(timedCtx, req)
	}
}

func shouldSkipUnaryInterceptors(fullMethod string) bool {
	return strings.HasPrefix(fullMethod, healthMethodPrefix)
}

func (s *Server) Serve(listener net.Listener) error {
	return s.grpcServer.Serve(listener)
}

func (s *Server) GracefulStop() {
	s.grpcServer.GracefulStop()
}

func (s *Server) Stop() {
	s.grpcServer.Stop()
}

func (s *Server) SetServingStatus(status healthpb.HealthCheckResponse_ServingStatus) {
	s.healthServer.SetServingStatus("", status)
	if s.serviceName != "" {
		s.healthServer.SetServingStatus(s.serviceName, status)
	}
}
