package keeper

import (
	"errors"
	"math/big"
	"testing"

	ibcroutertypes "github.com/ayushns01/aegislink/chain/aegislink/x/ibcrouter/types"
	limittypes "github.com/ayushns01/aegislink/chain/aegislink/x/limits/types"
)

func TestGovernanceRejectsUnauthorizedAssetStatusProposal(t *testing.T) {
	t.Parallel()

	governanceKeeper, _, _, _ := seededGovernanceKeeper(t)

	err := governanceKeeper.ApplyAssetStatusProposal("intruder", AssetStatusProposal{
		ProposalID: "asset-disable-unauthorized",
		AssetID:    "eth.usdc",
		Enabled:    false,
	})
	if !errors.Is(err, ErrUnauthorizedProposal) {
		t.Fatalf("expected unauthorized proposal error, got %v", err)
	}
}

func TestGovernanceAcceptsAuthorizedGuardianAndRecordsAppliedBy(t *testing.T) {
	t.Parallel()

	governanceKeeper, registryKeeper, limitsKeeper, routerKeeper := seededGovernanceKeeper(t)

	if err := governanceKeeper.ApplyAssetStatusProposal("guardian-1", AssetStatusProposal{
		ProposalID: "asset-disable-1",
		AssetID:    "eth.usdc",
		Enabled:    false,
	}); err != nil {
		t.Fatalf("apply authorized asset proposal: %v", err)
	}

	if err := governanceKeeper.ApplyLimitUpdateProposal("guardian-1", LimitUpdateProposal{
		ProposalID: "limit-update-1",
		Limit: limittypes.RateLimit{
			AssetID:       "eth.usdc",
			WindowBlocks: 1800,
			MaxAmount:     big.NewInt(900000000),
		},
	}); err != nil {
		t.Fatalf("apply authorized limit proposal: %v", err)
	}

	if err := governanceKeeper.ApplyRoutePolicyUpdateProposal("guardian-1", RoutePolicyUpdateProposal{
		ProposalID: "route-policy-1",
		RouteID:    "osmosis-fast",
		Policy: ibcroutertypes.RoutePolicy{
			AllowedMemoPrefixes: []string{"stake:"},
		},
	}); err != nil {
		t.Fatalf("apply authorized route proposal: %v", err)
	}

	asset, ok := registryKeeper.GetAsset("eth.usdc")
	if !ok || asset.Enabled {
		t.Fatalf("expected asset to be disabled by authorized proposal, got %+v exists=%t", asset, ok)
	}
	limit, ok := limitsKeeper.GetLimit("eth.usdc")
	if !ok || limit.WindowBlocks != 1800 {
		t.Fatalf("expected limit update to be applied, got %+v exists=%t", limit, ok)
	}
	profile, ok := routerKeeper.GetRouteProfile("osmosis-fast")
	if !ok || len(profile.Policy.AllowedMemoPrefixes) != 1 || profile.Policy.AllowedMemoPrefixes[0] != "stake:" {
		t.Fatalf("expected route policy update to be applied, got %+v exists=%t", profile, ok)
	}

	state := governanceKeeper.ExportState()
	if len(state.AppliedProposals) != 3 {
		t.Fatalf("expected three recorded proposals, got %d", len(state.AppliedProposals))
	}
	for _, proposal := range state.AppliedProposals {
		if proposal.AppliedBy != "guardian-1" {
			t.Fatalf("expected applied_by guardian-1, got %+v", proposal)
		}
	}
}
