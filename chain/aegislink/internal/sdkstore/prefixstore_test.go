package sdkstore

import (
	"os"
	"path/filepath"
	"testing"

	"cosmossdk.io/log"
	"cosmossdk.io/store/metrics"
	"cosmossdk.io/store/rootmulti"
	storetypes "cosmossdk.io/store/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/ayushns01/aegislink/chain/aegislink/testutil"
)

func TestJSONPrefixStorePersistsRecordsPerPrefix(t *testing.T) {
	store, keys := testutil.NewInMemoryCommitMultiStore(t, "example")

	prefixStore, err := NewJSONPrefixStore(store, keys["example"])
	if err != nil {
		t.Fatalf("new prefix store: %v", err)
	}

	type record struct {
		Value string `json:"value"`
	}
	if err := prefixStore.Save("asset", "eth.usdc", record{Value: "uethusdc"}); err != nil {
		t.Fatalf("save first asset record: %v", err)
	}
	if err := prefixStore.Save("asset", "eth.weth", record{Value: "uethweth"}); err != nil {
		t.Fatalf("save second asset record: %v", err)
	}
	if err := prefixStore.Save("limit", "eth.usdc", record{Value: "100"}); err != nil {
		t.Fatalf("save limit record: %v", err)
	}
	if err := prefixStore.Commit(); err != nil {
		t.Fatalf("commit prefix records: %v", err)
	}

	if !prefixStore.HasAny("asset") {
		t.Fatal("expected asset prefix data to exist")
	}
	if !prefixStore.HasAny("limit") {
		t.Fatal("expected limit prefix data to exist")
	}

	var assets []string
	if err := prefixStore.LoadAll("asset", func() any {
		return &record{}
	}, func(_ string, value any) error {
		assets = append(assets, value.(*record).Value)
		return nil
	}); err != nil {
		t.Fatalf("load asset prefix records: %v", err)
	}
	if len(assets) != 2 {
		t.Fatalf("expected 2 asset records, got %d", len(assets))
	}

	if err := prefixStore.ClearPrefix("asset"); err != nil {
		t.Fatalf("clear asset prefix: %v", err)
	}
	if err := prefixStore.Commit(); err != nil {
		t.Fatalf("commit cleared prefix: %v", err)
	}
	if prefixStore.HasAny("asset") {
		t.Fatal("expected cleared asset prefix to be empty")
	}
	if !prefixStore.HasAny("limit") {
		t.Fatal("expected limit prefix to remain after clearing assets")
	}
}

func TestJSONPrefixStoreCommitKeepsEmptyStoreReloadable(t *testing.T) {
	t.Parallel()

	storeDir := filepath.Join(t.TempDir(), "sdk-prefix-store")

	db, err := dbm.NewGoLevelDB("aegislink", storeDir, nil)
	if err != nil {
		t.Fatalf("open first leveldb: %v", err)
	}

	store := rootmulti.NewStore(db, log.NewNopLogger(), metrics.NewNoOpMetrics())
	keys := storetypes.NewKVStoreKeys("example")
	store.MountStoreWithDB(keys["example"], storetypes.StoreTypeIAVL, nil)
	if err := store.LoadLatestVersion(); err != nil {
		t.Fatalf("load first root store: %v", err)
	}
	if store.LastCommitID().Version == 0 {
		if err := store.SetInitialVersion(1); err != nil {
			t.Fatalf("set initial version: %v", err)
		}
	}

	prefixStore, err := NewJSONPrefixStore(store, keys["example"])
	if err != nil {
		t.Fatalf("new prefix store: %v", err)
	}
	if err := prefixStore.Commit(); err != nil {
		t.Fatalf("commit empty prefix store: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close first leveldb: %v", err)
	}

	db, err = dbm.NewGoLevelDB("aegislink", storeDir, nil)
	if err != nil {
		t.Fatalf("open second leveldb: %v", err)
	}
	defer db.Close()

	reloaded := rootmulti.NewStore(db, log.NewNopLogger(), metrics.NewNoOpMetrics())
	reloadedKeys := storetypes.NewKVStoreKeys("example")
	reloaded.MountStoreWithDB(reloadedKeys["example"], storetypes.StoreTypeIAVL, nil)
	if err := reloaded.LoadLatestVersion(); err != nil {
		t.Fatalf("reload root store after empty commit: %v", err)
	}

	reloadedPrefixStore, err := NewJSONPrefixStore(reloaded, reloadedKeys["example"])
	if err != nil {
		t.Fatalf("new reloaded prefix store: %v", err)
	}
	if reloadedPrefixStore.HasAny("asset") {
		t.Fatal("expected no asset records after empty commit reload")
	}

	entries, err := os.ReadDir(storeDir)
	if err != nil {
		t.Fatalf("read store dir: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected store directory to contain persisted leveldb files")
	}
}
