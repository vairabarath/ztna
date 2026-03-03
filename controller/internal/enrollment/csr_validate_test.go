package enrollment

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"net/url"
	"testing"
)

func TestValidateCSR(t *testing.T) {
	workspaceID := "ws-test"
	deviceID := "dev-1"

	csrPEM := mustBuildCSR(t, workspaceID, deviceID)
	if _, err := ValidateCSR(csrPEM, workspaceID, deviceID); err != nil {
		t.Fatalf("ValidateCSR error: %v", err)
	}
}

func TestValidateCSRWorkspaceMismatch(t *testing.T) {
	csrPEM := mustBuildCSR(t, "ws-a", "dev-1")
	if _, err := ValidateCSR(csrPEM, "ws-b", "dev-1"); err != ErrCSRWorkspaceMismatch {
		t.Fatalf("expected ErrCSRWorkspaceMismatch, got: %v", err)
	}
}

func TestValidateCSRMissingSAN(t *testing.T) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	req := &x509.CertificateRequest{
		Subject: pkix.Name{
			CommonName:   "dev-1",
			Organization: []string{"ws-test"},
		},
	}

	der, err := x509.CreateCertificateRequest(rand.Reader, req, priv)
	if err != nil {
		t.Fatalf("CreateCertificateRequest: %v", err)
	}
	csrPEM := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: der}))

	if _, err := ValidateCSR(csrPEM, "ws-test", "dev-1"); err != ErrCSRMissingSAN {
		t.Fatalf("expected ErrCSRMissingSAN, got: %v", err)
	}
}

func mustBuildCSR(t *testing.T, workspaceID, deviceID string) string {
	t.Helper()

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	uri, err := url.Parse("ztna://" + workspaceID + "/" + deviceID)
	if err != nil {
		t.Fatalf("Parse URI: %v", err)
	}

	req := &x509.CertificateRequest{
		Subject: pkix.Name{
			CommonName:   deviceID,
			Organization: []string{workspaceID},
		},
		URIs:     []*url.URL{uri},
		DNSNames: []string{deviceID + "." + workspaceID},
	}

	der, err := x509.CreateCertificateRequest(rand.Reader, req, priv)
	if err != nil {
		t.Fatalf("CreateCertificateRequest: %v", err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: der}))
}
