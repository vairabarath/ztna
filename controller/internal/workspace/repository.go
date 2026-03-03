package workspace

import "context"

type Repository interface {
	Insert(ctx context.Context, ws Workspace) error
	GetByID(ctx context.Context, id string) (Workspace, error)
}

type Workspace struct {
	ID                    string
	DisplayName           string
	CACertPEM             string
	CAPrivateKeyEncrypted []byte
}
