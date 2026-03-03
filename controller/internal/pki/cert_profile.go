package pki

import "time"

const (
	WorkspaceCALifetime = 10 * 365 * 24 * time.Hour
	DeviceCertLifetime  = 24 * time.Hour
)
