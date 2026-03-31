package types

import (
	"fmt"
	"strings"
)

type Attestation struct {
	MessageID   string
	PayloadHash string
	Signers     []string
	Threshold   uint32
	Expiry      uint64
}

func (a Attestation) ValidateBasic() error {
	if strings.TrimSpace(a.MessageID) == "" {
		return fmt.Errorf("%w: missing message id", ErrInvalidAttestation)
	}
	if strings.TrimSpace(a.PayloadHash) == "" {
		return fmt.Errorf("%w: missing payload hash", ErrInvalidAttestation)
	}
	if len(a.Signers) == 0 {
		return fmt.Errorf("%w: missing signers", ErrInvalidAttestation)
	}
	if a.Threshold == 0 {
		return fmt.Errorf("%w: missing threshold", ErrInvalidAttestation)
	}
	if int(a.Threshold) > len(a.Signers) {
		return fmt.Errorf("%w: threshold exceeds signer count", ErrInvalidAttestation)
	}
	if a.Expiry == 0 {
		return fmt.Errorf("%w: missing expiry", ErrInvalidAttestation)
	}
	seen := make(map[string]struct{}, len(a.Signers))
	for _, signer := range a.Signers {
		if strings.TrimSpace(signer) == "" {
			return fmt.Errorf("%w: empty signer", ErrInvalidAttestation)
		}
		if _, ok := seen[signer]; ok {
			return fmt.Errorf("%w: duplicate signer %q", ErrInvalidAttestation, signer)
		}
		seen[signer] = struct{}{}
	}
	return nil
}
