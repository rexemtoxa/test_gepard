package grpcserver

import (
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

type Server struct {
	grpcServer   *grpc.Server
	healthServer *health.Server
	serviceName  string
}

func NewServer(serviceName string, registerFns ...func(*grpc.Server)) *Server {
	server := grpc.NewServer()
	healthServer := health.NewServer()

	runtimeServer := &Server{
		grpcServer:   server,
		healthServer: healthServer,
		serviceName:  serviceName,
	}
	runtimeServer.SetServingStatus(healthpb.HealthCheckResponse_SERVING)

	healthpb.RegisterHealthServer(server, healthServer)
	for _, registerFn := range registerFns {
		registerFn(server)
	}
	reflection.Register(server)

	return runtimeServer
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
