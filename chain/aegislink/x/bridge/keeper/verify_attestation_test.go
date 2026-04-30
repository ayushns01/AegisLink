package keeper

import (
	"errors"
	"testing"

	bridgetypes "github.com/ayushns01/aegislink/chain/aegislink/x/bridge/types"
	bridgetestutil "github.com/ayushns01/aegislink/chain/aegislink/x/bridge/types/testutil"
)

func TestVerifyAttestationRejectsSignerNamesWithoutSignatureProofs(t *testing.T) {
	t.Parallel()

	keeper, claim, attestation, _, _, _ := newKeeperFixture(t)
	attestation.Proofs = nil

	_, err := keeper.ExecuteDepositClaim(claim, attestation)
	if !errors.Is(err, bridgetypes.ErrInvalidAttestation) {
		t.Fatalf("expected invalid attestation error, got %v", err)
	}
}

func TestVerifyAttestationRejectsSignatureOverWrongPayloadHash(t *testing.T) {
	t.Parallel()

	keeper, claim, attestation, _, _, _ := newKeeperFixture(t)

	wrongPayload := attestation
	wrongPayload.PayloadHash = "wrong-payload-hash"
	attestation.Proofs = signAttestationForTests(t, wrongPayload, 0, 1)

	_, err := keeper.ExecuteDepositClaim(claim, attestation)
	if err == nil {
		t.Fatal("expected wrong-payload signature proof to be rejected")
	}
}

func TestVerifyAttestationRejectsSignerOutsideActiveSignerSet(t *testing.T) {
	t.Parallel()

	keeper, claim, attestation, _, _, _ := newKeeperFixture(t)
	attestation.Proofs = signAttestationForTests(t, attestation, 2, 3)

	_, err := keeper.ExecuteDepositClaim(claim, attestation)
	if !errors.Is(err, ErrInsufficientAttestationQuorum) {
		t.Fatalf("expected insufficient attestation quorum for non-member signer proofs, got %v", err)
	}
}

func TestVerifyAttestationRequiresThresholdValidSignaturesFromActiveSet(t *testing.T) {
	t.Parallel()

	keeper, claim, attestation, _, _, _ := newKeeperFixture(t)
	attestation.Proofs = append(signAttestationForTests(t, attestation, 0), invalidProofForTests())

	_, err := keeper.ExecuteDepositClaim(claim, attestation)
	if !errors.Is(err, ErrInsufficientAttestationQuorum) {
		t.Fatalf("expected insufficient quorum with only one valid proof, got %v", err)
	}

	valid := validAttestation(claim)
	valid.Proofs = signAttestationForTests(t, valid, 0, 1)
	if _, err := keeper.ExecuteDepositClaim(claim, valid); err != nil {
		t.Fatalf("expected threshold-valid signature proofs to verify, got %v", err)
	}
}

func signAttestationForTests(t *testing.T, attestation bridgetypes.Attestation, signerIndexes ...int) []bridgetypes.AttestationProof {
	t.Helper()

	signers := bridgetestutil.DefaultHarnessAttestationSigners()
	proofs := make([]bridgetypes.AttestationProof, 0, len(signerIndexes))
	for _, idx := range signerIndexes {
		proof, err := bridgetypes.SignAttestationWithPrivateKeyHex(attestation, signers[idx].PrivateKeyHex)
		if err != nil {
			t.Fatalf("sign attestation with signer %d: %v", idx, err)
		}
		proofs = append(proofs, proof)
	}
	return proofs
}

func invalidProofForTests() bridgetypes.AttestationProof {
	return bridgetypes.AttestationProof{
		Signer:    bridgetestutil.DefaultHarnessAttestationSigners()[1].Address,
		Signature: []byte("not-a-valid-compact-signature"),
	}
}
