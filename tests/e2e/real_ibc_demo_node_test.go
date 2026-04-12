package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	aegisapp "github.com/ayushns01/aegislink/chain/aegislink/app"
	"github.com/ayushns01/aegislink/chain/aegislink/networked"
	bridgetypes "github.com/ayushns01/aegislink/chain/aegislink/x/bridge/types"
	ibcrouterkeeper "github.com/ayushns01/aegislink/chain/aegislink/x/ibcrouter/keeper"
	abcicli "github.com/cometbft/cometbft/abci/client"
	abcitypes "github.com/cometbft/cometbft/abci/types"
	rpchttp "github.com/cometbft/cometbft/rpc/client/http"
)

func TestRealIBCDemoNodeStartsAndExposesEndpoints(t *testing.T) {
	t.Parallel()

	homeDir := filepath.Join(t.TempDir(), "ibc-demo-home")
	bootstrapPublicAegisLinkTestnet(t, homeDir)

	readyPath := filepath.Join(t.TempDir(), "demo-node-ready.json")
	cmd, logs := startIBCDemoNodeProcess(t, homeDir, readyPath, nil)
	defer stopIBCDemoNodeProcess(t, cmd, logs)

	statusOutput := runGoCommandWithLocalCache(
		t,
		repoRoot(t),
		"run",
		"./chain/aegislink/cmd/aegislinkd",
		"demo-node",
		"status",
		"--home",
		homeDir,
		"--ready-file",
		readyPath,
	)

	var status struct {
		Status                 string   `json:"status"`
		ChainID                string   `json:"chain_id"`
		NodeID                 string   `json:"node_id"`
		RPCAddress             string   `json:"rpc_address"`
		CometRPCAddress        string   `json:"comet_rpc_address"`
		GRPCAddress            string   `json:"grpc_address"`
		ABCIAddress            string   `json:"abci_address"`
		ConfigPath             string   `json:"config_path"`
		CometGenesisPath       string   `json:"comet_genesis_path"`
		SDKGenesisPath         string   `json:"sdk_genesis_path"`
		NodeKeyPath            string   `json:"node_key_path"`
		PrivValidatorKeyPath   string   `json:"priv_validator_key_path"`
		PrivValidatorStatePath string   `json:"priv_validator_state_path"`
		CoreStoreKeys          []string `json:"core_store_keys"`
		SDKGenesisModules      []string `json:"sdk_genesis_modules"`
		Healthy                bool     `json:"healthy"`
	}
	if err := decodeLastJSONObject(statusOutput, &status); err != nil {
		t.Fatalf("decode demo-node status: %v\n%s", err, statusOutput)
	}
	if status.Status != "ready" || !status.Healthy {
		t.Fatalf("expected healthy ready demo node, got %+v", status)
	}
	if status.ChainID != "aegislink-public-testnet-1" {
		t.Fatalf("expected chain id aegislink-public-testnet-1, got %q", status.ChainID)
	}
	if status.RPCAddress == "" || status.CometRPCAddress == "" || status.GRPCAddress == "" {
		t.Fatalf("expected bound endpoints, got %+v", status)
	}
	if status.ABCIAddress == "" {
		t.Fatalf("expected abci address, got %+v", status)
	}
	if status.NodeID == "" {
		t.Fatalf("expected node id, got %+v", status)
	}
	for _, path := range []string{
		status.ConfigPath,
		status.CometGenesisPath,
		status.SDKGenesisPath,
		status.NodeKeyPath,
		status.PrivValidatorKeyPath,
		status.PrivValidatorStatePath,
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected status to report artifact %s: %v", path, err)
		}
	}
	for _, key := range []string{"auth", "bank", "bridge", "ibc", "transfer"} {
		if !containsStringE2E(status.CoreStoreKeys, key) {
			t.Fatalf("expected status core store keys to include %q in %+v", key, status.CoreStoreKeys)
		}
	}
	for _, moduleName := range []string{"auth", "bank", "ibc", "transfer"} {
		if !containsStringE2E(status.SDKGenesisModules, moduleName) {
			t.Fatalf("expected status sdk genesis modules to include %q in %+v", moduleName, status.SDKGenesisModules)
		}
	}

	abciClient := abcicli.NewSocketClient(status.ABCIAddress, true)
	if err := abciClient.Start(); err != nil {
		t.Fatalf("start abci client: %v", err)
	}
	defer func() {
		_ = abciClient.Stop()
	}()
	info, err := abciClient.Info(context.Background(), &abcitypes.RequestInfo{})
	if err != nil {
		t.Fatalf("abci info: %v", err)
	}
	if info.Data != "aegislink" {
		t.Fatalf("expected abci info data aegislink, got %+v", info)
	}

	cometClient, err := rpchttp.New("http://"+status.CometRPCAddress, "/websocket")
	if err != nil {
		t.Fatalf("create comet rpc client: %v", err)
	}
	cometStatus, err := cometClient.Status(context.Background())
	if err != nil {
		t.Fatalf("comet rpc status: %v", err)
	}
	if cometStatus.NodeInfo.Network != "aegislink-public-testnet-1" {
		t.Fatalf("expected comet rpc network aegislink-public-testnet-1, got %+v", cometStatus.NodeInfo)
	}

	for _, path := range []string{
		filepath.Join(homeDir, "config", "config.toml"),
		filepath.Join(homeDir, "config", "node_key.json"),
		filepath.Join(homeDir, "config", "priv_validator_key.json"),
		filepath.Join(homeDir, "data", "priv_validator_state.json"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected demo node startup to create %s: %v", path, err)
		}
	}
}

func TestRealIBCDemoNodeRemoteWorkflow(t *testing.T) {
	t.Parallel()

	homeDir := filepath.Join(t.TempDir(), "ibc-demo-home")
	cfg := initSDKDemoNodeHome(t, homeDir)
	app, err := aegisapp.LoadWithConfig(cfg)
	if err != nil {
		t.Fatalf("load sdk runtime app: %v", err)
	}
	if err := registerPublicBridgeAssets(t, app); err != nil {
		t.Fatalf("register public bridge assets: %v", err)
	}
	if err := app.IBCRouterKeeper.SetRoute(ibcrouterkeeper.Route{
		AssetID:            "eth",
		DestinationChainID: "osmosis-1",
		ChannelID:          "channel-0",
		DestinationDenom:   "ibc/ueth",
		Enabled:            true,
	}); err != nil {
		t.Fatalf("set ibc route: %v", err)
	}
	if err := app.Save(); err != nil {
		t.Fatalf("save sdk runtime app: %v", err)
	}
	if err := app.Close(); err != nil {
		t.Fatalf("close sdk runtime app: %v", err)
	}

	readyPath := filepath.Join(t.TempDir(), "demo-node-ready.json")
	cmd, logs := startIBCDemoNodeProcess(t, homeDir, readyPath, map[string]string{
		"AEGISLINK_DEMO_NODE_TICK_INTERVAL_MS": "10",
	})
	defer stopIBCDemoNodeProcess(t, cmd, logs)

	ready := readReadyFileE2E(t, readyPath)
	recipient := testCosmosWalletAddress()
	claim := depositClaim(t, bridgetypes.SourceAssetKindNativeETH, "", "eth", "0xruntime-demo-deposit", 1, 1, recipient, "1000000000000000")
	claim.DestinationChainID = "aegislink-public-testnet-1"
	attestation := testAttestationForClaim(t, claim)

	queueResult, err := networked.SubmitQueueDepositClaim(context.Background(), networked.Config{
		HomeDir:   homeDir,
		ReadyFile: readyPath,
	}, claim, attestation)
	if err != nil {
		t.Fatalf("submit queued deposit claim over comet rpc: %v", err)
	}
	var queued struct {
		Status string `json:"status"`
	}
	encodedQueued, err := json.Marshal(queueResult)
	if err != nil {
		t.Fatalf("marshal queue result: %v", err)
	}
	if err := json.Unmarshal(encodedQueued, &queued); err != nil {
		t.Fatalf("decode queue response: %v", err)
	}
	if queued.Status != "queued" {
		t.Fatalf("expected queued response, got %+v", queueResult)
	}

	waitForDemoNodeBalances(t, homeDir, readyPath, recipient, "ueth", claim.Amount.String())

	balanceOutput := runGoCommandWithLocalCache(
		t,
		repoRoot(t),
		"run",
		"./chain/aegislink/cmd/aegislinkd",
		"demo-node",
		"balances",
		"--home",
		homeDir,
		"--ready-file",
		readyPath,
		"--address",
		recipient,
	)
	var balances []struct {
		Address string `json:"address"`
		Denom   string `json:"denom"`
		Amount  string `json:"amount"`
	}
	if err := decodeLastJSONObject(balanceOutput, &balances); err != nil {
		t.Fatalf("decode demo-node balances output: %v\n%s", err, balanceOutput)
	}
	if len(balances) != 1 || balances[0].Denom != "ueth" || balances[0].Amount != claim.Amount.String() {
		t.Fatalf("unexpected demo-node balances output: %+v", balances)
	}

	transfer, err := networked.SubmitInitiateIBCTransfer(context.Background(), networked.Config{
		HomeDir:   homeDir,
		ReadyFile: readyPath,
	}, networked.InitiateIBCTransferPayload{
		AssetID:       "eth",
		Amount:        claim.Amount.String(),
		Receiver:      "osmo1q5nq6v24qq0584nf00wuhqrku4anlxaq05wsj8",
		TimeoutHeight: 120,
		Memo:          "real-ibc-demo",
	})
	if err != nil {
		t.Fatalf("submit initiate transfer over comet rpc: %v", err)
	}
	if transfer.TransferID == "" || transfer.Status != "pending" {
		t.Fatalf("unexpected transfer response: %+v", transfer)
	}

	transfersOutput := runGoCommandWithLocalCache(
		t,
		repoRoot(t),
		"run",
		"./chain/aegislink/cmd/aegislinkd",
		"demo-node",
		"transfers",
		"--home",
		homeDir,
		"--ready-file",
		readyPath,
	)
	var transfers []struct {
		TransferID string `json:"transfer_id"`
		Status     string `json:"status"`
		AssetID    string `json:"asset_id"`
	}
	if err := decodeLastJSONObject(transfersOutput, &transfers); err != nil {
		t.Fatalf("decode demo-node transfers output: %v\n%s", err, transfersOutput)
	}
	if len(transfers) != 1 || transfers[0].TransferID != transfer.TransferID || transfers[0].Status != "pending" || transfers[0].AssetID != "eth" {
		t.Fatalf("unexpected demo-node transfers output: %+v", transfers)
	}

	abciClient := abcicli.NewSocketClient(ready.ABCIAddress, true)
	if err := abciClient.Start(); err != nil {
		t.Fatalf("start abci client for transfer query: %v", err)
	}
	defer func() {
		_ = abciClient.Stop()
	}()
	query, err := abciClient.Query(context.Background(), &abcitypes.RequestQuery{Path: "/transfers"})
	if err != nil {
		t.Fatalf("abci query transfers: %v", err)
	}
	var abciTransfers []struct {
		TransferID string `json:"transfer_id"`
		Status     string `json:"status"`
		AssetID    string `json:"asset_id"`
	}
	if err := json.Unmarshal(query.Value, &abciTransfers); err != nil {
		t.Fatalf("decode abci transfer query: %v", err)
	}
	if len(abciTransfers) != 1 || abciTransfers[0].TransferID != transfer.TransferID || abciTransfers[0].Status != "pending" || abciTransfers[0].AssetID != "eth" {
		t.Fatalf("unexpected abci transfer query output: %+v", abciTransfers)
	}
}

func startIBCDemoNodeProcess(t *testing.T, homeDir, readyPath string, extraEnv map[string]string) (*exec.Cmd, *bytes.Buffer) {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	cmd := exec.CommandContext(ctx, "bash", "scripts/testnet/start_aegislink_ibc_demo.sh", homeDir)
	cmd.Dir = repoRoot(t)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Env = append([]string{}, os.Environ()...)
	cmd.Env = append(cmd.Env,
		"GOCACHE=/tmp/aegislink-gocache",
		"GOMODCACHE=/Users/ayushns01/go/pkg/mod",
		"AEGISLINK_DEMO_NODE_READY_FILE="+readyPath,
		"AEGISLINK_DEMO_NODE_RPC_ADDRESS=127.0.0.1:0",
		"AEGISLINK_DEMO_NODE_COMET_RPC_ADDRESS=127.0.0.1:0",
		"AEGISLINK_DEMO_NODE_GRPC_ADDRESS=127.0.0.1:0",
		"AEGISLINK_DEMO_NODE_ABCI_ADDRESS=127.0.0.1:0",
	)
	for key, value := range extraEnv {
		cmd.Env = append(cmd.Env, key+"="+value)
	}

	var logs bytes.Buffer
	cmd.Stdout = &logs
	cmd.Stderr = &logs
	if err := cmd.Start(); err != nil {
		t.Fatalf("start ibc demo node process: %v", err)
	}

	waitForReadyFileE2E(t, readyPath)
	return cmd, &logs
}

func stopIBCDemoNodeProcess(t *testing.T, cmd *exec.Cmd, logs *bytes.Buffer) {
	t.Helper()
	if cmd == nil || cmd.Process == nil {
		return
	}
	_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGINT)
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		select {
		case <-done:
		case <-time.After(2 * time.Second):
		}
		t.Fatalf("timed out stopping ibc demo node process\n%s", logs.String())
	}
}

func waitForReadyFileE2E(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for ready file %s", path)
}

func readReadyFileE2E(t *testing.T, path string) struct {
	RPCAddress      string `json:"rpc_address"`
	CometRPCAddress string `json:"comet_rpc_address"`
	ABCIAddress     string `json:"abci_address"`
} {
	t.Helper()
	var ready struct {
		RPCAddress      string `json:"rpc_address"`
		CometRPCAddress string `json:"comet_rpc_address"`
		ABCIAddress     string `json:"abci_address"`
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read ready file: %v", err)
	}
	if err := json.Unmarshal(data, &ready); err != nil {
		t.Fatalf("decode ready file: %v", err)
	}
	return ready
}

func waitForDemoNodeBalances(t *testing.T, homeDir, readyPath, address, denom, amount string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		balances, err := networked.QueryBalances(context.Background(), networked.Config{
			HomeDir:   homeDir,
			ReadyFile: readyPath,
		}, address)
		if err == nil && len(balances) == 1 && balances[0].Denom == denom && balances[0].Amount == amount {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for balance %s %s at %s", denom, amount, address)
}

func initSDKDemoNodeHome(t *testing.T, homeDir string) aegisapp.Config {
	t.Helper()
	cfg, err := aegisapp.InitHome(aegisapp.Config{
		HomeDir:           homeDir,
		AppName:           aegisapp.AppName,
		ChainID:           "aegislink-public-testnet-1",
		RuntimeMode:       aegisapp.RuntimeModeSDKStore,
		Modules:           []string{"bridge", "bank", "registry", "limits", "pauser", "ibcrouter", "governance"},
		AllowedSigners:    bridgetypes.DefaultHarnessSignerAddresses()[:3],
		RequiredThreshold: 2,
	}, false)
	if err != nil {
		t.Fatalf("init sdk demo node home: %v", err)
	}
	return cfg
}

func containsStringE2E(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
