package e2e

import (
	"errors"
	"path/filepath"
	"testing"

	aegisapp "github.com/ayushns01/aegislink/chain/aegislink/app"
	governancekeeper "github.com/ayushns01/aegislink/chain/aegislink/x/governance/keeper"
	registrytypes "github.com/ayushns01/aegislink/chain/aegislink/x/registry/types"
)

func TestGovernanceAuthRequiresAuthorizedGuardianForPolicyChanges(t *testing.T) {
	t.Parallel()

	homeDir := filepath.Join(t.TempDir(), "home")
	cfg, err := aegisapp.InitHome(aegisapp.Config{
		HomeDir:               homeDir,
		ChainID:               "aegislink-governance-1",
		GovernanceAuthorities: []string{"guardian-1"},
	}, false)
	if err != nil {
		t.Fatalf("init home: %v", err)
	}

	app, err := aegisapp.LoadWithConfig(cfg)
	if err != nil {
		t.Fatalf("load runtime: %v", err)
	}
	if err := app.RegisterAsset(sampleRuntimeAsset()); err != nil {
		t.Fatalf("register asset: %v", err)
	}
	if err := app.Save(); err != nil {
		t.Fatalf("save runtime: %v", err)
	}

	service := aegisapp.NewGovernanceTxService(app)
	err = service.ApplyAssetStatusProposal("intruder", governancekeeper.AssetStatusProposal{
		ProposalID: "asset-disable-unauthorized",
		AssetID:    "eth.usdc",
		Enabled:    false,
	})
	if !errors.Is(err, governancekeeper.ErrUnauthorizedProposal) {
		t.Fatalf("expected unauthorized governance proposal error, got %v", err)
	}

	if err := service.ApplyAssetStatusProposal("guardian-1", governancekeeper.AssetStatusProposal{
		ProposalID: "asset-disable-authorized",
		AssetID:    "eth.usdc",
		Enabled:    false,
	}); err != nil {
		t.Fatalf("apply authorized proposal: %v", err)
	}
	if err := app.Save(); err != nil {
		t.Fatalf("save runtime after governance proposal: %v", err)
	}

	reloaded, err := aegisapp.LoadWithConfig(cfg)
	if err != nil {
		t.Fatalf("reload runtime: %v", err)
	}
	asset, ok := reloaded.RegistryKeeper.GetAsset("eth.usdc")
	if !ok || asset.Enabled {
		t.Fatalf("expected asset to be disabled by authorized proposal, got %+v exists=%t", asset, ok)
	}
	proposals := reloaded.GovernanceKeeper.ExportState().AppliedProposals
	if len(proposals) != 1 {
		t.Fatalf("expected one applied proposal, got %d", len(proposals))
	}
	if proposals[0].AppliedBy != "guardian-1" {
		t.Fatalf("expected applied_by guardian-1, got %+v", proposals[0])
	}
}

func sampleRuntimeAsset() registrytypes.Asset {
	return registrytypes.Asset{
		AssetID:        "eth.usdc",
		SourceChainID:  "11155111",
		SourceContract: "0xasset",
		Denom:          "uethusdc",
		Decimals:       6,
		DisplayName:    "USDC",
		Enabled:        true,
	}
}
