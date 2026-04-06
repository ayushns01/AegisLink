package testutil

import (
	"testing"

	"cosmossdk.io/log"
	"cosmossdk.io/store/metrics"
	"cosmossdk.io/store/rootmulti"
	storetypes "cosmossdk.io/store/types"
	dbm "github.com/cosmos/cosmos-db"
)

func NewInMemoryCommitMultiStore(t *testing.T, moduleNames ...string) (*rootmulti.Store, map[string]*storetypes.KVStoreKey) {
	t.Helper()

	db := dbm.NewMemDB()
	store := rootmulti.NewStore(db, log.NewNopLogger(), metrics.NewNoOpMetrics())
	keys := storetypes.NewKVStoreKeys(moduleNames...)

	for _, moduleName := range moduleNames {
		store.MountStoreWithDB(keys[moduleName], storetypes.StoreTypeIAVL, nil)
	}

	if err := store.LoadLatestVersion(); err != nil {
		t.Fatalf("expected in-memory commit multi-store to load, got %v", err)
	}

	return store, keys
}
