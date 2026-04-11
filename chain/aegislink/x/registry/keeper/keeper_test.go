package keeper

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/ayushns01/aegislink/chain/aegislink/testutil"
	registrytypes "github.com/ayushns01/aegislink/chain/aegislink/x/registry/types"
)

func TestRegisterAssetRejectsInvalidMetadata(t *testing.T) {
	keeper := NewKeeper()

	err := keeper.RegisterAsset(registrytypes.Asset{
		AssetID:            "",
		SourceChainID:      "ethereum-1",
		SourceAssetKind:    registrytypes.SourceAssetKindERC20,
		SourceAssetAddress: "0xabc123",
		DisplaySymbol:      "USDC",
		Decimals:           6,
		Enabled:            true,
	})
	if !errors.Is(err, registrytypes.ErrInvalidAsset) {
		t.Fatalf("expected invalid asset error, got %v", err)
	}
}

func TestRegisterNativeETHAssetPersistsClassAndDerivedDenom(t *testing.T) {
	keeper := NewKeeper()
	asset := nativeETHAsset()

	if err := keeper.RegisterAsset(asset); err != nil {
		t.Fatalf("expected native asset registration to succeed, got %v", err)
	}

	stored, ok := keeper.GetAsset(asset.AssetID)
	if !ok {
		t.Fatal("expected stored asset to exist")
	}
	if stored.SourceAssetKind != registrytypes.SourceAssetKindNativeETH {
		t.Fatalf("expected native eth class, got %q", stored.SourceAssetKind)
	}
	if stored.SourceAssetAddress != "" {
		t.Fatalf("expected no source address for native ETH, got %q", stored.SourceAssetAddress)
	}
	if stored.DestinationDenom != "ueth" {
		t.Fatalf("expected destination denom ueth, got %q", stored.DestinationDenom)
	}
	if stored.Denom != stored.DestinationDenom {
		t.Fatalf("expected denom %q, got %q", stored.DestinationDenom, stored.Denom)
	}
}

func TestRegisterERC20AssetPersistsAddressAndDerivedDenom(t *testing.T) {
	keeper := NewKeeper()
	asset := erc20Asset()

	if err := keeper.RegisterAsset(asset); err != nil {
		t.Fatalf("expected erc20 registration to succeed, got %v", err)
	}

	stored, ok := keeper.GetAsset(asset.AssetID)
	if !ok {
		t.Fatal("expected stored asset to exist")
	}
	if stored.SourceAssetKind != registrytypes.SourceAssetKindERC20 {
		t.Fatalf("expected erc20 class, got %q", stored.SourceAssetKind)
	}
	if stored.SourceAssetAddress != "0xabc123" {
		t.Fatalf("expected source address to persist, got %q", stored.SourceAssetAddress)
	}
	if stored.DestinationDenom != "uethusdc" {
		t.Fatalf("expected destination denom uethusdc, got %q", stored.DestinationDenom)
	}
}

func TestDestinationDenomIsDerivedDeterministically(t *testing.T) {
	firstKeeper := NewKeeper()
	secondKeeper := NewKeeper()

	first := erc20Asset()
	first.DisplaySymbol = "  USDC  "
	first.SourceAssetAddress = " 0xabc123 "
	if err := firstKeeper.RegisterAsset(first); err != nil {
		t.Fatalf("expected first asset registration to succeed, got %v", err)
	}

	second := erc20Asset()
	second.DisplaySymbol = "USDC"
	second.SourceAssetAddress = "0xabc123"
	if err := secondKeeper.RegisterAsset(second); err != nil {
		t.Fatalf("expected second asset registration to succeed, got %v", err)
	}

	firstStored, _ := firstKeeper.GetAsset(first.AssetID)
	secondStored, _ := secondKeeper.GetAsset(second.AssetID)
	if firstStored.DestinationDenom != secondStored.DestinationDenom {
		t.Fatalf("expected deterministic destination denom, got %q and %q", firstStored.DestinationDenom, secondStored.DestinationDenom)
	}
}

func TestRegisterAssetRejectsDerivedDenomCollisions(t *testing.T) {
	keeper := NewKeeper()

	first := registrytypes.Asset{
		AssetID:            "eth.us-dc",
		SourceChainID:      "ethereum-1",
		SourceAssetKind:    registrytypes.SourceAssetKindERC20,
		SourceAssetAddress: "0xabc123",
		DisplaySymbol:      "US-DC",
		Decimals:           6,
		Enabled:            true,
	}
	second := registrytypes.Asset{
		AssetID:            "eth.us_dc",
		SourceChainID:      "ethereum-1",
		SourceAssetKind:    registrytypes.SourceAssetKindERC20,
		SourceAssetAddress: "0xdef456",
		DisplaySymbol:      "US_DC",
		Decimals:           6,
		Enabled:            true,
	}

	if err := keeper.RegisterAsset(first); err != nil {
		t.Fatalf("expected first registration to succeed, got %v", err)
	}
	if err := keeper.RegisterAsset(second); !errors.Is(err, ErrAssetAlreadyExists) {
		t.Fatalf("expected derived denom collision error, got %v", err)
	}
}

func TestStoreKeeperLoadsLegacyPrefixAssetWithDerivedMetadata(t *testing.T) {
	store, keys := testutil.NewInMemoryCommitMultiStore(t, "registry")

	legacy := registrytypes.Asset{
		AssetID:        "eth.usdc",
		SourceChainID:  "ethereum-sepolia",
		SourceContract: "0xabc123",
		Denom:          "uethusdc",
		Decimals:       6,
		DisplayName:    "Ethereum USDC",
		Enabled:        true,
	}
	raw, err := json.Marshal(legacy)
	if err != nil {
		t.Fatalf("marshal legacy asset: %v", err)
	}
	store.GetKVStore(keys["registry"]).Set([]byte("asset/eth.usdc"), raw)

	keeper, err := NewStoreKeeper(store, keys["registry"])
	if err != nil {
		t.Fatalf("expected store-backed keeper to load legacy asset, got %v", err)
	}

	stored, ok := keeper.GetAsset(legacy.AssetID)
	if !ok {
		t.Fatalf("expected asset %q after reload", legacy.AssetID)
	}
	if stored.DestinationDenom != "uethusdc" {
		t.Fatalf("expected derived destination denom uethusdc, got %q", stored.DestinationDenom)
	}
	if stored.Denom != stored.DestinationDenom {
		t.Fatalf("expected denom %q, got %q", stored.DestinationDenom, stored.Denom)
	}
}

func TestDisableAssetMarksAssetDisabled(t *testing.T) {
	keeper := NewKeeper()
	asset := erc20Asset()

	if err := keeper.RegisterAsset(asset); err != nil {
		t.Fatalf("expected asset registration to succeed, got %v", err)
	}
	if err := keeper.DisableAsset(asset.AssetID); err != nil {
		t.Fatalf("expected disable to succeed, got %v", err)
	}

	stored, ok := keeper.GetAsset(asset.AssetID)
	if !ok {
		t.Fatal("expected stored asset to exist")
	}
	if stored.Enabled {
		t.Fatal("expected stored asset to be disabled")
	}
}

func nativeETHAsset() registrytypes.Asset {
	return registrytypes.Asset{
		AssetID:         "eth.eth",
		SourceChainID:   "ethereum-1",
		SourceAssetKind: registrytypes.SourceAssetKindNativeETH,
		DisplaySymbol:   "ETH",
		Decimals:        18,
		Enabled:         true,
	}
}

func erc20Asset() registrytypes.Asset {
	return registrytypes.Asset{
		AssetID:            "eth.usdc",
		SourceChainID:      "ethereum-1",
		SourceAssetKind:    registrytypes.SourceAssetKindERC20,
		SourceAssetAddress: "0xabc123",
		DisplaySymbol:      "USDC",
		Decimals:           6,
		DisplayName:        "USDC",
		Enabled:            true,
	}
}
