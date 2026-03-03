package enrollment

import "errors"

var (
	ErrWorkspaceNotFound = errors.New("workspace not found")
	ErrInvalidInput      = errors.New("invalid enrollment input")
)
