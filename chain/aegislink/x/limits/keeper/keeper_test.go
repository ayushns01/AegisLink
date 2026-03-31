package keeper

import (
	"errors"
	"math/big"
	"testing"

	limittypes "github.com/ayushns01/aegislink/chain/aegislink/x/limits/types"
)

func TestSetLimitRejectsInvalidLimit(t *testing.T) {
	keeper := NewKeeper()

	err := keeper.SetLimit(limittypes.RateLimit{
		AssetID:       "",
		WindowSeconds: 600,
		MaxAmount:     big.NewInt(10),
	})
	if !errors.Is(err, limittypes.ErrInvalidRateLimit) {
		t.Fatalf("expected invalid rate limit error, got %v", err)
	}
}

func TestCheckTransferRejectsOverLimitRouteAttempt(t *testing.T) {
	keeper := NewKeeper()
	limit := validLimit()

	if err := keeper.SetLimit(limit); err != nil {
		t.Fatalf("expected set limit to succeed, got %v", err)
	}

	err := keeper.CheckTransfer(limit.AssetID, mustAmount("1001"))
	if !errors.Is(err, ErrRateLimitExceeded) {
		t.Fatalf("expected rate limit exceeded error, got %v", err)
	}
}

func TestCheckTransferAllowsAmountWithinLimit(t *testing.T) {
	keeper := NewKeeper()
	limit := validLimit()

	if err := keeper.SetLimit(limit); err != nil {
		t.Fatalf("expected set limit to succeed, got %v", err)
	}

	if err := keeper.CheckTransfer(limit.AssetID, mustAmount("1000")); err != nil {
		t.Fatalf("expected amount within limit to pass, got %v", err)
	}
}

func validLimit() limittypes.RateLimit {
	return limittypes.RateLimit{
		AssetID:       "eth.usdc",
		WindowSeconds: 600,
		MaxAmount:     mustAmount("1000"),
	}
}

func mustAmount(value string) *big.Int {
	amount, ok := new(big.Int).SetString(value, 10)
	if !ok {
		panic("invalid test amount")
	}
	return amount
}
