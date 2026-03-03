package grpcserver

import (
	"context"
	"testing"

	"github.com/igris/ztna/controller/internal/config"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestAdminInterceptor_AllowsNonAdminMethodWithoutToken(t *testing.T) {
	interceptor := adminUnaryAuthInterceptor(config.Config{AdminToken: "secret"})

	called := false
	_, err := interceptor(
		context.Background(),
		nil,
		&grpc.UnaryServerInfo{FullMethod: "/ztna.controlplane.v1.WorkspaceService/GetWorkspaceCA"},
		func(ctx context.Context, req any) (any, error) {
			called = true
			return "ok", nil
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatalf("expected handler to be called")
	}
}

func TestAdminInterceptor_RejectsMissingToken(t *testing.T) {
	interceptor := adminUnaryAuthInterceptor(config.Config{AdminToken: "secret"})

	_, err := interceptor(
		context.Background(),
		nil,
		&grpc.UnaryServerInfo{FullMethod: "/ztna.controlplane.v1.DeviceService/RevokeDevice"},
		func(ctx context.Context, req any) (any, error) { return "ok", nil },
	)
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated, got: %v", err)
	}
}

func TestAdminInterceptor_RejectsWrongToken(t *testing.T) {
	interceptor := adminUnaryAuthInterceptor(config.Config{AdminToken: "secret"})
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(adminTokenHeader, "wrong"))

	_, err := interceptor(
		ctx,
		nil,
		&grpc.UnaryServerInfo{FullMethod: "/ztna.controlplane.v1.DeviceService/RevokeDevice"},
		func(ctx context.Context, req any) (any, error) { return "ok", nil },
	)
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("expected PermissionDenied, got: %v", err)
	}
}

func TestAdminInterceptor_AllowsCorrectToken(t *testing.T) {
	interceptor := adminUnaryAuthInterceptor(config.Config{AdminToken: "secret"})
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(adminTokenHeader, "secret"))

	called := false
	_, err := interceptor(
		ctx,
		nil,
		&grpc.UnaryServerInfo{FullMethod: "/ztna.controlplane.v1.WorkspaceService/CreateWorkspace"},
		func(ctx context.Context, req any) (any, error) {
			called = true
			return "ok", nil
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatalf("expected handler to be called")
	}
}

func TestAdminInterceptor_RejectsWhenNotConfigured(t *testing.T) {
	interceptor := adminUnaryAuthInterceptor(config.Config{AdminToken: ""})
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(adminTokenHeader, "anything"))

	_, err := interceptor(
		ctx,
		nil,
		&grpc.UnaryServerInfo{FullMethod: "/ztna.controlplane.v1.WorkspaceService/CreateEnrollToken"},
		func(ctx context.Context, req any) (any, error) { return "ok", nil },
	)
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("expected FailedPrecondition, got: %v", err)
	}
}
