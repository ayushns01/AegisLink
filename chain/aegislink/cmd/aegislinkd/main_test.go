package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	aegisapp "github.com/ayushns01/aegislink/chain/aegislink/app"
	bridgekeeper "github.com/ayushns01/aegislink/chain/aegislink/x/bridge/keeper"
	bridgetypes "github.com/ayushns01/aegislink/chain/aegislink/x/bridge/types"
	ibcrouterkeeper "github.com/ayushns01/aegislink/chain/aegislink/x/ibcrouter/keeper"
	ibcroutertypes "github.com/ayushns01/aegislink/chain/aegislink/x/ibcrouter/types"
	limittypes "github.com/ayushns01/aegislink/chain/aegislink/x/limits/types"
	registrytypes "github.com/ayushns01/aegislink/chain/aegislink/x/registry/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
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
		Signers:     bridgetypes.DefaultHarnessSignerAddresses()[1:4],
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
		Signers:     bridgetypes.DefaultHarnessSignerAddresses()[1:4],
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
	if last["module_count"] != float64(7) {
		t.Fatalf("expected module count 7, got %+v", last["module_count"])
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

func TestRunDemoNodeStartEmitsBoundEndpointInfoAndServesHealth(t *testing.T) {
	t.Parallel()

	homeDir := filepath.Join(t.TempDir(), "demo-node-home")
	if _, err := aegisapp.InitHome(aegisapp.Config{
		HomeDir:     homeDir,
		ChainID:     "aegislink-ibc-demo-1",
		RuntimeMode: aegisapp.RuntimeModeSDKStore,
	}, false); err != nil {
		t.Fatalf("init home: %v", err)
	}

	readyPath := filepath.Join(t.TempDir(), "demo-node-ready.json")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	errCh := make(chan error, 1)
	go func() {
		errCh <- runWithContext(ctx, []string{
			"demo-node", "start",
			"--home", homeDir,
			"--rpc-address", "127.0.0.1:0",
			"--grpc-address", "127.0.0.1:0",
			"--ready-file", readyPath,
		}, &stdout, &stderr)
	}()

	var ready struct {
		Status      string `json:"status"`
		ChainID     string `json:"chain_id"`
		RPCAddress  string `json:"rpc_address"`
		GRPCAddress string `json:"grpc_address"`
	}
	waitForReadyFile(t, readyPath, &ready)
	if ready.Status != "ready" {
		t.Fatalf("expected ready status, got %+v", ready)
	}
	if ready.ChainID != "aegislink-ibc-demo-1" {
		t.Fatalf("expected chain id aegislink-ibc-demo-1, got %+v", ready)
	}
	if ready.RPCAddress == "" || ready.GRPCAddress == "" {
		t.Fatalf("expected bound addresses, got %+v", ready)
	}

	resp, err := http.Get("http://" + ready.RPCAddress + "/healthz")
	if err != nil {
		t.Fatalf("probe demo-node health endpoint: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected HTTP 200 health response, got %d", resp.StatusCode)
	}

	conn, err := net.DialTimeout("tcp", ready.GRPCAddress, time.Second)
	if err != nil {
		t.Fatalf("dial grpc endpoint: %v", err)
	}
	_ = conn.Close()

	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("demo node exited with error: %v\nstderr=%s", err, stderr.String())
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for demo node shutdown")
	}
}

func TestRunDemoNodeStartServesPersistedRuntimeState(t *testing.T) {
	t.Parallel()

	homeDir := filepath.Join(t.TempDir(), "demo-node-home")
	cfg := initSDKRuntimeHome(t, homeDir)
	app := seededSDKRuntimeApp(t, cfg)

	claim := validDepositClaim(t)
	if _, err := app.SubmitDepositClaim(claim, validAttestationForClaim(claim)); err != nil {
		t.Fatalf("submit deposit claim: %v", err)
	}
	app.SetCurrentHeight(73)
	if err := app.Close(); err != nil {
		t.Fatalf("close seeded app: %v", err)
	}

	readyPath := filepath.Join(t.TempDir(), "demo-node-ready.json")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	errCh := make(chan error, 1)
	go func() {
		errCh <- runWithContext(ctx, []string{
			"demo-node", "start",
			"--home", homeDir,
			"--rpc-address", "127.0.0.1:0",
			"--grpc-address", "127.0.0.1:0",
			"--ready-file", readyPath,
		}, &stdout, &stderr)
	}()

	var ready struct {
		RPCAddress string `json:"rpc_address"`
	}
	waitForReadyFile(t, readyPath, &ready)
	if ready.RPCAddress == "" {
		t.Fatal("expected demo node RPC address in ready file")
	}

	statusResp, err := http.Get("http://" + ready.RPCAddress + "/status")
	if err != nil {
		t.Fatalf("fetch demo-node status: %v", err)
	}
	defer statusResp.Body.Close()
	if statusResp.StatusCode != http.StatusOK {
		t.Fatalf("expected HTTP 200 status response, got %d", statusResp.StatusCode)
	}

	var status struct {
		AppName         string            `json:"app_name"`
		ChainID         string            `json:"chain_id"`
		CurrentHeight   uint64            `json:"current_height"`
		Assets          int               `json:"assets"`
		ProcessedClaims int               `json:"processed_claims"`
		SupplyByDenom   map[string]string `json:"supply_by_denom"`
	}
	if err := json.NewDecoder(statusResp.Body).Decode(&status); err != nil {
		t.Fatalf("decode demo-node status response: %v", err)
	}
	if status.AppName != aegisapp.AppName {
		t.Fatalf("expected app name %q, got %q", aegisapp.AppName, status.AppName)
	}
	if status.ChainID != "aegislink-sdk-1" {
		t.Fatalf("expected chain id aegislink-sdk-1, got %q", status.ChainID)
	}
	if status.CurrentHeight != 73 {
		t.Fatalf("expected current height 73, got %d", status.CurrentHeight)
	}
	if status.Assets != 1 {
		t.Fatalf("expected one registered asset, got %d", status.Assets)
	}
	if status.ProcessedClaims != 1 {
		t.Fatalf("expected one processed claim, got %d", status.ProcessedClaims)
	}
	if got := status.SupplyByDenom["uethusdc"]; got != claim.Amount.String() {
		t.Fatalf("expected uethusdc supply %s, got %q", claim.Amount.String(), got)
	}

	balancesResp, err := http.Get("http://" + ready.RPCAddress + "/balances?address=" + claim.Recipient)
	if err != nil {
		t.Fatalf("fetch demo-node balances: %v", err)
	}
	defer balancesResp.Body.Close()
	if balancesResp.StatusCode != http.StatusOK {
		t.Fatalf("expected HTTP 200 balances response, got %d", balancesResp.StatusCode)
	}

	var balances []struct {
		Address string `json:"address"`
		Denom   string `json:"denom"`
		Amount  string `json:"amount"`
	}
	if err := json.NewDecoder(balancesResp.Body).Decode(&balances); err != nil {
		t.Fatalf("decode demo-node balances response: %v", err)
	}
	if len(balances) != 1 {
		t.Fatalf("expected one wallet balance, got %+v", balances)
	}
	if balances[0].Address != claim.Recipient {
		t.Fatalf("expected recipient %q, got %q", claim.Recipient, balances[0].Address)
	}
	if balances[0].Denom != "uethusdc" {
		t.Fatalf("expected denom uethusdc, got %q", balances[0].Denom)
	}
	if balances[0].Amount != claim.Amount.String() {
		t.Fatalf("expected amount %s, got %q", claim.Amount.String(), balances[0].Amount)
	}

	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("demo node exited with error: %v\nstderr=%s", err, stderr.String())
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for demo node shutdown")
	}
}

func TestRunDemoNodeStatusReadsReadyFileAndProbesHealth(t *testing.T) {
	t.Parallel()

	homeDir := filepath.Join(t.TempDir(), "demo-node-home")
	if _, err := aegisapp.InitHome(aegisapp.Config{
		HomeDir:     homeDir,
		ChainID:     "aegislink-ibc-demo-1",
		RuntimeMode: aegisapp.RuntimeModeSDKStore,
	}, false); err != nil {
		t.Fatalf("init home: %v", err)
	}

	readyPath := filepath.Join(t.TempDir(), "demo-node-ready.json")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	errCh := make(chan error, 1)
	go func() {
		errCh <- runWithContext(ctx, []string{
			"demo-node", "start",
			"--home", homeDir,
			"--rpc-address", "127.0.0.1:0",
			"--grpc-address", "127.0.0.1:0",
			"--ready-file", readyPath,
		}, &stdout, &stderr)
	}()

	var ready struct {
		RPCAddress string `json:"rpc_address"`
	}
	waitForReadyFile(t, readyPath, &ready)

	var statusOut bytes.Buffer
	var statusErr bytes.Buffer
	if err := run([]string{
		"demo-node", "status",
		"--home", homeDir,
		"--ready-file", readyPath,
	}, &statusOut, &statusErr); err != nil {
		t.Fatalf("run demo-node status: %v\nstderr=%s", err, statusErr.String())
	}

	var status struct {
		Status      string `json:"status"`
		ChainID     string `json:"chain_id"`
		RPCAddress  string `json:"rpc_address"`
		GRPCAddress string `json:"grpc_address"`
		Healthy     bool   `json:"healthy"`
	}
	if err := json.Unmarshal(statusOut.Bytes(), &status); err != nil {
		t.Fatalf("decode demo-node status output: %v\n%s", err, statusOut.String())
	}
	if status.Status != "ready" {
		t.Fatalf("expected ready status, got %+v", status)
	}
	if status.ChainID != "aegislink-ibc-demo-1" {
		t.Fatalf("expected chain id aegislink-ibc-demo-1, got %+v", status)
	}
	if status.RPCAddress == "" || status.GRPCAddress == "" {
		t.Fatalf("expected bound endpoint info, got %+v", status)
	}
	if !status.Healthy {
		t.Fatalf("expected healthy demo node status, got %+v", status)
	}

	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("demo node exited with error: %v\nstderr=%s", err, stderr.String())
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for demo node shutdown")
	}
}

func TestRunDemoNodeQueuesDepositClaimOverHTTPAndAppliesItOnTick(t *testing.T) {
	t.Parallel()

	homeDir := filepath.Join(t.TempDir(), "demo-node-home")
	cfg := initSDKRuntimeHome(t, homeDir)
	app := seededSDKRuntimeApp(t, cfg)
	if err := app.Close(); err != nil {
		t.Fatalf("close seeded app: %v", err)
	}

	readyPath := filepath.Join(t.TempDir(), "demo-node-ready.json")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	errCh := make(chan error, 1)
	go func() {
		errCh <- runWithContext(ctx, []string{
			"demo-node", "start",
			"--home", homeDir,
			"--rpc-address", "127.0.0.1:0",
			"--grpc-address", "127.0.0.1:0",
			"--ready-file", readyPath,
			"--tick-interval-ms", "10",
		}, &stdout, &stderr)
	}()

	var ready struct {
		RPCAddress string `json:"rpc_address"`
	}
	waitForReadyFile(t, readyPath, &ready)

	claim := validDepositClaim(t)
	attestation := validAttestationForClaim(claim)
	resp, err := http.Post(
		"http://"+ready.RPCAddress+"/tx/queue-deposit-claim",
		"application/json",
		bytes.NewReader(marshalSubmissionPayload(t, claim, attestation)),
	)
	if err != nil {
		t.Fatalf("post queued deposit claim: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected HTTP 200 queue response, got %d body=%s", resp.StatusCode, string(body))
	}

	var queued struct {
		Status    string `json:"status"`
		MessageID string `json:"message_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&queued); err != nil {
		t.Fatalf("decode queued deposit response: %v", err)
	}
	if queued.Status != "queued" || queued.MessageID != claim.Identity.MessageID {
		t.Fatalf("unexpected queue response: %+v", queued)
	}

	var status struct {
		CurrentHeight   uint64            `json:"current_height"`
		ProcessedClaims int               `json:"processed_claims"`
		SupplyByDenom   map[string]string `json:"supply_by_denom"`
	}
	waitForHTTPJSON(t, "http://"+ready.RPCAddress+"/status", &status, func() bool {
		return status.CurrentHeight > 0 &&
			status.ProcessedClaims == 1 &&
			status.SupplyByDenom["uethusdc"] == claim.Amount.String()
	})

	var balances []struct {
		Address string `json:"address"`
		Denom   string `json:"denom"`
		Amount  string `json:"amount"`
	}
	waitForHTTPJSON(t, "http://"+ready.RPCAddress+"/balances?address="+claim.Recipient, &balances, func() bool {
		return len(balances) == 1 && balances[0].Amount == claim.Amount.String()
	})
	if balances[0].Address != claim.Recipient || balances[0].Denom != "uethusdc" {
		t.Fatalf("unexpected wallet balances: %+v", balances)
	}

	var cliOut bytes.Buffer
	var cliErr bytes.Buffer
	if err := run([]string{
		"demo-node", "balances",
		"--home", homeDir,
		"--ready-file", readyPath,
		"--address", claim.Recipient,
	}, &cliOut, &cliErr); err != nil {
		t.Fatalf("run demo-node balances: %v\nstderr=%s", err, cliErr.String())
	}

	var cliBalances []struct {
		Address string `json:"address"`
		Denom   string `json:"denom"`
		Amount  string `json:"amount"`
	}
	if err := json.Unmarshal(cliOut.Bytes(), &cliBalances); err != nil {
		t.Fatalf("decode demo-node balances output: %v\n%s", err, cliOut.String())
	}
	if len(cliBalances) != 1 || cliBalances[0].Amount != claim.Amount.String() {
		t.Fatalf("unexpected cli balances: %+v", cliBalances)
	}

	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("demo node exited with error: %v\nstderr=%s", err, stderr.String())
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for demo node shutdown")
	}
}

func TestRunDemoNodeInitiatesIBCTransferOverHTTPAndListsTransfers(t *testing.T) {
	t.Parallel()

	homeDir := filepath.Join(t.TempDir(), "demo-node-home")
	cfg := initSDKRuntimeHome(t, homeDir)
	app := seededSDKRuntimeApp(t, cfg)
	if err := app.IBCRouterKeeper.SetRoute(ibcrouterkeeper.Route{
		AssetID:            "eth.usdc",
		DestinationChainID: "osmosis-1",
		ChannelID:          "channel-0",
		DestinationDenom:   "ibc/uethusdc",
		Enabled:            true,
	}); err != nil {
		t.Fatalf("set route: %v", err)
	}
	if err := app.Close(); err != nil {
		t.Fatalf("close seeded app: %v", err)
	}

	readyPath := filepath.Join(t.TempDir(), "demo-node-ready.json")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	errCh := make(chan error, 1)
	go func() {
		errCh <- runWithContext(ctx, []string{
			"demo-node", "start",
			"--home", homeDir,
			"--rpc-address", "127.0.0.1:0",
			"--grpc-address", "127.0.0.1:0",
			"--ready-file", readyPath,
			"--tick-interval-ms", "0",
		}, &stdout, &stderr)
	}()

	var ready struct {
		RPCAddress string `json:"rpc_address"`
	}
	waitForReadyFile(t, readyPath, &ready)

	payload := map[string]any{
		"asset_id":       "eth.usdc",
		"amount":         "4200",
		"receiver":       "osmo1q5nq6v24qq0584nf00wuhqrku4anlxaq05wsj8",
		"timeout_height": 120,
		"memo":           "bridge-demo",
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal transfer payload: %v", err)
	}
	resp, err := http.Post(
		"http://"+ready.RPCAddress+"/tx/initiate-ibc-transfer",
		"application/json",
		bytes.NewReader(encoded),
	)
	if err != nil {
		t.Fatalf("post initiate ibc transfer: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected HTTP 200 initiate transfer response, got %d body=%s", resp.StatusCode, string(body))
	}

	var transfer struct {
		TransferID string `json:"transfer_id"`
		Status     string `json:"status"`
		Receiver   string `json:"receiver"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&transfer); err != nil {
		t.Fatalf("decode transfer response: %v", err)
	}
	if transfer.TransferID == "" || transfer.Status != "pending" {
		t.Fatalf("unexpected transfer response: %+v", transfer)
	}

	var transfers []struct {
		TransferID string `json:"transfer_id"`
		AssetID    string `json:"asset_id"`
		Amount     string `json:"amount"`
		Receiver   string `json:"receiver"`
		Status     string `json:"status"`
	}
	waitForHTTPJSON(t, "http://"+ready.RPCAddress+"/transfers", &transfers, func() bool {
		return len(transfers) == 1
	})
	if transfers[0].TransferID != transfer.TransferID {
		t.Fatalf("expected transfer id %q, got %+v", transfer.TransferID, transfers)
	}
	if transfers[0].AssetID != "eth.usdc" || transfers[0].Amount != "4200" || transfers[0].Receiver != payload["receiver"] || transfers[0].Status != "pending" {
		t.Fatalf("unexpected transfer list: %+v", transfers)
	}

	var cliOut bytes.Buffer
	var cliErr bytes.Buffer
	if err := run([]string{
		"demo-node", "transfers",
		"--home", homeDir,
		"--ready-file", readyPath,
	}, &cliOut, &cliErr); err != nil {
		t.Fatalf("run demo-node transfers: %v\nstderr=%s", err, cliErr.String())
	}

	var cliTransfers []struct {
		TransferID string `json:"transfer_id"`
		Status     string `json:"status"`
	}
	if err := json.Unmarshal(cliOut.Bytes(), &cliTransfers); err != nil {
		t.Fatalf("decode demo-node transfers output: %v\n%s", err, cliOut.String())
	}
	if len(cliTransfers) != 1 || cliTransfers[0].TransferID != transfer.TransferID || cliTransfers[0].Status != "pending" {
		t.Fatalf("unexpected cli transfers: %+v", cliTransfers)
	}

	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("demo node exited with error: %v\nstderr=%s", err, stderr.String())
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for demo node shutdown")
	}
}

func TestRunTxQueueDepositClaimPersistsPendingSubmission(t *testing.T) {
	t.Parallel()

	statePath := filepath.Join(t.TempDir(), "aegislink-state.json")
	app := seededRuntimeApp(t, statePath)
	claim := validDepositClaim(t)
	submissionPath := filepath.Join(t.TempDir(), "submission.json")
	writeSubmissionFile(t, submissionPath, claim, validAttestationForClaim(claim))
	if err := app.Save(); err != nil {
		t.Fatalf("save seeded state: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := run([]string{
		"tx", "queue-deposit-claim",
		"--state-path", statePath,
		"--submission-file", submissionPath,
	}, &stdout, &stderr); err != nil {
		t.Fatalf("run tx queue-deposit-claim: %v\nstderr=%s", err, stderr.String())
	}

	loaded, err := aegisapp.Load(statePath)
	if err != nil {
		t.Fatalf("reload state: %v", err)
	}
	if got := len(loaded.Status().ModuleNames); got == 0 {
		t.Fatalf("expected loaded runtime to remain valid")
	}
	if got := loaded.Status().PendingDepositClaims; got != 1 {
		t.Fatalf("expected one pending deposit claim, got %d", got)
	}

	var result struct {
		Status    string `json:"status"`
		MessageID string `json:"message_id"`
		AssetID   string `json:"asset_id"`
		Amount    string `json:"amount"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("decode output: %v\n%s", err, stdout.String())
	}
	if result.Status != "queued" {
		t.Fatalf("expected queued status, got %q", result.Status)
	}
	if result.MessageID != claim.Identity.MessageID {
		t.Fatalf("expected message id %q, got %q", claim.Identity.MessageID, result.MessageID)
	}
	if result.AssetID != claim.AssetID {
		t.Fatalf("expected asset id %q, got %q", claim.AssetID, result.AssetID)
	}
	if result.Amount != claim.Amount.String() {
		t.Fatalf("expected amount %s, got %q", claim.Amount.String(), result.Amount)
	}
}

func TestRunStartDaemonAdvancesHeightAndProcessesQueuedClaims(t *testing.T) {
	t.Parallel()

	homeDir := filepath.Join(t.TempDir(), "home")
	cfg, err := aegisapp.InitHome(aegisapp.Config{
		HomeDir: homeDir,
		ChainID: "aegislink-devnet-1",
	}, false)
	if err != nil {
		t.Fatalf("init home: %v", err)
	}
	app := seededRuntimeApp(t, cfg.StatePath)
	claim := validDepositClaim(t)
	submissionPath := filepath.Join(t.TempDir(), "submission.json")
	writeSubmissionFile(t, submissionPath, claim, validAttestationForClaim(claim))
	if err := app.Save(); err != nil {
		t.Fatalf("save seeded state: %v", err)
	}

	queueOutput := bytes.Buffer{}
	if err := run([]string{
		"tx", "queue-deposit-claim",
		"--home", homeDir,
		"--submission-file", submissionPath,
	}, &queueOutput, io.Discard); err != nil {
		t.Fatalf("run queue-deposit-claim: %v\n%s", err, queueOutput.String())
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := run([]string{
		"start",
		"--home", homeDir,
		"--daemon",
		"--tick-interval-ms", "1",
		"--max-blocks", "2",
	}, &stdout, &stderr); err != nil {
		t.Fatalf("run start daemon: %v\nstderr=%s", err, stderr.String())
	}

	var result struct {
		Status        string `json:"status"`
		CurrentHeight uint64 `json:"current_height"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("decode daemon output: %v\n%s", err, stdout.String())
	}
	if result.Status != "stopped" {
		t.Fatalf("expected stopped daemon status, got %q", result.Status)
	}
	if result.CurrentHeight < 52 {
		t.Fatalf("expected daemon height to advance past seed height, got %d", result.CurrentHeight)
	}

	loaded, err := aegisapp.LoadWithConfig(aegisapp.Config{
		HomeDir: homeDir,
	})
	if err != nil {
		t.Fatalf("reload state: %v", err)
	}
	if got := loaded.Status().PendingDepositClaims; got != 0 {
		t.Fatalf("expected queued claim to be drained, got %d", got)
	}
	if got := loaded.Status().ProcessedClaims; got != 1 {
		t.Fatalf("expected one processed claim, got %d", got)
	}
	if !strings.Contains(stderr.String(), "runtime_stop") {
		t.Fatalf("expected daemon stop log, got stderr=%s", stderr.String())
	}
}

func TestRunQuerySignerSetReturnsActiveSignerSet(t *testing.T) {
	t.Parallel()

	statePath := filepath.Join(t.TempDir(), "aegislink-state.json")
	app := seededRuntimeAppWithIBCRoute(t, statePath)
	app.SetCurrentHeight(90)
	if err := app.BridgeKeeper.UpsertSignerSet(bridgekeeper.SignerSet{
		Version:     2,
		Signers:     bridgetypes.DefaultHarnessSignerAddresses()[1:4],
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
		Signers:     bridgetypes.DefaultHarnessSignerAddresses()[1:4],
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

func TestMetricsRunQueryMetricsPrintsPrometheusSnapshot(t *testing.T) {
	t.Parallel()

	statePath := filepath.Join(t.TempDir(), "aegislink-state.json")
	app := seededRuntimeAppWithIBCRoute(t, statePath)
	app.SetCurrentHeight(75)
	claim := validDepositClaim(t)
	if _, err := app.SubmitDepositClaim(claim, validAttestationForClaim(claim)); err != nil {
		t.Fatalf("submit deposit claim: %v", err)
	}
	if _, err := app.InitiateIBCTransfer("eth.usdc", mustAmount(t, "25000000"), "osmo1receiver", 120, "swap:uosmo"); err != nil {
		t.Fatalf("initiate ibc transfer: %v", err)
	}
	if _, err := app.TimeoutIBCTransfer("ibc/eth.usdc/1"); err != nil {
		t.Fatalf("timeout ibc transfer: %v", err)
	}
	if err := app.Save(); err != nil {
		t.Fatalf("save state: %v", err)
	}

	var stdout bytes.Buffer
	if err := run([]string{"query", "metrics", "--state-path", statePath}, &stdout, io.Discard); err != nil {
		t.Fatalf("run query metrics: %v", err)
	}

	output := stdout.String()
	for _, expected := range []string{
		"aegislink_runtime_processed_claims_total",
		"aegislink_runtime_failed_claims_total",
		"aegislink_runtime_pending_transfers",
		"aegislink_runtime_timed_out_transfers_total",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected metrics output to contain %q\n%s", expected, output)
		}
	}
}

func TestRunTxSubmitDepositClaimPersistsAcceptedClaim(t *testing.T) {
	t.Parallel()

	statePath := filepath.Join(t.TempDir(), "aegislink-state.json")
	app, err := aegisapp.NewWithConfig(aegisapp.Config{
		AppName:           aegisapp.AppName,
		Modules:           []string{"bridge", "bank", "registry", "limits", "pauser", "governance"},
		StatePath:         statePath,
		AllowedSigners:    bridgetypes.DefaultHarnessSignerAddresses()[:3],
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
		"--owner-address", claim.Recipient,
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

func TestRunQueryRouteProfilesPrintsPersistedRouteProfiles(t *testing.T) {
	t.Parallel()

	statePath := filepath.Join(t.TempDir(), "aegislink-state.json")
	app := seededRuntimeAppWithIBCProfile(t, statePath)
	if err := app.Save(); err != nil {
		t.Fatalf("save state: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := run([]string{
		"query", "route-profiles",
		"--state-path", statePath,
	}, &stdout, &stderr); err != nil {
		t.Fatalf("run query route-profiles: %v\nstderr=%s", err, stderr.String())
	}

	var profiles []struct {
		RouteID            string `json:"route_id"`
		DestinationChainID string `json:"destination_chain_id"`
		ChannelID          string `json:"channel_id"`
		Enabled            bool   `json:"enabled"`
		Assets             []struct {
			AssetID          string `json:"asset_id"`
			DestinationDenom string `json:"destination_denom"`
		} `json:"assets"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &profiles); err != nil {
		t.Fatalf("decode output: %v\n%s", err, stdout.String())
	}
	if len(profiles) != 1 {
		t.Fatalf("expected one route profile, got %d", len(profiles))
	}
	if profiles[0].RouteID != "osmosis-public-wallet" {
		t.Fatalf("expected osmosis-public-wallet route id, got %q", profiles[0].RouteID)
	}
	if profiles[0].DestinationChainID != "osmosis-testnet" {
		t.Fatalf("expected osmosis-testnet destination chain, got %q", profiles[0].DestinationChainID)
	}
	if profiles[0].ChannelID != "channel-42" {
		t.Fatalf("expected channel-42, got %q", profiles[0].ChannelID)
	}
	if !profiles[0].Enabled {
		t.Fatal("expected route profile to be enabled")
	}
	if len(profiles[0].Assets) != 1 || profiles[0].Assets[0].AssetID != "eth.usdc" {
		t.Fatalf("expected route profile asset allowlist to include eth.usdc, got %+v", profiles[0].Assets)
	}
	if profiles[0].Assets[0].DestinationDenom != "ibc/uethusdc" {
		t.Fatalf("expected profile destination denom ibc/uethusdc, got %q", profiles[0].Assets[0].DestinationDenom)
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

func TestRunTxInitiateIBCTransferWithRouteProfilePersistsPendingTransfer(t *testing.T) {
	t.Parallel()

	statePath := filepath.Join(t.TempDir(), "aegislink-state.json")
	app := seededRuntimeAppWithIBCProfile(t, statePath)
	if err := app.Save(); err != nil {
		t.Fatalf("save state: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := run([]string{
		"tx", "initiate-ibc-transfer",
		"--state-path", statePath,
		"--route-id", "osmosis-public-wallet",
		"--asset-id", "eth.usdc",
		"--amount", "25000000",
		"--receiver", "osmo1recipient",
		"--timeout-height", "140",
		"--memo", "swap:uosmo",
	}, &stdout, &stderr); err != nil {
		t.Fatalf("run tx initiate-ibc-transfer with route profile: %v\nstderr=%s", err, stderr.String())
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
	if transfers[0].DestinationChainID != "osmosis-testnet" {
		t.Fatalf("expected osmosis-testnet destination chain, got %q", transfers[0].DestinationChainID)
	}
	if transfers[0].ChannelID != "channel-42" {
		t.Fatalf("expected channel-42, got %q", transfers[0].ChannelID)
	}
	if transfers[0].DestinationDenom != "ibc/uethusdc" {
		t.Fatalf("expected ibc/uethusdc destination denom, got %q", transfers[0].DestinationDenom)
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
	if result.DestinationChainID != "osmosis-testnet" {
		t.Fatalf("expected osmosis-testnet, got %q", result.DestinationChainID)
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

func TestRunTxApplyAssetStatusProposalRequiresAuthorizedAuthority(t *testing.T) {
	t.Parallel()

	statePath := filepath.Join(t.TempDir(), "aegislink-state.json")
	app := seededRuntimeApp(t, statePath)
	if err := app.Save(); err != nil {
		t.Fatalf("save seeded runtime: %v", err)
	}

	var stdout bytes.Buffer
	if err := run([]string{
		"tx", "apply-asset-status-proposal",
		"--state-path", statePath,
		"--proposal-id", "asset-disable-unauthorized",
		"--asset-id", "eth.usdc",
		"--enabled=false",
		"--authority", "intruder",
	}, &stdout, io.Discard); err == nil {
		t.Fatal("expected unauthorized governance proposal to be rejected")
	}

	stdout.Reset()
	if err := run([]string{
		"tx", "apply-asset-status-proposal",
		"--state-path", statePath,
		"--proposal-id", "asset-disable-authorized",
		"--asset-id", "eth.usdc",
		"--enabled=false",
		"--authority", "guardian-1",
	}, &stdout, io.Discard); err != nil {
		t.Fatalf("run authorized asset status proposal: %v", err)
	}

	reloaded, err := aegisapp.Load(statePath)
	if err != nil {
		t.Fatalf("reload runtime: %v", err)
	}
	asset, ok := reloaded.RegistryKeeper.GetAsset("eth.usdc")
	if !ok || asset.Enabled {
		t.Fatalf("expected asset to be disabled by authorized proposal, got %+v exists=%t", asset, ok)
	}
	proposals := reloaded.GovernanceKeeper.ExportState().AppliedProposals
	if len(proposals) != 1 || proposals[0].AppliedBy != "guardian-1" {
		t.Fatalf("expected applied_by guardian-1, got %+v", proposals)
	}
}

func writeSubmissionFile(t *testing.T, path string, claim bridgetypes.DepositClaim, attestation bridgetypes.Attestation) {
	t.Helper()

	encoded := marshalSubmissionPayload(t, claim, attestation)
	if err := os.WriteFile(path, encoded, 0o644); err != nil {
		t.Fatalf("write submission: %v", err)
	}
}

func marshalSubmissionPayload(t *testing.T, claim bridgetypes.DepositClaim, attestation bridgetypes.Attestation) []byte {
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
			MessageID        string                         `json:"message_id"`
			PayloadHash      string                         `json:"payload_hash"`
			Signers          []string                       `json:"signers"`
			Proofs           []bridgetypes.AttestationProof `json:"proofs"`
			Threshold        uint32                         `json:"threshold"`
			Expiry           uint64                         `json:"expiry"`
			SignerSetVersion uint64                         `json:"signer_set_version"`
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
	payload.Attestation.Proofs = append([]bridgetypes.AttestationProof(nil), attestation.Proofs...)
	payload.Attestation.Threshold = attestation.Threshold
	payload.Attestation.Expiry = attestation.Expiry
	payload.Attestation.SignerSetVersion = attestation.SignerSetVersion

	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal submission: %v", err)
	}
	return encoded
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
		Recipient:          sdk.AccAddress([]byte("main-test-wallet")).String(),
		Deadline:           100,
	}
}

func validAttestationForClaim(claim bridgetypes.DepositClaim) bridgetypes.Attestation {
	attestation := bridgetypes.Attestation{
		MessageID:        claim.Identity.MessageID,
		PayloadHash:      claim.Digest(),
		Signers:          bridgetypes.DefaultHarnessSignerAddresses()[:2],
		Threshold:        2,
		Expiry:           120,
		SignerSetVersion: 1,
	}
	for _, key := range bridgetypes.DefaultHarnessSignerPrivateKeys()[:2] {
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
		Modules:           []string{"bridge", "bank", "registry", "limits", "pauser", "governance"},
		StatePath:         statePath,
		AllowedSigners:    bridgetypes.DefaultHarnessSignerAddresses()[:3],
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

func waitForReadyFile(t *testing.T, path string, target any) {
	t.Helper()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(path)
		if err == nil {
			if err := json.Unmarshal(data, target); err != nil {
				t.Fatalf("decode ready file %s: %v", path, err)
			}
			return
		}
		if !os.IsNotExist(err) {
			t.Fatalf("read ready file %s: %v", path, err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for ready file %s", path)
}

func waitForHTTPJSON(t *testing.T, endpoint string, target any, satisfied func() bool) {
	t.Helper()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(endpoint)
		if err == nil {
			if resp.StatusCode == http.StatusOK {
				if err := json.NewDecoder(resp.Body).Decode(target); err == nil && satisfied() {
					_ = resp.Body.Close()
					return
				}
			}
			_ = resp.Body.Close()
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for HTTP condition at %s", endpoint)
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

func seededRuntimeAppWithIBCProfile(t *testing.T, statePath string) *aegisapp.App {
	t.Helper()

	app := seededRuntimeApp(t, statePath)
	if err := app.SetRouteProfile(ibcroutertypes.RouteProfile{
		RouteID:            "osmosis-public-wallet",
		DestinationChainID: "osmosis-testnet",
		ChannelID:          "channel-42",
		Enabled:            true,
		Assets: []ibcroutertypes.AssetRoute{
			{AssetID: "eth.usdc", DestinationDenom: "ibc/uethusdc"},
		},
		Policy: ibcroutertypes.RoutePolicy{
			AllowedMemoPrefixes: []string{"swap:", "stake:"},
			AllowedActionTypes:  []string{"swap", "stake"},
		},
	}); err != nil {
		t.Fatalf("set ibc route profile: %v", err)
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
		Modules:           []string{"bridge", "bank", "registry", "limits", "pauser", "ibcrouter", "governance"},
		AllowedSigners:    bridgetypes.DefaultHarnessSignerAddresses()[:3],
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
