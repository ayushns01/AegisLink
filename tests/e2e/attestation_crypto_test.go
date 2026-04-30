package e2e

import (
	"errors"
	"path/filepath"
	"testing"

	aegisapp "github.com/ayushns01/aegislink/chain/aegislink/app"
	bridgekeeper "github.com/ayushns01/aegislink/chain/aegislink/x/bridge/keeper"
	bridgetypes "github.com/ayushns01/aegislink/chain/aegislink/x/bridge/types"
	bridgetestutil "github.com/ayushns01/aegislink/chain/aegislink/x/bridge/types/testutil"
	limittypes "github.com/ayushns01/aegislink/chain/aegislink/x/limits/types"
)

func TestAttestationCryptoRequiresValidSignedActiveSetProofs(t *testing.T) {
	t.Parallel()

	t.Run("accepts valid signed attestation proofs", func(t *testing.T) {
		t.Parallel()

		app := newAttestationRuntime(t)
		claim := sampleAttestationDepositClaim(t, 1)
		attestation := signedAttestationForClaim(t, claim, 0, 1)

		app.SetCurrentHeight(50)
		result, err := app.SubmitDepositClaim(claim, attestation)
		if err != nil {
			t.Fatalf("submit deposit claim with valid signed attestation: %v", err)
		}
		if result.Status != bridgekeeper.ClaimStatusAccepted {
			t.Fatalf("expected accepted claim status, got %+v", result)
		}
	})

	t.Run("rejects signature proofs over the wrong attestation digest", func(t *testing.T) {
		t.Parallel()

		app := newAttestationRuntime(t)
		claim := sampleAttestationDepositClaim(t, 2)
		attestation := signedAttestationForClaim(t, claim, 0, 1)

		wrongDigestEnvelope := attestation
		wrongDigestEnvelope.PayloadHash = "wrong-payload-hash"
		attestation.Proofs = signAttestationWithSignerIndexes(t, wrongDigestEnvelope, 0, 1)

		app.SetCurrentHeight(50)
		_, err := app.SubmitDepositClaim(claim, attestation)
		if !errors.Is(err, bridgekeeper.ErrInsufficientAttestationQuorum) {
			t.Fatalf("expected insufficient quorum for forged signature proofs, got %v", err)
		}
	})
}

func newAttestationRuntime(t *testing.T) *aegisapp.App {
	t.Helper()

	app, err := aegisapp.NewWithConfig(aegisapp.Config{
		AppName:           aegisapp.AppName,
		StatePath:         filepath.Join(t.TempDir(), "aegislink-state.json"),
		AllowedSigners:    bridgetestutil.DefaultHarnessSignerAddresses()[:3],
		RequiredThreshold: 2,
	})
	if err != nil {
		t.Fatalf("new attestation runtime: %v", err)
	}
	if err := app.RegisterAsset(sampleRuntimeAsset()); err != nil {
		t.Fatalf("register runtime asset: %v", err)
	}
	if err := app.SetLimit(limittypes.RateLimit{
		AssetID:       "eth.usdc",
		WindowBlocks: 600,
		MaxAmount:     mustBigAmount(t, "1000000000000000000"),
	}); err != nil {
		t.Fatalf("set runtime limit: %v", err)
	}
	return app
}

func sampleAttestationDepositClaim(t *testing.T, nonce uint64) bridgetypes.DepositClaim {
	t.Helper()

	return depositClaimFromEvent(t, persistedDepositEvent{
		BlockNumber:    10,
		SourceChainID:  "11155111",
		SourceContract: "0xgateway",
		TxHash:         "0xdeposit-tx",
		LogIndex:       nonce,
		Nonce:          nonce,
		DepositID:      "deposit-attestation",
		MessageID:      "unused-event-message",
		AssetAddress:   "0xasset",
		AssetID:        "eth.usdc",
		Amount:         "100000000",
		Recipient:      "cosmos1recipient",
		Expiry:         100,
	})
}

func signedAttestationForClaim(t *testing.T, claim bridgetypes.DepositClaim, signerIndexes ...int) bridgetypes.Attestation {
	t.Helper()

	attestation := bridgetypes.Attestation{
		MessageID:        claim.Identity.MessageID,
		PayloadHash:      claim.Digest(),
		Signers:          bridgetestutil.DefaultHarnessSignerAddresses()[:2],
		Threshold:        2,
		Expiry:           120,
		SignerSetVersion: 1,
	}
	attestation.Proofs = signAttestationWithSignerIndexes(t, attestation, signerIndexes...)
	return attestation
}

func signAttestationWithSignerIndexes(t *testing.T, attestation bridgetypes.Attestation, signerIndexes ...int) []bridgetypes.AttestationProof {
	t.Helper()

	privateKeys := bridgetestutil.DefaultHarnessSignerPrivateKeys()
	proofs := make([]bridgetypes.AttestationProof, 0, len(signerIndexes))
	for _, idx := range signerIndexes {
		proof, err := bridgetypes.SignAttestationWithPrivateKeyHex(attestation, privateKeys[idx])
		if err != nil {
			t.Fatalf("sign attestation with signer index %d: %v", idx, err)
		}
		proofs = append(proofs, proof)
	}
	return proofs
}
