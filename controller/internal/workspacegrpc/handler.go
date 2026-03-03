package workspacegrpc

import (
	"context"
	"errors"

	"github.com/igris/ztna/controller/internal/token"
	"github.com/igris/ztna/controller/internal/workspace"
	controlplanev1 "github.com/igris/ztna/proto/gen/go/ztna/controlplane/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Handler struct {
	controlplanev1.UnimplementedWorkspaceServiceServer

	WorkspaceSvc workspace.Service
	TokenSvc     token.Service
}

func NewHandler(workspaceSvc workspace.Service, tokenSvc token.Service) *Handler {
	return &Handler{WorkspaceSvc: workspaceSvc, TokenSvc: tokenSvc}
}

func (h *Handler) CreateWorkspace(ctx context.Context, req *controlplanev1.CreateWorkspaceRequest) (*controlplanev1.CreateWorkspaceResponse, error) {
	if h.WorkspaceSvc == nil {
		return nil, status.Error(codes.FailedPrecondition, "workspace service is not configured")
	}

	out, err := h.WorkspaceSvc.CreateWorkspace(ctx, workspace.CreateWorkspaceInput{DisplayName: req.GetDisplayName()})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create workspace: %v", err)
	}

	return &controlplanev1.CreateWorkspaceResponse{
		WorkspaceId: out.WorkspaceID,
		CaCertPem:   out.CACertPEM,
		CreatedAt:   timestamppb.Now(),
	}, nil
}

func (h *Handler) GetWorkspaceCA(ctx context.Context, req *controlplanev1.GetWorkspaceCARequest) (*controlplanev1.GetWorkspaceCAResponse, error) {
	if h.WorkspaceSvc == nil {
		return nil, status.Error(codes.FailedPrecondition, "workspace service is not configured")
	}
	if req.GetWorkspaceId() == "" {
		return nil, status.Error(codes.InvalidArgument, "workspace_id is required")
	}

	caPEM, err := h.WorkspaceSvc.GetWorkspaceCA(ctx, req.GetWorkspaceId())
	if err != nil {
		if errors.Is(err, workspace.ErrNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Errorf(codes.Internal, "get workspace ca: %v", err)
	}

	return &controlplanev1.GetWorkspaceCAResponse{
		WorkspaceId: req.GetWorkspaceId(),
		CaCertPem:   caPEM,
	}, nil
}

func (h *Handler) CreateEnrollToken(ctx context.Context, req *controlplanev1.CreateEnrollTokenRequest) (*controlplanev1.CreateEnrollTokenResponse, error) {
	if h.TokenSvc == nil {
		return nil, status.Error(codes.FailedPrecondition, "token service is not configured")
	}
	if req.GetWorkspaceId() == "" {
		return nil, status.Error(codes.InvalidArgument, "workspace_id is required")
	}

	tokenType, err := mapProtoTokenType(req.GetType())
	if err != nil {
		return nil, err
	}

	expiresAt := req.GetExpiresAt().AsTime()
	out, createErr := h.TokenSvc.Create(ctx, token.CreateInput{
		WorkspaceID: req.GetWorkspaceId(),
		Type:        tokenType,
		ExpiresAt:   expiresAt,
	})
	if createErr != nil {
		return nil, status.Errorf(codes.Internal, "create enrollment token: %v", createErr)
	}

	return &controlplanev1.CreateEnrollTokenResponse{
		TokenId:   out.TokenID,
		Token:     out.Token,
		ExpiresAt: timestamppb.New(out.ExpiresAt),
	}, nil
}

func mapProtoTokenType(in controlplanev1.EnrollTokenType) (string, error) {
	switch in {
	case controlplanev1.EnrollTokenType_ENROLL_TOKEN_TYPE_CONNECTOR:
		return token.TypeConnector, nil
	case controlplanev1.EnrollTokenType_ENROLL_TOKEN_TYPE_AGENT:
		return token.TypeAgent, nil
	default:
		return "", status.Error(codes.InvalidArgument, "token type must be CONNECTOR or AGENT")
	}
}
