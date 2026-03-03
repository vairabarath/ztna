package workspace

import "errors"

var (
	ErrNotFound     = errors.New("workspace not found")
	ErrAlreadyExist = errors.New("workspace already exists")
)
