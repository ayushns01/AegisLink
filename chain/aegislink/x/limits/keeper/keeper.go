package keeper

import (
	"errors"
	"math/big"
	"strings"

	limittypes "github.com/ayushns01/aegislink/chain/aegislink/x/limits/types"
)

var (
	ErrLimitNotFound     = errors.New("rate limit not found")
	ErrInvalidTransfer   = errors.New("invalid transfer amount")
	ErrRateLimitExceeded = errors.New("rate limit exceeded")
)

type Keeper struct {
	limits map[string]limittypes.RateLimit
}

func NewKeeper() *Keeper {
	return &Keeper{
		limits: make(map[string]limittypes.RateLimit),
	}
}

func (k *Keeper) SetLimit(limit limittypes.RateLimit) error {
	if err := limit.ValidateBasic(); err != nil {
		return err
	}

	stored := canonicalLimit(limit)
	k.limits[limitKey(stored.AssetID)] = stored
	return nil
}

func (k *Keeper) GetLimit(assetID string) (limittypes.RateLimit, bool) {
	limit, ok := k.limits[limitKey(assetID)]
	return limit, ok
}

func (k *Keeper) CheckTransfer(assetID string, amount *big.Int) error {
	if amount == nil || amount.Sign() < 0 {
		return ErrInvalidTransfer
	}

	limit, ok := k.GetLimit(assetID)
	if !ok {
		return ErrLimitNotFound
	}
	if amount.Cmp(limit.MaxAmount) > 0 {
		return ErrRateLimitExceeded
	}
	return nil
}

func (k *Keeper) ExportLimits() []limittypes.RateLimit {
	limits := make([]limittypes.RateLimit, 0, len(k.limits))
	for _, limit := range k.limits {
		limits = append(limits, canonicalLimit(limit))
	}
	return limits
}

func (k *Keeper) ImportLimits(limits []limittypes.RateLimit) error {
	k.limits = make(map[string]limittypes.RateLimit, len(limits))
	for _, limit := range limits {
		if err := k.SetLimit(limit); err != nil {
			return err
		}
	}
	return nil
}

func limitKey(assetID string) string {
	return strings.TrimSpace(assetID)
}

func canonicalLimit(limit limittypes.RateLimit) limittypes.RateLimit {
	limit.AssetID = strings.TrimSpace(limit.AssetID)
	if limit.MaxAmount != nil {
		limit.MaxAmount = new(big.Int).Set(limit.MaxAmount)
	}
	return limit
}
