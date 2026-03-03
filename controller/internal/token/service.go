package token

import (
	"context"
	"time"
)

type Service interface {
	Create(ctx context.Context, in CreateInput) (CreateOutput, error)
	Validate(ctx context.Context, in ValidateInput) (ValidateOutput, error)
	MarkUsed(ctx context.Context, tokenID string) error
}

const (
	TypeConnector = "connector"
	TypeAgent     = "agent"
)

type CreateInput struct {
	WorkspaceID string
	Type        string
	ExpiresAt   time.Time
}

type CreateOutput struct {
	TokenID   string
	Token     string
	ExpiresAt time.Time
}

type ValidateInput struct {
	WorkspaceID string
	Type        string
	RawToken    string
}

type ValidateOutput struct {
	TokenID string
}
