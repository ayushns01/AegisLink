package keeper

import (
	"errors"
	"sort"
	"strings"

	storetypes "cosmossdk.io/store/types"
	"github.com/ayushns01/aegislink/chain/aegislink/internal/sdkstore"
)

var (
	ErrInvalidFlow = errors.New("invalid flow")
	ErrFlowPaused  = errors.New("flow is paused")
)

type Keeper struct {
	paused      map[string]bool
	prefixStore *sdkstore.JSONPrefixStore
	legacyStore *sdkstore.JSONStateStore
}

const pausedFlowPrefix = "paused"

func NewKeeper() *Keeper {
	return &Keeper{
		paused: make(map[string]bool),
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

	if prefixStore.HasAny(pausedFlowPrefix) {
		if err := keeper.loadFromPrefixStore(); err != nil {
			return nil, err
		}
		return keeper, nil
	}
	if stateStore.HasState() {
		var pausedFlows []string
		if err := stateStore.Load(&pausedFlows); err != nil {
			return nil, err
		}
		if err := keeper.ImportPausedFlows(pausedFlows); err != nil {
			return nil, err
		}
	}

	return keeper, nil
}

func (k *Keeper) Pause(flow string) error {
	key, err := flowKey(flow)
	if err != nil {
		return err
	}
	k.paused[key] = true
	return k.persist()
}

func (k *Keeper) Unpause(flow string) error {
	key, err := flowKey(flow)
	if err != nil {
		return err
	}
	delete(k.paused, key)
	return k.persist()
}

func (k *Keeper) IsPaused(flow string) bool {
	key, err := flowKey(flow)
	if err != nil {
		return false
	}
	return k.paused[key]
}

func (k *Keeper) AssertNotPaused(flow string) error {
	if k.IsPaused(flow) {
		return ErrFlowPaused
	}
	return nil
}

func (k *Keeper) ExportPausedFlows() []string {
	flows := make([]string, 0, len(k.paused))
	for flow := range k.paused {
		flows = append(flows, flow)
	}
	sort.Strings(flows)
	return flows
}

func (k *Keeper) ImportPausedFlows(flows []string) error {
	k.paused = make(map[string]bool, len(flows))
	for _, flow := range flows {
		key, err := flowKey(flow)
		if err != nil {
			return err
		}
		k.paused[key] = true
	}
	return k.persist()
}

func (k *Keeper) persist() error {
	if k.prefixStore == nil {
		return nil
	}
	if err := k.prefixStore.ClearPrefix(pausedFlowPrefix); err != nil {
		return err
	}
	for _, flow := range k.ExportPausedFlows() {
		if err := k.prefixStore.Save(pausedFlowPrefix, flow, flow); err != nil {
			return err
		}
	}
	return k.prefixStore.Commit()
}

func (k *Keeper) Flush() error {
	return k.persist()
}

func flowKey(flow string) (string, error) {
	key := strings.TrimSpace(flow)
	if key == "" {
		return "", ErrInvalidFlow
	}
	return key, nil
}

func (k *Keeper) loadFromPrefixStore() error {
	k.paused = make(map[string]bool)
	return k.prefixStore.LoadAll(pausedFlowPrefix, func() any {
		value := ""
		return &value
	}, func(_ string, value any) error {
		key, err := flowKey(*(value.(*string)))
		if err != nil {
			return err
		}
		k.paused[key] = true
		return nil
	})
}
