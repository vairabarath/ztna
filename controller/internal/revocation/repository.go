package revocation

import "context"

type Repository interface {
	Insert(ctx context.Context, in Entry) error
	Exists(ctx context.Context, workspaceID, fingerprint string) (bool, error)
}
