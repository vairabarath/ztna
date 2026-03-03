package enrollment

import "time"

// Profile is the issued certificate profile used by the signer.
type Profile struct {
	WorkspaceID string
	DeviceID    string
	NotAfter    time.Time
}

type SignInput struct {
	CSRPEM          string
	Profile         Profile
	CACertPEM       string
	CAPrivateKeyPEM string
}

// Signer signs CSRs using a workspace-scoped CA.
type Signer interface {
	Sign(in SignInput) (certPEM string, fingerprint string, err error)
}
