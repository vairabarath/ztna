package workspace

import "context"

type Service interface {
	CreateWorkspace(ctx context.Context, in CreateWorkspaceInput) (CreateWorkspaceOutput, error)
	GetWorkspaceCA(ctx context.Context, workspaceID string) (string, error)
}

type CreateWorkspaceInput struct {
	DisplayName string
}

type CreateWorkspaceOutput struct {
	WorkspaceID string
	CACertPEM   string
}
