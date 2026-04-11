package networked

import (
	"fmt"
	"sort"

	"cosmossdk.io/log"
	"cosmossdk.io/store/rootmulti"
	storetypes "cosmossdk.io/store/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	txsigning "github.com/cosmos/cosmos-sdk/types/tx/signing"
	authtx "github.com/cosmos/cosmos-sdk/x/auth/tx"

	aegisapp "github.com/ayushns01/aegislink/chain/aegislink/app"
)

type ChainApp struct {
	AppConfig         aegisapp.Config
	Logger            log.Logger
	DB                dbm.DB
	BaseApp           *baseapp.BaseApp
	MultiStore        *rootmulti.Store
	StoreKeys         map[string]*storetypes.KVStoreKey
	InterfaceRegistry codectypes.InterfaceRegistry
	Codec             codec.Codec
	TxConfig          client.TxConfig
}

func NewChainApp(cfg Config) (*ChainApp, error) {
	_, appCfg, err := ResolveConfig(cfg)
	if err != nil {
		return nil, err
	}
	return BuildChainApp(appCfg)
}

func BuildChainApp(appCfg aegisapp.Config) (*ChainApp, error) {
	logger := log.NewNopLogger()
	db := dbm.NewMemDB()
	interfaceRegistry := codectypes.NewInterfaceRegistry()
	protoCodec := codec.NewProtoCodec(interfaceRegistry)
	txConfig := authtx.NewTxConfig(protoCodec, []txsigning.SignMode{
		txsigning.SignMode_SIGN_MODE_DIRECT,
	})

	base := baseapp.NewBaseApp(appCfg.AppName, logger, db, txConfig.TxDecoder())
	multiStore, ok := base.CommitMultiStore().(*rootmulti.Store)
	if !ok {
		return nil, fmt.Errorf("unexpected commit multistore type %T", base.CommitMultiStore())
	}

	storeKeys := make(map[string]*storetypes.KVStoreKey)
	storeKeyNames := networkedStoreKeyNames(appCfg.Modules)
	mountKeys := make([]storetypes.StoreKey, 0, len(storeKeyNames))
	for _, keyName := range storeKeyNames {
		key := storetypes.NewKVStoreKey(keyName)
		storeKeys[keyName] = key
		mountKeys = append(mountKeys, key)
	}
	base.MountStores(mountKeys...)

	return &ChainApp{
		AppConfig:         appCfg,
		Logger:            logger,
		DB:                db,
		BaseApp:           base,
		MultiStore:        multiStore,
		StoreKeys:         storeKeys,
		InterfaceRegistry: interfaceRegistry,
		Codec:             protoCodec,
		TxConfig:          txConfig,
	}, nil
}

func (a *ChainApp) Close() error {
	if a == nil || a.DB == nil {
		return nil
	}
	return a.DB.Close()
}

func (a *ChainApp) SortedStoreKeyNames() []string {
	if a == nil || len(a.StoreKeys) == 0 {
		return nil
	}

	names := make([]string, 0, len(a.StoreKeys))
	for name := range a.StoreKeys {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func networkedStoreKeyNames(moduleNames []string) []string {
	seen := map[string]struct{}{}
	keys := make([]string, 0, len(moduleNames)+5)

	appendKey := func(name string) {
		if name == "" {
			return
		}
		if _, ok := seen[name]; ok {
			return
		}
		seen[name] = struct{}{}
		keys = append(keys, name)
	}

	for _, name := range moduleNames {
		appendKey(name)
	}
	for _, name := range []string{"auth", "bank", "capability", "ibc", "transfer"} {
		appendKey(name)
	}

	return keys
}
