package keeper

import (
	"errors"
	"testing"

	bridgetypes "github.com/ayushns01/aegislink/chain/aegislink/x/bridge/types"
)

func TestSignerSetAcceptsActiveVersionAtCurrentHeight(t *testing.T) {
	t.Parallel()

	keeper, claim, _, _, _, _ := newKeeperFixture(t)
	signers := bridgetypes.DefaultHarnessSignerAddresses()
	if err := keeper.UpsertSignerSet(SignerSet{
		Version:     2,
		Signers:     signers[3:6],
		Threshold:   2,
		ActivatedAt: 80,
	}); err != nil {
		t.Fatalf("expected signer set registration to succeed, got %v", err)
	}

	keeper.SetCurrentHeight(90)
	claim.Identity.Nonce = 9
	claim.Identity.SourceLogIndex = 9
	claim.Identity.SourceTxHash = "0xphase7-active"
	claim.Identity.MessageID = claim.Identity.DerivedMessageID()
	attestation := validAttestation(claim)
	attestation.SignerSetVersion = 2
	attestation.Signers = signers[3:5]
	attestation.Proofs = signAttestationForTestsFromHelpers(attestation, 3, 4)

	if _, err := keeper.ExecuteDepositClaim(claim, attestation); err != nil {
		t.Fatalf("expected active signer set to verify, got %v", err)
	}
}

func TestSignerSetRejectsVersionBeforeActivation(t *testing.T) {
	t.Parallel()

	keeper, claim, _, _, _, _ := newKeeperFixture(t)
	signers := bridgetypes.DefaultHarnessSignerAddresses()
	if err := keeper.UpsertSignerSet(SignerSet{
		Version:     2,
		Signers:     signers[3:6],
		Threshold:   2,
		ActivatedAt: 80,
	}); err != nil {
		t.Fatalf("expected signer set registration to succeed, got %v", err)
	}

	keeper.SetCurrentHeight(60)
	attestation := validAttestation(claim)
	attestation.SignerSetVersion = 2
	attestation.Signers = signers[3:5]
	attestation.Proofs = signAttestationForTestsFromHelpers(attestation, 3, 4)

	_, err := keeper.ExecuteDepositClaim(claim, attestation)
	if !errors.Is(err, ErrSignerSetVersionMismatch) {
		t.Fatalf("expected signer set mismatch error, got %v", err)
	}
}

func TestSignerSetRejectsExpiredSet(t *testing.T) {
	t.Parallel()

	keeper, claim, _, _, _, _ := newKeeperFixture(t)
	signers := bridgetypes.DefaultHarnessSignerAddresses()
	if err := keeper.UpsertSignerSet(SignerSet{
		Version:     1,
		Signers:     signers[:3],
		Threshold:   2,
		ActivatedAt: 1,
		ExpiresAt:   55,
	}); err != nil {
		t.Fatalf("expected signer set update to succeed, got %v", err)
	}

	keeper.SetCurrentHeight(56)
	attestation := validAttestation(claim)
	attestation.SignerSetVersion = 1

	_, err := keeper.ExecuteDepositClaim(claim, attestation)
	if !errors.Is(err, ErrSignerSetInactive) {
		t.Fatalf("expected signer set inactive error, got %v", err)
	}
}

func TestSignerSetRejectsMismatchAgainstActiveVersion(t *testing.T) {
	t.Parallel()

	keeper, claim, _, _, _, _ := newKeeperFixture(t)
	signers := bridgetypes.DefaultHarnessSignerAddresses()
	if err := keeper.UpsertSignerSet(SignerSet{
		Version:     2,
		Signers:     signers[3:6],
		Threshold:   2,
		ActivatedAt: 80,
	}); err != nil {
		t.Fatalf("expected signer set registration to succeed, got %v", err)
	}

	keeper.SetCurrentHeight(90)
	attestation := validAttestation(claim)
	attestation.SignerSetVersion = 1

	_, err := keeper.ExecuteDepositClaim(claim, attestation)
	if !errors.Is(err, ErrSignerSetVersionMismatch) {
		t.Fatalf("expected signer set mismatch error, got %v", err)
	}
}
