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

	activeSignerSet, err := k.ActiveSignerSet()
	if err != nil {
		return err
	}
	if attestation.SignerSetVersion != activeSignerSet.Version {
		return ErrSignerSetVersionMismatch
	}
	if attestation.Threshold < activeSignerSet.Threshold {
		return ErrInsufficientAttestationQuorum
	}

	verifiedSigners := uint32(0)
	seen := make(map[string]struct{}, len(attestation.Signers))
	allowed := make(map[string]struct{}, len(activeSignerSet.Signers))
	for _, signer := range activeSignerSet.Signers {
		allowed[signer] = struct{}{}
	}
	for _, signer := range attestation.Signers {
		if _, exists := seen[signer]; exists {
			continue
		}
		seen[signer] = struct{}{}

		if _, ok := allowed[signer]; ok {
			verifiedSigners++
		}
	}
	if verifiedSigners < activeSignerSet.Threshold {
		return ErrInsufficientAttestationQuorum
	}

	return nil
}
