package token

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"
)

type service struct {
	repo Repository
	now  func() time.Time
}

func NewService(repo Repository) Service {
	return &service{repo: repo, now: time.Now}
}

func (s *service) Create(ctx context.Context, in CreateInput) (CreateOutput, error) {
	if in.WorkspaceID == "" {
		return CreateOutput{}, fmt.Errorf("workspace id is required")
	}
	if in.Type == "" {
		return CreateOutput{}, fmt.Errorf("token type is required")
	}

	expiresAt := in.ExpiresAt.UTC()
	if expiresAt.IsZero() {
		expiresAt = s.now().UTC().Add(15 * time.Minute)
	}

	raw, err := newRandomToken(32)
	if err != nil {
		return CreateOutput{}, err
	}
	hashed := Hash(raw)
	tokenID, err := newRandomToken(16)
	if err != nil {
		return CreateOutput{}, err
	}

	record := EnrollmentToken{
		ID:          tokenID,
		WorkspaceID: in.WorkspaceID,
		Type:        in.Type,
		TokenHash:   hashed,
		ExpiresAt:   expiresAt,
		Used:        false,
	}
	if err := s.repo.Insert(ctx, record); err != nil {
		return CreateOutput{}, err
	}

	return CreateOutput{
		TokenID:   tokenID,
		Token:     raw,
		ExpiresAt: expiresAt,
	}, nil
}

func (s *service) Validate(ctx context.Context, in ValidateInput) (ValidateOutput, error) {
	if in.WorkspaceID == "" || in.RawToken == "" {
		return ValidateOutput{}, ErrTokenNotFound
	}

	hashed := Hash(in.RawToken)
	record, err := s.repo.GetByWorkspaceAndHash(ctx, in.WorkspaceID, hashed)
	if err != nil {
		return ValidateOutput{}, err
	}

	if !ConstantTimeEqual(record.TokenHash, hashed) {
		return ValidateOutput{}, ErrTokenNotFound
	}
	if record.Type != in.Type {
		return ValidateOutput{}, ErrTokenType
	}
	if record.Used {
		return ValidateOutput{}, ErrTokenUsed
	}
	if s.now().UTC().After(record.ExpiresAt.UTC()) {
		return ValidateOutput{}, ErrTokenExpired
	}

	return ValidateOutput{TokenID: record.ID}, nil
}

func (s *service) MarkUsed(ctx context.Context, tokenID string) error {
	if tokenID == "" {
		return ErrTokenNotFound
	}
	return s.repo.MarkUsed(ctx, tokenID, s.now().UTC())
}

func newRandomToken(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
