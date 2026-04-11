package types

import (
	"errors"
	"testing"
)

func TestAssetValidateBasic(t *testing.T) {
	erc20 := Asset{
		AssetID:            "eth.usdc",
		SourceChainID:      "ethereum-1",
		SourceAssetKind:    SourceAssetKindERC20,
		SourceAssetAddress: "0xabc123",
		DisplaySymbol:      "USDC",
		Decimals:           6,
	}

	if err := erc20.ValidateBasic(); err != nil {
		t.Fatalf("expected valid erc20 asset, got error: %v", err)
	}

	nativeETH := Asset{
		AssetID:         "eth.eth",
		SourceChainID:   "ethereum-1",
		SourceAssetKind: SourceAssetKindNativeETH,
		DisplaySymbol:   "ETH",
		Decimals:        18,
	}

	if err := nativeETH.ValidateBasic(); err != nil {
		t.Fatalf("expected valid native asset, got error: %v", err)
	}

	cases := map[string]func(*Asset){
		"missing asset id":          func(a *Asset) { a.AssetID = "" },
		"missing source chain id":   func(a *Asset) { a.SourceChainID = "" },
		"missing source asset kind": func(a *Asset) { a.SourceAssetKind = SourceAssetKindUnspecified },
		"missing display symbol":    func(a *Asset) { a.DisplaySymbol = "" },
		"missing erc20 source address": func(a *Asset) {
			a.SourceAssetKind = SourceAssetKindERC20
			a.SourceAssetAddress = ""
			a.SourceContract = ""
		},
		"too many decimals": func(a *Asset) { a.Decimals = 19 },
	}

	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			asset := erc20
			mutate(&asset)
			if err := asset.ValidateBasic(); !errors.Is(err, ErrInvalidAsset) {
				t.Fatalf("expected invalid asset error, got: %v", err)
			}
		})
	}
}
