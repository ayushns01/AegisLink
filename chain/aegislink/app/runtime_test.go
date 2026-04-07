package app

import (
	"errors"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"

	bridgekeeper "github.com/ayushns01/aegislink/chain/aegislink/x/bridge/keeper"
	bridgetypes "github.com/ayushns01/aegislink/chain/aegislink/x/bridge/types"
	ibcrouterkeeper "github.com/ayushns01/aegislink/chain/aegislink/x/ibcrouter/keeper"
	limittypes "github.com/ayushns01/aegislink/chain/aegislink/x/limits/types"
	registrytypes "github.com/ayushns01/aegislink/chain/aegislink/x/registry/types"
)

func TestSaveAndLoadPreservesBridgeRuntimeState(t *testing.T) {
	t.Parallel()

	statePath := filepath.Join(t.TempDir(), "aegislink-state.json")
	app, err := NewWithConfig(Config{
		AppName:           AppName,
		Modules:           []string{"bridge", "registry", "limits", "pauser", "governance"},
		StatePath:         statePath,
		AllowedSigners:    []string{"relayer-1", "relayer-2", "relayer-3"},
		RequiredThreshold: 2,
	})
	if err != nil {
		t.Fatalf("new app: %v", err)
	}

	asset := registrytypes.Asset{
		AssetID:        "eth.usdc",
		SourceChainID:  "11155111",
		SourceContract: "0xasset",
		Denom:          "uethusdc",
		Decimals:       6,
		DisplayName:    "USDC",
		Enabled:        true,
	}
	if err := app.RegisterAsset(asset); err != nil {
		t.Fatalf("register asset: %v", err)
	}
	if err := app.SetLimit(limittypes.RateLimit{
		AssetID:       asset.AssetID,
		WindowSeconds: 600,
		MaxAmount:     mustAmount(t, "1000000000000000000"),
	}); err != nil {
		t.Fatalf("set limit: %v", err)
	}
	if err := app.Pause("maintenance"); err != nil {
		t.Fatalf("pause maintenance flow: %v", err)
	}
	if err := app.IBCRouterKeeper.SetRoute(ibcrouterkeeper.Route{
		AssetID:            asset.AssetID,
		DestinationChainID: "osmosis-1",
		ChannelID:          "channel-0",
		DestinationDenom:   "ibc/usdc",
		Enabled:            true,
	}); err != nil {
		t.Fatalf("set route: %v", err)
	}

	claim := validDepositClaim(t)
	attestation := validAttestationForClaim(claim)
	app.SetCurrentHeight(50)
	if _, err := app.SubmitDepositClaim(claim, attestation); err != nil {
		t.Fatalf("submit deposit: %v", err)
	}

	app.SetCurrentHeight(60)
	withdrawal, err := app.ExecuteWithdrawal(claim.AssetID, claim.Amount, "0xrecipient", 120, []byte("threshold-proof"))
	if err != nil {
		t.Fatalf("execute withdrawal: %v", err)
	}
	transfer, err := app.IBCRouterKeeper.InitiateTransfer(asset.AssetID, mustAmount(t, "50000000"), "osmo1recipient", 140, "swap")
	if err != nil {
		t.Fatalf("initiate ibc transfer: %v", err)
	}
	if _, err := app.IBCRouterKeeper.AcknowledgeFailure(transfer.TransferID, "ack failed"); err != nil {
		t.Fatalf("ack failure: %v", err)
	}
	if _, err := app.IBCRouterKeeper.MarkRefunded(transfer.TransferID); err != nil {
		t.Fatalf("mark refunded: %v", err)
	}

	if err := app.Save(); err != nil {
		t.Fatalf("save state: %v", err)
	}

	loaded, err := Load(statePath)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}

	if _, ok := loaded.RegistryKeeper.GetAsset(asset.AssetID); !ok {
		t.Fatalf("expected registered asset to persist")
	}
	if _, ok := loaded.LimitsKeeper.GetLimit(asset.AssetID); !ok {
		t.Fatalf("expected rate limit to persist")
	}
	if !loaded.PauserKeeper.IsPaused("maintenance") {
		t.Fatalf("expected paused maintenance flow to persist")
	}
	route, ok := loaded.IBCRouterKeeper.GetRoute(asset.AssetID)
	if !ok {
		t.Fatalf("expected ibc route to persist")
	}
	if route.ChannelID != "channel-0" {
		t.Fatalf("expected route channel channel-0, got %q", route.ChannelID)
	}
	if supply := loaded.BridgeKeeper.SupplyForDenom(asset.Denom); supply.Sign() != 0 {
		t.Fatalf("expected burned supply to persist as zero, got %s", supply.String())
	}

	withdrawals := loaded.Withdrawals(60, 60)
	if len(withdrawals) != 1 {
		t.Fatalf("expected one persisted withdrawal, got %d", len(withdrawals))
	}
	if withdrawals[0].Identity.MessageID != withdrawal.Identity.MessageID {
		t.Fatalf("expected persisted withdrawal %q, got %q", withdrawal.Identity.MessageID, withdrawals[0].Identity.MessageID)
	}
	transfers := loaded.IBCRouterKeeper.ExportTransfers()
	if len(transfers) != 1 {
		t.Fatalf("expected one persisted ibc transfer, got %d", len(transfers))
	}
	if transfers[0].Status != ibcrouterkeeper.TransferStatusRefunded {
		t.Fatalf("expected refunded ibc transfer status, got %q", transfers[0].Status)
	}

	loaded.SetCurrentHeight(60)
	if _, err := loaded.SubmitDepositClaim(claim, attestation); !errors.Is(err, bridgekeeper.ErrDuplicateClaim) {
		t.Fatalf("expected duplicate claim rejection after reload, got %v", err)
	}
}

func TestStoreRuntimePreservesBridgeStateAcrossReload(t *testing.T) {
	t.Parallel()

	storePath := filepath.Join(t.TempDir(), "aegislink-store")
	app, err := NewWithConfig(Config{
		AppName:           AppName,
		RuntimeMode:       RuntimeModeSDKStore,
		Modules:           []string{"bridge", "registry", "limits", "pauser", "ibcrouter", "governance"},
		StatePath:         storePath,
		AllowedSigners:    []string{"relayer-1", "relayer-2", "relayer-3"},
		RequiredThreshold: 2,
	})
	if err != nil {
		t.Fatalf("new store runtime app: %v", err)
	}

	asset := registrytypes.Asset{
		AssetID:        "eth.usdc",
		SourceChainID:  "11155111",
		SourceContract: "0xasset",
		Denom:          "uethusdc",
		Decimals:       6,
		DisplayName:    "USDC",
		Enabled:        true,
	}
	if err := app.RegisterAsset(asset); err != nil {
		t.Fatalf("register asset: %v", err)
	}
	if err := app.SetLimit(limittypes.RateLimit{
		AssetID:       asset.AssetID,
		WindowSeconds: 600,
		MaxAmount:     mustAmount(t, "1000000000000000000"),
	}); err != nil {
		t.Fatalf("set limit: %v", err)
	}
	if err := app.IBCRouterKeeper.SetRoute(ibcrouterkeeper.Route{
		AssetID:            asset.AssetID,
		DestinationChainID: "osmosis-1",
		ChannelID:          "channel-0",
		DestinationDenom:   "ibc/usdc",
		Enabled:            true,
	}); err != nil {
		t.Fatalf("set route: %v", err)
	}

	claim := validDepositClaim(t)
	attestation := validAttestationForClaim(claim)
	app.SetCurrentHeight(50)
	if _, err := app.SubmitDepositClaim(claim, attestation); err != nil {
		t.Fatalf("submit deposit: %v", err)
	}
	if err := app.Close(); err != nil {
		t.Fatalf("close store runtime app: %v", err)
	}

	reloaded, err := LoadWithConfig(Config{
		RuntimeMode: RuntimeModeSDKStore,
		StatePath:   storePath,
		Modules:     []string{"bridge", "registry", "limits", "pauser", "ibcrouter", "governance"},
	})
	if err != nil {
		t.Fatalf("reload store runtime: %v", err)
	}
	defer reloaded.Close()

	if supply := reloaded.BridgeKeeper.SupplyForDenom(asset.Denom); supply.Cmp(claim.Amount) != 0 {
		t.Fatalf("expected persisted supply %s, got %s", claim.Amount.String(), supply.String())
	}
	if _, ok := reloaded.RegistryKeeper.GetAsset(asset.AssetID); !ok {
		t.Fatalf("expected registered asset after reload")
	}
	if len(reloaded.IBCRouterKeeper.ExportRoutes()) != 1 {
		t.Fatalf("expected one route after reload, got %d", len(reloaded.IBCRouterKeeper.ExportRoutes()))
	}
}

func TestSDKHomeRuntimePreservesIBCRouteAcrossReload(t *testing.T) {
	t.Parallel()

	homeDir := filepath.Join(t.TempDir(), "sdk-home")
	if _, err := InitHome(Config{
		HomeDir:     homeDir,
		RuntimeMode: RuntimeModeSDKStore,
	}, false); err != nil {
		t.Fatalf("init sdk home: %v", err)
	}

	app, err := LoadWithConfig(Config{
		HomeDir:     homeDir,
		RuntimeMode: RuntimeModeSDKStore,
	})
	if err != nil {
		t.Fatalf("load sdk home runtime: %v", err)
	}

	asset := registrytypes.Asset{
		AssetID:        "eth.usdc",
		SourceChainID:  "11155111",
		SourceContract: "0xasset",
		Denom:          "uethusdc",
		Decimals:       6,
		DisplayName:    "USDC",
		Enabled:        true,
	}
	if err := app.RegisterAsset(asset); err != nil {
		t.Fatalf("register asset: %v", err)
	}
	if err := app.SetLimit(limittypes.RateLimit{
		AssetID:       asset.AssetID,
		WindowSeconds: 600,
		MaxAmount:     mustAmount(t, "1000000000000000000"),
	}); err != nil {
		t.Fatalf("set limit: %v", err)
	}
	if err := app.IBCRouterKeeper.SetRoute(ibcrouterkeeper.Route{
		AssetID:            asset.AssetID,
		DestinationChainID: "osmo-local-1",
		ChannelID:          "channel-0",
		DestinationDenom:   "ibc/uethusdc",
		Enabled:            true,
	}); err != nil {
		t.Fatalf("set route: %v", err)
	}
	if err := app.Save(); err != nil {
		t.Fatalf("save sdk home runtime: %v", err)
	}
	if err := app.Close(); err != nil {
		t.Fatalf("close sdk home runtime: %v", err)
	}

	reloaded, err := LoadWithConfig(Config{
		HomeDir:     homeDir,
		RuntimeMode: RuntimeModeSDKStore,
	})
	if err != nil {
		t.Fatalf("reload sdk home runtime: %v", err)
	}
	defer reloaded.Close()

	routes := reloaded.IBCRouterKeeper.ExportRoutes()
	if len(routes) != 1 {
		t.Fatalf("expected one route after sdk-home reload, got %d", len(routes))
	}
	if routes[0].DestinationChainID != "osmo-local-1" {
		t.Fatalf("expected osmo-local-1 route, got %+v", routes[0])
	}
}

func TestInitHomeCreatesRuntimeArtifactsAndStatusSummary(t *testing.T) {
	t.Parallel()

	homeDir := filepath.Join(t.TempDir(), "home")
	cfg, err := InitHome(Config{
		HomeDir: homeDir,
		ChainID: "aegislink-devnet-1",
		AppName: AppName,
		Modules: []string{"bridge", "registry", "limits", "pauser", "ibcrouter", "governance"},
	}, false)
	if err != nil {
		t.Fatalf("init home: %v", err)
	}

	if _, err := os.Stat(cfg.ConfigPath); err != nil {
		t.Fatalf("expected config file: %v", err)
	}
	if _, err := os.Stat(cfg.GenesisPath); err != nil {
		t.Fatalf("expected genesis file: %v", err)
	}
	if _, err := os.Stat(cfg.StatePath); err != nil {
		t.Fatalf("expected state file: %v", err)
	}

	loaded, err := LoadWithConfig(Config{HomeDir: homeDir})
	if err != nil {
		t.Fatalf("load with config: %v", err)
	}
	status := loaded.Status()
	if status.ChainID != "aegislink-devnet-1" {
		t.Fatalf("expected chain id aegislink-devnet-1, got %q", status.ChainID)
	}
	if status.HomeDir != homeDir {
		t.Fatalf("expected home %q, got %q", homeDir, status.HomeDir)
	}
	if !status.Initialized {
		t.Fatal("expected initialized runtime")
	}
	if status.Modules != 6 {
		t.Fatalf("expected 6 modules, got %d", status.Modules)
	}
	if status.FailedClaims != 0 {
		t.Fatalf("expected zero failed claims on fresh runtime, got %d", status.FailedClaims)
	}
}

func TestInitHomeCreatesSDKStoreRuntimeArtifacts(t *testing.T) {
	t.Parallel()

	homeDir := filepath.Join(t.TempDir(), "home")
	cfg, err := InitHome(Config{
		HomeDir:     homeDir,
		ChainID:     "aegislink-sdk-1",
		AppName:     AppName,
		RuntimeMode: RuntimeModeSDKStore,
		Modules:     []string{"bridge", "registry", "limits", "pauser", "ibcrouter", "governance"},
	}, false)
	if err != nil {
		t.Fatalf("init home: %v", err)
	}

	if _, err := os.Stat(cfg.ConfigPath); err != nil {
		t.Fatalf("expected config file: %v", err)
	}
	if _, err := os.Stat(cfg.GenesisPath); err != nil {
		t.Fatalf("expected genesis file: %v", err)
	}
	info, err := os.Stat(cfg.StatePath)
	if err != nil {
		t.Fatalf("expected state path: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("expected sdk runtime state path to be a directory, got file at %s", cfg.StatePath)
	}
}

func TestResolveConfigRejectsThresholdAboveSignerCount(t *testing.T) {
	t.Parallel()

	_, err := ResolveConfig(Config{
		HomeDir:           filepath.Join(t.TempDir(), "home"),
		AllowedSigners:    []string{"relayer-1"},
		RequiredThreshold: 2,
	})
	if err == nil {
		t.Fatal("expected invalid threshold error")
	}
	if !strings.Contains(err.Error(), "required threshold") {
		t.Fatalf("expected threshold validation error, got %v", err)
	}
}

func validDepositClaim(t *testing.T) bridgetypes.DepositClaim {
	identity := bridgetypes.ClaimIdentity{
		Kind:           bridgetypes.ClaimKindDeposit,
		SourceChainID:  "11155111",
		SourceContract: "0xgateway",
		SourceTxHash:   "0xdeposit-tx",
		SourceLogIndex: 7,
		Nonce:          1,
	}
	identity.MessageID = identity.DerivedMessageID()

	return bridgetypes.DepositClaim{
		Identity:           identity,
		DestinationChainID: "aegislink-1",
		AssetID:            "eth.usdc",
		Amount:             mustAmount(t, "100000000"),
		Recipient:          "cosmos1recipient",
		Deadline:           100,
	}
}

func validAttestationForClaim(claim bridgetypes.DepositClaim) bridgetypes.Attestation {
	return bridgetypes.Attestation{
		MessageID:        claim.Identity.MessageID,
		PayloadHash:      claim.Digest(),
		Signers:          []string{"relayer-1", "relayer-2"},
		Threshold:        2,
		Expiry:           120,
		SignerSetVersion: 1,
	}
}

func mustAmount(t *testing.T, value string) *big.Int {
	t.Helper()

	amount, ok := new(big.Int).SetString(value, 10)
	if !ok {
		t.Fatalf("invalid amount %q", value)
	}
	return amount
}
