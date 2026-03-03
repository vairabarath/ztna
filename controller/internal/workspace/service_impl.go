package workspace

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/igris/ztna/controller/internal/pki"
)

type Encryptor interface {
	Encrypt(plaintext []byte) ([]byte, error)
}

type service struct {
	repo      Repository
	encryptor Encryptor
}

func NewService(repo Repository, encryptor Encryptor) Service {
	return &service{repo: repo, encryptor: encryptor}
}

func (s *service) CreateWorkspace(ctx context.Context, in CreateWorkspaceInput) (CreateWorkspaceOutput, error) {
	if s.repo == nil {
		return CreateWorkspaceOutput{}, fmt.Errorf("workspace repository is nil")
	}
	if s.encryptor == nil {
		return CreateWorkspaceOutput{}, fmt.Errorf("encryptor is nil")
	}

	workspaceID, err := newWorkspaceID()
	if err != nil {
		return CreateWorkspaceOutput{}, err
	}

	caCertPEM, caKeyPEM, err := pki.NewWorkspaceCA(workspaceID, pki.WorkspaceCALifetime)
	if err != nil {
		return CreateWorkspaceOutput{}, fmt.Errorf("create workspace ca: %w", err)
	}

	encryptedKey, err := s.encryptor.Encrypt([]byte(caKeyPEM))
	if err != nil {
		return CreateWorkspaceOutput{}, fmt.Errorf("encrypt workspace ca key: %w", err)
	}

	ws := Workspace{
		ID:                    workspaceID,
		DisplayName:           in.DisplayName,
		CACertPEM:             caCertPEM,
		CAPrivateKeyEncrypted: encryptedKey,
	}
	if err := s.repo.Insert(ctx, ws); err != nil {
		if errors.Is(err, ErrAlreadyExist) {
			return CreateWorkspaceOutput{}, err
		}
		return CreateWorkspaceOutput{}, fmt.Errorf("insert workspace: %w", err)
	}

	return CreateWorkspaceOutput{
		WorkspaceID: workspaceID,
		CACertPEM:   caCertPEM,
	}, nil
}

func (s *service) GetWorkspaceCA(ctx context.Context, workspaceID string) (string, error) {
	if workspaceID == "" {
		return "", fmt.Errorf("workspace id is required")
	}

	ws, err := s.repo.GetByID(ctx, workspaceID)
	if err != nil {
		return "", err
	}
	return ws.CACertPEM, nil
}

func newWorkspaceID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
