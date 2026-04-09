package keeper

import (
	"math/big"

	limittypes "github.com/ayushns01/aegislink/chain/aegislink/x/limits/types"
)

type StateSnapshot struct {
	Limits []limittypes.RateLimit   `json:"limits"`
	Usage  []limittypes.WindowUsage `json:"usage"`
}

func (k *Keeper) ExportState() StateSnapshot {
	return StateSnapshot{
		Limits: k.ExportLimits(),
		Usage:  k.ExportUsage(),
	}
}

func (k *Keeper) ExportUsage() []limittypes.WindowUsage {
	usage := make([]limittypes.WindowUsage, 0, len(k.usage))
	for _, record := range k.usage {
		usage = append(usage, canonicalUsage(record))
	}
	return usage
}

func (k *Keeper) CurrentUsage(assetID string, atHeight uint64) (limittypes.WindowUsage, bool) {
	record, ok := k.usage[limitKey(assetID)]
	if !ok {
		return limittypes.WindowUsage{}, false
	}
	limit, ok := k.GetLimit(assetID)
	if !ok {
		return limittypes.WindowUsage{}, false
	}
	record = activeUsage(limit, record, atHeight)
	if record.UsedAmount == nil || record.UsedAmount.Sign() == 0 {
		return limittypes.WindowUsage{}, false
	}
	return canonicalUsage(record), true
}

func (k *Keeper) CheckTransferAtHeight(assetID string, amount *big.Int, atHeight uint64) error {
	if amount == nil || amount.Sign() < 0 {
		return ErrInvalidTransfer
	}

	limit, ok := k.GetLimit(assetID)
	if !ok {
		return ErrLimitNotFound
	}

	current := activeUsage(limit, k.usage[limitKey(assetID)], atHeight)
	next := cloneBigInt(current.UsedAmount)
	next.Add(next, amount)
	if next.Cmp(limit.MaxAmount) > 0 {
		return ErrRateLimitExceeded
	}
	return nil
}

func (k *Keeper) RecordTransferAtHeight(assetID string, amount *big.Int, atHeight uint64) error {
	if err := k.CheckTransferAtHeight(assetID, amount, atHeight); err != nil {
		return err
	}

	limit, _ := k.GetLimit(assetID)
	current := activeUsage(limit, k.usage[limitKey(assetID)], atHeight)
	current.AssetID = limit.AssetID
	current.UsedAmount = cloneBigInt(current.UsedAmount)
	current.UsedAmount.Add(current.UsedAmount, amount)
	if current.WindowStart == 0 {
		current.WindowStart = atHeight
	}
	k.usage[limitKey(assetID)] = canonicalUsage(current)
	return k.persist()
}

func (k *Keeper) ImportUsage(usage []limittypes.WindowUsage) error {
	k.usage = make(map[string]limittypes.WindowUsage, len(usage))
	for _, record := range usage {
		if err := record.ValidateBasic(); err != nil {
			return err
		}
		k.usage[limitKey(record.AssetID)] = canonicalUsage(record)
	}
	return k.persist()
}

func activeUsage(limit limittypes.RateLimit, usage limittypes.WindowUsage, atHeight uint64) limittypes.WindowUsage {
	usage = canonicalUsage(usage)
	if usage.AssetID == "" {
		usage.AssetID = limit.AssetID
	}
	if usage.UsedAmount == nil {
		usage.UsedAmount = big.NewInt(0)
	}
	if usage.WindowStart == 0 {
		return usage
	}
	if atHeight >= usage.WindowStart+limit.WindowSeconds {
		return limittypes.WindowUsage{
			AssetID:     limit.AssetID,
			WindowStart: atHeight,
			UsedAmount:  big.NewInt(0),
		}
	}
	return usage
}

func canonicalUsage(usage limittypes.WindowUsage) limittypes.WindowUsage {
	usage.AssetID = limitKey(usage.AssetID)
	usage.UsedAmount = cloneBigInt(usage.UsedAmount)
	return usage
}

func cloneBigInt(value *big.Int) *big.Int {
	if value == nil {
		return big.NewInt(0)
	}
	return new(big.Int).Set(value)
}
