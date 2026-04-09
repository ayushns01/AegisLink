package keeper

import (
	"encoding/json"
	"errors"
	"math/big"
	"strings"

	storetypes "cosmossdk.io/store/types"
	"github.com/ayushns01/aegislink/chain/aegislink/internal/sdkstore"
	limittypes "github.com/ayushns01/aegislink/chain/aegislink/x/limits/types"
)

var (
	ErrLimitNotFound     = errors.New("rate limit not found")
	ErrInvalidTransfer   = errors.New("invalid transfer amount")
	ErrRateLimitExceeded = errors.New("rate limit exceeded")
)

type Keeper struct {
	limits      map[string]limittypes.RateLimit
	usage       map[string]limittypes.WindowUsage
	prefixStore *sdkstore.JSONPrefixStore
	legacyStore *sdkstore.JSONStateStore
}

const (
	limitPrefix = "limit"
	usagePrefix = "usage"
)

func NewKeeper() *Keeper {
	return &Keeper{
		limits: make(map[string]limittypes.RateLimit),
		usage:  make(map[string]limittypes.WindowUsage),
	}
}

func NewStoreKeeper(multiStore storetypes.CommitMultiStore, key *storetypes.KVStoreKey) (*Keeper, error) {
	prefixStore, err := sdkstore.NewJSONPrefixStore(multiStore, key)
	if err != nil {
		return nil, err
	}
	stateStore, err := sdkstore.NewJSONStateStore(multiStore, key)
	if err != nil {
		return nil, err
	}

	keeper := NewKeeper()
	keeper.prefixStore = prefixStore
	keeper.legacyStore = stateStore

	if prefixStore.HasAny(limitPrefix) || prefixStore.HasAny(usagePrefix) {
		if err := keeper.loadFromPrefixStore(); err != nil {
			return nil, err
		}
		return keeper, nil
	}

	var raw json.RawMessage
	if err := stateStore.Load(&raw); err != nil {
		return nil, err
	}
	if len(raw) > 0 {
		switch raw[0] {
		case '[':
			var legacyLimits []limittypes.RateLimit
			if err := json.Unmarshal(raw, &legacyLimits); err != nil {
				return nil, err
			}
			if err := keeper.ImportLimits(legacyLimits); err != nil {
				return nil, err
			}
		default:
			var state StateSnapshot
			if err := json.Unmarshal(raw, &state); err != nil {
				return nil, err
			}
			if err := keeper.ImportState(state); err != nil {
				return nil, err
			}
		}
	}

	return keeper, nil
}

func (k *Keeper) SetLimit(limit limittypes.RateLimit) error {
	if err := limit.ValidateBasic(); err != nil {
		return err
	}

	stored := canonicalLimit(limit)
	k.limits[limitKey(stored.AssetID)] = stored
	return k.persist()
}

func (k *Keeper) GetLimit(assetID string) (limittypes.RateLimit, bool) {
	limit, ok := k.limits[limitKey(assetID)]
	return limit, ok
}

func (k *Keeper) CheckTransfer(assetID string, amount *big.Int) error {
	return k.CheckTransferAtHeight(assetID, amount, 0)
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
		if err := limit.ValidateBasic(); err != nil {
			return err
		}
		stored := canonicalLimit(limit)
		k.limits[limitKey(stored.AssetID)] = stored
	}
	return k.persist()
}

func (k *Keeper) persist() error {
	if k.prefixStore == nil {
		return nil
	}
	if err := k.prefixStore.ClearPrefix(limitPrefix); err != nil {
		return err
	}
	if err := k.prefixStore.ClearPrefix(usagePrefix); err != nil {
		return err
	}
	for key, limit := range k.limits {
		if err := k.prefixStore.Save(limitPrefix, key, canonicalLimit(limit)); err != nil {
			return err
		}
	}
	for key, usage := range k.usage {
		if err := k.prefixStore.Save(usagePrefix, key, canonicalUsage(usage)); err != nil {
			return err
		}
	}
	return k.prefixStore.Commit()
}

func (k *Keeper) Flush() error {
	return k.persist()
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

func (k *Keeper) ImportState(state StateSnapshot) error {
	if err := k.ImportLimits(state.Limits); err != nil {
		return err
	}
	return k.ImportUsage(state.Usage)
}

func (k *Keeper) loadFromPrefixStore() error {
	k.limits = make(map[string]limittypes.RateLimit)
	k.usage = make(map[string]limittypes.WindowUsage)

	if err := k.prefixStore.LoadAll(limitPrefix, func() any {
		return &limittypes.RateLimit{}
	}, func(_ string, value any) error {
		limit := canonicalLimit(*(value.(*limittypes.RateLimit)))
		k.limits[limitKey(limit.AssetID)] = limit
		return nil
	}); err != nil {
		return err
	}

	return k.prefixStore.LoadAll(usagePrefix, func() any {
		return &limittypes.WindowUsage{}
	}, func(_ string, value any) error {
		record := canonicalUsage(*(value.(*limittypes.WindowUsage)))
		k.usage[limitKey(record.AssetID)] = record
		return nil
	})
}
