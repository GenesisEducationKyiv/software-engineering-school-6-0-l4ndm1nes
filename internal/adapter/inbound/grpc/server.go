package grpc

import (
	"fmt"
	"log/slog"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/user/github-release-notification-api/internal/adapter/inbound/grpc/pb"
)

type Server struct {
	grpcServer *grpc.Server
	port       string
	logger     *slog.Logger
}

func NewServer(handler *Handler, port string, logger *slog.Logger) *Server {
	srv := grpc.NewServer()
	pb.RegisterSubscriptionServiceServer(srv, handler)
	reflection.Register(srv)

	return &Server{
		grpcServer: srv,
		port:       port,
		logger:     logger,
	}
}

func (s *Server) Start() error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", s.port))
	if err != nil {
		return fmt.Errorf("failed to listen on port %s: %w", s.port, err)
	}

	s.logger.Info("gRPC server starting", "port", s.port)
	return s.grpcServer.Serve(lis)
}

func (s *Server) Stop() {
	s.logger.Info("gRPC server stopping")
	s.grpcServer.GracefulStop()
}
