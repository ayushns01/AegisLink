package types

import (
	"errors"
	"fmt"
	"strings"
)

var ErrInvalidAsset = errors.New("invalid asset")

type Asset struct {
	AssetID        string
	SourceChainID  string
	SourceContract string
	Denom          string
	Decimals       uint32
	DisplayName    string
	Enabled        bool
}

func (a Asset) ValidateBasic() error {
	if strings.TrimSpace(a.AssetID) == "" {
		return fmt.Errorf("%w: missing asset id", ErrInvalidAsset)
	}
	if strings.TrimSpace(a.SourceChainID) == "" {
		return fmt.Errorf("%w: missing source chain id", ErrInvalidAsset)
	}
	if strings.TrimSpace(a.SourceContract) == "" {
		return fmt.Errorf("%w: missing source contract", ErrInvalidAsset)
	}
	if strings.TrimSpace(a.Denom) == "" {
		return fmt.Errorf("%w: missing denom", ErrInvalidAsset)
	}
	if a.Decimals > 18 {
		return fmt.Errorf("%w: decimals must be <= 18", ErrInvalidAsset)
	}
	return nil
}
