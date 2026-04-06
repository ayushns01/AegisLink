package keeper

import (
	"testing"

	"github.com/ayushns01/aegislink/chain/aegislink/testutil"
)

func TestSDKKeeperPersistsPausedFlowsAcrossReload(t *testing.T) {
	store, keys := testutil.NewInMemoryCommitMultiStore(t, "pauser")

	keeper, err := NewStoreKeeper(store, keys["pauser"])
	if err != nil {
		t.Fatalf("expected store-backed keeper to initialize, got %v", err)
	}

	if err := keeper.Pause("eth.usdc"); err != nil {
		t.Fatalf("expected pause to succeed, got %v", err)
	}

	reloaded, err := NewStoreKeeper(store, keys["pauser"])
	if err != nil {
		t.Fatalf("expected store-backed keeper reload to succeed, got %v", err)
	}

	if !reloaded.IsPaused("eth.usdc") {
		t.Fatalf("expected paused flow after reload")
	}
}
