package evm

import (
	"context"
	"errors"
	"math/big"
	"testing"

	bridgetypes "github.com/ayushns01/aegislink/chain/aegislink/x/bridge/types"
)

func TestDepositEventReplayKeyMatchesClaimIdentity(t *testing.T) {
	t.Parallel()

	event := DepositEvent{
		SourceChainID:  "11155111",
		SourceContract: "0xgateway",
		TxHash:         "0xtx-1",
		LogIndex:       3,
		Nonce:          9,
		MessageID:      "message-9",
		DepositID:      "deposit-9",
		AssetAddress:   "0xasset",
		AssetID:        "uusdc",
		Amount:         big.NewInt(42),
		Recipient:      "aegis1recipient",
		Expiry:         88,
	}

	want := bridgetypes.ReplayKey(
		bridgetypes.ClaimKindDeposit,
		event.SourceChainID,
		event.SourceContract,
		event.TxHash,
		event.LogIndex,
		event.Nonce,
	)
	if got := event.ReplayKey(); got != want {
		t.Fatalf("expected replay key %q, got %q", want, got)
	}
}

func TestTemporaryErrorWrapsUnderlyingFailure(t *testing.T) {
	t.Parallel()

	base := errors.New("rpc timeout")
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

func TestReleaserValidatesAndForwardsRequest(t *testing.T) {
	t.Parallel()

	releaser := NewReleaser(&stubReleaseTarget{})
	request := ReleaseRequest{
		MessageID:    "message-9",
		AssetAddress: "0xasset",
		Amount:       big.NewInt(42),
		Recipient:    "0xrecipient",
		Deadline:     88,
		Signature:    []byte("proof"),
	}

	releaseID, err := releaser.ReleaseWithdrawal(context.Background(), request)
	if err != nil {
		t.Fatalf("expected valid release, got error: %v", err)
	}
	if releaseID != "release-1" {
		t.Fatalf("expected release id release-1, got %q", releaseID)
	}

	target := releaser.target.(*stubReleaseTarget)
	if len(target.requests) != 1 {
		t.Fatalf("expected one forwarded request, got %d", len(target.requests))
	}
	if string(target.requests[0].Signature) != string(request.Signature) {
		t.Fatalf("expected forwarded signature %q, got %q", request.Signature, target.requests[0].Signature)
	}
}

func TestReleaseRequestValidateRejectsZeroAddressRecipient(t *testing.T) {
	t.Parallel()

	request := ReleaseRequest{
		MessageID:    "message-9",
		AssetAddress: "0xasset",
		Amount:       big.NewInt(42),
		Recipient:    "0x0000000000000000000000000000000000000000",
		Deadline:     88,
		Signature:    []byte("proof"),
	}

	if err := request.Validate(); err == nil {
		t.Fatalf("expected zero-address recipient to be rejected")
	}
}

type stubReleaseTarget struct {
	requests []ReleaseRequest
	err      error
}

func (s *stubReleaseTarget) ReleaseWithdrawal(_ context.Context, request ReleaseRequest) (string, error) {
	s.requests = append(s.requests, request)
	return "release-1", s.err
}
