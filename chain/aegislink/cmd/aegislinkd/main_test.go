package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"

	aegisapp "github.com/ayushns01/aegislink/chain/aegislink/app"
	bridgekeeper "github.com/ayushns01/aegislink/chain/aegislink/x/bridge/keeper"
	bridgetypes "github.com/ayushns01/aegislink/chain/aegislink/x/bridge/types"
	ibcrouterkeeper "github.com/ayushns01/aegislink/chain/aegislink/x/ibcrouter/keeper"
	limittypes "github.com/ayushns01/aegislink/chain/aegislink/x/limits/types"
	registrytypes "github.com/ayushns01/aegislink/chain/aegislink/x/registry/types"
)

func TestRunInitCreatesRuntimeHomeArtifacts(t *testing.T) {
	t.Parallel()

	homeDir := filepath.Join(t.TempDir(), "home")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := run([]string{
		"init",
		"--home", homeDir,
		"--chain-id", "aegislink-devnet-1",
	}, &stdout, &stderr); err != nil {
		t.Fatalf("run init: %v\nstderr=%s", err, stderr.String())
	}

	var result struct {
		Status      string `json:"status"`
		ChainID     string `json:"chain_id"`
		HomeDir     string `json:"home_dir"`
		ConfigPath  string `json:"config_path"`
		GenesisPath string `json:"genesis_path"`
		StatePath   string `json:"state_path"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("decode init output: %v\n%s", err, stdout.String())
	}
	if result.Status != "initialized" {
		t.Fatalf("expected initialized status, got %q", result.Status)
	}
	if result.ChainID != "aegislink-devnet-1" {
		t.Fatalf("expected chain id aegislink-devnet-1, got %q", result.ChainID)
	}
	if result.HomeDir != homeDir {
		t.Fatalf("expected home dir %q, got %q", homeDir, result.HomeDir)
	}
	for _, path := range []string{result.ConfigPath, result.GenesisPath, result.StatePath} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected artifact %s: %v", path, err)
		}
	}
}

func TestRunStartLoadsInitializedRuntimeFromHome(t *testing.T) {
	t.Parallel()

	homeDir := filepath.Join(t.TempDir(), "home")
	if _, err := aegisapp.InitHome(aegisapp.Config{
		HomeDir: homeDir,
		ChainID: "aegislink-devnet-1",
	}, false); err != nil {
		t.Fatalf("init home: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := run([]string{
		"start",
		"--home", homeDir,
	}, &stdout, &stderr); err != nil {
		t.Fatalf("run start: %v\nstderr=%s", err, stderr.String())
	}

	var result struct {
		Status  string `json:"status"`
		ChainID string `json:"chain_id"`
		HomeDir string `json:"home_dir"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("decode start output: %v\n%s", err, stdout.String())
	}
	if result.Status != "started" {
		t.Fatalf("expected started status, got %q", result.Status)
	}
	if result.ChainID != "aegislink-devnet-1" {
		t.Fatalf("expected chain id aegislink-devnet-1, got %q", result.ChainID)
	}
	if result.HomeDir != homeDir {
		t.Fatalf("expected home dir %q, got %q", homeDir, result.HomeDir)
	}
}

func TestRunQueryStatusPrintsRuntimeSummary(t *testing.T) {
	t.Parallel()

	homeDir := filepath.Join(t.TempDir(), "home")
	cfg, err := aegisapp.InitHome(aegisapp.Config{
		HomeDir: homeDir,
		ChainID: "aegislink-devnet-1",
	}, false)
	if err != nil {
		t.Fatalf("init home: %v", err)
	}
	app := seededRuntimeAppWithIBCRoute(t, cfg.StatePath)
	app.SetCurrentHeight(90)
	if err := app.BridgeKeeper.UpsertSignerSet(bridgekeeper.SignerSet{
		Version:     2,
		Signers:     []string{"relayer-2", "relayer-4", "relayer-5"},
		Threshold:   2,
		ActivatedAt: 80,
	}); err != nil {
		t.Fatalf("upsert signer set: %v", err)
	}
	if err := app.Save(); err != nil {
		t.Fatalf("save seeded state: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := run([]string{
		"query", "status",
		"--home", homeDir,
	}, &stdout, &stderr); err != nil {
		t.Fatalf("run query status: %v\nstderr=%s", err, stderr.String())
	}

	var status struct {
		AppName                string   `json:"app_name"`
		ChainID                string   `json:"chain_id"`
		Initialized            bool     `json:"initialized"`
		Assets                 int      `json:"assets"`
		Routes                 int      `json:"routes"`
		EnabledRouteIDs        []string `json:"enabled_route_ids"`
		ActiveSignerSetVersion uint64   `json:"active_signer_set_version"`
		SignerSetCount         int      `json:"signer_set_count"`
		SignerSetVersions      []uint64 `json:"signer_set_versions"`
		FailedClaims           int      `json:"failed_claims"`
		Transfers              int      `json:"transfers"`
		PendingTransfers       int      `json:"pending_transfers"`
		HomeDir                string   `json:"home_dir"`
		StatePath              string   `json:"state_path"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &status); err != nil {
		t.Fatalf("decode status output: %v\n%s", err, stdout.String())
	}
	if status.AppName != aegisapp.AppName {
		t.Fatalf("expected app name %q, got %q", aegisapp.AppName, status.AppName)
	}
	if status.ChainID != "aegislink-devnet-1" {
		t.Fatalf("expected chain id aegislink-devnet-1, got %q", status.ChainID)
	}
	if !status.Initialized {
		t.Fatal("expected initialized status")
	}
	if status.Assets != 1 || status.Routes != 1 {
		t.Fatalf("unexpected status counts: %+v", status)
	}
	if len(status.EnabledRouteIDs) != 1 || status.EnabledRouteIDs[0] != "eth.usdc@osmosis-1:channel-0" {
		t.Fatalf("expected enabled route id eth.usdc@osmosis-1:channel-0, got %+v", status.EnabledRouteIDs)
	}
	if status.ActiveSignerSetVersion != 2 {
		t.Fatalf("expected active signer set version 2, got %d", status.ActiveSignerSetVersion)
	}
	if status.SignerSetCount != 2 {
		t.Fatalf("expected signer set count 2, got %d", status.SignerSetCount)
	}
	if len(status.SignerSetVersions) != 2 || status.SignerSetVersions[0] != 1 || status.SignerSetVersions[1] != 2 {
		t.Fatalf("expected signer set versions [1 2], got %+v", status.SignerSetVersions)
	}
	if status.FailedClaims != 0 {
		t.Fatalf("expected zero failed claims in seeded runtime, got %d", status.FailedClaims)
	}
	if status.Transfers != 0 || status.PendingTransfers != 0 {
		t.Fatalf("expected zero transfers in seeded runtime, got %+v", status)
	}
	if status.HomeDir != homeDir {
		t.Fatalf("expected home dir %q, got %q", homeDir, status.HomeDir)
	}
	if status.StatePath != cfg.StatePath {
		t.Fatalf("expected state path %q, got %q", cfg.StatePath, status.StatePath)
	}
}

func TestRunStartLogsStructuredStartupSummary(t *testing.T) {
	t.Parallel()

	homeDir := filepath.Join(t.TempDir(), "home")
	cfg, err := aegisapp.InitHome(aegisapp.Config{
		HomeDir: homeDir,
		ChainID: "aegislink-devnet-1",
	}, false)
	if err != nil {
		t.Fatalf("init home: %v", err)
	}
	app := seededRuntimeAppWithIBCRoute(t, cfg.StatePath)
	app.SetCurrentHeight(90)
	if err := app.BridgeKeeper.UpsertSignerSet(bridgekeeper.SignerSet{
		Version:     2,
		Signers:     []string{"relayer-2", "relayer-4", "relayer-5"},
		Threshold:   2,
		ActivatedAt: 80,
	}); err != nil {
		t.Fatalf("upsert signer set: %v", err)
	}
	if err := app.Save(); err != nil {
		t.Fatalf("save seeded state: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := run([]string{
		"start",
		"--home", homeDir,
	}, &stdout, &stderr); err != nil {
		t.Fatalf("run start: %v\nstderr=%s", err, stderr.String())
	}

	events := parseJSONLogLines(t, stderr.String())
	if len(events) == 0 {
		t.Fatal("expected structured log output")
	}

	last := events[len(events)-1]
	if last["event"] != "runtime_start" {
		t.Fatalf("expected runtime_start event, got %+v", last)
	}
	if last["chain_id"] != "aegislink-devnet-1" {
		t.Fatalf("expected chain id aegislink-devnet-1, got %+v", last["chain_id"])
	}
	if last["home_dir"] != homeDir {
		t.Fatalf("expected home dir %q, got %+v", homeDir, last["home_dir"])
	}
	if last["module_count"] != float64(5) {
		t.Fatalf("expected module count 5, got %+v", last["module_count"])
	}
	if last["configured_signers"] != float64(3) {
		t.Fatalf("expected configured signers 3, got %+v", last["configured_signers"])
	}
	enabled, ok := last["enabled_route_ids"].([]any)
	if !ok || len(enabled) != 1 || enabled[0] != "eth.usdc@osmosis-1:channel-0" {
		t.Fatalf("expected enabled route ids to include eth.usdc@osmosis-1:channel-0, got %+v", last["enabled_route_ids"])
	}
	if last["active_signer_set_version"] != float64(2) {
		t.Fatalf("expected active signer set version 2, got %+v", last["active_signer_set_version"])
	}
	if last["signer_set_count"] != float64(2) {
		t.Fatalf("expected signer set count 2, got %+v", last["signer_set_count"])
	}
}

func TestRunQuerySignerSetReturnsActiveSignerSet(t *testing.T) {
	t.Parallel()

	statePath := filepath.Join(t.TempDir(), "aegislink-state.json")
	app := seededRuntimeAppWithIBCRoute(t, statePath)
	app.SetCurrentHeight(90)
	if err := app.BridgeKeeper.UpsertSignerSet(bridgekeeper.SignerSet{
		Version:     2,
		Signers:     []string{"relayer-2", "relayer-4", "relayer-5"},
		Threshold:   2,
		ActivatedAt: 80,
	}); err != nil {
		t.Fatalf("upsert signer set: %v", err)
	}
	if err := app.Save(); err != nil {
		t.Fatalf("save state: %v", err)
	}

	var stdout bytes.Buffer
	if err := run([]string{"query", "signer-set", "--state-path", statePath}, &stdout, io.Discard); err != nil {
		t.Fatalf("run query signer-set: %v", err)
	}

	var set struct {
		Version     uint64   `json:"version"`
		Signers     []string `json:"signers"`
		Threshold   uint32   `json:"threshold"`
		ActivatedAt uint64   `json:"activated_at"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &set); err != nil {
		t.Fatalf("decode signer set output: %v\n%s", err, stdout.String())
	}
	if set.Version != 2 {
		t.Fatalf("expected active signer set version 2, got %d", set.Version)
	}
	if set.ActivatedAt != 80 {
		t.Fatalf("expected activated_at 80, got %d", set.ActivatedAt)
	}
	if len(set.Signers) != 3 {
		t.Fatalf("expected three signers, got %+v", set.Signers)
	}
}

func TestRunQuerySignerSetsListsHistory(t *testing.T) {
	t.Parallel()

	statePath := filepath.Join(t.TempDir(), "aegislink-state.json")
	app := seededRuntimeAppWithIBCRoute(t, statePath)
	app.SetCurrentHeight(90)
	if err := app.BridgeKeeper.UpsertSignerSet(bridgekeeper.SignerSet{
		Version:     2,
		Signers:     []string{"relayer-2", "relayer-4", "relayer-5"},
		Threshold:   2,
		ActivatedAt: 80,
	}); err != nil {
		t.Fatalf("upsert signer set: %v", err)
	}
	if err := app.Save(); err != nil {
		t.Fatalf("save state: %v", err)
	}

	var stdout bytes.Buffer
	if err := run([]string{"query", "signer-sets", "--state-path", statePath}, &stdout, io.Discard); err != nil {
		t.Fatalf("run query signer-sets: %v", err)
	}

	var sets []struct {
		Version uint64 `json:"version"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &sets); err != nil {
		t.Fatalf("decode signer set history output: %v\n%s", err, stdout.String())
	}
	if len(sets) != 2 {
		t.Fatalf("expected two signer sets, got %d", len(sets))
	}
	if sets[0].Version != 1 || sets[1].Version != 2 {
		t.Fatalf("expected versions [1 2], got %+v", sets)
	}
}

func TestRunTxSubmitDepositClaimPersistsAcceptedClaim(t *testing.T) {
	t.Parallel()

	statePath := filepath.Join(t.TempDir(), "aegislink-state.json")
	app, err := aegisapp.NewWithConfig(aegisapp.Config{
		AppName:           aegisapp.AppName,
		Modules:           []string{"bridge", "registry", "limits", "pauser"},
		StatePath:         statePath,
		AllowedSigners:    []string{"relayer-1", "relayer-2", "relayer-3"},
		RequiredThreshold: 2,
	})
	if err != nil {
		t.Fatalf("new app: %v", err)
	}
	if err := app.RegisterAsset(registrytypes.Asset{
		AssetID:        "eth.usdc",
		SourceChainID:  "11155111",
		SourceContract: "0xasset",
		Denom:          "uethusdc",
		Decimals:       6,
		DisplayName:    "USDC",
		Enabled:        true,
	}); err != nil {
		t.Fatalf("register asset: %v", err)
	}
	if err := app.SetLimit(limittypes.RateLimit{
		AssetID:       "eth.usdc",
		WindowSeconds: 600,
		MaxAmount:     mustAmount(t, "1000000000000000000"),
	}); err != nil {
		t.Fatalf("set limit: %v", err)
	}
	app.SetCurrentHeight(50)
	if err := app.Save(); err != nil {
		t.Fatalf("save state: %v", err)
	}

	submissionPath := filepath.Join(t.TempDir(), "submission.json")
	claim := validDepositClaim(t)
	writeSubmissionFile(t, submissionPath, claim, validAttestationForClaim(claim))

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := run([]string{
		"tx", "submit-deposit-claim",
		"--state-path", statePath,
		"--submission-file", submissionPath,
	}, &stdout, &stderr); err != nil {
		t.Fatalf("run tx submit-deposit-claim: %v\nstderr=%s", err, stderr.String())
	}

	loaded, err := aegisapp.Load(statePath)
	if err != nil {
		t.Fatalf("reload state: %v", err)
	}
	if supply := loaded.BridgeKeeper.SupplyForDenom("uethusdc"); supply.String() != "100000000" {
		t.Fatalf("expected minted supply 100000000, got %s", supply.String())
	}

	var result struct {
		Status    string `json:"status"`
		MessageID string `json:"message_id"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("decode output: %v\n%s", err, stdout.String())
	}
	if result.Status != "accepted" {
		t.Fatalf("expected accepted status, got %q", result.Status)
	}
}

func TestRunTxSubmitDepositClaimUsesSDKStoreRuntimeHome(t *testing.T) {
	t.Parallel()

	homeDir := filepath.Join(t.TempDir(), "sdk-home")
	cfg := initSDKRuntimeHome(t, homeDir)
	app := seededSDKRuntimeApp(t, cfg)
	app.SetCurrentHeight(50)
	if err := app.Close(); err != nil {
		t.Fatalf("close seeded app: %v", err)
	}

	submissionPath := filepath.Join(t.TempDir(), "submission.json")
	claim := validDepositClaim(t)
	writeSubmissionFile(t, submissionPath, claim, validAttestationForClaim(claim))

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := run([]string{
		"tx", "submit-deposit-claim",
		"--home", homeDir,
		"--submission-file", submissionPath,
	}, &stdout, &stderr); err != nil {
		t.Fatalf("run tx submit-deposit-claim: %v\nstderr=%s", err, stderr.String())
	}

	loaded, err := aegisapp.LoadWithConfig(aegisapp.Config{
		HomeDir:     homeDir,
		RuntimeMode: aegisapp.RuntimeModeSDKStore,
	})
	if err != nil {
		t.Fatalf("reload sdk runtime: %v", err)
	}
	defer func() {
		if err := loaded.Close(); err != nil {
			t.Fatalf("close loaded sdk runtime: %v", err)
		}
	}()
	if supply := loaded.BridgeKeeper.SupplyForDenom("uethusdc"); supply.String() != "100000000" {
		t.Fatalf("expected minted supply 100000000, got %s", supply.String())
	}
}

func TestRunQueryClaimPrintsPersistedAcceptedClaim(t *testing.T) {
	t.Parallel()

	statePath := filepath.Join(t.TempDir(), "aegislink-state.json")
	app := seededRuntimeApp(t, statePath)
	claim := validDepositClaim(t)
	if _, err := app.SubmitDepositClaim(claim, validAttestationForClaim(claim)); err != nil {
		t.Fatalf("submit deposit claim: %v", err)
	}
	if err := app.Save(); err != nil {
		t.Fatalf("save state: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := run([]string{
		"query", "claim",
		"--state-path", statePath,
		"--message-id", claim.Identity.MessageID,
	}, &stdout, &stderr); err != nil {
		t.Fatalf("run query claim: %v\nstderr=%s", err, stderr.String())
	}

	var result struct {
		MessageID string `json:"message_id"`
		AssetID   string `json:"asset_id"`
		Denom     string `json:"denom"`
		Amount    string `json:"amount"`
		Status    string `json:"status"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("decode output: %v\n%s", err, stdout.String())
	}
	if result.MessageID != claim.Identity.MessageID {
		t.Fatalf("expected message id %q, got %q", claim.Identity.MessageID, result.MessageID)
	}
	if result.AssetID != claim.AssetID {
		t.Fatalf("expected asset id %q, got %q", claim.AssetID, result.AssetID)
	}
	if result.Denom != "uethusdc" {
		t.Fatalf("expected denom uethusdc, got %q", result.Denom)
	}
	if result.Amount != claim.Amount.String() {
		t.Fatalf("expected amount %s, got %q", claim.Amount.String(), result.Amount)
	}
	if result.Status != "accepted" {
		t.Fatalf("expected accepted status, got %q", result.Status)
	}
}

func TestRunQueryClaimUsesSDKStoreRuntimeHome(t *testing.T) {
	t.Parallel()

	homeDir := filepath.Join(t.TempDir(), "sdk-home")
	cfg := initSDKRuntimeHome(t, homeDir)
	app := seededSDKRuntimeApp(t, cfg)

	claim := validDepositClaim(t)
	if _, err := app.SubmitDepositClaim(claim, validAttestationForClaim(claim)); err != nil {
		t.Fatalf("submit deposit claim: %v", err)
	}
	if err := app.Close(); err != nil {
		t.Fatalf("close seeded app: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := run([]string{
		"query", "claim",
		"--home", homeDir,
		"--message-id", claim.Identity.MessageID,
	}, &stdout, &stderr); err != nil {
		t.Fatalf("run query claim: %v\nstderr=%s", err, stderr.String())
	}

	var result struct {
		MessageID string `json:"message_id"`
		AssetID   string `json:"asset_id"`
		Denom     string `json:"denom"`
		Amount    string `json:"amount"`
		Status    string `json:"status"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("decode output: %v\n%s", err, stdout.String())
	}
	if result.MessageID != claim.Identity.MessageID {
		t.Fatalf("expected message id %q, got %q", claim.Identity.MessageID, result.MessageID)
	}
	if result.AssetID != claim.AssetID {
		t.Fatalf("expected asset id %q, got %q", claim.AssetID, result.AssetID)
	}
	if result.Denom != "uethusdc" {
		t.Fatalf("expected denom uethusdc, got %q", result.Denom)
	}
	if result.Amount != claim.Amount.String() {
		t.Fatalf("expected amount %s, got %q", claim.Amount.String(), result.Amount)
	}
	if result.Status != "accepted" {
		t.Fatalf("expected accepted status, got %q", result.Status)
	}
}

func TestRunTxExecuteWithdrawalPersistsWithdrawalAndBurnsSupply(t *testing.T) {
	t.Parallel()

	statePath := filepath.Join(t.TempDir(), "aegislink-state.json")
	app := seededRuntimeApp(t, statePath)
	claim := validDepositClaim(t)
	if _, err := app.SubmitDepositClaim(claim, validAttestationForClaim(claim)); err != nil {
		t.Fatalf("submit deposit claim: %v", err)
	}
	if err := app.Save(); err != nil {
		t.Fatalf("save state: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := run([]string{
		"tx", "execute-withdrawal",
		"--state-path", statePath,
		"--asset-id", claim.AssetID,
		"--amount", "25000000",
		"--recipient", "0xrecipient",
		"--deadline", "140",
		"--signature-base64", base64.StdEncoding.EncodeToString([]byte("threshold-proof")),
		"--height", "60",
	}, &stdout, &stderr); err != nil {
		t.Fatalf("run tx execute-withdrawal: %v\nstderr=%s", err, stderr.String())
	}

	loaded, err := aegisapp.Load(statePath)
	if err != nil {
		t.Fatalf("reload state: %v", err)
	}
	if supply := loaded.BridgeKeeper.SupplyForDenom("uethusdc"); supply.String() != "75000000" {
		t.Fatalf("expected remaining supply 75000000, got %s", supply.String())
	}

	withdrawals := loaded.Withdrawals(60, 60)
	if len(withdrawals) != 1 {
		t.Fatalf("expected one stored withdrawal, got %d", len(withdrawals))
	}
	if withdrawals[0].Recipient != "0xrecipient" {
		t.Fatalf("expected recipient 0xrecipient, got %q", withdrawals[0].Recipient)
	}
	if string(withdrawals[0].Signature) != "threshold-proof" {
		t.Fatalf("expected decoded signature threshold-proof, got %q", withdrawals[0].Signature)
	}

	var result struct {
		MessageID   string `json:"message_id"`
		AssetID     string `json:"asset_id"`
		Amount      string `json:"amount"`
		Recipient   string `json:"recipient"`
		BlockHeight uint64 `json:"block_height"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("decode output: %v\n%s", err, stdout.String())
	}
	if result.AssetID != claim.AssetID {
		t.Fatalf("expected asset id %q, got %q", claim.AssetID, result.AssetID)
	}
	if result.Amount != "25000000" {
		t.Fatalf("expected amount 25000000, got %q", result.Amount)
	}
	if result.Recipient != "0xrecipient" {
		t.Fatalf("expected recipient 0xrecipient, got %q", result.Recipient)
	}
	if result.BlockHeight != 60 {
		t.Fatalf("expected block height 60, got %d", result.BlockHeight)
	}
	if result.MessageID == "" {
		t.Fatal("expected withdrawal message id")
	}
}

func TestRunQueryRoutesPrintsPersistedIBCRoutes(t *testing.T) {
	t.Parallel()

	statePath := filepath.Join(t.TempDir(), "aegislink-state.json")
	app := seededRuntimeAppWithIBCRoute(t, statePath)
	if err := app.Save(); err != nil {
		t.Fatalf("save state: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := run([]string{
		"query", "routes",
		"--state-path", statePath,
	}, &stdout, &stderr); err != nil {
		t.Fatalf("run query routes: %v\nstderr=%s", err, stderr.String())
	}

	var routes []struct {
		AssetID            string `json:"asset_id"`
		DestinationChainID string `json:"destination_chain_id"`
		ChannelID          string `json:"channel_id"`
		DestinationDenom   string `json:"destination_denom"`
		Enabled            bool   `json:"enabled"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &routes); err != nil {
		t.Fatalf("decode output: %v\n%s", err, stdout.String())
	}
	if len(routes) != 1 {
		t.Fatalf("expected one route, got %d", len(routes))
	}
	if routes[0].DestinationChainID != "osmosis-1" {
		t.Fatalf("expected osmosis-1, got %q", routes[0].DestinationChainID)
	}
	if routes[0].ChannelID != "channel-0" {
		t.Fatalf("expected channel-0, got %q", routes[0].ChannelID)
	}
	if !routes[0].Enabled {
		t.Fatal("expected route to be enabled")
	}
}

func TestRunTxInitiateIBCTransferPersistsPendingTransfer(t *testing.T) {
	t.Parallel()

	statePath := filepath.Join(t.TempDir(), "aegislink-state.json")
	app := seededRuntimeAppWithIBCRoute(t, statePath)
	if err := app.Save(); err != nil {
		t.Fatalf("save state: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := run([]string{
		"tx", "initiate-ibc-transfer",
		"--state-path", statePath,
		"--asset-id", "eth.usdc",
		"--amount", "25000000",
		"--receiver", "osmo1recipient",
		"--timeout-height", "140",
		"--memo", "swap:uosmo",
	}, &stdout, &stderr); err != nil {
		t.Fatalf("run tx initiate-ibc-transfer: %v\nstderr=%s", err, stderr.String())
	}

	loaded, err := aegisapp.Load(statePath)
	if err != nil {
		t.Fatalf("reload state: %v", err)
	}
	transfers := loaded.IBCRouterKeeper.ExportTransfers()
	if len(transfers) != 1 {
		t.Fatalf("expected one transfer, got %d", len(transfers))
	}
	if transfers[0].Status != ibcrouterkeeper.TransferStatusPending {
		t.Fatalf("expected pending transfer, got %q", transfers[0].Status)
	}
	if transfers[0].Memo != "swap:uosmo" {
		t.Fatalf("expected memo swap:uosmo, got %q", transfers[0].Memo)
	}

	var result struct {
		TransferID         string `json:"transfer_id"`
		DestinationChainID string `json:"destination_chain_id"`
		Status             string `json:"status"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("decode output: %v\n%s", err, stdout.String())
	}
	if result.TransferID == "" {
		t.Fatal("expected transfer id")
	}
	if result.DestinationChainID != "osmosis-1" {
		t.Fatalf("expected osmosis-1, got %q", result.DestinationChainID)
	}
	if result.Status != "pending" {
		t.Fatalf("expected pending status, got %q", result.Status)
	}
}

func TestRunTxFailAndRefundIBCTransferPersistRecoverableState(t *testing.T) {
	t.Parallel()

	statePath := filepath.Join(t.TempDir(), "aegislink-state.json")
	app := seededRuntimeAppWithIBCRoute(t, statePath)
	transfer, err := app.IBCRouterKeeper.InitiateTransfer("eth.usdc", mustAmount(t, "25000000"), "osmo1recipient", 140, "swap:uosmo")
	if err != nil {
		t.Fatalf("initiate transfer: %v", err)
	}
	if err := app.Save(); err != nil {
		t.Fatalf("save state: %v", err)
	}

	var failStdout bytes.Buffer
	var failStderr bytes.Buffer
	if err := run([]string{
		"tx", "fail-ibc-transfer",
		"--state-path", statePath,
		"--transfer-id", transfer.TransferID,
		"--reason", "ack failed",
	}, &failStdout, &failStderr); err != nil {
		t.Fatalf("run tx fail-ibc-transfer: %v\nstderr=%s", err, failStderr.String())
	}

	var refundStdout bytes.Buffer
	var refundStderr bytes.Buffer
	if err := run([]string{
		"tx", "refund-ibc-transfer",
		"--state-path", statePath,
		"--transfer-id", transfer.TransferID,
	}, &refundStdout, &refundStderr); err != nil {
		t.Fatalf("run tx refund-ibc-transfer: %v\nstderr=%s", err, refundStderr.String())
	}

	loaded, err := aegisapp.Load(statePath)
	if err != nil {
		t.Fatalf("reload state: %v", err)
	}
	transfers := loaded.IBCRouterKeeper.ExportTransfers()
	if len(transfers) != 1 {
		t.Fatalf("expected one transfer, got %d", len(transfers))
	}
	if transfers[0].Status != ibcrouterkeeper.TransferStatusRefunded {
		t.Fatalf("expected refunded transfer, got %q", transfers[0].Status)
	}
	if transfers[0].FailureReason != "ack failed" {
		t.Fatalf("expected failure reason ack failed, got %q", transfers[0].FailureReason)
	}
}

func TestRunQueryTransfersPrintsPersistedTransferLifecycle(t *testing.T) {
	t.Parallel()

	statePath := filepath.Join(t.TempDir(), "aegislink-state.json")
	app := seededRuntimeAppWithIBCRoute(t, statePath)
	transfer, err := app.IBCRouterKeeper.InitiateTransfer("eth.usdc", mustAmount(t, "25000000"), "osmo1recipient", 140, "swap:uosmo")
	if err != nil {
		t.Fatalf("initiate transfer: %v", err)
	}
	if _, err := app.IBCRouterKeeper.TimeoutTransfer(transfer.TransferID); err != nil {
		t.Fatalf("timeout transfer: %v", err)
	}
	if err := app.Save(); err != nil {
		t.Fatalf("save state: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := run([]string{
		"query", "transfers",
		"--state-path", statePath,
	}, &stdout, &stderr); err != nil {
		t.Fatalf("run query transfers: %v\nstderr=%s", err, stderr.String())
	}

	var transfers []struct {
		TransferID    string `json:"transfer_id"`
		Status        string `json:"status"`
		FailureReason string `json:"failure_reason"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &transfers); err != nil {
		t.Fatalf("decode output: %v\n%s", err, stdout.String())
	}
	if len(transfers) != 1 {
		t.Fatalf("expected one transfer, got %d", len(transfers))
	}
	if transfers[0].TransferID != transfer.TransferID {
		t.Fatalf("expected transfer id %q, got %q", transfer.TransferID, transfers[0].TransferID)
	}
	if transfers[0].Status != "timed_out" {
		t.Fatalf("expected timed_out status, got %q", transfers[0].Status)
	}
}

func TestRunTxCompleteIBCTransferPersistsCompletedTransfer(t *testing.T) {
	t.Parallel()

	statePath := filepath.Join(t.TempDir(), "aegislink-state.json")
	app := seededRuntimeAppWithIBCRoute(t, statePath)
	transfer, err := app.IBCRouterKeeper.InitiateTransfer("eth.usdc", mustAmount(t, "25000000"), "osmo1recipient", 140, "swap:uosmo")
	if err != nil {
		t.Fatalf("initiate transfer: %v", err)
	}
	if err := app.Save(); err != nil {
		t.Fatalf("save state: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := run([]string{
		"tx", "complete-ibc-transfer",
		"--state-path", statePath,
		"--transfer-id", transfer.TransferID,
	}, &stdout, &stderr); err != nil {
		t.Fatalf("run tx complete-ibc-transfer: %v\nstderr=%s", err, stderr.String())
	}

	loaded, err := aegisapp.Load(statePath)
	if err != nil {
		t.Fatalf("reload state: %v", err)
	}
	transfers := loaded.IBCRouterKeeper.ExportTransfers()
	if len(transfers) != 1 {
		t.Fatalf("expected one transfer, got %d", len(transfers))
	}
	if transfers[0].Status != ibcrouterkeeper.TransferStatusCompleted {
		t.Fatalf("expected completed transfer, got %q", transfers[0].Status)
	}
	if transfers[0].FailureReason != "" {
		t.Fatalf("expected empty failure reason, got %q", transfers[0].FailureReason)
	}

	var result struct {
		TransferID string `json:"transfer_id"`
		Status     string `json:"status"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("decode output: %v\n%s", err, stdout.String())
	}
	if result.TransferID != transfer.TransferID {
		t.Fatalf("expected transfer id %q, got %q", transfer.TransferID, result.TransferID)
	}
	if result.Status != "completed" {
		t.Fatalf("expected completed status, got %q", result.Status)
	}
}

func writeSubmissionFile(t *testing.T, path string, claim bridgetypes.DepositClaim, attestation bridgetypes.Attestation) {
	t.Helper()

	payload := struct {
		Claim struct {
			Kind               string `json:"kind"`
			SourceChainID      string `json:"source_chain_id"`
			SourceContract     string `json:"source_contract"`
			SourceTxHash       string `json:"source_tx_hash"`
			SourceLogIndex     uint64 `json:"source_log_index"`
			Nonce              uint64 `json:"nonce"`
			MessageID          string `json:"message_id"`
			DestinationChainID string `json:"destination_chain_id"`
			AssetID            string `json:"asset_id"`
			Amount             string `json:"amount"`
			Recipient          string `json:"recipient"`
			Deadline           uint64 `json:"deadline"`
		} `json:"claim"`
		Attestation struct {
			MessageID        string   `json:"message_id"`
			PayloadHash      string   `json:"payload_hash"`
			Signers          []string `json:"signers"`
			Threshold        uint32   `json:"threshold"`
			Expiry           uint64   `json:"expiry"`
			SignerSetVersion uint64   `json:"signer_set_version"`
		} `json:"attestation"`
	}{}

	payload.Claim.Kind = string(claim.Identity.Kind)
	payload.Claim.SourceChainID = claim.Identity.SourceChainID
	payload.Claim.SourceContract = claim.Identity.SourceContract
	payload.Claim.SourceTxHash = claim.Identity.SourceTxHash
	payload.Claim.SourceLogIndex = claim.Identity.SourceLogIndex
	payload.Claim.Nonce = claim.Identity.Nonce
	payload.Claim.MessageID = claim.Identity.MessageID
	payload.Claim.DestinationChainID = claim.DestinationChainID
	payload.Claim.AssetID = claim.AssetID
	payload.Claim.Amount = claim.Amount.String()
	payload.Claim.Recipient = claim.Recipient
	payload.Claim.Deadline = claim.Deadline

	payload.Attestation.MessageID = attestation.MessageID
	payload.Attestation.PayloadHash = attestation.PayloadHash
	payload.Attestation.Signers = append([]string(nil), attestation.Signers...)
	payload.Attestation.Threshold = attestation.Threshold
	payload.Attestation.Expiry = attestation.Expiry
	payload.Attestation.SignerSetVersion = attestation.SignerSetVersion

	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal submission: %v", err)
	}
	if err := os.WriteFile(path, encoded, 0o644); err != nil {
		t.Fatalf("write submission: %v", err)
	}
}

func validDepositClaim(t *testing.T) bridgetypes.DepositClaim {
	t.Helper()

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

func parseJSONLogLines(t *testing.T, raw string) []map[string]any {
	t.Helper()

	lines := strings.Split(strings.TrimSpace(raw), "\n")
	var events []map[string]any
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var event map[string]any
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatalf("decode log line %q: %v", line, err)
		}
		events = append(events, event)
	}
	return events
}

func seededRuntimeApp(t *testing.T, statePath string) *aegisapp.App {
	t.Helper()

	app, err := aegisapp.NewWithConfig(aegisapp.Config{
		AppName:           aegisapp.AppName,
		Modules:           []string{"bridge", "registry", "limits", "pauser"},
		StatePath:         statePath,
		AllowedSigners:    []string{"relayer-1", "relayer-2", "relayer-3"},
		RequiredThreshold: 2,
	})
	if err != nil {
		t.Fatalf("new app: %v", err)
	}
	if err := app.RegisterAsset(registrytypes.Asset{
		AssetID:        "eth.usdc",
		SourceChainID:  "11155111",
		SourceContract: "0xasset",
		Denom:          "uethusdc",
		Decimals:       6,
		DisplayName:    "USDC",
		Enabled:        true,
	}); err != nil {
		t.Fatalf("register asset: %v", err)
	}
	if err := app.SetLimit(limittypes.RateLimit{
		AssetID:       "eth.usdc",
		WindowSeconds: 600,
		MaxAmount:     mustAmount(t, "1000000000000000000"),
	}); err != nil {
		t.Fatalf("set limit: %v", err)
	}
	app.SetCurrentHeight(50)
	return app
}

func seededRuntimeAppWithIBCRoute(t *testing.T, statePath string) *aegisapp.App {
	t.Helper()

	app := seededRuntimeApp(t, statePath)
	if err := app.IBCRouterKeeper.SetRoute(ibcrouterkeeper.Route{
		AssetID:            "eth.usdc",
		DestinationChainID: "osmosis-1",
		ChannelID:          "channel-0",
		DestinationDenom:   "ibc/uatom-usdc",
		Enabled:            true,
	}); err != nil {
		t.Fatalf("set ibc route: %v", err)
	}
	return app
}

func initSDKRuntimeHome(t *testing.T, homeDir string) aegisapp.Config {
	t.Helper()

	cfg, err := aegisapp.InitHome(aegisapp.Config{
		HomeDir:           homeDir,
		AppName:           aegisapp.AppName,
		ChainID:           "aegislink-sdk-1",
		RuntimeMode:       aegisapp.RuntimeModeSDKStore,
		Modules:           []string{"bridge", "registry", "limits", "pauser", "ibcrouter"},
		AllowedSigners:    []string{"relayer-1", "relayer-2", "relayer-3"},
		RequiredThreshold: 2,
	}, false)
	if err != nil {
		t.Fatalf("init sdk runtime home: %v", err)
	}
	return cfg
}

func seededSDKRuntimeApp(t *testing.T, cfg aegisapp.Config) *aegisapp.App {
	t.Helper()

	app, err := aegisapp.LoadWithConfig(cfg)
	if err != nil {
		t.Fatalf("load sdk runtime app: %v", err)
	}
	if err := app.RegisterAsset(registrytypes.Asset{
		AssetID:        "eth.usdc",
		SourceChainID:  "11155111",
		SourceContract: "0xasset",
		Denom:          "uethusdc",
		Decimals:       6,
		DisplayName:    "USDC",
		Enabled:        true,
	}); err != nil {
		t.Fatalf("register asset: %v", err)
	}
	if err := app.SetLimit(limittypes.RateLimit{
		AssetID:       "eth.usdc",
		WindowSeconds: 600,
		MaxAmount:     mustAmount(t, "1000000000000000000"),
	}); err != nil {
		t.Fatalf("set limit: %v", err)
	}
	return app
}
