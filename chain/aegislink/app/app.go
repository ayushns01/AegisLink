package app

import (
	bridgemodule "github.com/ayushns01/aegislink/chain/aegislink/x/bridge"
	bridgekeeper "github.com/ayushns01/aegislink/chain/aegislink/x/bridge/keeper"
	bridgetypes "github.com/ayushns01/aegislink/chain/aegislink/x/bridge/types"
	ibcroutermodule "github.com/ayushns01/aegislink/chain/aegislink/x/ibcrouter"
	ibcrouterkeeper "github.com/ayushns01/aegislink/chain/aegislink/x/ibcrouter/keeper"
	limitsmodule "github.com/ayushns01/aegislink/chain/aegislink/x/limits"
	limitskeeper "github.com/ayushns01/aegislink/chain/aegislink/x/limits/keeper"
	limittypes "github.com/ayushns01/aegislink/chain/aegislink/x/limits/types"
	pausermodule "github.com/ayushns01/aegislink/chain/aegislink/x/pauser"
	pauserkeeper "github.com/ayushns01/aegislink/chain/aegislink/x/pauser/keeper"
	registrymodule "github.com/ayushns01/aegislink/chain/aegislink/x/registry"
	registrykeeper "github.com/ayushns01/aegislink/chain/aegislink/x/registry/keeper"
	registrytypes "github.com/ayushns01/aegislink/chain/aegislink/x/registry/types"
	"math/big"
	"os"
	"sort"
	"strings"
)

type App struct {
	Config          Config
	BridgeKeeper    *bridgekeeper.Keeper
	IBCRouterKeeper *ibcrouterkeeper.Keeper
	RegistryKeeper  *registrykeeper.Keeper
	LimitsKeeper    *limitskeeper.Keeper
	PauserKeeper    *pauserkeeper.Keeper
	encoding        EncodingConfig
	storeKeys       map[string]string
	modules         []string
}

type Status struct {
	AppName            string            `json:"app_name"`
	ChainID            string            `json:"chain_id"`
	RuntimeMode        string            `json:"runtime_mode"`
	HomeDir            string            `json:"home_dir"`
	ConfigPath         string            `json:"config_path"`
	GenesisPath        string            `json:"genesis_path"`
	StatePath          string            `json:"state_path"`
	Initialized        bool              `json:"initialized"`
	ModuleNames        []string          `json:"module_names"`
	Modules            int               `json:"modules"`
	AllowedSigners     []string          `json:"allowed_signers"`
	EnabledRouteIDs    []string          `json:"enabled_route_ids"`
	RequiredThreshold  uint32            `json:"required_threshold"`
	CurrentHeight      uint64            `json:"current_height"`
	Assets             int               `json:"assets"`
	Limits             int               `json:"limits"`
	PausedFlows        int               `json:"paused_flows"`
	ProcessedClaims    int               `json:"processed_claims"`
	FailedClaims       uint64            `json:"failed_claims"`
	Withdrawals        int               `json:"withdrawals"`
	Routes             int               `json:"routes"`
	Transfers          int               `json:"transfers"`
	PendingTransfers   int               `json:"pending_transfers"`
	CompletedTransfers int               `json:"completed_transfers"`
	FailedTransfers    int               `json:"failed_transfers"`
	TimedOutTransfers  int               `json:"timed_out_transfers"`
	RefundedTransfers  int               `json:"refunded_transfers"`
	SupplyByDenom      map[string]string `json:"supply_by_denom"`
}

func New() *App {
	return NewWithConfig(DefaultConfig())
}

func NewWithConfig(cfg Config) *App {
	cfg = normalizeConfig(cfg)

	registryKeeper := registrykeeper.NewKeeper()
	limitsKeeper := limitskeeper.NewKeeper()
	pauserKeeper := pauserkeeper.NewKeeper()
	ibcRouterKeeper := ibcrouterkeeper.NewKeeper()
	bridgeKeeper := bridgekeeper.NewKeeper(registryKeeper, limitsKeeper, pauserKeeper, cfg.AllowedSigners, cfg.RequiredThreshold)

	bridgeAppModule := bridgemodule.NewAppModule(bridgeKeeper)
	registryAppModule := registrymodule.NewAppModule(registryKeeper)
	limitsAppModule := limitsmodule.NewAppModule(limitsKeeper)
	pauserAppModule := pausermodule.NewAppModule(pauserKeeper)
	ibcRouterAppModule := ibcroutermodule.NewAppModule(ibcRouterKeeper)
	modules := []string{
		bridgeAppModule.Name(),
		registryAppModule.Name(),
		limitsAppModule.Name(),
		pauserAppModule.Name(),
		ibcRouterAppModule.Name(),
	}

	return &App{
		Config:          cfg,
		BridgeKeeper:    bridgeKeeper,
		IBCRouterKeeper: ibcRouterKeeper,
		RegistryKeeper:  registryKeeper,
		LimitsKeeper:    limitsKeeper,
		PauserKeeper:    pauserKeeper,
		encoding:        DefaultEncodingConfig(modules),
		storeKeys:       defaultStoreKeys(modules),
		modules:         modules,
	}
}

func Load(statePath string) (*App, error) {
	cfg := DefaultConfig()
	cfg.StatePath = statePath
	return LoadWithConfig(cfg)
}

func LoadWithConfig(cfg Config) (*App, error) {
	resolved, err := ResolveConfig(cfg)
	if err != nil {
		return nil, err
	}
	app := NewWithConfig(resolved)
	state, err := loadRuntimeState(app.Config.StatePath)
	if err != nil {
		return nil, err
	}

	if err := app.RegistryKeeper.ImportAssets(state.Assets); err != nil {
		return nil, err
	}
	if err := app.LimitsKeeper.ImportLimits(state.Limits); err != nil {
		return nil, err
	}
	if err := app.PauserKeeper.ImportPausedFlows(state.PausedFlows); err != nil {
		return nil, err
	}
	if err := app.BridgeKeeper.ImportState(state.Bridge); err != nil {
		return nil, err
	}
	if err := app.IBCRouterKeeper.ImportState(state.IBCRouter); err != nil {
		return nil, err
	}

	return app, nil
}

func (a *App) ModuleNames() []string {
	modules := make([]string, len(a.modules))
	copy(modules, a.modules)
	return modules
}

func (a *App) StoreKeys() map[string]string {
	storeKeys := make(map[string]string, len(a.storeKeys))
	for moduleName, key := range a.storeKeys {
		storeKeys[moduleName] = key
	}
	return storeKeys
}

func (a *App) EncodingConfig() EncodingConfig {
	return a.encoding
}

func (a *App) DefaultGenesis() Genesis {
	return DefaultGenesis(a.Config)
}

func (a *App) SetCurrentHeight(height uint64) {
	a.BridgeKeeper.SetCurrentHeight(height)
}

func (a *App) RegisterAsset(asset registrytypes.Asset) error {
	return a.RegistryKeeper.RegisterAsset(asset)
}

func (a *App) SetLimit(limit limittypes.RateLimit) error {
	return a.LimitsKeeper.SetLimit(limit)
}

func (a *App) Pause(flow string) error {
	return a.PauserKeeper.Pause(flow)
}

func (a *App) Unpause(flow string) error {
	return a.PauserKeeper.Unpause(flow)
}

func (a *App) SubmitDepositClaim(claim bridgetypes.DepositClaim, attestation bridgetypes.Attestation) (bridgekeeper.ClaimResult, error) {
	return a.BridgeKeeper.ExecuteDepositClaim(claim, attestation)
}

func (a *App) ExecuteWithdrawal(assetID string, amount *big.Int, recipient string, deadline uint64, signature []byte) (bridgekeeper.WithdrawalRecord, error) {
	return a.BridgeKeeper.ExecuteWithdrawal(assetID, amount, recipient, deadline, signature)
}

func (a *App) Withdrawals(fromHeight, toHeight uint64) []bridgekeeper.WithdrawalRecord {
	return a.BridgeKeeper.Withdrawals(fromHeight, toHeight)
}

func (a *App) Routes() []ibcrouterkeeper.Route {
	return a.IBCRouterKeeper.ExportRoutes()
}

func (a *App) Transfers() []ibcrouterkeeper.TransferRecord {
	return a.IBCRouterKeeper.ExportTransfers()
}

func (a *App) InitiateIBCTransfer(assetID string, amount *big.Int, receiver string, timeoutHeight uint64, memo string) (ibcrouterkeeper.TransferRecord, error) {
	return a.IBCRouterKeeper.InitiateTransfer(assetID, amount, receiver, timeoutHeight, memo)
}

func (a *App) FailIBCTransfer(transferID, reason string) (ibcrouterkeeper.TransferRecord, error) {
	return a.IBCRouterKeeper.AcknowledgeFailure(transferID, reason)
}

func (a *App) TimeoutIBCTransfer(transferID string) (ibcrouterkeeper.TransferRecord, error) {
	return a.IBCRouterKeeper.TimeoutTransfer(transferID)
}

func (a *App) CompleteIBCTransfer(transferID string) (ibcrouterkeeper.TransferRecord, error) {
	return a.IBCRouterKeeper.AcknowledgeSuccess(transferID)
}

func (a *App) RefundIBCTransfer(transferID string) (ibcrouterkeeper.TransferRecord, error) {
	return a.IBCRouterKeeper.MarkRefunded(transferID)
}

func (a *App) Save() error {
	return persistRuntimeState(a.Config.StatePath, runtimeState{
		Assets:      a.RegistryKeeper.ExportAssets(),
		Limits:      a.LimitsKeeper.ExportLimits(),
		PausedFlows: a.PauserKeeper.ExportPausedFlows(),
		Bridge:      a.BridgeKeeper.ExportState(),
		IBCRouter:   a.IBCRouterKeeper.ExportState(),
	})
}

func (a *App) Status() Status {
	bridgeState := a.BridgeKeeper.ExportState()
	transfers := a.IBCRouterKeeper.ExportTransfers()
	routes := a.IBCRouterKeeper.ExportRoutes()

	status := Status{
		AppName:           a.Config.AppName,
		ChainID:           a.Config.ChainID,
		RuntimeMode:       a.Config.RuntimeMode,
		HomeDir:           a.Config.HomeDir,
		ConfigPath:        a.Config.ConfigPath,
		GenesisPath:       a.Config.GenesisPath,
		StatePath:         a.Config.StatePath,
		Initialized:       runtimeInitialized(a.Config),
		ModuleNames:       a.ModuleNames(),
		Modules:           len(a.modules),
		AllowedSigners:    append([]string(nil), a.Config.AllowedSigners...),
		EnabledRouteIDs:   enabledRouteIDs(routes),
		RequiredThreshold: a.Config.RequiredThreshold,
		CurrentHeight:     bridgeState.CurrentHeight,
		Assets:            len(a.RegistryKeeper.ExportAssets()),
		Limits:            len(a.LimitsKeeper.ExportLimits()),
		PausedFlows:       len(a.PauserKeeper.ExportPausedFlows()),
		ProcessedClaims:   len(bridgeState.ProcessedClaims),
		FailedClaims:      a.BridgeKeeper.RejectedClaims(),
		Withdrawals:       len(bridgeState.Withdrawals),
		Routes:            len(a.IBCRouterKeeper.ExportRoutes()),
		Transfers:         len(transfers),
		SupplyByDenom:     bridgeState.SupplyByDenom,
	}

	for _, transfer := range transfers {
		switch transfer.Status {
		case ibcrouterkeeper.TransferStatusPending:
			status.PendingTransfers++
		case ibcrouterkeeper.TransferStatusCompleted:
			status.CompletedTransfers++
		case ibcrouterkeeper.TransferStatusAckFailed:
			status.FailedTransfers++
		case ibcrouterkeeper.TransferStatusTimedOut:
			status.TimedOutTransfers++
		case ibcrouterkeeper.TransferStatusRefunded:
			status.RefundedTransfers++
		}
	}

	return status
}

func enabledRouteIDs(routes []ibcrouterkeeper.Route) []string {
	ids := make([]string, 0, len(routes))
	for _, route := range routes {
		if !route.Enabled {
			continue
		}
		ids = append(ids, routeID(route))
	}
	sort.Strings(ids)
	return ids
}

func routeID(route ibcrouterkeeper.Route) string {
	return route.AssetID + "@" + route.DestinationChainID + ":" + route.ChannelID
}

func defaultStoreKeys(modules []string) map[string]string {
	keys := make(map[string]string, len(modules))
	for _, moduleName := range modules {
		keys[moduleName] = AppName + "/" + moduleName
	}
	return keys
}

func normalizeConfig(cfg Config) Config {
	defaults := DefaultConfig()
	if cfg.AppName == "" {
		cfg.AppName = defaults.AppName
	}
	if cfg.ChainID == "" {
		cfg.ChainID = defaults.ChainID
	}
	if cfg.RuntimeMode == "" {
		cfg.RuntimeMode = defaults.RuntimeMode
	}
	if cfg.HomeDir == "" {
		cfg.HomeDir = defaults.HomeDir
	}
	if cfg.ConfigPath == "" {
		cfg.ConfigPath = runtimeConfigPath(cfg.HomeDir)
	}
	if cfg.GenesisPath == "" {
		cfg.GenesisPath = runtimeGenesisPath(cfg.HomeDir)
	}
	if cfg.StatePath == "" {
		cfg.StatePath = runtimeStatePath(cfg.HomeDir)
	}
	if len(cfg.Modules) == 0 {
		cfg.Modules = append([]string(nil), defaults.Modules...)
	}
	if len(cfg.AllowedSigners) == 0 {
		cfg.AllowedSigners = append([]string(nil), defaults.AllowedSigners...)
	}
	if cfg.RequiredThreshold == 0 {
		cfg.RequiredThreshold = defaults.RequiredThreshold
	}
	return cfg
}

func runtimeInitialized(cfg Config) bool {
	for _, path := range []string{cfg.ConfigPath, cfg.GenesisPath, cfg.StatePath} {
		if strings.TrimSpace(path) == "" {
			return false
		}
		if _, err := os.Stat(path); err != nil {
			return false
		}
	}
	return true
}
