package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"sort"

	bankkeeper "github.com/ayushns01/aegislink/chain/aegislink/x/bank/keeper"
	bridgekeeper "github.com/ayushns01/aegislink/chain/aegislink/x/bridge/keeper"
	governancekeeper "github.com/ayushns01/aegislink/chain/aegislink/x/governance/keeper"
	ibcrouterkeeper "github.com/ayushns01/aegislink/chain/aegislink/x/ibcrouter/keeper"
	limitskeeper "github.com/ayushns01/aegislink/chain/aegislink/x/limits/keeper"
	limittypes "github.com/ayushns01/aegislink/chain/aegislink/x/limits/types"
	registrytypes "github.com/ayushns01/aegislink/chain/aegislink/x/registry/types"
)

var ErrWalletStateMigrationRequired = errors.New("wallet state migration required")

type runtimeState struct {
	Assets        []registrytypes.Asset          `json:"assets"`
	Bank          bankkeeper.StateSnapshot       `json:"bank"`
	Limits        limitskeeper.StateSnapshot     `json:"limits"`
	PausedFlows   []string                       `json:"paused_flows"`
	PendingClaims []QueuedDepositClaim           `json:"pending_claims"`
	Bridge        bridgekeeper.StateSnapshot     `json:"bridge"`
	IBCRouter     ibcrouterkeeper.StateSnapshot  `json:"ibc_router"`
	Governance    governancekeeper.StateSnapshot `json:"governance"`

	bankPresent bool
}

func loadRuntimeState(path string) (runtimeState, error) {
	if path == "" {
		return runtimeState{}, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return runtimeState{}, nil
		}
		return runtimeState{}, err
	}

	var raw struct {
		Assets        []registrytypes.Asset          `json:"assets"`
		Bank          *bankkeeper.StateSnapshot      `json:"bank"`
		Limits        json.RawMessage                `json:"limits"`
		PausedFlows   []string                       `json:"paused_flows"`
		PendingClaims []QueuedDepositClaim           `json:"pending_claims"`
		Bridge        bridgekeeper.StateSnapshot     `json:"bridge"`
		IBCRouter     ibcrouterkeeper.StateSnapshot  `json:"ibc_router"`
		Governance    governancekeeper.StateSnapshot `json:"governance"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return runtimeState{}, err
	}

	state := runtimeState{
		Assets:        raw.Assets,
		PausedFlows:   raw.PausedFlows,
		PendingClaims: raw.PendingClaims,
		Bridge:        raw.Bridge,
		IBCRouter:     raw.IBCRouter,
		Governance:    raw.Governance,
	}
	if raw.Bank != nil {
		state.Bank = *raw.Bank
		state.bankPresent = true
	}
	if len(raw.Limits) > 0 {
		switch raw.Limits[0] {
		case '[':
			var legacyLimits []limittypes.RateLimit
			if err := json.Unmarshal(raw.Limits, &legacyLimits); err != nil {
				return runtimeState{}, err
			}
			state.Limits = limitskeeper.StateSnapshot{Limits: legacyLimits}
		default:
			if err := json.Unmarshal(raw.Limits, &state.Limits); err != nil {
				return runtimeState{}, err
			}
		}
	}
	return state, nil
}

func (s runtimeState) resolvedBankState() (bankkeeper.StateSnapshot, error) {
	if s.bankPresent {
		return s.Bank, nil
	}
	if len(s.Bridge.ProcessedClaims) == 0 && len(s.Bridge.SupplyByDenom) == 0 && len(s.Bridge.Withdrawals) == 0 {
		return bankkeeper.StateSnapshot{}, nil
	}
	if len(s.Bridge.Withdrawals) > 0 {
		return bankkeeper.StateSnapshot{}, ErrWalletStateMigrationRequired
	}

	aggregated := make(map[string]map[string]*big.Int)
	supplyByDenom := make(map[string]*big.Int)
	for denom, amount := range s.Bridge.SupplyByDenom {
		if amount == "" {
			continue
		}
		parsed, err := parseBase10BigInt(amount)
		if err != nil {
			return bankkeeper.StateSnapshot{}, err
		}
		supplyByDenom[denom] = parsed
	}

	for _, claim := range s.Bridge.ProcessedClaims {
		if claim.Status != bridgekeeper.ClaimStatusAccepted {
			continue
		}
		if claim.Recipient == "" {
			return bankkeeper.StateSnapshot{}, ErrWalletStateMigrationRequired
		}
		amount, err := parseBase10BigInt(claim.Amount)
		if err != nil {
			return bankkeeper.StateSnapshot{}, err
		}
		if _, ok := aggregated[claim.Recipient]; !ok {
			aggregated[claim.Recipient] = make(map[string]*big.Int)
		}
		if _, ok := aggregated[claim.Recipient][claim.Denom]; !ok {
			aggregated[claim.Recipient][claim.Denom] = big.NewInt(0)
		}
		aggregated[claim.Recipient][claim.Denom].Add(aggregated[claim.Recipient][claim.Denom], amount)
	}

	state := bankkeeper.StateSnapshot{}
	totals := make(map[string]*big.Int)
	addresses := make([]string, 0, len(aggregated))
	for address := range aggregated {
		addresses = append(addresses, address)
	}
	sort.Strings(addresses)
	for _, address := range addresses {
		denoms := make([]string, 0, len(aggregated[address]))
		for denom := range aggregated[address] {
			denoms = append(denoms, denom)
		}
		sort.Strings(denoms)
		for _, denom := range denoms {
			amount := aggregated[address][denom]
			state.Balances = append(state.Balances, bankkeeper.BalanceRecord{
				Address: address,
				Denom:   denom,
				Amount:  amount.String(),
			})
			if _, ok := totals[denom]; !ok {
				totals[denom] = big.NewInt(0)
			}
			totals[denom].Add(totals[denom], amount)
		}
	}
	for denom, supply := range supplyByDenom {
		total, ok := totals[denom]
		if !ok || total.Cmp(supply) != 0 {
			return bankkeeper.StateSnapshot{}, ErrWalletStateMigrationRequired
		}
	}
	return state, nil
}

func parseBase10BigInt(value string) (*big.Int, error) {
	if value == "" {
		return big.NewInt(0), nil
	}
	parsed, ok := new(big.Int).SetString(value, 10)
	if !ok {
		return nil, fmt.Errorf("%w: invalid balance amount %q", ErrWalletStateMigrationRequired, value)
	}
	return parsed, nil
}

func persistRuntimeState(path string, state runtimeState) error {
	if path == "" {
		return nil
	}

	encoded, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "aegislink-state-*.json")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.Write(encoded); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
