package e2e

import (
	"encoding/base64"
	"errors"
	"math/big"
	"testing"

	bridgekeeper "github.com/ayushns01/aegislink/chain/aegislink/x/bridge/keeper"
	limitkeeper "github.com/ayushns01/aegislink/chain/aegislink/x/limits/keeper"
	limittypes "github.com/ayushns01/aegislink/chain/aegislink/x/limits/types"
	pauserkeeper "github.com/ayushns01/aegislink/chain/aegislink/x/pauser/keeper"
	registrykeeper "github.com/ayushns01/aegislink/chain/aegislink/x/registry/keeper"
	registrytypes "github.com/ayushns01/aegislink/chain/aegislink/x/registry/types"
)

func TestInboundRoundTripRelayerClaimIsAcceptedByAegisLink(t *testing.T) {
	t.Parallel()

	fixtures := writeInboundFixtures(t)
	runRelayerOnce(t, fixtures)

	submissions := loadCosmosOutbox(t, fixtures.cosmosOutboxPath)
	if len(submissions) != 1 {
		t.Fatalf("expected one cosmos submission, got %d", len(submissions))
	}

	claim, attestation := decodeSubmission(t, submissions[0])
	server, keeper, _, _, _ := newInboundServer(t, inboundServerOptions{})

	result, err := server.SubmitDepositClaim(claim, attestation)
	if err != nil {
		t.Fatalf("expected inbound claim acceptance, got error: %v", err)
	}
	if result.Status != bridgekeeper.ClaimStatusAccepted {
		t.Fatalf("expected accepted status, got %q", result.Status)
	}
	if result.Denom != "uethusdc" {
		t.Fatalf("expected denom uethusdc, got %q", result.Denom)
	}
	if supply := keeper.SupplyForDenom("uethusdc"); supply.Cmp(claim.Amount) != 0 {
		t.Fatalf("expected supply %s, got %s", claim.Amount, supply)
	}
}

func TestInboundRoundTripRejectsReplaySubmission(t *testing.T) {
	t.Parallel()

	fixtures := writeInboundFixtures(t)
	runRelayerOnce(t, fixtures)

	submissions := loadCosmosOutbox(t, fixtures.cosmosOutboxPath)
	claim, attestation := decodeSubmission(t, submissions[0])
	server, _, _, _, _ := newInboundServer(t, inboundServerOptions{})

	if _, err := server.SubmitDepositClaim(claim, attestation); err != nil {
		t.Fatalf("expected first submission to succeed, got %v", err)
	}
	if _, err := server.SubmitDepositClaim(claim, attestation); !errors.Is(err, bridgekeeper.ErrDuplicateClaim) {
		t.Fatalf("expected duplicate claim error, got %v", err)
	}
}

func TestInboundRoundTripRejectsPausedAsset(t *testing.T) {
	t.Parallel()

	fixtures := writeInboundFixtures(t)
	runRelayerOnce(t, fixtures)

	submissions := loadCosmosOutbox(t, fixtures.cosmosOutboxPath)
	claim, attestation := decodeSubmission(t, submissions[0])
	server, _, _, _, _ := newInboundServer(t, inboundServerOptions{paused: true})

	if _, err := server.SubmitDepositClaim(claim, attestation); !errors.Is(err, bridgekeeper.ErrAssetPaused) {
		t.Fatalf("expected paused asset error, got %v", err)
	}
}

func TestInboundRoundTripRejectsDisabledAsset(t *testing.T) {
	t.Parallel()

	fixtures := writeInboundFixtures(t)
	runRelayerOnce(t, fixtures)

	submissions := loadCosmosOutbox(t, fixtures.cosmosOutboxPath)
	claim, attestation := decodeSubmission(t, submissions[0])
	server, _, _, _, _ := newInboundServer(t, inboundServerOptions{disableAsset: true})

	if _, err := server.SubmitDepositClaim(claim, attestation); !errors.Is(err, bridgekeeper.ErrAssetDisabled) {
		t.Fatalf("expected disabled asset error, got %v", err)
	}
}

func TestFullBridgeLoopBurnsSupplyAndProducesEthereumRelease(t *testing.T) {
	t.Parallel()

	fixtures := writeInboundFixtures(t)
	runRelayerOnce(t, fixtures)

	submissions := loadCosmosOutbox(t, fixtures.cosmosOutboxPath)
	if len(submissions) != 1 {
		t.Fatalf("expected one cosmos submission, got %d", len(submissions))
	}
	claim, attestation := decodeSubmission(t, submissions[0])
	server, keeper, _, _, _ := newInboundServer(t, inboundServerOptions{})

	if _, err := server.SubmitDepositClaim(claim, attestation); err != nil {
		t.Fatalf("expected inbound claim acceptance, got %v", err)
	}

	keeper.SetCurrentHeight(60)
	withdrawal, err := keeper.ExecuteWithdrawal(claim.AssetID, claim.Amount, "0xrecipient", 120, []byte("threshold-proof"))
	if err != nil {
		t.Fatalf("expected withdrawal execution, got %v", err)
	}
	if supply := keeper.SupplyForDenom("uethusdc"); supply.Sign() != 0 {
		t.Fatalf("expected burned supply to reach zero, got %s", supply.String())
	}

	writeJSON(t, fixtures.cosmosStatePath, persistedWithdrawalState{
		LatestHeight: 61,
		Withdrawals: []persistedWithdrawal{
			{
				BlockHeight:    withdrawal.BlockHeight,
				Kind:           string(withdrawal.Identity.Kind),
				SourceChainID:  withdrawal.Identity.SourceChainID,
				SourceContract: withdrawal.Identity.SourceContract,
				SourceTxHash:   withdrawal.Identity.SourceTxHash,
				SourceLogIndex: withdrawal.Identity.SourceLogIndex,
				Nonce:          withdrawal.Identity.Nonce,
				MessageID:      withdrawal.Identity.MessageID,
				AssetID:        withdrawal.AssetID,
				AssetAddress:   withdrawal.AssetAddress,
				Amount:         withdrawal.Amount.String(),
				Recipient:      withdrawal.Recipient,
				Deadline:       withdrawal.Deadline,
				Signature:      base64.StdEncoding.EncodeToString(withdrawal.Signature),
			},
		},
	})

	runRelayerOnce(t, fixtures)

	requests := loadEVMOutbox(t, fixtures.evmOutboxPath)
	if len(requests) != 1 {
		t.Fatalf("expected one ethereum release request, got %d", len(requests))
	}
	if requests[0].MessageID != withdrawal.Identity.MessageID {
		t.Fatalf("expected release message id %q, got %q", withdrawal.Identity.MessageID, requests[0].MessageID)
	}
	if requests[0].AssetAddress != "0xasset" {
		t.Fatalf("expected release asset address 0xasset, got %q", requests[0].AssetAddress)
	}
	if requests[0].Amount != claim.Amount.String() {
		t.Fatalf("expected release amount %s, got %s", claim.Amount.String(), requests[0].Amount)
	}
	if requests[0].Recipient != "0xrecipient" {
		t.Fatalf("expected release recipient 0xrecipient, got %q", requests[0].Recipient)
	}
}

type inboundServerOptions struct {
	disableAsset bool
	paused       bool
}

func newInboundServer(t *testing.T, opts inboundServerOptions) (bridgekeeper.MsgServer, *bridgekeeper.Keeper, *registrykeeper.Keeper, *limitkeeper.Keeper, *pauserkeeper.Keeper) {
	t.Helper()

	registry := registrykeeper.NewKeeper()
	limits := limitkeeper.NewKeeper()
	pauser := pauserkeeper.NewKeeper()

	enabled := true
	if opts.disableAsset {
		enabled = false
	}

	if err := registry.RegisterAsset(registrytypes.Asset{
		AssetID:        "eth.usdc",
		SourceChainID:  "11155111",
		SourceContract: "0xasset",
		Denom:          "uethusdc",
		Decimals:       6,
		DisplayName:    "USDC",
		Enabled:        enabled,
	}); err != nil {
		t.Fatalf("register asset: %v", err)
	}

	if err := limits.SetLimit(limittypes.RateLimit{
		AssetID:       "eth.usdc",
		WindowSeconds: 600,
		MaxAmount:     mustAmount(t, "1000000000000000000"),
	}); err != nil {
		t.Fatalf("set limit: %v", err)
	}

	if opts.paused {
		if err := pauser.Pause("eth.usdc"); err != nil {
			t.Fatalf("pause asset: %v", err)
		}
	}

	keeper := bridgekeeper.NewKeeper(registry, limits, pauser, []string{"relayer-1", "relayer-2", "relayer-3"}, 2)
	keeper.SetCurrentHeight(50)
	return bridgekeeper.NewMsgServer(keeper), keeper, registry, limits, pauser
}

func mustAmount(t *testing.T, value string) *big.Int {
	t.Helper()

	amount, ok := new(big.Int).SetString(value, 10)
	if !ok {
		t.Fatalf("invalid amount %q", value)
	}
	return amount
}
