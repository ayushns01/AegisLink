package networked

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"cosmossdk.io/log"
	"cosmossdk.io/store/rootmulti"
	storetypes "cosmossdk.io/store/types"
	signing "cosmossdk.io/x/tx/signing"
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
	"github.com/cosmos/gogoproto/proto"
	transfermodule "github.com/cosmos/ibc-go/v10/modules/apps/transfer"
	transferkeeper "github.com/cosmos/ibc-go/v10/modules/apps/transfer/keeper"
	transfertypes "github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"
	ibcmodule "github.com/cosmos/ibc-go/v10/modules/core"
	clienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types"
	channeltypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"
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

const (
	DemoLocalhostTransferPortID              = transfertypes.PortID
	DemoLocalhostTransferChannelID           = "channel-0"
	DemoLocalhostTransferCounterpartyChannel = "channel-1"
)

type LocalhostTransferRequest struct {
	Sender              string
	Coin                sdk.Coin
	Receiver            string
	TimeoutHeight       clienttypes.Height
	TimeoutTimestamp    uint64
	Memo                string
	SourcePort          string
	SourceChannel       string
	CounterpartyChannel string
}

type LocalhostTransferResult struct {
	Sender                string
	PortID                string
	ChannelID             string
	CounterpartyChannelID string
	Sequence              uint64
	PacketCommitment      []byte
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
	accountAddressCodec := addresscodec.NewBech32Codec("cosmos")
	interfaceRegistry, err := codectypes.NewInterfaceRegistryWithOptions(codectypes.InterfaceRegistryOptions{
		ProtoFiles: proto.HybridResolver,
		SigningOptions: signing.Options{
			AddressCodec:          accountAddressCodec,
			ValidatorAddressCodec: addresscodec.NewBech32Codec("cosmosvaloper"),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("build interface registry: %w", err)
	}
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
	authority, err := accountAddressCodec.BytesToString(authtypes.NewModuleAddress("gov"))
	if err != nil {
		return nil, fmt.Errorf("encode authority address: %w", err)
	}

	base := baseapp.NewBaseApp(appCfg.AppName, logger, db, txConfig.TxDecoder(), baseapp.SetChainID(appCfg.ChainID))
	base.SetInterfaceRegistry(interfaceRegistry)
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
	if err := bootstrapFirstCommittedBlock(base); err != nil {
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

func (a *ChainApp) ExecuteLocalhostTransfer(req LocalhostTransferRequest) (LocalhostTransferResult, error) {
	if a == nil || a.BaseApp == nil {
		return LocalhostTransferResult{}, fmt.Errorf("chain app is not initialized")
	}

	request, sender, err := a.normalizeLocalhostTransferRequest(req)
	if err != nil {
		return LocalhostTransferResult{}, err
	}

	headerHeight := a.BaseApp.LastBlockHeight()
	if headerHeight <= 0 {
		headerHeight = 1
	}
	ctx := a.BaseApp.NewUncachedContext(false, cmtproto.Header{
		ChainID: a.AppConfig.ChainID,
		Height:  headerHeight,
		Time:    time.Now().UTC(),
	})

	if err := a.ensureLocalhostTransferPath(ctx, request.SourcePort, request.SourceChannel, request.CounterpartyChannel); err != nil {
		return LocalhostTransferResult{}, err
	}
	if err := a.ensureTransferSenderBalance(ctx, sender, request.Coin); err != nil {
		return LocalhostTransferResult{}, err
	}
	a.MultiStore.Commit()

	msg := transfertypes.NewMsgTransfer(
		request.SourcePort,
		request.SourceChannel,
		request.Coin,
		request.Sender,
		request.Receiver,
		request.TimeoutHeight,
		request.TimeoutTimestamp,
		request.Memo,
	)
	txBuilder := a.TxConfig.NewTxBuilder()
	if err := txBuilder.SetMsgs(msg); err != nil {
		return LocalhostTransferResult{}, fmt.Errorf("set localhost transfer tx msg: %w", err)
	}
	txBytes, err := a.TxConfig.TxEncoder()(txBuilder.GetTx())
	if err != nil {
		return LocalhostTransferResult{}, fmt.Errorf("encode localhost transfer tx: %w", err)
	}
	finalizeResponse, err := a.BaseApp.FinalizeBlock(&abcitypes.RequestFinalizeBlock{
		Height: a.BaseApp.LastBlockHeight() + 1,
		Time:   time.Now().UTC(),
		Txs:    [][]byte{txBytes},
	})
	if err != nil {
		return LocalhostTransferResult{}, fmt.Errorf("finalize localhost transfer block: %w", err)
	}
	if len(finalizeResponse.TxResults) != 1 {
		return LocalhostTransferResult{}, fmt.Errorf("expected exactly one localhost transfer tx result, got %d", len(finalizeResponse.TxResults))
	}
	if finalizeResponse.TxResults[0].Code != 0 {
		return LocalhostTransferResult{}, fmt.Errorf("execute localhost transfer tx: %s", finalizeResponse.TxResults[0].Log)
	}
	if _, err := a.BaseApp.Commit(); err != nil {
		return LocalhostTransferResult{}, fmt.Errorf("commit localhost transfer block: %w", err)
	}

	ctx = a.BaseApp.NewUncachedContext(false, cmtproto.Header{
		ChainID: a.AppConfig.ChainID,
		Height:  a.BaseApp.LastBlockHeight(),
		Time:    time.Now().UTC(),
	})
	nextSequence, found := a.IBCKeeper.ChannelKeeper.GetNextSequenceSend(ctx, request.SourcePort, request.SourceChannel)
	if !found || nextSequence <= 1 {
		return LocalhostTransferResult{}, fmt.Errorf("localhost transfer next send sequence was not recorded for %s/%s", request.SourcePort, request.SourceChannel)
	}
	sequence := nextSequence - 1

	commitment := a.IBCKeeper.ChannelKeeper.GetPacketCommitment(ctx, request.SourcePort, request.SourceChannel, sequence)
	if len(commitment) == 0 {
		return LocalhostTransferResult{}, fmt.Errorf("missing packet commitment for %s/%s sequence %d", request.SourcePort, request.SourceChannel, sequence)
	}

	return LocalhostTransferResult{
		Sender:                request.Sender,
		PortID:                request.SourcePort,
		ChannelID:             request.SourceChannel,
		CounterpartyChannelID: request.CounterpartyChannel,
		Sequence:              sequence,
		PacketCommitment:      append([]byte(nil), commitment...),
	}, nil
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

func bootstrapFirstCommittedBlock(base *baseapp.BaseApp) error {
	if base == nil {
		return nil
	}
	if _, err := base.FinalizeBlock(&abcitypes.RequestFinalizeBlock{
		Height: 1,
		Time:   time.Now().UTC(),
	}); err != nil {
		return fmt.Errorf("finalize first committed block: %w", err)
	}
	if _, err := base.Commit(); err != nil {
		return fmt.Errorf("commit first block: %w", err)
	}
	return nil
}

func (a *ChainApp) normalizeLocalhostTransferRequest(req LocalhostTransferRequest) (LocalhostTransferRequest, sdk.AccAddress, error) {
	if a == nil || a.AccountKeeper.AddressCodec() == nil {
		return LocalhostTransferRequest{}, nil, fmt.Errorf("account keeper address codec is not initialized")
	}
	if strings.TrimSpace(req.Sender) == "" {
		sender, err := a.defaultLocalhostTransferSender()
		if err != nil {
			return LocalhostTransferRequest{}, nil, err
		}
		req.Sender = sender
	}
	senderBytes, err := a.AccountKeeper.AddressCodec().StringToBytes(strings.TrimSpace(req.Sender))
	if err != nil {
		return LocalhostTransferRequest{}, nil, fmt.Errorf("decode localhost transfer sender: %w", err)
	}
	if err := req.Coin.Validate(); err != nil {
		return LocalhostTransferRequest{}, nil, fmt.Errorf("invalid localhost transfer coin: %w", err)
	}
	if !req.Coin.IsPositive() {
		return LocalhostTransferRequest{}, nil, fmt.Errorf("localhost transfer amount must be positive")
	}
	if strings.TrimSpace(req.Receiver) == "" {
		return LocalhostTransferRequest{}, nil, fmt.Errorf("missing localhost transfer receiver")
	}
	if req.TimeoutHeight.IsZero() && req.TimeoutTimestamp == 0 {
		return LocalhostTransferRequest{}, nil, fmt.Errorf("missing localhost transfer timeout")
	}
	if strings.TrimSpace(req.SourcePort) == "" {
		req.SourcePort = DemoLocalhostTransferPortID
	}
	if strings.TrimSpace(req.SourceChannel) == "" {
		req.SourceChannel = DemoLocalhostTransferChannelID
	}
	if strings.TrimSpace(req.CounterpartyChannel) == "" {
		req.CounterpartyChannel = DemoLocalhostTransferCounterpartyChannel
	}

	req.Sender = strings.TrimSpace(req.Sender)
	req.Receiver = strings.TrimSpace(req.Receiver)
	req.SourcePort = strings.TrimSpace(req.SourcePort)
	req.SourceChannel = strings.TrimSpace(req.SourceChannel)
	req.CounterpartyChannel = strings.TrimSpace(req.CounterpartyChannel)

	return req, sdk.AccAddress(senderBytes), nil
}

func (a *ChainApp) ensureLocalhostTransferPath(ctx sdk.Context, sourcePort, sourceChannel, counterpartyChannel string) error {
	if a == nil {
		return fmt.Errorf("chain app is not initialized")
	}
	if _, found := a.IBCKeeper.ConnectionKeeper.GetConnection(ctx, ibcexported.LocalhostConnectionID); !found {
		a.IBCKeeper.ConnectionKeeper.CreateSentinelLocalhostConnection(ctx)
	}
	a.IBCKeeper.ConnectionKeeper.SetClientConnectionPaths(ctx, ibcexported.LocalhostClientID, []string{ibcexported.LocalhostConnectionID})

	if _, found := a.IBCKeeper.ChannelKeeper.GetChannel(ctx, sourcePort, sourceChannel); !found {
		channel := channeltypes.NewChannel(
			channeltypes.OPEN,
			channeltypes.UNORDERED,
			channeltypes.NewCounterparty(sourcePort, counterpartyChannel),
			[]string{ibcexported.LocalhostConnectionID},
			transfertypes.V1,
		)
		a.IBCKeeper.ChannelKeeper.SetChannel(ctx, sourcePort, sourceChannel, channel)
	}
	if _, found := a.IBCKeeper.ChannelKeeper.GetNextSequenceSend(ctx, sourcePort, sourceChannel); !found {
		a.IBCKeeper.ChannelKeeper.SetNextSequenceSend(ctx, sourcePort, sourceChannel, 1)
	}
	if _, found := a.IBCKeeper.ChannelKeeper.GetNextSequenceRecv(ctx, sourcePort, sourceChannel); !found {
		a.IBCKeeper.ChannelKeeper.SetNextSequenceRecv(ctx, sourcePort, sourceChannel, 1)
	}
	if _, found := a.IBCKeeper.ChannelKeeper.GetNextSequenceAck(ctx, sourcePort, sourceChannel); !found {
		a.IBCKeeper.ChannelKeeper.SetNextSequenceAck(ctx, sourcePort, sourceChannel, 1)
	}

	return nil
}

func (a *ChainApp) defaultLocalhostTransferSender() (string, error) {
	if a == nil || a.AccountKeeper.AddressCodec() == nil {
		return "", fmt.Errorf("account keeper address codec is not initialized")
	}
	return a.AccountKeeper.AddressCodec().BytesToString(bytesRepeat(0x42, 20))
}

func bytesRepeat(b byte, count int) []byte {
	buf := make([]byte, count)
	for i := range buf {
		buf[i] = b
	}
	return buf
}

func (a *ChainApp) ensureTransferSenderBalance(ctx sdk.Context, sender sdk.AccAddress, coin sdk.Coin) error {
	if a == nil {
		return fmt.Errorf("chain app is not initialized")
	}
	if account := a.AccountKeeper.GetAccount(ctx, sender); account == nil {
		a.AccountKeeper.SetAccount(ctx, a.AccountKeeper.NewAccountWithAddress(ctx, sender))
	}

	current := a.BankKeeper.GetBalance(ctx, sender, coin.Denom)
	if !current.Amount.LT(coin.Amount) {
		return nil
	}

	shortfall := coin.Amount.Sub(current.Amount)
	fundingCoin := sdk.NewCoin(coin.Denom, shortfall)
	if err := a.BankKeeper.MintCoins(ctx, transfertypes.ModuleName, sdk.NewCoins(fundingCoin)); err != nil {
		return fmt.Errorf("mint localhost transfer funding: %w", err)
	}
	if err := a.BankKeeper.SendCoinsFromModuleToAccount(ctx, transfertypes.ModuleName, sender, sdk.NewCoins(fundingCoin)); err != nil {
		return fmt.Errorf("fund localhost transfer sender: %w", err)
	}

	return nil
}
