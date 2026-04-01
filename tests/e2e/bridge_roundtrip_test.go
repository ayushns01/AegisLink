package e2e

import (
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
		SourceContract: "0xgateway",
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
