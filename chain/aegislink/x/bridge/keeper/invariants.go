package keeper

import (
	"errors"
	"fmt"
	"math/big"
)

func (k *Keeper) CheckAccountingInvariant() error {
	expected := make(map[string]*big.Int, len(k.supplyByDenom))

	for _, claim := range k.processedClaims {
		if claim.Status != ClaimStatusAccepted {
			continue
		}
		if _, ok := expected[claim.Denom]; !ok {
			expected[claim.Denom] = big.NewInt(0)
		}
		expected[claim.Denom].Add(expected[claim.Denom], claim.Amount)
	}

	for _, withdrawal := range k.withdrawals {
		asset, ok := k.registryKeeper.GetAsset(withdrawal.AssetID)
		if !ok {
			return k.tripCircuit(fmt.Errorf("%w: missing asset metadata for withdrawal asset %s", ErrAccountingInvariantBroken, withdrawal.AssetID))
		}
		denom := bridgeDenomForAsset(asset)
		if _, ok := expected[denom]; !ok {
			expected[denom] = big.NewInt(0)
		}
		expected[denom].Sub(expected[denom], withdrawal.Amount)
		if expected[denom].Sign() < 0 {
			return k.tripCircuit(fmt.Errorf("%w: negative expected supply for asset %s denom %s", ErrAccountingInvariantBroken, withdrawal.AssetID, denom))
		}
	}

	for denom, actual := range k.supplyByDenom {
		want := expected[denom]
		if want == nil {
			want = big.NewInt(0)
		}
		if actual.Cmp(want) != 0 {
			return k.tripCircuit(fmt.Errorf("%w: denom %s supply mismatch want=%s got=%s", ErrAccountingInvariantBroken, denom, want.String(), actual.String()))
		}
		delete(expected, denom)
	}

	for denom, remaining := range expected {
		if remaining.Sign() == 0 {
			continue
		}
		return k.tripCircuit(fmt.Errorf("%w: denom %s missing supply entry want=%s", ErrAccountingInvariantBroken, denom, remaining.String()))
	}

	return nil
}

func (k *Keeper) CircuitBreakerTripped() bool {
	return k.circuitBreakerTripped
}

func (k *Keeper) LastInvariantError() string {
	return k.lastInvariantError
}

func (k *Keeper) ensureCircuitHealthy() error {
	if k.circuitBreakerTripped {
		if k.lastInvariantError != "" {
			return fmt.Errorf("%w: %s", ErrBridgeCircuitOpen, k.lastInvariantError)
		}
		return ErrBridgeCircuitOpen
	}
	return nil
}

func (k *Keeper) tripCircuit(reason error) error {
	k.circuitBreakerTripped = true
	k.lastInvariantError = reason.Error()
	_ = k.persist()
	return reason
}

func isCircuitError(err error) bool {
	return errors.Is(err, ErrAccountingInvariantBroken) || errors.Is(err, ErrBridgeCircuitOpen)
}
