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

	digest, err := attestation.SigningDigest()
	if err != nil {
		return err
	}

	verifiedSigners := uint32(0)
	seen := make(map[string]struct{}, len(attestation.Proofs))
	allowed := make(map[string]struct{}, len(activeSignerSet.Signers))
	for _, signer := range activeSignerSet.Signers {
		allowed[bridgetypes.NormalizeSignerAddress(signer)] = struct{}{}
	}
	for _, proof := range attestation.Proofs {
		recovered, err := verifyProof(proof, digest)
		if err != nil {
			continue
		}
		if _, exists := seen[recovered]; exists {
			continue
		}
		seen[recovered] = struct{}{}

		if _, ok := allowed[recovered]; ok {
			verifiedSigners++
		}
	}
	if verifiedSigners < activeSignerSet.Threshold {
		return ErrInsufficientAttestationQuorum
	}

	return nil
}
