package keeper

import (
	"math/big"
	"testing"

	"github.com/ayushns01/aegislink/chain/aegislink/testutil"
	limittypes "github.com/ayushns01/aegislink/chain/aegislink/x/limits/types"
)

func TestSDKKeeperPersistsRateLimitsAcrossReload(t *testing.T) {
	store, keys := testutil.NewInMemoryCommitMultiStore(t, "limits")

	keeper, err := NewStoreKeeper(store, keys["limits"])
	if err != nil {
		t.Fatalf("expected store-backed keeper to initialize, got %v", err)
	}

	limit := limittypes.RateLimit{
		AssetID:       "eth.usdc",
		WindowSeconds: 600,
		MaxAmount:     big.NewInt(100000000),
	}
	if err := keeper.SetLimit(limit); err != nil {
		t.Fatalf("expected rate limit registration to succeed, got %v", err)
	}

	reloaded, err := NewStoreKeeper(store, keys["limits"])
	if err != nil {
		t.Fatalf("expected store-backed keeper reload to succeed, got %v", err)
	}

	stored, ok := reloaded.GetLimit(limit.AssetID)
	if !ok {
		t.Fatalf("expected limit %q after reload", limit.AssetID)
	}
	if stored.MaxAmount.Cmp(limit.MaxAmount) != 0 {
		t.Fatalf("expected max amount %s, got %s", limit.MaxAmount.String(), stored.MaxAmount.String())
	}
}
