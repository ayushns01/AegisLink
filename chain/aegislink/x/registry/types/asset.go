package types

import (
	"errors"
	"fmt"
	"strings"
)

var ErrInvalidAsset = errors.New("invalid asset")

type SourceAssetKind string

const (
	SourceAssetKindUnspecified SourceAssetKind = ""
	SourceAssetKindNativeETH   SourceAssetKind = "native_eth"
	SourceAssetKindERC20       SourceAssetKind = "erc20"
)

type Asset struct {
	AssetID            string
	SourceChainID      string
	SourceAssetKind    SourceAssetKind
	SourceAssetAddress string
	SourceContract     string
	Denom              string
	DestinationDenom   string
	Decimals           uint32
	DisplayName        string
	DisplaySymbol      string
	Enabled            bool
}

func (a Asset) ValidateBasic() error {
	if strings.TrimSpace(a.AssetID) == "" {
		return fmt.Errorf("%w: missing asset id", ErrInvalidAsset)
	}
	if strings.TrimSpace(a.SourceChainID) == "" {
		return fmt.Errorf("%w: missing source chain id", ErrInvalidAsset)
	}
	if a.SourceAssetKind != SourceAssetKindNativeETH && a.SourceAssetKind != SourceAssetKindERC20 {
		return fmt.Errorf("%w: invalid source asset kind", ErrInvalidAsset)
	}
	if strings.TrimSpace(a.DisplaySymbol) == "" && strings.TrimSpace(a.DisplayName) == "" {
		return fmt.Errorf("%w: missing display symbol", ErrInvalidAsset)
	}
	if a.SourceAssetKind == SourceAssetKindERC20 {
		if strings.TrimSpace(a.SourceAssetAddress) == "" && strings.TrimSpace(a.SourceContract) == "" {
			return fmt.Errorf("%w: missing source asset address", ErrInvalidAsset)
		}
	}
	if a.Decimals > 18 {
		return fmt.Errorf("%w: decimals must be <= 18", ErrInvalidAsset)
	}
	return nil
}
