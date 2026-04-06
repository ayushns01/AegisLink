package keeper

import (
	"errors"
	"sort"
	"strings"

	"github.com/ayushns01/aegislink/chain/aegislink/internal/sdkstore"
	storetypes "cosmossdk.io/store/types"
)

var (
	ErrInvalidFlow = errors.New("invalid flow")
	ErrFlowPaused  = errors.New("flow is paused")
)

type Keeper struct {
	paused     map[string]bool
	stateStore *sdkstore.JSONStateStore
}

func NewKeeper() *Keeper {
	return &Keeper{
		paused: make(map[string]bool),
	}
}

func NewStoreKeeper(multiStore storetypes.CommitMultiStore, key *storetypes.KVStoreKey) (*Keeper, error) {
	stateStore, err := sdkstore.NewJSONStateStore(multiStore, key)
	if err != nil {
		return nil, err
	}

	keeper := NewKeeper()
	keeper.stateStore = stateStore

	var pausedFlows []string
	if err := stateStore.Load(&pausedFlows); err != nil {
		return nil, err
	}
	if err := keeper.ImportPausedFlows(pausedFlows); err != nil {
		return nil, err
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
		if err := k.Pause(flow); err != nil {
			return err
		}
	}
	return k.persist()
}

func (k *Keeper) persist() error {
	if k.stateStore == nil {
		return nil
	}
	return k.stateStore.Save(k.ExportPausedFlows())
}

func flowKey(flow string) (string, error) {
	key := strings.TrimSpace(flow)
	if key == "" {
		return "", ErrInvalidFlow
	}
	return key, nil
}
