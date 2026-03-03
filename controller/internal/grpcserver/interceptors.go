package grpcserver

import (
	"context"
	"log"
	"time"

	"google.golang.org/grpc"
)

func unaryLoggingInterceptor(
	ctx context.Context,
	req any,
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (any, error) {
	start := time.Now()
	resp, err := handler(ctx, req)
	log.Printf("grpc unary method=%s duration=%s err=%v", info.FullMethod, time.Since(start), err)
	return resp, err
}

func streamLoggingInterceptor(
	srv any,
	ss grpc.ServerStream,
	info *grpc.StreamServerInfo,
	handler grpc.StreamHandler,
) error {
	start := time.Now()
	err := handler(srv, ss)
	log.Printf("grpc stream method=%s duration=%s err=%v", info.FullMethod, time.Since(start), err)
	return err
}
