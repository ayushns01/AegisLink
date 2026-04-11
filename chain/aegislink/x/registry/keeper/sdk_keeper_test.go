package keeper

import (
	"testing"

	storetypes "cosmossdk.io/store/types"
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
		AssetID:            "eth.usdc",
		SourceChainID:      "ethereum-sepolia",
		SourceAssetKind:    registrytypes.SourceAssetKindERC20,
		SourceAssetAddress: "0xabc123",
		DisplaySymbol:      "USDC",
		Decimals:           6,
		Enabled:            true,
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
	if stored.DestinationDenom != "uethusdc" {
		t.Fatalf("expected destination denom uethusdc, got %q", stored.DestinationDenom)
	}
	if stored.Denom != stored.DestinationDenom {
		t.Fatalf("expected denom %q, got %q", stored.DestinationDenom, stored.Denom)
	}
}

func TestSDKKeeperStoresAssetsUnderPrefixKeys(t *testing.T) {
	store, keys := testutil.NewInMemoryCommitMultiStore(t, "registry")

	keeper, err := NewStoreKeeper(store, keys["registry"])
	if err != nil {
		t.Fatalf("expected store-backed keeper to initialize, got %v", err)
	}
	if err := keeper.RegisterAsset(registrytypes.Asset{
		AssetID:            "eth.usdc",
		SourceChainID:      "ethereum-sepolia",
		SourceAssetKind:    registrytypes.SourceAssetKindERC20,
		SourceAssetAddress: "0xabc123",
		DisplaySymbol:      "USDC",
		Decimals:           6,
		Enabled:            true,
	}); err != nil {
		t.Fatalf("register asset: %v", err)
	}

	kv := store.GetKVStore(keys["registry"])
	if raw := kv.Get([]byte("state")); len(raw) != 0 {
		t.Fatalf("expected legacy state blob to be absent, got %q", string(raw))
	}
	iter := storetypes.KVStorePrefixIterator(kv, []byte("asset/"))
	defer iter.Close()
	if !iter.Valid() {
		t.Fatal("expected at least one asset/ prefix record")
	}
}
