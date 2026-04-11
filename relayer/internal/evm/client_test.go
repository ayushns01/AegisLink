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
		SourceChainID:   "11155111",
		SourceContract:  "0xgateway",
		TxHash:          "0xtx-1",
		LogIndex:        3,
		Nonce:           9,
		MessageID:       "message-9",
		DepositID:       "deposit-9",
		SourceAssetKind: bridgetypes.SourceAssetKindERC20,
		AssetAddress:    "0xasset",
		AssetID:         "uusdc",
		Amount:          big.NewInt(42),
		Recipient:       "aegis1recipient",
		Expiry:          88,
	}

	want := bridgetypes.ReplayKey(
		bridgetypes.ClaimKindDeposit,
		event.SourceAssetKind,
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

func TestDepositEventClaimSetsNativeETHSourceAssetKind(t *testing.T) {
	t.Parallel()

	event := DepositEvent{
		SourceChainID:  "11155111",
		SourceContract: "0xgateway",
		TxHash:         "0xtx-native",
		LogIndex:       1,
		Nonce:          2,
		AssetAddress:   "0x0000000000000000000000000000000000000000",
		AssetID:        "eth",
		Amount:         big.NewInt(1_000_000_000_000_000_000),
		Recipient:      "cosmos1native",
		Expiry:         120,
	}

	claim := event.Claim("aegislink-public-1")
	if claim.Identity.SourceAssetKind != bridgetypes.SourceAssetKindNativeETH {
		t.Fatalf("expected native eth source asset kind, got %q", claim.Identity.SourceAssetKind)
	}
	if claim.Identity.SourceContract != event.SourceContract {
		t.Fatalf("expected source contract %q, got %q", event.SourceContract, claim.Identity.SourceContract)
	}
}

func TestDepositEventClaimReplayIdentityDiffersByAssetKind(t *testing.T) {
	t.Parallel()

	base := DepositEvent{
		SourceChainID:  "11155111",
		SourceContract: "0xgateway",
		TxHash:         "0xtx-shared",
		LogIndex:       4,
		Nonce:          8,
		AssetID:        "eth",
		Amount:         big.NewInt(42),
		Recipient:      "cosmos1recipient",
		Expiry:         120,
	}
	erc20 := base
	erc20.SourceAssetKind = bridgetypes.SourceAssetKindERC20
	erc20.AssetAddress = "0x00000000000000000000000000000000000000ff"
	native := base
	native.SourceAssetKind = bridgetypes.SourceAssetKindNativeETH
	native.AssetAddress = "0x0000000000000000000000000000000000000000"

	if erc20.ReplayKey() == native.ReplayKey() {
		t.Fatal("expected replay keys to differ by source asset kind")
	}
	if erc20.Claim("aegislink-public-1").Digest() == native.Claim("aegislink-public-1").Digest() {
		t.Fatal("expected claim digests to differ by source asset kind")
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
