package keeper

import (
	"testing"

	"github.com/ayushns01/aegislink/chain/aegislink/testutil"
	registrytypes "github.com/ayushns01/aegislink/chain/aegislink/x/registry/types"
)

func TestSDKKeeperPersistsAssetsAcrossReload(t *testing.T) {
	store, keys := testutil.NewInMemoryCommitMultiStore(t, "registry")

	keeper, err := NewStoreKeeper(store, keys["registry"])
	if err != nil {
		t.Fatalf("expected store-backed keeper to initialize, got %v", err)
	}

	asset := registrytypes.Asset{
		AssetID:        "eth.usdc",
		SourceChainID:  "ethereum-sepolia",
		SourceContract: "0xabc123",
		Denom:          "uethusdc",
		Decimals:       6,
		DisplayName:    "Ethereum USDC",
		Enabled:        true,
	}
	if err := keeper.RegisterAsset(asset); err != nil {
		t.Fatalf("expected asset registration to succeed, got %v", err)
	}

	reloaded, err := NewStoreKeeper(store, keys["registry"])
	if err != nil {
		t.Fatalf("expected store-backed keeper reload to succeed, got %v", err)
	}

	stored, ok := reloaded.GetAsset(asset.AssetID)
	if !ok {
		t.Fatalf("expected asset %q after reload", asset.AssetID)
	}
	if stored.Denom != asset.Denom {
		t.Fatalf("expected denom %q, got %q", asset.Denom, stored.Denom)
	}
}
