package keeper

import (
	"errors"
	"math/big"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/ayushns01/aegislink/chain/aegislink/testutil"
)

func TestKeeperCreditsBalancesAndPersistsAcrossReload(t *testing.T) {
	t.Parallel()

	store, keys := testutil.NewInMemoryCommitMultiStore(t, "bank")
	k, err := NewStoreKeeper(store, keys["bank"])
	if err != nil {
		t.Fatalf("new store keeper: %v", err)
	}

	address := sdk.AccAddress([]byte("wallet-bridge-unit-test")).String()
	if err := k.Credit(address, "ueth", big.NewInt(100)); err != nil {
		t.Fatalf("credit ueth: %v", err)
	}
	if err := k.Credit(address, "uethusdc", big.NewInt(25)); err != nil {
		t.Fatalf("credit uethusdc: %v", err)
	}

	if got := k.BalanceOf(address, "ueth"); got.String() != "100" {
		t.Fatalf("expected ueth balance 100, got %s", got.String())
	}
	if got := k.BalanceOf(address, "uethusdc"); got.String() != "25" {
		t.Fatalf("expected uethusdc balance 25, got %s", got.String())
	}

	reloaded, err := NewStoreKeeper(store, keys["bank"])
	if err != nil {
		t.Fatalf("reload keeper: %v", err)
	}
	balances, err := reloaded.Balances(address)
	if err != nil {
		t.Fatalf("list balances: %v", err)
	}
	if len(balances) != 2 {
		t.Fatalf("expected two balances, got %d", len(balances))
	}
	if balances[0].Denom != "ueth" || balances[0].Amount != "100" {
		t.Fatalf("unexpected first balance: %+v", balances[0])
	}
	if balances[1].Denom != "uethusdc" || balances[1].Amount != "25" {
		t.Fatalf("unexpected second balance: %+v", balances[1])
	}
}

func TestKeeperRejectsInvalidBech32Address(t *testing.T) {
	t.Parallel()

	k := NewKeeper()
	if _, err := k.Balances("not-a-bech32-address"); !errors.Is(err, ErrInvalidAddress) {
		t.Fatalf("expected invalid address error, got %v", err)
	}
}
