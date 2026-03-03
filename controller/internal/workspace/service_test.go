package workspace

import (
	"context"
	"testing"

	"github.com/igris/ztna/controller/internal/pki"
)

type testEncryptor struct{}

func (testEncryptor) Encrypt(plaintext []byte) ([]byte, error) {
	return append([]byte("enc:"), plaintext...), nil
}

func TestCreateWorkspace(t *testing.T) {
	repo := NewMemoryRepository()
	svc := NewService(repo, testEncryptor{})

	out, err := svc.CreateWorkspace(context.Background(), CreateWorkspaceInput{DisplayName: "acme"})
	if err != nil {
		t.Fatalf("CreateWorkspace error: %v", err)
	}
	if out.WorkspaceID == "" {
		t.Fatalf("workspace id is empty")
	}
	if out.CACertPEM == "" {
		t.Fatalf("ca cert pem is empty")
	}

	cert, err := pki.ParseCertificatePEM(out.CACertPEM)
	if err != nil {
		t.Fatalf("parse cert: %v", err)
	}
	if len(cert.Subject.Organization) == 0 || cert.Subject.Organization[0] != out.WorkspaceID {
		t.Fatalf("expected cert organization to include workspace id, got %+v", cert.Subject.Organization)
	}

	stored, err := repo.GetByID(context.Background(), out.WorkspaceID)
	if err != nil {
		t.Fatalf("get stored workspace: %v", err)
	}
	if len(stored.CAPrivateKeyEncrypted) == 0 {
		t.Fatalf("encrypted key is empty")
	}
}
