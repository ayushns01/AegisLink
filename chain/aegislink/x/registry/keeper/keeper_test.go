package keeper

import (
	"errors"
	"testing"

	registrytypes "github.com/ayushns01/aegislink/chain/aegislink/x/registry/types"
)

func TestRegisterAssetRejectsInvalidMetadata(t *testing.T) {
	keeper := NewKeeper()

	err := keeper.RegisterAsset(registrytypes.Asset{
		AssetID:        "",
		SourceChainID:  "ethereum-1",
		SourceContract: "0xabc123",
		Denom:          "uethusdc",
		Decimals:       6,
		Enabled:        true,
	})
	if !errors.Is(err, registrytypes.ErrInvalidAsset) {
		t.Fatalf("expected invalid asset error, got %v", err)
	}
}

func TestRegisterAssetRejectsDuplicates(t *testing.T) {
	keeper := NewKeeper()
	asset := validAsset()

	if err := keeper.RegisterAsset(asset); err != nil {
		t.Fatalf("expected first registration to succeed, got %v", err)
	}

	err := keeper.RegisterAsset(asset)
	if !errors.Is(err, ErrAssetAlreadyExists) {
		t.Fatalf("expected duplicate asset error, got %v", err)
	}
}

func TestDisableAssetMarksAssetDisabled(t *testing.T) {
	keeper := NewKeeper()
	asset := validAsset()

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

func validAsset() registrytypes.Asset {
	return registrytypes.Asset{
		AssetID:        "eth.usdc",
		SourceChainID:  "ethereum-1",
		SourceContract: "0xabc123",
		Denom:          "uethusdc",
		Decimals:       6,
		DisplayName:    "USDC",
		Enabled:        true,
	}
}
