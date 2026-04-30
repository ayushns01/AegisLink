package app

import (
	"encoding/json"
	"errors"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"

	bridgekeeper "github.com/ayushns01/aegislink/chain/aegislink/x/bridge/keeper"
	bridgetypes "github.com/ayushns01/aegislink/chain/aegislink/x/bridge/types"
	bridgetestutil "github.com/ayushns01/aegislink/chain/aegislink/x/bridge/types/testutil"
	governancekeeper "github.com/ayushns01/aegislink/chain/aegislink/x/governance/keeper"
	ibcrouterkeeper "github.com/ayushns01/aegislink/chain/aegislink/x/ibcrouter/keeper"
	limitskeeper "github.com/ayushns01/aegislink/chain/aegislink/x/limits/keeper"
	limittypes "github.com/ayushns01/aegislink/chain/aegislink/x/limits/types"
	registrytypes "github.com/ayushns01/aegislink/chain/aegislink/x/registry/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func TestSaveAndLoadPreservesBridgeRuntimeState(t *testing.T) {
	t.Parallel()

	statePath := filepath.Join(t.TempDir(), "aegislink-state.json")
	app, err := NewWithConfig(Config{
		AppName:           AppName,
		Modules:           []string{"bridge", "bank", "registry", "limits", "pauser", "governance"},
		StatePath:         statePath,
		AllowedSigners:    bridgetestutil.DefaultHarnessSignerAddresses()[:3],
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
		WindowBlocks: 600,
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
	withdrawalOwner := sdk.AccAddress([]byte("withdrawal-owner-runtime-test")).String()
	if err := app.BankKeeper.Credit(withdrawalOwner, "uethusdc", claim.Amount); err != nil {
		t.Fatalf("seed withdrawal owner balance: %v", err)
	}
	withdrawal, err := app.ExecuteWithdrawal(withdrawalOwner, claim.AssetID, claim.Amount, "0xrecipient", 120, []byte("threshold-proof"))
	if err != nil {
		t.Fatalf("execute withdrawal: %v", err)
	}
	if got := app.BankKeeper.BalanceOf(withdrawalOwner, "uethusdc"); got.Sign() != 0 {
		t.Fatalf("expected withdrawal owner balance to be debited, got %s", got.String())
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
	usage, ok := loaded.LimitsKeeper.CurrentUsage(asset.AssetID, 60)
	if !ok {
		t.Fatalf("expected rolling-window usage to persist")
	}
	if usage.UsedAmount.Cmp(mustAmount(t, "200000000")) != 0 {
		t.Fatalf("expected persisted rolling-window usage 200000000, got %s", usage.UsedAmount.String())
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
		Modules:           []string{"bridge", "bank", "registry", "limits", "pauser", "ibcrouter", "governance"},
		StatePath:         storePath,
		AllowedSigners:    bridgetestutil.DefaultHarnessSignerAddresses()[:3],
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
		WindowBlocks: 600,
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
		Modules:     []string{"bridge", "bank", "registry", "limits", "pauser", "ibcrouter", "governance"},
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

func TestSaveAndLoadPreservesWalletBalances(t *testing.T) {
	t.Parallel()

	statePath := filepath.Join(t.TempDir(), "wallet-state.json")
	app, err := NewWithConfig(Config{
		AppName:           AppName,
		Modules:           []string{"bank", "bridge", "registry", "limits", "pauser", "governance"},
		StatePath:         statePath,
		AllowedSigners:    bridgetestutil.DefaultHarnessSignerAddresses()[:3],
		RequiredThreshold: 2,
	})
	if err != nil {
		t.Fatalf("new app: %v", err)
	}

	recipient := sdk.AccAddress([]byte("wallet-runtime-test")).String()
	if err := app.RegisterAsset(registrytypes.Asset{
		AssetID:         "eth",
		SourceChainID:   "11155111",
		SourceAssetKind: registrytypes.SourceAssetKindNativeETH,
		Denom:           "ueth",
		Decimals:        18,
		DisplayName:     "Ether",
		DisplaySymbol:   "ETH",
		Enabled:         true,
	}); err != nil {
		t.Fatalf("register native asset: %v", err)
	}
	if err := app.RegisterAsset(registrytypes.Asset{
		AssetID:            "eth.usdc",
		SourceChainID:      "11155111",
		SourceAssetKind:    registrytypes.SourceAssetKindERC20,
		SourceAssetAddress: "0xusdc",
		Denom:              "uethusdc",
		Decimals:           6,
		DisplayName:        "USD Coin",
		DisplaySymbol:      "USDC",
		Enabled:            true,
	}); err != nil {
		t.Fatalf("register erc20 asset: %v", err)
	}
	if err := app.SetLimit(limittypes.RateLimit{
		AssetID:       "eth",
		WindowBlocks: 600,
		MaxAmount:     mustAmount(t, "2000000000000000000"),
	}); err != nil {
		t.Fatalf("set eth limit: %v", err)
	}
	if err := app.SetLimit(limittypes.RateLimit{
		AssetID:       "eth.usdc",
		WindowBlocks: 600,
		MaxAmount:     mustAmount(t, "100000000"),
	}); err != nil {
		t.Fatalf("set erc20 limit: %v", err)
	}

	nativeClaim := depositClaimForWalletTest(t, bridgetypes.SourceAssetKindNativeETH, "", "eth", "0xnative", 1, 1, recipient, "1000000000000000000")
	if _, err := app.SubmitDepositClaim(nativeClaim, attestationForWalletTest(t, nativeClaim)); err != nil {
		t.Fatalf("submit native claim: %v", err)
	}
	erc20Claim := depositClaimForWalletTest(t, bridgetypes.SourceAssetKindERC20, "0xusdc", "eth.usdc", "0xerc20", 2, 2, recipient, "25000000")
	if _, err := app.SubmitDepositClaim(erc20Claim, attestationForWalletTest(t, erc20Claim)); err != nil {
		t.Fatalf("submit erc20 claim: %v", err)
	}
	if err := app.Save(); err != nil {
		t.Fatalf("save wallet state: %v", err)
	}

	loaded, err := Load(statePath)
	if err != nil {
		t.Fatalf("load wallet state: %v", err)
	}
	balances, err := loaded.WalletBalances(recipient)
	if err != nil {
		t.Fatalf("wallet balances: %v", err)
	}
	if len(balances) != 2 {
		t.Fatalf("expected two persisted balances, got %d", len(balances))
	}
	if balances[0].Denom != "ueth" || balances[0].Amount != "1000000000000000000" {
		t.Fatalf("unexpected native balance: %+v", balances[0])
	}
	if balances[1].Denom != "uethusdc" || balances[1].Amount != "25000000" {
		t.Fatalf("unexpected erc20 balance: %+v", balances[1])
	}
}

func TestLoadSynthesizesWalletBalancesWhenLegacyRuntimeOmitsBankState(t *testing.T) {
	t.Parallel()

	statePath := filepath.Join(t.TempDir(), "legacy-wallet-state.json")
	recipient := sdk.AccAddress([]byte("legacy-wallet-recipient")).String()

	asset := registrytypes.Asset{
		AssetID:         "eth",
		SourceChainID:   "11155111",
		SourceAssetKind: registrytypes.SourceAssetKindNativeETH,
		Denom:           "ueth",
		Decimals:        18,
		DisplayName:     "Ether",
		DisplaySymbol:   "ETH",
		Enabled:         true,
	}
	raw := map[string]any{
		"assets": []registrytypes.Asset{asset},
		"limits": limitskeeper.StateSnapshot{},
		"bridge": bridgekeeper.StateSnapshot{
			ProcessedClaims: []bridgekeeper.ClaimRecordSnapshot{
				{
					ClaimKey:  "legacy-claim-1",
					MessageID: "legacy-msg-1",
					Denom:     "ueth",
					AssetID:   "eth",
					Recipient: recipient,
					Amount:    "1000000000000000000",
					Status:    bridgekeeper.ClaimStatusAccepted,
				},
			},
			SupplyByDenom: map[string]string{
				"ueth": "1000000000000000000",
			},
		},
		"ibc_router":   ibcrouterkeeper.StateSnapshot{},
		"governance":   governancekeeper.StateSnapshot{},
		"paused_flows": []string{},
	}
	encoded, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		t.Fatalf("marshal legacy state: %v", err)
	}
	if err := os.WriteFile(statePath, encoded, 0o644); err != nil {
		t.Fatalf("write legacy state: %v", err)
	}

	loaded, err := Load(statePath)
	if err != nil {
		t.Fatalf("load legacy wallet state: %v", err)
	}
	balances, err := loaded.WalletBalances(recipient)
	if err != nil {
		t.Fatalf("wallet balances: %v", err)
	}
	if len(balances) != 1 {
		t.Fatalf("expected one synthesized balance, got %d (%+v)", len(balances), balances)
	}
	if balances[0].Denom != "ueth" || balances[0].Amount != "1000000000000000000" {
		t.Fatalf("unexpected synthesized balance: %+v", balances[0])
	}
}

func TestLoadRejectsLegacyRuntimeMissingBankStateWithoutRecipientMetadata(t *testing.T) {
	t.Parallel()

	statePath := filepath.Join(t.TempDir(), "ambiguous-wallet-state.json")
	raw := map[string]any{
		"bridge": bridgekeeper.StateSnapshot{
			ProcessedClaims: []bridgekeeper.ClaimRecordSnapshot{
				{
					ClaimKey:  "legacy-claim-1",
					MessageID: "legacy-msg-1",
					Denom:     "ueth",
					AssetID:   "eth",
					Amount:    "1000000000000000000",
					Status:    bridgekeeper.ClaimStatusAccepted,
				},
			},
			SupplyByDenom: map[string]string{
				"ueth": "1000000000000000000",
			},
		},
	}
	encoded, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		t.Fatalf("marshal ambiguous legacy state: %v", err)
	}
	if err := os.WriteFile(statePath, encoded, 0o644); err != nil {
		t.Fatalf("write ambiguous legacy state: %v", err)
	}

	if _, err := Load(statePath); !errors.Is(err, ErrWalletStateMigrationRequired) {
		t.Fatalf("expected wallet migration error, got %v", err)
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
		WindowBlocks: 600,
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
		Modules: []string{"bridge", "bank", "registry", "limits", "pauser", "ibcrouter", "governance"},
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
	if status.Modules != 7 {
		t.Fatalf("expected 7 modules, got %d", status.Modules)
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
		Modules:     []string{"bridge", "bank", "registry", "limits", "pauser", "ibcrouter", "governance"},
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

func TestLoadWithConfigMigratesLegacySDKStoreConfigWithoutBankModule(t *testing.T) {
	t.Parallel()

	homeDir := filepath.Join(t.TempDir(), "legacy-home")
	legacy := Config{
		AppName:     AppName,
		ChainID:     "aegislink-sdk-legacy-1",
		RuntimeMode: RuntimeModeSDKStore,
		HomeDir:     homeDir,
		ConfigPath:  runtimeConfigPath(homeDir),
		GenesisPath: runtimeGenesisPath(homeDir),
		StatePath:   runtimeStorePath(homeDir),
		Modules:     []string{"bridge", "registry", "limits", "pauser", "ibcrouter", "governance"},
	}

	if err := os.MkdirAll(filepath.Dir(legacy.ConfigPath), 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.MkdirAll(legacy.StatePath, 0o755); err != nil {
		t.Fatalf("mkdir store dir: %v", err)
	}
	if err := writeConfigFile(legacy.ConfigPath, legacy); err != nil {
		t.Fatalf("write legacy config: %v", err)
	}
	if err := writeGenesisFile(legacy.GenesisPath, DefaultGenesis(legacy)); err != nil {
		t.Fatalf("write legacy genesis: %v", err)
	}

	loaded, err := LoadWithConfig(Config{
		HomeDir:     homeDir,
		RuntimeMode: RuntimeModeSDKStore,
	})
	if err != nil {
		t.Fatalf("load legacy sdk config: %v", err)
	}
	defer loaded.Close()

	if !containsModule(loaded.Config.Modules, "bank") {
		t.Fatalf("expected migrated module list to include bank, got %+v", loaded.Config.Modules)
	}
}

func TestResolveConfigRejectsThresholdAboveSignerCount(t *testing.T) {
	t.Parallel()

	_, err := ResolveConfig(Config{
		HomeDir:           filepath.Join(t.TempDir(), "home"),
		AllowedSigners:    bridgetestutil.DefaultHarnessSignerAddresses()[:1],
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
		Recipient:          sdk.AccAddress([]byte("runtime-test-wallet")).String(),
		Deadline:           100,
	}
}

func validAttestationForClaim(claim bridgetypes.DepositClaim) bridgetypes.Attestation {
	attestation := bridgetypes.Attestation{
		MessageID:        claim.Identity.MessageID,
		PayloadHash:      claim.Digest(),
		Signers:          bridgetestutil.DefaultHarnessSignerAddresses()[:2],
		Threshold:        2,
		Expiry:           120,
		SignerSetVersion: 1,
	}
	for _, key := range bridgetestutil.DefaultHarnessSignerPrivateKeys()[:2] {
		proof, err := bridgetypes.SignAttestationWithPrivateKeyHex(attestation, key)
		if err != nil {
			panic(err)
		}
		attestation.Proofs = append(attestation.Proofs, proof)
	}
	return attestation
}

func mustAmount(t *testing.T, value string) *big.Int {
	t.Helper()

	amount, ok := new(big.Int).SetString(value, 10)
	if !ok {
		t.Fatalf("invalid amount %q", value)
	}
	return amount
}

func containsModule(modules []string, target string) bool {
	for _, moduleName := range modules {
		if moduleName == target {
			return true
		}
	}
	return false
}

func depositClaimForWalletTest(t *testing.T, sourceAssetKind, sourceContract, assetID, txHash string, logIndex, nonce uint64, recipient, amount string) bridgetypes.DepositClaim {
	t.Helper()

	identity := bridgetypes.ClaimIdentity{
		Kind:            bridgetypes.ClaimKindDeposit,
		SourceAssetKind: sourceAssetKind,
		SourceChainID:   "11155111",
		SourceContract:  sourceContract,
		SourceTxHash:    txHash,
		SourceLogIndex:  logIndex,
		Nonce:           nonce,
	}
	identity.MessageID = identity.DerivedMessageID()

	return bridgetypes.DepositClaim{
		Identity:           identity,
		DestinationChainID: "aegislink-local-1",
		AssetID:            assetID,
		Amount:             mustAmount(t, amount),
		Recipient:          recipient,
		Deadline:           120,
	}
}

func attestationForWalletTest(t *testing.T, claim bridgetypes.DepositClaim) bridgetypes.Attestation {
	t.Helper()

	attestation := bridgetypes.Attestation{
		MessageID:        claim.Identity.MessageID,
		PayloadHash:      claim.Digest(),
		Signers:          bridgetestutil.DefaultHarnessSignerAddresses()[:2],
		Threshold:        2,
		Expiry:           200,
		SignerSetVersion: 1,
	}
	for _, key := range bridgetestutil.DefaultHarnessSignerPrivateKeys()[:2] {
		proof, err := bridgetypes.SignAttestationWithPrivateKeyHex(attestation, key)
		if err != nil {
			t.Fatalf("sign attestation: %v", err)
		}
		attestation.Proofs = append(attestation.Proofs, proof)
	}
	return attestation
}
