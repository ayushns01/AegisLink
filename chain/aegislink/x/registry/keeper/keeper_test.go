package keeper

import (
	"errors"
	"testing"

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
