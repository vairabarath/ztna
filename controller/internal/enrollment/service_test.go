package enrollment

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/igris/ztna/controller/internal/device"
	"github.com/igris/ztna/controller/internal/token"
	"github.com/igris/ztna/controller/internal/workspace"
)

func TestEnrollSuccess(t *testing.T) {
	ctx := context.Background()

	encryptor, err := workspace.NewAESGCMEncryptor([]byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatalf("NewAESGCMEncryptor: %v", err)
	}

	workspaceRepo := workspace.NewMemoryRepository()
	workspaceSvc := workspace.NewService(workspaceRepo, encryptor)

	createdWS, err := workspaceSvc.CreateWorkspace(ctx, workspace.CreateWorkspaceInput{DisplayName: "acme"})
	if err != nil {
		t.Fatalf("CreateWorkspace: %v", err)
	}

	tokenRepo := token.NewMemoryRepository()
	tokenSvc := token.NewService(tokenRepo)
	createdToken, err := tokenSvc.Create(ctx, token.CreateInput{
		WorkspaceID: createdWS.WorkspaceID,
		Type:        token.TypeConnector,
		ExpiresAt:   time.Now().UTC().Add(10 * time.Minute),
	})
	if err != nil {
		t.Fatalf("Create token: %v", err)
	}

	deviceRepo := device.NewMemoryRepository()
	svc := NewService(workspaceRepo, tokenSvc, deviceRepo, encryptor, NewLocalSigner(), nil)

	deviceID := "dev-1"
	csrPEM := mustBuildCSR(t, createdWS.WorkspaceID, deviceID)

	out, err := svc.Enroll(ctx, EnrollInput{
		WorkspaceID:    createdWS.WorkspaceID,
		BootstrapToken: createdToken.Token,
		Type:           token.TypeConnector,
		CSRPEM:         csrPEM,
		DeviceID:       deviceID,
	})
	if err != nil {
		t.Fatalf("Enroll: %v", err)
	}
	if out.CertificatePEM == "" || out.CACertPEM == "" || out.CertFingerprint == "" {
		t.Fatalf("expected populated enrollment output")
	}

	stored, err := deviceRepo.GetByID(ctx, createdWS.WorkspaceID, deviceID)
	if err != nil {
		t.Fatalf("GetByID device: %v", err)
	}
	if stored.CertFingerprint != out.CertFingerprint {
		t.Fatalf("fingerprint mismatch: got=%s want=%s", stored.CertFingerprint, out.CertFingerprint)
	}
	if stored.Status != "active" {
		t.Fatalf("expected device status active, got=%s", stored.Status)
	}

	_, err = tokenSvc.Validate(ctx, token.ValidateInput{
		WorkspaceID: createdWS.WorkspaceID,
		Type:        token.TypeConnector,
		RawToken:    createdToken.Token,
	})
	if !errors.Is(err, token.ErrTokenUsed) {
		t.Fatalf("expected ErrTokenUsed after enrollment, got: %v", err)
	}
}

func TestRenewSuccess(t *testing.T) {
	ctx := context.Background()
	encryptor, err := workspace.NewAESGCMEncryptor([]byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatalf("NewAESGCMEncryptor: %v", err)
	}

	workspaceRepo := workspace.NewMemoryRepository()
	workspaceSvc := workspace.NewService(workspaceRepo, encryptor)
	createdWS, err := workspaceSvc.CreateWorkspace(ctx, workspace.CreateWorkspaceInput{DisplayName: "acme"})
	if err != nil {
		t.Fatalf("CreateWorkspace: %v", err)
	}

	tokenRepo := token.NewMemoryRepository()
	tokenSvc := token.NewService(tokenRepo)
	deviceRepo := device.NewMemoryRepository()

	svc := NewService(workspaceRepo, tokenSvc, deviceRepo, encryptor, NewLocalSigner(), nil)
	deviceID := "dev-renew"
	csrPEM := mustBuildCSR(t, createdWS.WorkspaceID, deviceID)

	out, err := svc.Renew(ctx, RenewInput{
		WorkspaceID: createdWS.WorkspaceID,
		CSRPEM:      csrPEM,
		DeviceID:    deviceID,
	})
	if err != nil {
		t.Fatalf("Renew: %v", err)
	}
	if out.CertificatePEM == "" || out.CertFingerprint == "" {
		t.Fatalf("expected populated renewal output")
	}
}

type fakeTxRunner struct {
	called bool
}

func (f *fakeTxRunner) RunInTx(ctx context.Context, fn func(txCtx context.Context) error) error {
	f.called = true
	return fn(ctx)
}

func TestEnrollUsesTxRunnerWhenConfigured(t *testing.T) {
	ctx := context.Background()
	encryptor, err := workspace.NewAESGCMEncryptor([]byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatalf("NewAESGCMEncryptor: %v", err)
	}

	workspaceRepo := workspace.NewMemoryRepository()
	workspaceSvc := workspace.NewService(workspaceRepo, encryptor)
	createdWS, err := workspaceSvc.CreateWorkspace(ctx, workspace.CreateWorkspaceInput{DisplayName: "acme"})
	if err != nil {
		t.Fatalf("CreateWorkspace: %v", err)
	}

	tokenRepo := token.NewMemoryRepository()
	tokenSvc := token.NewService(tokenRepo)
	createdToken, err := tokenSvc.Create(ctx, token.CreateInput{
		WorkspaceID: createdWS.WorkspaceID,
		Type:        token.TypeConnector,
		ExpiresAt:   time.Now().UTC().Add(10 * time.Minute),
	})
	if err != nil {
		t.Fatalf("Create token: %v", err)
	}

	runner := &fakeTxRunner{}
	deviceRepo := device.NewMemoryRepository()
	svc := NewService(workspaceRepo, tokenSvc, deviceRepo, encryptor, NewLocalSigner(), runner)

	_, err = svc.Enroll(ctx, EnrollInput{
		WorkspaceID:    createdWS.WorkspaceID,
		BootstrapToken: createdToken.Token,
		Type:           token.TypeConnector,
		CSRPEM:         mustBuildCSR(t, createdWS.WorkspaceID, "dev-tx"),
		DeviceID:       "dev-tx",
	})
	if err != nil {
		t.Fatalf("Enroll: %v", err)
	}
	if !runner.called {
		t.Fatalf("expected tx runner to be called")
	}
}
