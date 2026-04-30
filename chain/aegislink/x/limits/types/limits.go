package types

import (
	"errors"
	"fmt"
	"math/big"
	"strings"
)

var ErrInvalidRateLimit = errors.New("invalid rate limit")

type RateLimit struct {
	AssetID string
	// WindowBlocks is the sliding-window length measured in block heights, not
	// wall-clock seconds. A window of 600 means 600 blocks (≈10 min at 1 s/block,
	// ≈60 min at 6 s/block). Operators must account for actual block time when
	// setting this value.
	WindowBlocks uint64
	MaxAmount    *big.Int
}

func (l RateLimit) ValidateBasic() error {
	if strings.TrimSpace(l.AssetID) == "" {
		return fmt.Errorf("%w: missing asset id", ErrInvalidRateLimit)
	}
	if l.WindowBlocks == 0 {
		return fmt.Errorf("%w: missing window blocks", ErrInvalidRateLimit)
	}
	if l.MaxAmount == nil || l.MaxAmount.Sign() <= 0 {
		return fmt.Errorf("%w: missing max amount", ErrInvalidRateLimit)
	}
	return nil
}

type WindowUsage struct {
	AssetID     string
	WindowStart uint64
	UsedAmount  *big.Int
}

func (u WindowUsage) ValidateBasic() error {
	if strings.TrimSpace(u.AssetID) == "" {
		return fmt.Errorf("%w: missing usage asset id", ErrInvalidRateLimit)
	}
	if u.UsedAmount == nil || u.UsedAmount.Sign() < 0 {
		return fmt.Errorf("%w: invalid used amount", ErrInvalidRateLimit)
	}
	return nil
}
