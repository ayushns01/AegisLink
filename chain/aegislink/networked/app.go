package networked

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"cosmossdk.io/log"
	"cosmossdk.io/store/rootmulti"
	storetypes "cosmossdk.io/store/types"
	upgradekeeper "cosmossdk.io/x/upgrade/keeper"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	abcitypes "github.com/cometbft/cometbft/abci/types"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	addresscodec "github.com/cosmos/cosmos-sdk/codec/address"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	txsigning "github.com/cosmos/cosmos-sdk/types/tx/signing"
	authmodule "github.com/cosmos/cosmos-sdk/x/auth"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	authtx "github.com/cosmos/cosmos-sdk/x/auth/tx"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	bankmodule "github.com/cosmos/cosmos-sdk/x/bank"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	paramtypes "github.com/cosmos/cosmos-sdk/x/params/types"
	transfermodule "github.com/cosmos/ibc-go/v10/modules/apps/transfer"
	transferkeeper "github.com/cosmos/ibc-go/v10/modules/apps/transfer/keeper"
	transfertypes "github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"
	ibcmodule "github.com/cosmos/ibc-go/v10/modules/core"
	porttypes "github.com/cosmos/ibc-go/v10/modules/core/05-port/types"
	ibcexported "github.com/cosmos/ibc-go/v10/modules/core/exported"
	ibckeeper "github.com/cosmos/ibc-go/v10/modules/core/keeper"

	aegisapp "github.com/ayushns01/aegislink/chain/aegislink/app"
)

type ChainApp struct {
	AppConfig          aegisapp.Config
	Logger             log.Logger
	DB                 dbm.DB
	BaseApp            *baseapp.BaseApp
	MultiStore         *rootmulti.Store
	StoreKeys          map[string]*storetypes.KVStoreKey
	LegacyAmino        *codec.LegacyAmino
	InterfaceRegistry  codectypes.InterfaceRegistry
	Codec              codec.Codec
	TxConfig           client.TxConfig
	BasicModuleManager module.BasicManager
	AccountKeeper      authkeeper.AccountKeeper
	BankKeeper         bankkeeper.BaseKeeper
	Authority          string
	UpgradeKeeper      *upgradekeeper.Keeper
	IBCKeeper          *ibckeeper.Keeper
	IBCModule          ibcmodule.AppModule
	TransferKeeper     transferkeeper.Keeper
	TransferModule     transfermodule.AppModule
	ModuleManager      *module.Manager
	Configurator       module.Configurator
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
	legacyAmino := codec.NewLegacyAmino()
	interfaceRegistry := codectypes.NewInterfaceRegistry()
	protoCodec := codec.NewProtoCodec(interfaceRegistry)
	basicModuleManager := module.NewBasicManager(
		authmodule.AppModuleBasic{},
		bankmodule.AppModuleBasic{},
		ibcmodule.AppModuleBasic{},
		transfermodule.AppModuleBasic{},
	)
	basicModuleManager.RegisterLegacyAminoCodec(legacyAmino)
	basicModuleManager.RegisterInterfaces(interfaceRegistry)
	txConfig := authtx.NewTxConfig(protoCodec, []txsigning.SignMode{
		txsigning.SignMode_SIGN_MODE_DIRECT,
	})
	accountAddressCodec := addresscodec.NewBech32Codec("cosmos")
	authority, err := accountAddressCodec.BytesToString(authtypes.NewModuleAddress("gov"))
	if err != nil {
		return nil, fmt.Errorf("encode authority address: %w", err)
	}

	base := baseapp.NewBaseApp(appCfg.AppName, logger, db, txConfig.TxDecoder(), baseapp.SetChainID(appCfg.ChainID))
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

	moduleAccountPermissions := map[string][]string{
		transfertypes.ModuleName: {authtypes.Minter, authtypes.Burner},
	}
	accountKeeper := authkeeper.NewAccountKeeper(
		protoCodec,
		runtime.NewKVStoreService(storeKeys[authtypes.StoreKey]),
		authtypes.ProtoBaseAccount,
		moduleAccountPermissions,
		accountAddressCodec,
		"cosmos",
		authority,
	)
	bankKeeper := bankkeeper.NewBaseKeeper(
		protoCodec,
		runtime.NewKVStoreService(storeKeys["bank"]),
		accountKeeper,
		map[string]bool{},
		authority,
		logger,
	)

	upgradeKeeper := upgradekeeper.NewKeeper(
		map[int64]bool{},
		runtime.NewKVStoreService(storeKeys[upgradetypes.StoreKey]),
		protoCodec,
		appCfg.HomeDir,
		nil,
		authority,
	)
	ibcKeeper := ibckeeper.NewKeeper(
		protoCodec,
		runtime.NewKVStoreService(storeKeys[ibcexported.StoreKey]),
		noopIBCParamSubspace{},
		upgradeKeeper,
		authority,
	)
	ibcModule := ibcmodule.NewAppModule(ibcKeeper)
	transferKeeper := transferkeeper.NewKeeper(
		protoCodec,
		runtime.NewKVStoreService(storeKeys[transfertypes.StoreKey]),
		noopIBCParamSubspace{},
		ibcKeeper.ChannelKeeper,
		ibcKeeper.ChannelKeeper,
		base.MsgServiceRouter(),
		accountKeeper,
		bankKeeper,
		authority,
	)
	transferIBCModule := transfermodule.NewIBCModule(transferKeeper)
	ibcKeeper.SetRouter(
		porttypes.NewRouter().AddRoute(transfertypes.ModuleName, transferIBCModule),
	)
	transferModule := transfermodule.NewAppModule(transferKeeper)
	authAppModule := authmodule.NewAppModule(protoCodec, accountKeeper, nil, nil)
	bankAppModule := bankmodule.NewAppModule(protoCodec, bankKeeper, accountKeeper, nil)
	moduleManager := module.NewManager(
		authAppModule,
		bankAppModule,
		ibcModule,
		transferModule,
	)
	moduleManager.SetOrderInitGenesis(
		authtypes.ModuleName,
		banktypes.ModuleName,
		ibcexported.ModuleName,
		transfertypes.ModuleName,
	)
	moduleManager.SetOrderBeginBlockers(
		authtypes.ModuleName,
		banktypes.ModuleName,
		ibcexported.ModuleName,
		transfertypes.ModuleName,
	)
	moduleManager.SetOrderEndBlockers(
		authtypes.ModuleName,
		banktypes.ModuleName,
		ibcexported.ModuleName,
		transfertypes.ModuleName,
	)

	base.MsgServiceRouter().SetInterfaceRegistry(interfaceRegistry)
	base.GRPCQueryRouter().SetInterfaceRegistry(interfaceRegistry)
	configurator := module.NewConfigurator(protoCodec, base.MsgServiceRouter(), base.GRPCQueryRouter())
	if err := moduleManager.RegisterServices(configurator); err != nil {
		return nil, fmt.Errorf("register module services: %w", err)
	}
	if err := configurator.Error(); err != nil {
		return nil, fmt.Errorf("configure module services: %w", err)
	}

	base.SetInitChainer(func(ctx sdk.Context, req *abcitypes.RequestInitChain) (*abcitypes.ResponseInitChain, error) {
		genesisState := basicModuleManager.DefaultGenesis(protoCodec)
		if len(req.AppStateBytes) > 0 {
			if err := json.Unmarshal(req.AppStateBytes, &genesisState); err != nil {
				return nil, fmt.Errorf("decode app state bytes: %w", err)
			}
		}
		if err := initModulesGenesis(ctx, protoCodec, moduleManager, genesisState); err != nil {
			return nil, err
		}
		return &abcitypes.ResponseInitChain{}, nil
	})
	base.SetBeginBlocker(func(ctx sdk.Context) (sdk.BeginBlock, error) {
		return moduleManager.BeginBlock(ctx)
	})
	base.SetEndBlocker(func(ctx sdk.Context) (sdk.EndBlock, error) {
		return moduleManager.EndBlock(ctx)
	})

	if err := base.LoadLatestVersion(); err != nil {
		return nil, fmt.Errorf("load latest baseapp version: %w", err)
	}
	defaultGenesisState := basicModuleManager.DefaultGenesis(protoCodec)
	if err := bootstrapDefaultModuleState(base, moduleManager, protoCodec, defaultGenesisState, appCfg.ChainID); err != nil {
		return nil, err
	}

	return &ChainApp{
		AppConfig:          appCfg,
		Logger:             logger,
		DB:                 db,
		BaseApp:            base,
		MultiStore:         multiStore,
		StoreKeys:          storeKeys,
		LegacyAmino:        legacyAmino,
		InterfaceRegistry:  interfaceRegistry,
		Codec:              protoCodec,
		TxConfig:           txConfig,
		BasicModuleManager: basicModuleManager,
		AccountKeeper:      accountKeeper,
		BankKeeper:         bankKeeper,
		Authority:          authority,
		UpgradeKeeper:      upgradeKeeper,
		IBCKeeper:          ibcKeeper,
		IBCModule:          ibcModule,
		TransferKeeper:     transferKeeper,
		TransferModule:     transferModule,
		ModuleManager:      moduleManager,
		Configurator:       configurator,
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

func (a *ChainApp) DefaultGenesis() map[string]json.RawMessage {
	if a == nil || a.BasicModuleManager == nil || a.Codec == nil {
		return nil
	}
	return a.BasicModuleManager.DefaultGenesis(a.Codec)
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
	for _, name := range []string{"auth", authtypes.StoreKey, "bank", "capability", "ibc", "transfer", upgradetypes.StoreKey} {
		appendKey(name)
	}

	return keys
}

type noopIBCParamSubspace struct{}

func (noopIBCParamSubspace) GetParamSet(_ sdk.Context, _ paramtypes.ParamSet) {}

func initModulesGenesis(ctx sdk.Context, cdc codec.JSONCodec, moduleManager *module.Manager, genesisState map[string]json.RawMessage) error {
	if moduleManager == nil {
		return nil
	}
	for _, moduleName := range moduleManager.OrderInitGenesis {
		moduleGenesis := genesisState[moduleName]
		if len(moduleGenesis) == 0 {
			continue
		}
		mod := moduleManager.Modules[moduleName]
		switch typedModule := mod.(type) {
		case module.HasGenesis:
			typedModule.InitGenesis(ctx, cdc, moduleGenesis)
		case module.HasABCIGenesis:
			typedModule.InitGenesis(ctx, cdc, moduleGenesis)
		}
	}
	return nil
}

func bootstrapDefaultModuleState(
	base *baseapp.BaseApp,
	moduleManager *module.Manager,
	cdc codec.JSONCodec,
	genesisState map[string]json.RawMessage,
	chainID string,
) error {
	if base == nil || moduleManager == nil {
		return nil
	}

	ctx := base.NewUncachedContext(false, cmtproto.Header{ChainID: chainID})
	if _, err := moduleManager.InitGenesis(ctx, cdc, genesisState); err != nil && !strings.Contains(err.Error(), "validator set is empty after InitGenesis") {
		return fmt.Errorf("bootstrap module genesis: %w", err)
	}
	return nil
}
