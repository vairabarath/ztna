package revocation

import "errors"

var ErrAlreadyRevoked = errors.New("certificate already revoked")
