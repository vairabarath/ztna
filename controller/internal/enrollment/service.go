package enrollment

import "context"
import "time"

type Service interface {
	Enroll(ctx context.Context, in EnrollInput) (EnrollOutput, error)
	Renew(ctx context.Context, in RenewInput) (RenewOutput, error)
}

type EnrollInput struct {
	WorkspaceID    string
	BootstrapToken string
	Type           string
	CSRPEM         string
	DeviceID       string
	Hostname       string
	Metadata       map[string]string
}

type EnrollOutput struct {
	CertificatePEM  string
	CACertPEM       string
	CertFingerprint string
	ExpiresAt       time.Time
}

type RenewInput struct {
	WorkspaceID string
	CSRPEM      string
	DeviceID    string
}

type RenewOutput struct {
	CertificatePEM  string
	CACertPEM       string
	CertFingerprint string
	ExpiresAt       time.Time
}
