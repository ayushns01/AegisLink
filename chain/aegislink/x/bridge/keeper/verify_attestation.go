package keeper

import bridgetypes "github.com/ayushns01/aegislink/chain/aegislink/x/bridge/types"

func (k *Keeper) verifyDepositClaim(claim bridgetypes.DepositClaim, attestation bridgetypes.Attestation) error {
	if err := attestation.ValidateBasic(); err != nil {
		return err
	}
	if claim.Identity.MessageID != attestation.MessageID {
		return ErrMessageIDMismatch
	}
	if claim.Digest() != attestation.PayloadHash {
		return ErrPayloadHashMismatch
	}
	if k.currentHeight > claim.Deadline {
		return ErrFinalityWindowExpired
	}
	if k.currentHeight > attestation.Expiry {
		return ErrAttestationExpired
	}
	if attestation.Threshold < k.requiredThreshold {
		return ErrInsufficientAttestationQuorum
	}

	verifiedSigners := uint32(0)
	seen := make(map[string]struct{}, len(attestation.Signers))
	for _, signer := range attestation.Signers {
		if _, exists := seen[signer]; exists {
			continue
		}
		seen[signer] = struct{}{}

		if _, allowed := k.allowedSigners[signer]; allowed {
			verifiedSigners++
		}
	}
	if verifiedSigners < k.requiredThreshold {
		return ErrInsufficientAttestationQuorum
	}

	return nil
}
