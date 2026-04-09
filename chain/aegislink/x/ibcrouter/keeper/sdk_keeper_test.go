package keeper

import (
	"math/big"
	"testing"

	storetypes "cosmossdk.io/store/types"
	"github.com/ayushns01/aegislink/chain/aegislink/testutil"
	ibcroutertypes "github.com/ayushns01/aegislink/chain/aegislink/x/ibcrouter/types"
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

func TestSDKKeeperPersistsRouteProfilesAcrossReload(t *testing.T) {
	store, keys := testutil.NewInMemoryCommitMultiStore(t, "ibcrouter")

	keeper, err := NewStoreKeeper(store, keys["ibcrouter"])
	if err != nil {
		t.Fatalf("expected store-backed keeper to initialize, got %v", err)
	}

	if err := keeper.SetRouteProfile(ibcroutertypes.RouteProfile{
		RouteID:            "osmosis-fast",
		DestinationChainID: "osmosis-local-1",
		ChannelID:          "channel-0",
		Enabled:            true,
		Assets: []ibcroutertypes.AssetRoute{
			{AssetID: "eth.usdc", DestinationDenom: "ibc/uethusdc"},
			{AssetID: "eth.weth", DestinationDenom: "ibc/uethweth"},
		},
		Policy: ibcroutertypes.RoutePolicy{
			AllowedMemoPrefixes: []string{"swap:"},
		},
	}); err != nil {
		t.Fatalf("expected route profile registration to succeed, got %v", err)
	}

	reloaded, err := NewStoreKeeper(store, keys["ibcrouter"])
	if err != nil {
		t.Fatalf("expected store-backed keeper reload to succeed, got %v", err)
	}

	profile, ok := reloaded.GetRouteProfile("osmosis-fast")
	if !ok {
		t.Fatalf("expected route profile after reload")
	}
	if profile.DestinationChainID != "osmosis-local-1" {
		t.Fatalf("expected persisted destination chain, got %q", profile.DestinationChainID)
	}
	if len(profile.Assets) != 2 {
		t.Fatalf("expected 2 persisted route-profile assets, got %d", len(profile.Assets))
	}
	if len(profile.Policy.AllowedMemoPrefixes) != 1 || profile.Policy.AllowedMemoPrefixes[0] != "swap:" {
		t.Fatalf("expected persisted memo policy, got %#v", profile.Policy.AllowedMemoPrefixes)
	}
}

func TestSDKKeeperStoresRoutesAndTransfersUnderPrefixKeys(t *testing.T) {
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
	if _, err := keeper.InitiateTransfer("eth.usdc", big.NewInt(25000000), "osmo1receiver", 100, "swap:uosmo"); err != nil {
		t.Fatalf("expected transfer initiation to succeed, got %v", err)
	}

	kv := store.GetKVStore(keys["ibcrouter"])
	if raw := kv.Get([]byte("state")); len(raw) != 0 {
		t.Fatalf("expected legacy state blob to be absent, got %q", string(raw))
	}
	routeIter := storetypes.KVStorePrefixIterator(kv, []byte("route/"))
	defer routeIter.Close()
	if !routeIter.Valid() {
		t.Fatal("expected at least one route/ prefix record")
	}
	transferIter := storetypes.KVStorePrefixIterator(kv, []byte("transfer/"))
	defer transferIter.Close()
	if !transferIter.Valid() {
		t.Fatal("expected at least one transfer/ prefix record")
	}
}
