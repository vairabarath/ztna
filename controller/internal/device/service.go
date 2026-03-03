package device

import "context"

type Service interface {
	SetActiveFingerprint(ctx context.Context, workspaceID, deviceID, fingerprint string) error
	Revoke(ctx context.Context, workspaceID, deviceID, reason string) error
}
