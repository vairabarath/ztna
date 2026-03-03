package pki

import "testing"

func TestNewWorkspaceCA(t *testing.T) {
	certPEM, keyPEM, err := NewWorkspaceCA("ws-test", WorkspaceCALifetime)
	if err != nil {
		t.Fatalf("NewWorkspaceCA error: %v", err)
	}
	if certPEM == "" {
		t.Fatalf("cert PEM is empty")
	}
	if keyPEM == "" {
		t.Fatalf("key PEM is empty")
	}

	cert, err := ParseCertificatePEM(certPEM)
	if err != nil {
		t.Fatalf("ParseCertificatePEM error: %v", err)
	}
	if !cert.IsCA {
		t.Fatalf("expected IsCA=true")
	}
	if len(cert.Subject.Organization) == 0 || cert.Subject.Organization[0] != "ws-test" {
		t.Fatalf("expected org ws-test, got %+v", cert.Subject.Organization)
	}

	if _, err := ParsePrivateKeyPEM(keyPEM); err != nil {
		t.Fatalf("ParsePrivateKeyPEM error: %v", err)
	}
}
