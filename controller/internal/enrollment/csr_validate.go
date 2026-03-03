package enrollment

import (
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
)

var (
	ErrCSRInvalid           = errors.New("invalid csr")
	ErrCSRSignatureInvalid  = errors.New("csr signature invalid")
	ErrCSRWorkspaceMismatch = errors.New("csr workspace does not match request workspace")
	ErrCSRDeviceMismatch    = errors.New("csr device id does not match request device id")
	ErrCSRMissingSAN        = errors.New("csr missing required san entries")
)

func ValidateCSR(csrPEM, workspaceID, deviceID string) (*x509.CertificateRequest, error) {
	if csrPEM == "" || workspaceID == "" || deviceID == "" {
		return nil, ErrCSRInvalid
	}

	block, _ := pem.Decode([]byte(csrPEM))
	if block == nil {
		return nil, ErrCSRInvalid
	}
	csr, err := x509.ParseCertificateRequest(block.Bytes)
	if err != nil {
		return nil, ErrCSRInvalid
	}
	if err := csr.CheckSignature(); err != nil {
		return nil, ErrCSRSignatureInvalid
	}

	if csr.Subject.CommonName != deviceID {
		return nil, ErrCSRDeviceMismatch
	}
	if !containsString(csr.Subject.Organization, workspaceID) {
		return nil, ErrCSRWorkspaceMismatch
	}

	expectedURI := fmt.Sprintf("ztna://%s/%s", workspaceID, deviceID)
	expectedDNS := fmt.Sprintf("%s.%s", deviceID, workspaceID)
	if !hasURI(csr, expectedURI) || !hasDNS(csr, expectedDNS) {
		return nil, ErrCSRMissingSAN
	}

	return csr, nil
}

func hasURI(csr *x509.CertificateRequest, expected string) bool {
	for _, uri := range csr.URIs {
		if uri.String() == expected {
			return true
		}
	}
	return false
}

func hasDNS(csr *x509.CertificateRequest, expected string) bool {
	for _, dns := range csr.DNSNames {
		if dns == expected {
			return true
		}
	}
	return false
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}
