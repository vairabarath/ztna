package enrollment

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"math/big"
	"time"

	"github.com/igris/ztna/controller/internal/pki"
)

type LocalSigner struct {
	now func() time.Time
}

func NewLocalSigner() *LocalSigner {
	return &LocalSigner{now: time.Now}
}

func (s *LocalSigner) Sign(in SignInput) (string, string, error) {
	block, _ := pem.Decode([]byte(in.CSRPEM))
	if block == nil {
		return "", "", ErrCSRInvalid
	}
	csr, err := x509.ParseCertificateRequest(block.Bytes)
	if err != nil {
		return "", "", ErrCSRInvalid
	}

	caCert, err := pki.ParseCertificatePEM(in.CACertPEM)
	if err != nil {
		return "", "", fmt.Errorf("parse ca cert: %w", err)
	}
	caKey, err := pki.ParsePrivateKeyPEM(in.CAPrivateKeyPEM)
	if err != nil {
		return "", "", fmt.Errorf("parse ca key: %w", err)
	}

	serial, err := randomSerialNumber()
	if err != nil {
		return "", "", err
	}

	notBefore := s.now().UTC().Add(-1 * time.Minute)
	notAfter := in.Profile.NotAfter.UTC()

	tpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      csr.Subject,
		NotBefore:    notBefore,
		NotAfter:     notAfter,
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageClientAuth,
			x509.ExtKeyUsageServerAuth,
		},
		DNSNames:    csr.DNSNames,
		IPAddresses: csr.IPAddresses,
		URIs:        csr.URIs,
	}

	der, err := x509.CreateCertificate(rand.Reader, tpl, caCert, csr.PublicKey, caKey)
	if err != nil {
		return "", "", fmt.Errorf("create certificate: %w", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	fingerprint := sha256.Sum256(der)
	return string(certPEM), hex.EncodeToString(fingerprint[:]), nil
}

func randomSerialNumber() (*big.Int, error) {
	max := new(big.Int).Lsh(big.NewInt(1), 128)
	return rand.Int(rand.Reader, max)
}
