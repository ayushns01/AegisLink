package types

import (
	"fmt"
	"strings"
)

type Attestation struct {
	MessageID        string
	PayloadHash      string
	Signers          []string
	Proofs           []AttestationProof
	Threshold        uint32
	Expiry           uint64
	SignerSetVersion uint64
}

func (a Attestation) ValidateBasic() error {
	if err := a.validateEnvelope(); err != nil {
		return err
	}
	if len(a.Proofs) == 0 {
		return fmt.Errorf("%w: missing proofs", ErrInvalidAttestation)
	}
	if int(a.Threshold) > len(a.Proofs) {
		return fmt.Errorf("%w: threshold exceeds proof count", ErrInvalidAttestation)
	}
	seen := make(map[string]struct{}, len(a.Proofs))
	for _, proof := range a.Proofs {
		signer := strings.TrimSpace(proof.Signer)
		if signer == "" {
			return fmt.Errorf("%w: empty proof signer", ErrInvalidAttestation)
		}
		if len(proof.Signature) == 0 {
			return fmt.Errorf("%w: missing proof signature", ErrInvalidAttestation)
		}
		if _, ok := seen[signer]; ok {
			return fmt.Errorf("%w: duplicate proof signer %q", ErrInvalidAttestation, signer)
		}
		seen[signer] = struct{}{}
	}
	if len(a.Signers) > 0 {
		seen = make(map[string]struct{}, len(a.Signers))
		for _, signer := range a.Signers {
			signer = strings.TrimSpace(signer)
			if signer == "" {
				return fmt.Errorf("%w: empty signer", ErrInvalidAttestation)
			}
			if _, ok := seen[signer]; ok {
				return fmt.Errorf("%w: duplicate signer %q", ErrInvalidAttestation, signer)
			}
			seen[signer] = struct{}{}
		}
	}
	return nil
}

func (a Attestation) validateEnvelope() error {
	if strings.TrimSpace(a.MessageID) == "" {
		return fmt.Errorf("%w: missing message id", ErrInvalidAttestation)
	}
	if strings.TrimSpace(a.PayloadHash) == "" {
		return fmt.Errorf("%w: missing payload hash", ErrInvalidAttestation)
	}
	if a.Threshold == 0 {
		return fmt.Errorf("%w: missing threshold", ErrInvalidAttestation)
	}
	if a.Expiry == 0 {
		return fmt.Errorf("%w: missing expiry", ErrInvalidAttestation)
	}
	if a.SignerSetVersion == 0 {
		return fmt.Errorf("%w: missing signer set version", ErrInvalidAttestation)
	}
	return nil
}
