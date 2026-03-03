package revocation

import "context"

type Service interface {
	IsRevoked(ctx context.Context, workspaceID, fingerprint string) (bool, error)
	Revoke(ctx context.Context, in Entry) error
	Subscribe(workspaceID string) (<-chan Entry, func())
}

type Entry struct {
	WorkspaceID      string
	DeviceID         string
	CertFingerprint  string
	Reason           string
	RevokedUnixMilli int64
}
