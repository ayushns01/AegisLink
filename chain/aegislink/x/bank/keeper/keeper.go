package keeper

import (
	"errors"
	"math/big"
	"sort"
	"strings"

	storetypes "cosmossdk.io/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/ayushns01/aegislink/chain/aegislink/internal/sdkstore"
)

var (
	ErrInvalidAddress = errors.New("invalid wallet address")
	ErrInvalidDenom   = errors.New("invalid denom")
	ErrInvalidAmount  = errors.New("invalid balance amount")
)

type BalanceRecord struct {
	Address string `json:"address"`
	Denom   string `json:"denom"`
	Amount  string `json:"amount"`
}

type StateSnapshot struct {
	Balances []BalanceRecord `json:"balances"`
}

type Keeper struct {
	balances    map[string]map[string]*big.Int
	prefixStore *sdkstore.JSONPrefixStore
	legacyStore *sdkstore.JSONStateStore
	persistHook func() error
}

const balancePrefix = "balance"

func NewKeeper() *Keeper {
	return &Keeper{
		balances: make(map[string]map[string]*big.Int),
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

	if prefixStore.HasAny(balancePrefix) {
		if err := keeper.loadFromPrefixStore(); err != nil {
			return nil, err
		}
		return keeper, nil
	}
	if stateStore.HasState() {
		var state StateSnapshot
		if err := stateStore.Load(&state); err != nil {
			return nil, err
		}
		if err := keeper.ImportState(state); err != nil {
			return nil, err
		}
	}

	return keeper, nil
}

func (k *Keeper) Credit(address, denom string, amount *big.Int) error {
	normalizedAddress := strings.TrimSpace(address)
	if normalizedAddress == "" {
		return ErrInvalidAddress
	}
	normalizedDenom, err := normalizeDenom(denom)
	if err != nil {
		return err
	}
	normalizedAmount, err := normalizeAmount(amount)
	if err != nil {
		return err
	}

	if _, ok := k.balances[normalizedAddress]; !ok {
		k.balances[normalizedAddress] = make(map[string]*big.Int)
	}
	if _, ok := k.balances[normalizedAddress][normalizedDenom]; !ok {
		k.balances[normalizedAddress][normalizedDenom] = big.NewInt(0)
	}
	k.balances[normalizedAddress][normalizedDenom].Add(k.balances[normalizedAddress][normalizedDenom], normalizedAmount)
	return k.persist()
}

func (k *Keeper) NormalizeAddress(address string) (string, error) {
	return normalizeAddress(address)
}

func (k *Keeper) BalanceOf(address, denom string) *big.Int {
	normalizedAddress := strings.TrimSpace(address)
	normalizedDenom := strings.TrimSpace(denom)
	if normalizedDenom == "" {
		return big.NewInt(0)
	}

	denoms, ok := k.balances[normalizedAddress]
	if !ok {
		return big.NewInt(0)
	}
	amount, ok := denoms[normalizedDenom]
	if !ok {
		return big.NewInt(0)
	}
	return cloneAmount(amount)
}

func (k *Keeper) Balances(address string) ([]BalanceRecord, error) {
	normalizedAddress, err := normalizeAddress(address)
	if err != nil {
		return nil, err
	}

	denoms := k.balances[normalizedAddress]
	balances := make([]BalanceRecord, 0, len(denoms))
	for denom, amount := range denoms {
		balances = append(balances, BalanceRecord{
			Address: normalizedAddress,
			Denom:   denom,
			Amount:  amount.String(),
		})
	}
	sort.Slice(balances, func(i, j int) bool {
		return balances[i].Denom < balances[j].Denom
	})
	return balances, nil
}

func (k *Keeper) ExportState() StateSnapshot {
	addresses := make([]string, 0, len(k.balances))
	for address := range k.balances {
		addresses = append(addresses, address)
	}
	sort.Strings(addresses)

	state := StateSnapshot{}
	for _, address := range addresses {
		denoms := make([]string, 0, len(k.balances[address]))
		for denom := range k.balances[address] {
			denoms = append(denoms, denom)
		}
		sort.Strings(denoms)
		for _, denom := range denoms {
			state.Balances = append(state.Balances, BalanceRecord{
				Address: address,
				Denom:   denom,
				Amount:  k.balances[address][denom].String(),
			})
		}
	}
	return state
}

func (k *Keeper) ImportState(state StateSnapshot) error {
	return k.loadState(state, false)
}

func (k *Keeper) Flush() error {
	return k.persist()
}

func (k *Keeper) loadFromPrefixStore() error {
	state := StateSnapshot{}
	if err := k.prefixStore.LoadAll(balancePrefix, func() any {
		record := BalanceRecord{}
		return &record
	}, func(_ string, value any) error {
		record := *(value.(*BalanceRecord))
		state.Balances = append(state.Balances, record)
		return nil
	}); err != nil {
		return err
	}
	return k.loadState(state, false)
}

func (k *Keeper) loadState(state StateSnapshot, persist bool) error {
	nextBalances := make(map[string]map[string]*big.Int)
	for _, record := range state.Balances {
		address := strings.TrimSpace(record.Address)
		if address == "" {
			return ErrInvalidAddress
		}
		denom, err := normalizeDenom(record.Denom)
		if err != nil {
			return err
		}
		amount, err := normalizeAmountString(record.Amount)
		if err != nil {
			return err
		}
		if _, ok := nextBalances[address]; !ok {
			nextBalances[address] = make(map[string]*big.Int)
		}
		nextBalances[address][denom] = amount
	}
	k.balances = nextBalances
	if persist {
		return k.persist()
	}
	return nil
}

func (k *Keeper) persist() error {
	if k.persistHook != nil {
		if err := k.persistHook(); err != nil {
			return err
		}
	}
	if k.prefixStore == nil {
		return nil
	}
	if err := k.prefixStore.ClearPrefix(balancePrefix); err != nil {
		return err
	}
	for _, record := range k.ExportBalances() {
		if err := k.prefixStore.Save(balancePrefix, balanceKey(record.Address, record.Denom), record); err != nil {
			return err
		}
	}
	return k.prefixStore.Commit()
}

func (k *Keeper) ExportBalances() []BalanceRecord {
	return k.ExportState().Balances
}

func normalizeAddress(address string) (string, error) {
	trimmed := strings.TrimSpace(address)
	if trimmed == "" {
		return "", ErrInvalidAddress
	}
	if _, err := sdk.AccAddressFromBech32(trimmed); err != nil {
		return "", ErrInvalidAddress
	}
	return trimmed, nil
}

// SetPersistHookForTesting injects a persist failure hook for targeted tests.
func (k *Keeper) SetPersistHookForTesting(hook func() error) {
	k.persistHook = hook
}

func normalizeDenom(denom string) (string, error) {
	trimmed := strings.TrimSpace(denom)
	if trimmed == "" {
		return "", ErrInvalidDenom
	}
	return trimmed, nil
}

func normalizeAmount(amount *big.Int) (*big.Int, error) {
	if amount == nil || amount.Sign() <= 0 {
		return nil, ErrInvalidAmount
	}
	return cloneAmount(amount), nil
}

func normalizeAmountString(amount string) (*big.Int, error) {
	trimmed := strings.TrimSpace(amount)
	if trimmed == "" {
		return nil, ErrInvalidAmount
	}
	parsed, ok := new(big.Int).SetString(trimmed, 10)
	if !ok || parsed.Sign() <= 0 {
		return nil, ErrInvalidAmount
	}
	return parsed, nil
}

func cloneAmount(amount *big.Int) *big.Int {
	if amount == nil {
		return big.NewInt(0)
	}
	return new(big.Int).Set(amount)
}

func balanceKey(address, denom string) string {
	return strings.TrimSpace(address) + "|" + strings.TrimSpace(denom)
}
