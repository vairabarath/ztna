package token

import (
	"context"
	"time"
)

type Repository interface {
	Insert(ctx context.Context, in EnrollmentToken) error
	GetByWorkspaceAndHash(ctx context.Context, workspaceID string, tokenHash []byte) (EnrollmentToken, error)
	MarkUsed(ctx context.Context, tokenID string, usedAt time.Time) error
}

type EnrollmentToken struct {
	ID          string
	WorkspaceID string
	Type        string
	TokenHash   []byte
	ExpiresAt   time.Time
	Used        bool
}
