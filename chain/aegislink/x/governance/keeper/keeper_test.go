package keeper

import (
	"errors"
	"math/big"
	"testing"

	ibcrouterkeeper "github.com/ayushns01/aegislink/chain/aegislink/x/ibcrouter/keeper"
	ibcroutertypes "github.com/ayushns01/aegislink/chain/aegislink/x/ibcrouter/types"
	limitskeeper "github.com/ayushns01/aegislink/chain/aegislink/x/limits/keeper"
	limittypes "github.com/ayushns01/aegislink/chain/aegislink/x/limits/types"
	registrykeeper "github.com/ayushns01/aegislink/chain/aegislink/x/registry/keeper"
	registrytypes "github.com/ayushns01/aegislink/chain/aegislink/x/registry/types"
)

func TestAssetStatusProposalDisablesAndEnablesAsset(t *testing.T) {
	t.Parallel()

	governanceKeeper, registryKeeper, _, _ := seededGovernanceKeeper(t)

	if err := governanceKeeper.ApplyAssetStatusProposal("guardian-1", AssetStatusProposal{
		ProposalID: "asset-disable-1",
		AssetID:    "eth.usdc",
		Enabled:    false,
	}); err != nil {
		t.Fatalf("apply disable asset proposal: %v", err)
	}

	asset, ok := registryKeeper.GetAsset("eth.usdc")
	if !ok {
		t.Fatalf("expected asset to exist after disable")
	}
	if asset.Enabled {
		t.Fatalf("expected asset to be disabled after proposal")
	}

	if err := governanceKeeper.ApplyAssetStatusProposal("guardian-1", AssetStatusProposal{
		ProposalID: "asset-enable-1",
		AssetID:    "eth.usdc",
		Enabled:    true,
	}); err != nil {
		t.Fatalf("apply enable asset proposal: %v", err)
	}

	asset, ok = registryKeeper.GetAsset("eth.usdc")
	if !ok {
		t.Fatalf("expected asset to exist after enable")
	}
	if !asset.Enabled {
		t.Fatalf("expected asset to be enabled after proposal")
	}

	if len(governanceKeeper.ExportState().AppliedProposals) != 2 {
		t.Fatalf("expected two recorded proposals, got %d", len(governanceKeeper.ExportState().AppliedProposals))
	}
}

func TestLimitUpdateProposalAppliesNewLimit(t *testing.T) {
	t.Parallel()

	governanceKeeper, _, limitsKeeper, _ := seededGovernanceKeeper(t)

	if err := governanceKeeper.ApplyLimitUpdateProposal("guardian-1", LimitUpdateProposal{
		ProposalID: "limit-update-1",
		Limit: limittypes.RateLimit{
			AssetID:       "eth.usdc",
			WindowBlocks: 1800,
			MaxAmount:     big.NewInt(900000000),
		},
	}); err != nil {
		t.Fatalf("apply limit update proposal: %v", err)
	}

	limit, ok := limitsKeeper.GetLimit("eth.usdc")
	if !ok {
		t.Fatalf("expected limit to exist after update")
	}
	if limit.WindowBlocks != 1800 {
		t.Fatalf("expected updated window seconds, got %d", limit.WindowBlocks)
	}
	if limit.MaxAmount.Cmp(big.NewInt(900000000)) != 0 {
		t.Fatalf("expected updated max amount 900000000, got %s", limit.MaxAmount.String())
	}
}

func TestRoutePolicyUpdateProposalChangesProfileBehavior(t *testing.T) {
	t.Parallel()

	governanceKeeper, _, _, routerKeeper := seededGovernanceKeeper(t)

	if err := governanceKeeper.ApplyRoutePolicyUpdateProposal("guardian-1", RoutePolicyUpdateProposal{
		ProposalID: "route-policy-1",
		RouteID:    "osmosis-fast",
		Policy: ibcroutertypes.RoutePolicy{
			AllowedMemoPrefixes: []string{"stake:"},
		},
	}); err != nil {
		t.Fatalf("apply route policy update proposal: %v", err)
	}

	_, err := routerKeeper.InitiateTransferWithProfile("osmosis-fast", "eth.usdc", big.NewInt(1000000), "osmo1receiver", 150, "swap:uosmo")
	if !errors.Is(err, ibcrouterkeeper.ErrRouteProfilePolicyViolation) {
		t.Fatalf("expected swap memo to be rejected after policy change, got %v", err)
	}

	record, err := routerKeeper.InitiateTransferWithProfile("osmosis-fast", "eth.usdc", big.NewInt(1000000), "osmo1receiver", 151, "stake:uosmo")
	if err != nil {
		t.Fatalf("expected stake memo to be allowed after policy change, got %v", err)
	}
	if record.DestinationChainID != "osmosis-1" {
		t.Fatalf("expected destination chain osmosis-1, got %q", record.DestinationChainID)
	}
}

func seededGovernanceKeeper(t *testing.T) (*Keeper, *registrykeeper.Keeper, *limitskeeper.Keeper, *ibcrouterkeeper.Keeper) {
	t.Helper()

	registryKeeper := registrykeeper.NewKeeper()
	limitsKeeper := limitskeeper.NewKeeper()
	routerKeeper := ibcrouterkeeper.NewKeeper()

	if err := registryKeeper.RegisterAsset(registrytypes.Asset{
		AssetID:        "eth.usdc",
		SourceChainID:  "ethereum-sepolia",
		SourceContract: "0xabc123",
		Denom:          "uethusdc",
		Decimals:       6,
		DisplayName:    "Ethereum USDC",
		Enabled:        true,
	}); err != nil {
		t.Fatalf("register asset: %v", err)
	}

	if err := limitsKeeper.SetLimit(limittypes.RateLimit{
		AssetID:       "eth.usdc",
		WindowBlocks: 600,
		MaxAmount:     big.NewInt(250000000),
	}); err != nil {
		t.Fatalf("set limit: %v", err)
	}

	if err := routerKeeper.SetRouteProfile(ibcroutertypes.RouteProfile{
		RouteID:            "osmosis-fast",
		DestinationChainID: "osmosis-1",
		ChannelID:          "channel-0",
		Enabled:            true,
		Assets: []ibcroutertypes.AssetRoute{
			{AssetID: "eth.usdc", DestinationDenom: "ibc/uethusdc"},
		},
		Policy: ibcroutertypes.RoutePolicy{
			AllowedMemoPrefixes: []string{"swap:"},
		},
	}); err != nil {
		t.Fatalf("set route profile: %v", err)
	}

	return NewKeeper(registryKeeper, limitsKeeper, routerKeeper, []string{"guardian-1"}), registryKeeper, limitsKeeper, routerKeeper
}
