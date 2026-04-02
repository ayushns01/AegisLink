package app

import (
	bridgemodule "github.com/ayushns01/aegislink/chain/aegislink/x/bridge"
	bridgekeeper "github.com/ayushns01/aegislink/chain/aegislink/x/bridge/keeper"
	bridgetypes "github.com/ayushns01/aegislink/chain/aegislink/x/bridge/types"
	limitsmodule "github.com/ayushns01/aegislink/chain/aegislink/x/limits"
	limitskeeper "github.com/ayushns01/aegislink/chain/aegislink/x/limits/keeper"
	limittypes "github.com/ayushns01/aegislink/chain/aegislink/x/limits/types"
	pausermodule "github.com/ayushns01/aegislink/chain/aegislink/x/pauser"
	pauserkeeper "github.com/ayushns01/aegislink/chain/aegislink/x/pauser/keeper"
	registrymodule "github.com/ayushns01/aegislink/chain/aegislink/x/registry"
	registrykeeper "github.com/ayushns01/aegislink/chain/aegislink/x/registry/keeper"
	registrytypes "github.com/ayushns01/aegislink/chain/aegislink/x/registry/types"
	"math/big"
)

type App struct {
	Config         Config
	BridgeKeeper   *bridgekeeper.Keeper
	RegistryKeeper *registrykeeper.Keeper
	LimitsKeeper   *limitskeeper.Keeper
	PauserKeeper   *pauserkeeper.Keeper
	modules        []string
}

func New() *App {
	return NewWithConfig(DefaultConfig())
}

func NewWithConfig(cfg Config) *App {
	cfg = normalizeConfig(cfg)

	registryKeeper := registrykeeper.NewKeeper()
	limitsKeeper := limitskeeper.NewKeeper()
	pauserKeeper := pauserkeeper.NewKeeper()
	bridgeKeeper := bridgekeeper.NewKeeper(registryKeeper, limitsKeeper, pauserKeeper, cfg.AllowedSigners, cfg.RequiredThreshold)

	bridgeAppModule := bridgemodule.NewAppModule(bridgeKeeper)
	registryAppModule := registrymodule.NewAppModule(registryKeeper)
	limitsAppModule := limitsmodule.NewAppModule(limitsKeeper)
	pauserAppModule := pausermodule.NewAppModule(pauserKeeper)

	return &App{
		Config:         cfg,
		BridgeKeeper:   bridgeKeeper,
		RegistryKeeper: registryKeeper,
		LimitsKeeper:   limitsKeeper,
		PauserKeeper:   pauserKeeper,
		modules: []string{
			bridgeAppModule.Name(),
			registryAppModule.Name(),
			limitsAppModule.Name(),
			pauserAppModule.Name(),
		},
	}
}

func Load(statePath string) (*App, error) {
	cfg := DefaultConfig()
	cfg.StatePath = statePath
	return LoadWithConfig(cfg)
}

func LoadWithConfig(cfg Config) (*App, error) {
	app := NewWithConfig(cfg)
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

	return app, nil
}

func (a *App) ModuleNames() []string {
	modules := make([]string, len(a.modules))
	copy(modules, a.modules)
	return modules
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

func (a *App) Save() error {
	return persistRuntimeState(a.Config.StatePath, runtimeState{
		Assets:      a.RegistryKeeper.ExportAssets(),
		Limits:      a.LimitsKeeper.ExportLimits(),
		PausedFlows: a.PauserKeeper.ExportPausedFlows(),
		Bridge:      a.BridgeKeeper.ExportState(),
	})
}

func normalizeConfig(cfg Config) Config {
	defaults := DefaultConfig()
	if cfg.AppName == "" {
		cfg.AppName = defaults.AppName
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
