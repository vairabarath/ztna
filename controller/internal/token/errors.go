package token

import "errors"

var (
	ErrTokenAlreadyExists = errors.New("enrollment token already exists")
	ErrTokenNotFound      = errors.New("enrollment token not found")
	ErrTokenUsed          = errors.New("enrollment token already used")
	ErrTokenExpired       = errors.New("enrollment token expired")
	ErrTokenType          = errors.New("enrollment token type mismatch")
)
