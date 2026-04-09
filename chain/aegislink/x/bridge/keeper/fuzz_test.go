package keeper

import (
	"errors"
	"math/big"
	"testing"
)

func FuzzBridgeSupplyNeverGoesNegative(f *testing.F) {
	f.Add(uint64(10_000_000), uint64(5_000_000))
	f.Add(uint64(10_000_000), uint64(20_000_000))

	f.Fuzz(func(t *testing.T, depositSeed uint64, withdrawalSeed uint64) {
		t.Parallel()

		keeper, claim, _, _, _, _ := newKeeperFixture(t)

		depositAmount := normalizeFuzzAmount(depositSeed, 100_000_000)
		withdrawalAmount := normalizeFuzzAmount(withdrawalSeed, 200_000_000)

		claim.Amount = big.NewInt(int64(depositAmount))
		attestation := validAttestation(claim)
		if _, err := keeper.ExecuteDepositClaim(claim, attestation); err != nil {
			t.Fatalf("deposit claim failed: %v", err)
		}

		keeper.SetCurrentHeight(60)
		_, err := keeper.ExecuteWithdrawal(claim.AssetID, big.NewInt(int64(withdrawalAmount)), "0xrecipient", 120, []byte("proof"))

		supply := keeper.SupplyForDenom("uethusdc")
		if supply.Sign() < 0 {
			t.Fatalf("supply went negative: %s", supply.String())
		}

		if withdrawalAmount > depositAmount {
			if !errors.Is(err, ErrInsufficientSupply) {
				t.Fatalf("expected insufficient supply when withdrawing %d from %d, got %v", withdrawalAmount, depositAmount, err)
			}
			if supply.Cmp(big.NewInt(int64(depositAmount))) != 0 {
				t.Fatalf("expected supply to remain %d after rejected burn, got %s", depositAmount, supply.String())
			}
			return
		}

		if err != nil {
			t.Fatalf("expected withdrawal to succeed, got %v", err)
		}
		expected := new(big.Int).Sub(big.NewInt(int64(depositAmount)), big.NewInt(int64(withdrawalAmount)))
		if supply.Cmp(expected) != 0 {
			t.Fatalf("expected supply %s, got %s", expected.String(), supply.String())
		}
	})
}

func normalizeFuzzAmount(seed uint64, max uint64) uint64 {
	if max == 0 {
		return 1
	}
	return (seed % max) + 1
}
