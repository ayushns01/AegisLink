package types

import (
	"errors"
	"testing"
)

func TestAssetValidateBasic(t *testing.T) {
	asset := Asset{
		AssetID:        "eth.usdc",
		SourceChainID:  "ethereum-1",
		SourceContract: "0xabc123",
		Denom:          "uethusdc",
		Decimals:       6,
	}

	if err := asset.ValidateBasic(); err != nil {
		t.Fatalf("expected valid asset, got error: %v", err)
	}

	cases := map[string]func(*Asset){
		"missing asset id":       func(a *Asset) { a.AssetID = "" },
		"missing source chain id": func(a *Asset) { a.SourceChainID = "" },
		"missing source contract": func(a *Asset) { a.SourceContract = "" },
		"missing denom":          func(a *Asset) { a.Denom = "" },
		"too many decimals":      func(a *Asset) { a.Decimals = 19 },
	}

	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			asset := asset
			mutate(&asset)
			if err := asset.ValidateBasic(); !errors.Is(err, ErrInvalidAsset) {
				t.Fatalf("expected invalid asset error, got: %v", err)
			}
		})
	}
}
