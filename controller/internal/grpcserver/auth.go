package grpcserver

import (
	"context"
	"crypto/subtle"

	"github.com/igris/ztna/controller/internal/config"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const adminTokenHeader = "x-admin-token"

var adminUnaryMethods = map[string]struct{}{
	"/ztna.controlplane.v1.WorkspaceService/CreateWorkspace":   {},
	"/ztna.controlplane.v1.WorkspaceService/CreateEnrollToken": {},
	"/ztna.controlplane.v1.DeviceService/RevokeDevice":         {},
	"/ztna.controlplane.v1.DeviceService/ListDevices":          {},
}

func adminUnaryAuthInterceptor(cfg config.Config) grpc.UnaryServerInterceptor {
	expected := cfg.AdminToken

	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		if _, ok := adminUnaryMethods[info.FullMethod]; !ok {
			return handler(ctx, req)
		}

		if expected == "" {
			return nil, status.Error(codes.FailedPrecondition, "admin auth is not configured")
		}

		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "missing request metadata")
		}

		tokens := md.Get(adminTokenHeader)
		if len(tokens) == 0 || tokens[0] == "" {
			return nil, status.Error(codes.Unauthenticated, "missing admin token")
		}

		if !secureTokenEqual(expected, tokens[0]) {
			return nil, status.Error(codes.PermissionDenied, "invalid admin token")
		}

		return handler(ctx, req)
	}
}

func secureTokenEqual(expected, provided string) bool {
	e := []byte(expected)
	p := []byte(provided)
	if len(e) != len(p) {
		return false
	}
	return subtle.ConstantTimeCompare(e, p) == 1
}
