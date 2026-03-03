package grpcserver

import (
	"context"
	"errors"
	"fmt"
	"net"

	"github.com/igris/ztna/controller/internal/config"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

// Server owns the root gRPC server lifecycle.
type Server struct {
	lis  net.Listener
	grpc *grpc.Server
}

func New(cfg config.Config) (*Server, error) {
	lis, err := net.Listen("tcp", cfg.ListenAddr)
	if err != nil {
		return nil, err
	}

	opts := []grpc.ServerOption{
		grpc.ChainUnaryInterceptor(
			adminUnaryAuthInterceptor(cfg),
			unaryLoggingInterceptor,
		),
		grpc.ChainStreamInterceptor(streamLoggingInterceptor),
	}
	if cfg.TLSEnabled {
		creds, err := serverTransportCredentials(cfg)
		if err != nil {
			return nil, fmt.Errorf("build TLS credentials: %w", err)
		}
		opts = append(opts, grpc.Creds(creds))
	} else {
		opts = append(opts, grpc.Creds(insecure.NewCredentials()))
	}

	g := grpc.NewServer(opts...)

	h := health.NewServer()
	h.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	healthpb.RegisterHealthServer(g, h)
	reflection.Register(g)

	return &Server{lis: lis, grpc: g}, nil
}

func (s *Server) Start() error {
	return s.grpc.Serve(s.lis)
}

func (s *Server) GRPC() *grpc.Server {
	return s.grpc
}

func (s *Server) GracefulStop(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		s.grpc.GracefulStop()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return errors.New("graceful stop deadline exceeded")
	}
}

func (s *Server) Stop() {
	s.grpc.Stop()
}
