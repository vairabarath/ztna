package token

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCreateValidateAndMarkUsed(t *testing.T) {
	repo := NewMemoryRepository()
	svc := NewService(repo).(*service)
	now := time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return now }

	created, err := svc.Create(context.Background(), CreateInput{
		WorkspaceID: "ws-1",
		Type:        "connector",
		ExpiresAt:   now.Add(5 * time.Minute),
	})
	if err != nil {
		t.Fatalf("create token: %v", err)
	}
	if created.Token == "" {
		t.Fatalf("token is empty")
	}

	validateOut, err := svc.Validate(context.Background(), ValidateInput{
		WorkspaceID: "ws-1",
		Type:        "connector",
		RawToken:    created.Token,
	})
	if err != nil {
		t.Fatalf("validate token: %v", err)
	}
	if validateOut.TokenID == "" {
		t.Fatalf("expected token id in validate output")
	}

	if err := svc.MarkUsed(context.Background(), created.TokenID); err != nil {
		t.Fatalf("mark used: %v", err)
	}

	_, err = svc.Validate(context.Background(), ValidateInput{
		WorkspaceID: "ws-1",
		Type:        "connector",
		RawToken:    created.Token,
	})
	if !errors.Is(err, ErrTokenUsed) {
		t.Fatalf("expected ErrTokenUsed, got: %v", err)
	}
}

func TestValidateExpiredToken(t *testing.T) {
	repo := NewMemoryRepository()
	svc := NewService(repo).(*service)
	now := time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return now }

	created, err := svc.Create(context.Background(), CreateInput{
		WorkspaceID: "ws-1",
		Type:        "agent",
		ExpiresAt:   now.Add(1 * time.Minute),
	})
	if err != nil {
		t.Fatalf("create token: %v", err)
	}

	svc.now = func() time.Time { return now.Add(2 * time.Minute) }
	_, err = svc.Validate(context.Background(), ValidateInput{
		WorkspaceID: "ws-1",
		Type:        "agent",
		RawToken:    created.Token,
	})
	if !errors.Is(err, ErrTokenExpired) {
		t.Fatalf("expected ErrTokenExpired, got: %v", err)
	}
}
