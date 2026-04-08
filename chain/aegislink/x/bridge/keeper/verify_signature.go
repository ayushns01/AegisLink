package keeper

import (
	"errors"

	"github.com/decred/dcrd/dcrec/secp256k1/v4/ecdsa"
	bridgetypes "github.com/ayushns01/aegislink/chain/aegislink/x/bridge/types"
)

func verifyProof(proof bridgetypes.AttestationProof, digest []byte) (string, error) {
	if len(proof.Signature) == 0 {
		return "", ErrInvalidAttestationSignature
	}

	publicKey, _, err := ecdsa.RecoverCompact(proof.Signature, digest)
	if err != nil {
		return "", errors.Join(ErrInvalidAttestationSignature, err)
	}
	recovered := bridgetypes.SignerAddressFromPublicKey(publicKey)
	if recovered != bridgetypes.NormalizeSignerAddress(proof.Signer) {
		return "", ErrInvalidAttestationSignature
	}
	return recovered, nil
}
