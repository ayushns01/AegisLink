package app

import (
	bridgemodule "github.com/ayushns01/aegislink/chain/aegislink/x/bridge"
	bridgekeeper "github.com/ayushns01/aegislink/chain/aegislink/x/bridge/keeper"
	bridgetypes "github.com/ayushns01/aegislink/chain/aegislink/x/bridge/types"
	governancemodule "github.com/ayushns01/aegislink/chain/aegislink/x/governance"
	governancekeeper "github.com/ayushns01/aegislink/chain/aegislink/x/governance/keeper"
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
	"sync"
)

type App struct {
	mu                   sync.RWMutex
	Config               Config
	BridgeKeeper         *bridgekeeper.Keeper
	IBCRouterKeeper      *ibcrouterkeeper.Keeper
	RegistryKeeper       *registrykeeper.Keeper
	LimitsKeeper         *limitskeeper.Keeper
	PauserKeeper         *pauserkeeper.Keeper
	GovernanceKeeper     *governancekeeper.Keeper
	encoding             EncodingConfig
	storeKeys            map[string]string
	storeRuntime         *storeRuntime
	modules              []string
	pendingDepositClaims []QueuedDepositClaim
}

type Status struct {
	AppName                string            `json:"app_name"`
	ChainID                string            `json:"chain_id"`
	RuntimeMode            string            `json:"runtime_mode"`
	HomeDir                string            `json:"home_dir"`
	ConfigPath             string            `json:"config_path"`
	GenesisPath            string            `json:"genesis_path"`
	StatePath              string            `json:"state_path"`
	Initialized            bool              `json:"initialized"`
	ModuleNames            []string          `json:"module_names"`
	Modules                int               `json:"modules"`
	AllowedSigners         []string          `json:"allowed_signers"`
	GovernanceAuthorities  []string          `json:"governance_authorities"`
	ActiveSignerSetVersion uint64            `json:"active_signer_set_version"`
	ActiveSignerThreshold  uint32            `json:"active_signer_threshold"`
	SignerSetCount         int               `json:"signer_set_count"`
	SignerSetVersions      []uint64          `json:"signer_set_versions"`
	EnabledRouteIDs        []string          `json:"enabled_route_ids"`
	RequiredThreshold      uint32            `json:"required_threshold"`
	CurrentHeight          uint64            `json:"current_height"`
	Assets                 int               `json:"assets"`
	Limits                 int               `json:"limits"`
	PausedFlows            int               `json:"paused_flows"`
	ProcessedClaims        int               `json:"processed_claims"`
	FailedClaims           uint64            `json:"failed_claims"`
	BridgeCircuitOpen      bool              `json:"bridge_circuit_open"`
	LastInvariantError     string            `json:"last_invariant_error"`
	PendingDepositClaims   int               `json:"pending_deposit_claims"`
	Withdrawals            int               `json:"withdrawals"`
	Routes                 int               `json:"routes"`
	Transfers              int               `json:"transfers"`
	PendingTransfers       int               `json:"pending_transfers"`
	CompletedTransfers     int               `json:"completed_transfers"`
	FailedTransfers        int               `json:"failed_transfers"`
	TimedOutTransfers      int               `json:"timed_out_transfers"`
	RefundedTransfers      int               `json:"refunded_transfers"`
	GovernanceProposals    int               `json:"governance_proposals"`
	SupplyByDenom          map[string]string `json:"supply_by_denom"`
}

func New() *App {
	app, err := NewWithConfig(DefaultConfig())
	if err != nil {
		panic(err)
	}
	return app
}

func NewWithConfig(cfg Config) (*App, error) {
	cfg = normalizeConfig(cfg)
	if cfg.RuntimeMode == RuntimeModeSDKStore {
		return newStoreBackedApp(cfg)
	}

	registryKeeper := registrykeeper.NewKeeper()
	limitsKeeper := limitskeeper.NewKeeper()
	pauserKeeper := pauserkeeper.NewKeeper()
	ibcRouterKeeper := ibcrouterkeeper.NewKeeper()
	governanceKeeper := governancekeeper.NewKeeper(registryKeeper, limitsKeeper, ibcRouterKeeper, cfg.GovernanceAuthorities)
	bridgeKeeper := bridgekeeper.NewKeeper(registryKeeper, limitsKeeper, pauserKeeper, cfg.AllowedSigners, cfg.RequiredThreshold)

	return newAppFromKeepers(cfg, bridgeKeeper, ibcRouterKeeper, registryKeeper, limitsKeeper, pauserKeeper, governanceKeeper), nil
}

func newAppFromKeepers(
	cfg Config,
	bridgeKeeper *bridgekeeper.Keeper,
	ibcRouterKeeper *ibcrouterkeeper.Keeper,
	registryKeeper *registrykeeper.Keeper,
	limitsKeeper *limitskeeper.Keeper,
	pauserKeeper *pauserkeeper.Keeper,
	governanceKeeper *governancekeeper.Keeper,
) *App {
	bridgeAppModule := bridgemodule.NewAppModule(bridgeKeeper)
	registryAppModule := registrymodule.NewAppModule(registryKeeper)
	limitsAppModule := limitsmodule.NewAppModule(limitsKeeper)
	pauserAppModule := pausermodule.NewAppModule(pauserKeeper)
	ibcRouterAppModule := ibcroutermodule.NewAppModule(ibcRouterKeeper)
	governanceAppModule := governancemodule.NewAppModule(governanceKeeper)
	modules := []string{
		bridgeAppModule.Name(),
		registryAppModule.Name(),
		limitsAppModule.Name(),
		pauserAppModule.Name(),
		ibcRouterAppModule.Name(),
		governanceAppModule.Name(),
	}

	return &App{
		Config:           cfg,
		BridgeKeeper:     bridgeKeeper,
		IBCRouterKeeper:  ibcRouterKeeper,
		RegistryKeeper:   registryKeeper,
		LimitsKeeper:     limitsKeeper,
		PauserKeeper:     pauserKeeper,
		GovernanceKeeper: governanceKeeper,
		encoding:         DefaultEncodingConfig(modules),
		storeKeys:        defaultStoreKeys(modules),
		modules:          modules,
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
	app, err := NewWithConfig(resolved)
	if err != nil {
		return nil, err
	}
	if resolved.RuntimeMode == RuntimeModeSDKStore {
		nodeState, err := loadRuntimeNodeState(resolved)
		if err != nil {
			return nil, err
		}
		app.pendingDepositClaims = append([]QueuedDepositClaim(nil), nodeState.PendingClaims...)
		return app, nil
	}

	state, err := loadRuntimeState(app.Config.StatePath)
	if err != nil {
		return nil, err
	}

	if err := app.RegistryKeeper.ImportAssets(state.Assets); err != nil {
		return nil, err
	}
	if err := app.LimitsKeeper.ImportState(state.Limits); err != nil {
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
	if err := app.GovernanceKeeper.ImportState(state.Governance); err != nil {
		return nil, err
	}
	app.pendingDepositClaims = append([]QueuedDepositClaim(nil), state.PendingClaims...)

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
	a.mu.Lock()
	defer a.mu.Unlock()
	a.BridgeKeeper.SetCurrentHeight(height)
}

func (a *App) QueueDepositClaim(claim bridgetypes.DepositClaim, attestation bridgetypes.Attestation) error {
	if err := claim.ValidateBasic(); err != nil {
		return err
	}
	if err := attestation.ValidateBasic(); err != nil {
		return err
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	currentHeight := a.BridgeKeeper.ExportState().CurrentHeight
	a.pendingDepositClaims = append(a.pendingDepositClaims, QueuedDepositClaim{
		Claim:            claim,
		Attestation:      attestation,
		EnqueuedAtHeight: currentHeight,
	})
	return nil
}

func (a *App) AdvanceBlock() BlockProgress {
	a.mu.Lock()
	defer a.mu.Unlock()

	bridgeState := a.BridgeKeeper.ExportState()
	nextHeight := bridgeState.CurrentHeight + 1
	a.BridgeKeeper.SetCurrentHeight(nextHeight)

	progress := BlockProgress{
		Height:              nextHeight,
		BridgeCurrentHeight: nextHeight,
	}
	if len(a.pendingDepositClaims) == 0 {
		return progress
	}

	pending := append([]QueuedDepositClaim(nil), a.pendingDepositClaims...)
	a.pendingDepositClaims = nil
	for _, submission := range pending {
		if _, err := a.submitDepositClaimLocked(submission.Claim, submission.Attestation); err != nil {
			progress.LastSubmissionMessage = err.Error()
			continue
		}
		progress.AppliedQueuedClaims++
		progress.LastSubmissionMessage = submission.Claim.Identity.MessageID
	}
	progress.PendingQueuedClaims = len(a.pendingDepositClaims)
	return progress
}

func (a *App) RegisterAsset(asset registrytypes.Asset) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.RegistryKeeper.RegisterAsset(asset)
}

func (a *App) SetLimit(limit limittypes.RateLimit) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.LimitsKeeper.SetLimit(limit)
}

func (a *App) Pause(flow string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.PauserKeeper.Pause(flow)
}

func (a *App) Unpause(flow string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.PauserKeeper.Unpause(flow)
}

func (a *App) SubmitDepositClaim(claim bridgetypes.DepositClaim, attestation bridgetypes.Attestation) (bridgekeeper.ClaimResult, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.submitDepositClaimLocked(claim, attestation)
}

func (a *App) submitDepositClaimLocked(claim bridgetypes.DepositClaim, attestation bridgetypes.Attestation) (bridgekeeper.ClaimResult, error) {
	return a.BridgeKeeper.ExecuteDepositClaim(claim, attestation)
}

func (a *App) ExecuteWithdrawal(assetID string, amount *big.Int, recipient string, deadline uint64, signature []byte) (bridgekeeper.WithdrawalRecord, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.BridgeKeeper.ExecuteWithdrawal(assetID, amount, recipient, deadline, signature)
}

func (a *App) Withdrawals(fromHeight, toHeight uint64) []bridgekeeper.WithdrawalRecord {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.BridgeKeeper.Withdrawals(fromHeight, toHeight)
}

func (a *App) ActiveSignerSet() (bridgekeeper.SignerSet, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.BridgeKeeper.ActiveSignerSet()
}

func (a *App) SignerSets() []bridgekeeper.SignerSet {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.BridgeKeeper.ExportSignerSets()
}

func (a *App) Routes() []ibcrouterkeeper.Route {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.IBCRouterKeeper.ExportRoutes()
}

func (a *App) SetRoute(route ibcrouterkeeper.Route) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.IBCRouterKeeper.SetRoute(route)
}

func (a *App) Transfers() []ibcrouterkeeper.TransferRecord {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.IBCRouterKeeper.ExportTransfers()
}

func (a *App) InitiateIBCTransfer(assetID string, amount *big.Int, receiver string, timeoutHeight uint64, memo string) (ibcrouterkeeper.TransferRecord, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.IBCRouterKeeper.InitiateTransfer(assetID, amount, receiver, timeoutHeight, memo)
}

func (a *App) FailIBCTransfer(transferID, reason string) (ibcrouterkeeper.TransferRecord, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.IBCRouterKeeper.AcknowledgeFailure(transferID, reason)
}

func (a *App) TimeoutIBCTransfer(transferID string) (ibcrouterkeeper.TransferRecord, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.IBCRouterKeeper.TimeoutTransfer(transferID)
}

func (a *App) CompleteIBCTransfer(transferID string) (ibcrouterkeeper.TransferRecord, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.IBCRouterKeeper.AcknowledgeSuccess(transferID)
}

func (a *App) RefundIBCTransfer(transferID string) (ibcrouterkeeper.TransferRecord, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.IBCRouterKeeper.MarkRefunded(transferID)
}

func (a *App) ApplyAssetStatusProposal(authority string, proposal governancekeeper.AssetStatusProposal) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.GovernanceKeeper.ApplyAssetStatusProposal(authority, proposal)
}

func (a *App) ApplyLimitUpdateProposal(authority string, proposal governancekeeper.LimitUpdateProposal) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.GovernanceKeeper.ApplyLimitUpdateProposal(authority, proposal)
}

func (a *App) ApplyRoutePolicyUpdateProposal(authority string, proposal governancekeeper.RoutePolicyUpdateProposal) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.GovernanceKeeper.ApplyRoutePolicyUpdateProposal(authority, proposal)
}

func (a *App) Save() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.Config.RuntimeMode == RuntimeModeSDKStore {
		for _, flush := range []func() error{
			a.RegistryKeeper.Flush,
			a.LimitsKeeper.Flush,
			a.PauserKeeper.Flush,
			a.BridgeKeeper.Flush,
			a.IBCRouterKeeper.Flush,
			a.GovernanceKeeper.Flush,
		} {
			if err := flush(); err != nil {
				return err
			}
		}
		if a.storeRuntime != nil && a.storeRuntime.multi != nil {
			a.storeRuntime.multi.Commit()
		}
		return persistRuntimeNodeState(a.Config, a.pendingDepositClaims)
	}
	return persistRuntimeState(a.Config.StatePath, runtimeState{
		Assets:        a.RegistryKeeper.ExportAssets(),
		Limits:        a.LimitsKeeper.ExportState(),
		PausedFlows:   a.PauserKeeper.ExportPausedFlows(),
		PendingClaims: append([]QueuedDepositClaim(nil), a.pendingDepositClaims...),
		Bridge:        a.BridgeKeeper.ExportState(),
		IBCRouter:     a.IBCRouterKeeper.ExportState(),
		Governance:    a.GovernanceKeeper.ExportState(),
	})
}

func (a *App) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.Config.RuntimeMode == RuntimeModeSDKStore {
		for _, flush := range []func() error{
			a.RegistryKeeper.Flush,
			a.LimitsKeeper.Flush,
			a.PauserKeeper.Flush,
			a.BridgeKeeper.Flush,
			a.IBCRouterKeeper.Flush,
			a.GovernanceKeeper.Flush,
		} {
			if err := flush(); err != nil {
				return err
			}
		}
		if a.storeRuntime != nil && a.storeRuntime.multi != nil {
			a.storeRuntime.multi.Commit()
		}
		if err := persistRuntimeNodeState(a.Config, a.pendingDepositClaims); err != nil {
			return err
		}
	}
	if a.storeRuntime == nil || a.storeRuntime.db == nil {
		return nil
	}
	return a.storeRuntime.db.Close()
}

func (a *App) Status() Status {
	a.mu.RLock()
	defer a.mu.RUnlock()
	bridgeState := a.BridgeKeeper.ExportState()
	transfers := a.IBCRouterKeeper.ExportTransfers()
	routes := a.IBCRouterKeeper.ExportRoutes()

	status := Status{
		AppName:               a.Config.AppName,
		ChainID:               a.Config.ChainID,
		RuntimeMode:           a.Config.RuntimeMode,
		HomeDir:               a.Config.HomeDir,
		ConfigPath:            a.Config.ConfigPath,
		GenesisPath:           a.Config.GenesisPath,
		StatePath:             a.Config.StatePath,
		Initialized:           runtimeInitialized(a.Config),
		ModuleNames:           a.ModuleNames(),
		Modules:               len(a.modules),
		AllowedSigners:        append([]string(nil), a.Config.AllowedSigners...),
		GovernanceAuthorities: append([]string(nil), a.Config.GovernanceAuthorities...),
		EnabledRouteIDs:       enabledRouteIDs(routes),
		RequiredThreshold:     a.Config.RequiredThreshold,
		CurrentHeight:         bridgeState.CurrentHeight,
		Assets:                len(a.RegistryKeeper.ExportAssets()),
		Limits:                len(a.LimitsKeeper.ExportLimits()),
		PausedFlows:           len(a.PauserKeeper.ExportPausedFlows()),
		ProcessedClaims:       len(bridgeState.ProcessedClaims),
		FailedClaims:          a.BridgeKeeper.RejectedClaims(),
		BridgeCircuitOpen:     a.BridgeKeeper.CircuitBreakerTripped(),
		LastInvariantError:    a.BridgeKeeper.LastInvariantError(),
		PendingDepositClaims:  len(a.pendingDepositClaims),
		Withdrawals:           len(bridgeState.Withdrawals),
		Routes:                len(a.IBCRouterKeeper.ExportRoutes()),
		Transfers:             len(transfers),
		GovernanceProposals:   len(a.GovernanceKeeper.ExportState().AppliedProposals),
		SupplyByDenom:         bridgeState.SupplyByDenom,
	}

	signerSets := a.BridgeKeeper.ExportSignerSets()
	status.SignerSetCount = len(signerSets)
	status.SignerSetVersions = make([]uint64, 0, len(signerSets))
	for _, signerSet := range signerSets {
		status.SignerSetVersions = append(status.SignerSetVersions, signerSet.Version)
	}
	if activeSignerSet, err := a.BridgeKeeper.ActiveSignerSet(); err == nil {
		status.ActiveSignerSetVersion = activeSignerSet.Version
		status.ActiveSignerThreshold = activeSignerSet.Threshold
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
		if cfg.RuntimeMode == RuntimeModeSDKStore {
			cfg.StatePath = runtimeStorePath(cfg.HomeDir)
		} else {
			cfg.StatePath = runtimeStatePath(cfg.HomeDir)
		}
	}
	if len(cfg.Modules) == 0 {
		cfg.Modules = append([]string(nil), defaults.Modules...)
	}
	if len(cfg.AllowedSigners) == 0 {
		cfg.AllowedSigners = append([]string(nil), defaults.AllowedSigners...)
	}
	if len(cfg.GovernanceAuthorities) == 0 {
		cfg.GovernanceAuthorities = append([]string(nil), defaults.GovernanceAuthorities...)
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
