package cosmos

import (
	"context"
	"errors"
	"math/big"
	"testing"

	bridgetypes "github.com/ayushns01/aegislink/chain/aegislink/x/bridge/types"
)

func TestWithdrawalValidateAcceptsCanonicalReleaseShape(t *testing.T) {
	t.Parallel()

	identity := bridgetypes.ClaimIdentity{
		Kind:           bridgetypes.ClaimKindWithdrawal,
		SourceChainID:  "aegislink-1",
		SourceContract: "bridge",
		SourceTxHash:   "0xcosmos-tx",
		SourceLogIndex: 2,
		Nonce:          4,
	}
	identity.MessageID = identity.DerivedMessageID()

	withdrawal := Withdrawal{
		BlockHeight:  14,
		Identity:     identity,
		AssetID:      "uusdc",
		AssetAddress: "0xasset",
		Amount:       big.NewInt(50),
		Recipient:    "0xrecipient",
		Deadline:     77,
		Signature:    []byte("threshold-proof"),
	}

	if err := withdrawal.Validate(); err != nil {
		t.Fatalf("expected valid withdrawal, got error: %v", err)
	}
}

func TestTemporaryErrorWrapsUnderlyingFailure(t *testing.T) {
	t.Parallel()

	base := errors.New("abci unavailable")
	err := TemporaryError{Err: base}

	if !errors.Is(err, base) {
		t.Fatalf("expected wrapped error to match base error")
	}

	var temporary interface{ Temporary() bool }
	if !errors.As(err, &temporary) {
		t.Fatalf("expected temporary marker to be discoverable")
	}
	if !temporary.Temporary() {
		t.Fatalf("expected error to be marked temporary")
	}
}

func TestSubmitterValidatesAndForwardsDepositClaim(t *testing.T) {
	t.Parallel()

	submitter := NewSubmitter(&stubClaimSink{})
	claim := validDepositClaim()
	attestation := validAttestationForClaim(claim)

	if err := submitter.SubmitDepositClaim(context.Background(), claim, attestation); err != nil {
		t.Fatalf("expected valid submission, got error: %v", err)
	}

	sink := submitter.sink.(*stubClaimSink)
	if len(sink.claims) != 1 {
		t.Fatalf("expected one forwarded claim, got %d", len(sink.claims))
	}
	if sink.claims[0].Digest() != claim.Digest() {
		t.Fatalf("expected forwarded claim digest %q, got %q", claim.Digest(), sink.claims[0].Digest())
	}
	if sink.attestations[0].PayloadHash != attestation.PayloadHash {
		t.Fatalf("expected forwarded attestation payload hash %q, got %q", attestation.PayloadHash, sink.attestations[0].PayloadHash)
	}
}

type stubClaimSink struct {
	claims       []bridgetypes.DepositClaim
	attestations []bridgetypes.Attestation
	err          error
}

func (s *stubClaimSink) SubmitDepositClaim(_ context.Context, claim bridgetypes.DepositClaim, attestation bridgetypes.Attestation) error {
	s.claims = append(s.claims, claim)
	s.attestations = append(s.attestations, attestation)
	return s.err
}

func validDepositClaim() bridgetypes.DepositClaim {
	identity := bridgetypes.ClaimIdentity{
		Kind:           bridgetypes.ClaimKindDeposit,
		SourceChainID:  "11155111",
		SourceContract: "0xgateway",
		SourceTxHash:   "0xdeposit-tx",
		SourceLogIndex: 2,
		Nonce:          4,
	}
	identity.MessageID = identity.DerivedMessageID()

	return bridgetypes.DepositClaim{
		Identity:           identity,
		DestinationChainID: "aegislink-1",
		AssetID:            "uusdc",
		Amount:             big.NewInt(50),
		Recipient:          "aegis1recipient",
		Deadline:           77,
	}
}

func validAttestationForClaim(claim bridgetypes.DepositClaim) bridgetypes.Attestation {
	return bridgetypes.Attestation{
		MessageID:   claim.Identity.MessageID,
		PayloadHash: claim.Digest(),
		Signers:     []string{"signer-1", "signer-2"},
		Threshold:   2,
		Expiry:      120,
	}
}
