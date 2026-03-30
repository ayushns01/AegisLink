package types

import "errors"

var (
	ErrInvalidClaim       = errors.New("invalid claim")
	ErrInvalidAttestation = errors.New("invalid attestation")
)
