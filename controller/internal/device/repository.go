package device

import "context"

type Repository interface {
	Upsert(ctx context.Context, d Device) error
	GetByID(ctx context.Context, workspaceID, deviceID string) (Device, error)
	ListByWorkspace(ctx context.Context, workspaceID string) ([]Device, error)
	UpdateStatus(ctx context.Context, workspaceID, deviceID, status string) error
}

type Device struct {
	WorkspaceID      string
	DeviceID         string
	CertFingerprint  string
	Status           string
	LastSeenUnixTime int64
}
