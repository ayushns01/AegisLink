package keeper

import (
	"math/big"
	"testing"

	"github.com/ayushns01/aegislink/chain/aegislink/testutil"
)

func TestSDKKeeperPersistsTransfersAcrossReload(t *testing.T) {
	store, keys := testutil.NewInMemoryCommitMultiStore(t, "ibcrouter")

	keeper, err := NewStoreKeeper(store, keys["ibcrouter"])
	if err != nil {
		t.Fatalf("expected store-backed keeper to initialize, got %v", err)
	}

	if err := keeper.SetRoute(Route{
		AssetID:            "eth.usdc",
		DestinationChainID: "osmosis-local-1",
		ChannelID:          "channel-0",
		DestinationDenom:   "ibc/uethusdc",
		Enabled:            true,
	}); err != nil {
		t.Fatalf("expected route registration to succeed, got %v", err)
	}

	record, err := keeper.InitiateTransfer("eth.usdc", big.NewInt(25000000), "osmo1receiver", 100, "swap:uosmo")
	if err != nil {
		t.Fatalf("expected transfer initiation to succeed, got %v", err)
	}

	reloaded, err := NewStoreKeeper(store, keys["ibcrouter"])
	if err != nil {
		t.Fatalf("expected store-backed keeper reload to succeed, got %v", err)
	}

	route, ok := reloaded.GetRoute("eth.usdc")
	if !ok {
		t.Fatalf("expected route after reload")
	}
	if !route.Enabled {
		t.Fatalf("expected route to stay enabled after reload")
	}

	transfers := reloaded.ExportTransfers()
	if len(transfers) != 1 {
		t.Fatalf("expected one transfer after reload, got %d", len(transfers))
	}
	if transfers[0].TransferID != record.TransferID {
		t.Fatalf("expected transfer id %q, got %q", record.TransferID, transfers[0].TransferID)
	}
}
