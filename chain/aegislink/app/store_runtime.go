package app

import (
	"fmt"
	"os"
	"path/filepath"

	"cosmossdk.io/log"
	"cosmossdk.io/store/metrics"
	"cosmossdk.io/store/rootmulti"
	storetypes "cosmossdk.io/store/types"
	dbm "github.com/cosmos/cosmos-db"

	bridgekeeper "github.com/ayushns01/aegislink/chain/aegislink/x/bridge/keeper"
	governancekeeper "github.com/ayushns01/aegislink/chain/aegislink/x/governance/keeper"
	ibcrouterkeeper "github.com/ayushns01/aegislink/chain/aegislink/x/ibcrouter/keeper"
	limitskeeper "github.com/ayushns01/aegislink/chain/aegislink/x/limits/keeper"
	pauserkeeper "github.com/ayushns01/aegislink/chain/aegislink/x/pauser/keeper"
	registrykeeper "github.com/ayushns01/aegislink/chain/aegislink/x/registry/keeper"
)

type storeRuntime struct {
	db        dbm.DB
	multi     *rootmulti.Store
	storeKeys map[string]*storetypes.KVStoreKey
}

func newStoreRuntime(cfg Config) (*storeRuntime, error) {
	storePath := cfg.StatePath
	if storePath == "" {
		storePath = runtimeStorePath(cfg.HomeDir)
	}
	if err := os.MkdirAll(filepath.Dir(storePath), 0o755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(storePath, 0o755); err != nil {
		return nil, err
	}

	db, err := dbm.NewGoLevelDB(AppName, storePath, nil)
	if err != nil {
		return nil, fmt.Errorf("open sdk store db: %w", err)
	}

	multi := rootmulti.NewStore(db, log.NewNopLogger(), metrics.NewNoOpMetrics())
	storeKeys := storetypes.NewKVStoreKeys(cfg.Modules...)
	for _, moduleName := range cfg.Modules {
		multi.MountStoreWithDB(storeKeys[moduleName], storetypes.StoreTypeIAVL, nil)
	}
	if err := multi.LoadLatestVersion(); err != nil {
		return nil, fmt.Errorf("load latest sdk store version: %w", err)
	}

	return &storeRuntime{
		db:        db,
		multi:     multi,
		storeKeys: storeKeys,
	}, nil
}

func newStoreBackedApp(cfg Config) (*App, error) {
	runtime, err := newStoreRuntime(cfg)
	if err != nil {
		return nil, err
	}

	registryKeeper, err := registrykeeper.NewStoreKeeper(runtime.multi, runtime.storeKeys["registry"])
	if err != nil {
		return nil, err
	}
	limitsKeeper, err := limitskeeper.NewStoreKeeper(runtime.multi, runtime.storeKeys["limits"])
	if err != nil {
		return nil, err
	}
	pauserKeeper, err := pauserkeeper.NewStoreKeeper(runtime.multi, runtime.storeKeys["pauser"])
	if err != nil {
		return nil, err
	}
	ibcRouterKeeper, err := ibcrouterkeeper.NewStoreKeeper(runtime.multi, runtime.storeKeys["ibcrouter"])
	if err != nil {
		return nil, err
	}
	governanceKeeper, err := governancekeeper.NewStoreKeeper(runtime.multi, runtime.storeKeys["governance"], registryKeeper, limitsKeeper, ibcRouterKeeper, cfg.GovernanceAuthorities)
	if err != nil {
		return nil, err
	}
	bridgeKeeper, err := bridgekeeper.NewStoreKeeper(
		runtime.multi,
		runtime.storeKeys["bridge"],
		registryKeeper,
		limitsKeeper,
		pauserKeeper,
		cfg.AllowedSigners,
		cfg.RequiredThreshold,
	)
	if err != nil {
		return nil, err
	}

	app := newAppFromKeepers(cfg, bridgeKeeper, ibcRouterKeeper, registryKeeper, limitsKeeper, pauserKeeper, governanceKeeper)
	app.storeRuntime = runtime
	app.Config.StatePath = runtimeStorePath(cfg.HomeDir)
	if cfg.StatePath != "" {
		app.Config.StatePath = cfg.StatePath
	}

	return app, nil
}
