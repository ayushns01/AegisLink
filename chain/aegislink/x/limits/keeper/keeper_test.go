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
		WindowBlocks: 600,
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

func TestCheckTransferAccumulatesUsageWithinWindow(t *testing.T) {
	keeper := NewKeeper()
	limit := validLimit()

	if err := keeper.SetLimit(limit); err != nil {
		t.Fatalf("expected set limit to succeed, got %v", err)
	}
	if err := keeper.CheckTransferAtHeight(limit.AssetID, mustAmount("600"), 100); err != nil {
		t.Fatalf("expected first transfer to fit inside fresh window, got %v", err)
	}
	if err := keeper.RecordTransferAtHeight(limit.AssetID, mustAmount("600"), 100); err != nil {
		t.Fatalf("expected first transfer usage to record, got %v", err)
	}
	if err := keeper.CheckTransferAtHeight(limit.AssetID, mustAmount("500"), 105); !errors.Is(err, ErrRateLimitExceeded) {
		t.Fatalf("expected cumulative usage to exceed window, got %v", err)
	}
	if err := keeper.CheckTransferAtHeight(limit.AssetID, mustAmount("400"), 105); err != nil {
		t.Fatalf("expected remaining window capacity to allow smaller transfer, got %v", err)
	}
}

func TestCheckTransferAllowsAgainAfterWindowExpires(t *testing.T) {
	keeper := NewKeeper()
	limit := validLimit()

	if err := keeper.SetLimit(limit); err != nil {
		t.Fatalf("expected set limit to succeed, got %v", err)
	}
	if err := keeper.RecordTransferAtHeight(limit.AssetID, mustAmount("900"), 100); err != nil {
		t.Fatalf("expected usage to record, got %v", err)
	}
	if err := keeper.CheckTransferAtHeight(limit.AssetID, mustAmount("200"), 105); !errors.Is(err, ErrRateLimitExceeded) {
		t.Fatalf("expected transfer inside same window to be rejected, got %v", err)
	}
	if err := keeper.CheckTransferAtHeight(limit.AssetID, mustAmount("900"), 701); err != nil {
		t.Fatalf("expected transfer after window reset to pass, got %v", err)
	}
}

func TestCheckTransferTracksUsageSeparatelyPerAsset(t *testing.T) {
	keeper := NewKeeper()
	if err := keeper.SetLimit(limittypes.RateLimit{
		AssetID:       "eth.usdc",
		WindowBlocks: 600,
		MaxAmount:     mustAmount("1000"),
	}); err != nil {
		t.Fatalf("expected usdc limit to succeed, got %v", err)
	}
	if err := keeper.SetLimit(limittypes.RateLimit{
		AssetID:       "eth.usdt",
		WindowBlocks: 600,
		MaxAmount:     mustAmount("500"),
	}); err != nil {
		t.Fatalf("expected usdt limit to succeed, got %v", err)
	}

	if err := keeper.RecordTransferAtHeight("eth.usdc", mustAmount("900"), 100); err != nil {
		t.Fatalf("expected usdc usage to record, got %v", err)
	}
	if err := keeper.CheckTransferAtHeight("eth.usdc", mustAmount("200"), 101); !errors.Is(err, ErrRateLimitExceeded) {
		t.Fatalf("expected usdc limit to reject overflowing usage, got %v", err)
	}
	if err := keeper.CheckTransferAtHeight("eth.usdt", mustAmount("500"), 101); err != nil {
		t.Fatalf("expected usdt limit to remain independent, got %v", err)
	}
}

func validLimit() limittypes.RateLimit {
	return limittypes.RateLimit{
		AssetID:       "eth.usdc",
		WindowBlocks: 600,
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
