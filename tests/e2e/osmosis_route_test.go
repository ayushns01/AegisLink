package e2e

import (
	"context"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	aegisapp "github.com/ayushns01/aegislink/chain/aegislink/app"
	bridgetypes "github.com/ayushns01/aegislink/chain/aegislink/x/bridge/types"
	ibcrouterkeeper "github.com/ayushns01/aegislink/chain/aegislink/x/ibcrouter/keeper"
)

func TestOsmosisRouteRuntimeCanInitiateTransferThroughCLI(t *testing.T) {
	t.Parallel()

	statePath := writeRuntimeChainBootstrapWithOsmosisRoute(t)
	output := runGoCommand(
		t,
		repoRoot(t),
		nil,
		"run",
		"./chain/aegislink/cmd/aegislinkd",
		"tx",
		"initiate-ibc-transfer",
		"--state-path",
		statePath,
		"--asset-id",
		"eth.usdc",
		"--amount",
		"25000000",
		"--receiver",
		"osmo1recipient",
		"--timeout-height",
		"140",
		"--memo",
		"swap:uosmo",
	)

	var result struct {
		TransferID string `json:"transfer_id"`
		Status     string `json:"status"`
	}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("decode initiate output: %v\n%s", err, output)
	}
	if result.TransferID == "" {
		t.Fatal("expected transfer id")
	}
	if result.Status != "pending" {
		t.Fatalf("expected pending status, got %q", result.Status)
	}

	queryOutput := runGoCommand(
		t,
		repoRoot(t),
		nil,
		"run",
		"./chain/aegislink/cmd/aegislinkd",
		"query",
		"transfers",
		"--state-path",
		statePath,
	)
	var transfers []struct {
		TransferID         string `json:"transfer_id"`
		DestinationChainID string `json:"destination_chain_id"`
		ChannelID          string `json:"channel_id"`
		DestinationDenom   string `json:"destination_denom"`
		Status             string `json:"status"`
		Memo               string `json:"memo"`
	}
	if err := json.Unmarshal([]byte(queryOutput), &transfers); err != nil {
		t.Fatalf("decode transfer output: %v\n%s", err, queryOutput)
	}
	if len(transfers) != 1 {
		t.Fatalf("expected one transfer, got %d", len(transfers))
	}
	if transfers[0].TransferID != result.TransferID {
		t.Fatalf("expected transfer id %q, got %q", result.TransferID, transfers[0].TransferID)
	}
	if transfers[0].DestinationChainID != "osmosis-1" {
		t.Fatalf("expected osmosis-1, got %q", transfers[0].DestinationChainID)
	}
	if transfers[0].ChannelID != "channel-0" {
		t.Fatalf("expected channel-0, got %q", transfers[0].ChannelID)
	}
	if transfers[0].DestinationDenom != "ibc/uatom-usdc" {
		t.Fatalf("expected ibc/uatom-usdc, got %q", transfers[0].DestinationDenom)
	}
	if transfers[0].Memo != "swap:uosmo" {
		t.Fatalf("expected memo swap:uosmo, got %q", transfers[0].Memo)
	}
}

func TestOsmosisRouteRuntimeCanRecoverFailedTransferThroughCLI(t *testing.T) {
	t.Parallel()

	statePath := writeRuntimeChainBootstrapWithOsmosisRoute(t)
	app, err := aegisapp.Load(statePath)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	transfer, err := app.IBCRouterKeeper.InitiateTransfer("eth.usdc", mustBigAmount(t, "25000000"), "osmo1recipient", 140, "swap:uosmo")
	if err != nil {
		t.Fatalf("initiate transfer: %v", err)
	}
	if err := app.Save(); err != nil {
		t.Fatalf("save state: %v", err)
	}

	_ = runGoCommand(
		t,
		repoRoot(t),
		nil,
		"run",
		"./chain/aegislink/cmd/aegislinkd",
		"tx",
		"fail-ibc-transfer",
		"--state-path",
		statePath,
		"--transfer-id",
		transfer.TransferID,
		"--reason",
		"ack failed",
	)
	_ = runGoCommand(
		t,
		repoRoot(t),
		nil,
		"run",
		"./chain/aegislink/cmd/aegislinkd",
		"tx",
		"refund-ibc-transfer",
		"--state-path",
		statePath,
		"--transfer-id",
		transfer.TransferID,
	)

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

func TestOsmosisRouteRuntimeCanExposeTimedOutTransferThroughCLI(t *testing.T) {
	t.Parallel()

	statePath := writeRuntimeChainBootstrapWithOsmosisRoute(t)
	app, err := aegisapp.Load(statePath)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	transfer, err := app.IBCRouterKeeper.InitiateTransfer("eth.usdc", mustBigAmount(t, "25000000"), "osmo1recipient", 140, "")
	if err != nil {
		t.Fatalf("initiate transfer: %v", err)
	}
	if err := app.Save(); err != nil {
		t.Fatalf("save state: %v", err)
	}

	_ = runGoCommand(
		t,
		repoRoot(t),
		nil,
		"run",
		"./chain/aegislink/cmd/aegislinkd",
		"tx",
		"timeout-ibc-transfer",
		"--state-path",
		statePath,
		"--transfer-id",
		transfer.TransferID,
	)

	output := runGoCommand(
		t,
		repoRoot(t),
		nil,
		"run",
		"./chain/aegislink/cmd/aegislinkd",
		"query",
		"transfers",
		"--state-path",
		statePath,
	)
	var transfers []struct {
		TransferID string `json:"transfer_id"`
		Status     string `json:"status"`
	}
	if err := json.Unmarshal([]byte(output), &transfers); err != nil {
		t.Fatalf("decode transfer output: %v\n%s", err, output)
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

func TestOsmosisRouteRuntimeCanCompleteTransferThroughCLI(t *testing.T) {
	t.Parallel()

	statePath := writeRuntimeChainBootstrapWithOsmosisRoute(t)
	app, err := aegisapp.Load(statePath)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	transfer, err := app.IBCRouterKeeper.InitiateTransfer("eth.usdc", mustBigAmount(t, "25000000"), "osmo1recipient", 140, "swap:uosmo")
	if err != nil {
		t.Fatalf("initiate transfer: %v", err)
	}
	if err := app.Save(); err != nil {
		t.Fatalf("save state: %v", err)
	}

	output := runGoCommand(
		t,
		repoRoot(t),
		nil,
		"run",
		"./chain/aegislink/cmd/aegislinkd",
		"tx",
		"complete-ibc-transfer",
		"--state-path",
		statePath,
		"--transfer-id",
		transfer.TransferID,
	)

	var result struct {
		TransferID string `json:"transfer_id"`
		Status     string `json:"status"`
	}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("decode complete output: %v\n%s", err, output)
	}
	if result.TransferID != transfer.TransferID {
		t.Fatalf("expected transfer id %q, got %q", transfer.TransferID, result.TransferID)
	}
	if result.Status != "completed" {
		t.Fatalf("expected completed status, got %q", result.Status)
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
}

func TestRouteRelayerCompletesPendingTransferAgainstLocalTarget(t *testing.T) {
	t.Parallel()

	statePath := writeRuntimeChainBootstrapWithOsmosisRoute(t)
	app, err := aegisapp.Load(statePath)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	transfer, err := app.IBCRouterKeeper.InitiateTransfer("eth.usdc", mustBigAmount(t, "25000000"), "osmo1recipient", 140, "swap:uosmo")
	if err != nil {
		t.Fatalf("initiate transfer: %v", err)
	}
	if err := app.Save(); err != nil {
		t.Fatalf("save state: %v", err)
	}

	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST request, got %s", r.Method)
		}
		if r.URL.Path != "/transfers" {
			t.Fatalf("expected /transfers path, got %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "completed",
		})
	}))
	defer target.Close()

	runRouteRelayerOnce(t, statePath, target.URL)

	loaded, err := aegisapp.Load(statePath)
	if err != nil {
		t.Fatalf("reload state: %v", err)
	}
	transfers := loaded.IBCRouterKeeper.ExportTransfers()
	if len(transfers) != 1 {
		t.Fatalf("expected one transfer, got %d", len(transfers))
	}
	if transfers[0].TransferID != transfer.TransferID {
		t.Fatalf("expected transfer id %q, got %q", transfer.TransferID, transfers[0].TransferID)
	}
	if transfers[0].Status != ibcrouterkeeper.TransferStatusCompleted {
		t.Fatalf("expected completed transfer, got %q", transfers[0].Status)
	}
}

func TestRouteRelayerPersistsIBCPacketReceiptAndSwapIntentInMockTarget(t *testing.T) {
	t.Parallel()

	statePath := writeRuntimeChainBootstrapWithOsmosisRoute(t)
	app, err := aegisapp.Load(statePath)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	_, err = app.IBCRouterKeeper.InitiateTransfer("eth.usdc", mustBigAmount(t, "25000000"), "osmo1recipient", 140, "swap:uosmo")
	if err != nil {
		t.Fatalf("initiate transfer: %v", err)
	}
	if err := app.Save(); err != nil {
		t.Fatalf("save state: %v", err)
	}

	targetStatePath := filepath.Join(t.TempDir(), "mock-osmosis.json")
	target := startMockOsmosisTarget(t, targetStatePath, "success")
	defer target.cancel()

	runRouteRelayerOnce(t, statePath, target.url)

	var state struct {
		Receipts []struct {
			Packet struct {
				Sequence uint64 `json:"sequence"`
				Data     struct {
					Memo string `json:"memo"`
				} `json:"data"`
			} `json:"packet"`
			DenomTrace struct {
				IBCDenom string `json:"ibc_denom"`
			} `json:"denom_trace"`
		} `json:"receipts"`
		Swaps []struct {
			TransferID  string `json:"transfer_id"`
			OutputDenom string `json:"output_denom"`
		} `json:"swaps"`
	}
	readJSONFile(t, targetStatePath, &state)

	if len(state.Receipts) != 1 {
		t.Fatalf("expected one receipt, got %d", len(state.Receipts))
	}
	if state.Receipts[0].Packet.Sequence != 1 {
		t.Fatalf("expected packet sequence 1, got %d", state.Receipts[0].Packet.Sequence)
	}
	if state.Receipts[0].Packet.Data.Memo != "swap:uosmo" {
		t.Fatalf("expected memo swap:uosmo, got %q", state.Receipts[0].Packet.Data.Memo)
	}
	if state.Receipts[0].DenomTrace.IBCDenom != "ibc/uatom-usdc" {
		t.Fatalf("expected ibc denom ibc/uatom-usdc, got %q", state.Receipts[0].DenomTrace.IBCDenom)
	}
	if len(state.Swaps) != 1 {
		t.Fatalf("expected one swap record, got %d", len(state.Swaps))
	}
	if state.Swaps[0].OutputDenom != "uosmo" {
		t.Fatalf("expected output denom uosmo, got %q", state.Swaps[0].OutputDenom)
	}
}

func readJSONFile(t *testing.T, path string, out any) {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if err := json.Unmarshal(data, out); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
}

type mockOsmosisRuntime struct {
	url       string
	statePath string
	cancel    context.CancelFunc
	cmd       *exec.Cmd
}

func startMockOsmosisTarget(t *testing.T, statePath, mode string) *mockOsmosisRuntime {
	t.Helper()

	port := reservePort(t)
	addr := "127.0.0.1:" + mustFormatPort(port)
	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, "go", "run", "./relayer/cmd/mock-osmosis-target")
	cmd.Dir = repoRoot(t)
	cmd.Env = append(os.Environ(),
		"GOCACHE=/tmp/aegislink-e2e-go-cache/build",
		"GOMODCACHE=/tmp/aegislink-e2e-go-cache/mod",
		"AEGISLINK_MOCK_OSMOSIS_ADDR="+addr,
		"AEGISLINK_MOCK_OSMOSIS_MODE="+mode,
		"AEGISLINK_MOCK_OSMOSIS_STATE_PATH="+statePath,
	)
	if err := cmd.Start(); err != nil {
		cancel()
		t.Fatalf("start mock osmosis target: %v", err)
	}

	runtime := &mockOsmosisRuntime{
		url:       "http://" + addr,
		statePath: statePath,
		cancel:    cancel,
		cmd:       cmd,
	}
	t.Cleanup(func() {
		cancel()
		_ = cmd.Wait()
	})

	waitForHTTP(t, runtime.url+"/transfers")
	return runtime
}

func waitForHTTP(t *testing.T, url string) {
	t.Helper()

	client := &http.Client{Timeout: 200 * time.Millisecond}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err == nil {
			_ = resp.Body.Close()
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("http endpoint %s did not become ready", url)
}

func mustFormatPort(port int) string {
	return strconv.Itoa(port)
}

func TestFullBridgeLoopCanRouteDepositToCompletedOsmosisTransfer(t *testing.T) {
	t.Parallel()

	anvil := startAnvilRuntime(t)
	contracts := deployBridgeContractsToAnvil(t, anvil.rpcURL)
	receipt := createAnvilDeposit(t, anvil.rpcURL, contracts, "25000000", "cosmos1recipient", "10000000000")
	if len(receipt.Logs) != 1 {
		t.Fatalf("expected one deposit log, got %d", len(receipt.Logs))
	}

	identity := depositClaimIdentityFromAnvilReceipt(t, contracts.Gateway, receipt)
	claim := depositClaimForOsmosisRoute(identity, "25000000")
	fixtures := writeEmptyRelayerFixtures(t)
	writeJSON(t, fixtures.voteStatePath, persistedVoteState{
		Votes: []persistedVote{
			{MessageID: claim.Identity.MessageID, PayloadHash: claim.Digest(), Signer: "relayer-1", Expiry: 10000000100},
			{MessageID: claim.Identity.MessageID, PayloadHash: claim.Digest(), Signer: "relayer-2", Expiry: 10000000100},
		},
	})

	statePath := writeRuntimeChainBootstrapWithOsmosisRouteAndAssetAddress(t, contracts.Token)
	runRelayerOnceAgainstRuntimeAndRPC(t, fixtures, statePath, anvil.rpcURL, contracts.Gateway)

	loaded, err := aegisapp.Load(statePath)
	if err != nil {
		t.Fatalf("load runtime state: %v", err)
	}
	if supply := loaded.BridgeKeeper.SupplyForDenom("uethusdc"); supply.String() != "25000000" {
		t.Fatalf("expected minted supply 25000000, got %s", supply.String())
	}

	initiateOutput := runGoCommand(
		t,
		repoRoot(t),
		nil,
		"run",
		"./chain/aegislink/cmd/aegislinkd",
		"tx",
		"initiate-ibc-transfer",
		"--state-path",
		statePath,
		"--asset-id",
		"eth.usdc",
		"--amount",
		"25000000",
		"--receiver",
		"osmo1recipient",
		"--timeout-height",
		"140",
		"--memo",
		"swap:uosmo",
	)
	var initiated struct {
		TransferID string `json:"transfer_id"`
		Status     string `json:"status"`
	}
	if err := json.Unmarshal([]byte(initiateOutput), &initiated); err != nil {
		t.Fatalf("decode initiate output: %v\n%s", err, initiateOutput)
	}
	if initiated.Status != "pending" {
		t.Fatalf("expected pending transfer, got %q", initiated.Status)
	}

	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST request, got %s", r.Method)
		}
		if r.URL.Path != "/transfers" {
			t.Fatalf("expected /transfers path, got %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "completed",
		})
	}))
	defer target.Close()

	runRouteRelayerOnce(t, statePath, target.URL)

	queryOutput := runGoCommand(
		t,
		repoRoot(t),
		nil,
		"run",
		"./chain/aegislink/cmd/aegislinkd",
		"query",
		"transfers",
		"--state-path",
		statePath,
	)
	var transfers []struct {
		TransferID string `json:"transfer_id"`
		Status     string `json:"status"`
	}
	if err := json.Unmarshal([]byte(queryOutput), &transfers); err != nil {
		t.Fatalf("decode transfer output: %v\n%s", err, queryOutput)
	}
	if len(transfers) != 1 {
		t.Fatalf("expected one transfer, got %d", len(transfers))
	}
	if transfers[0].TransferID != initiated.TransferID {
		t.Fatalf("expected transfer id %q, got %q", initiated.TransferID, transfers[0].TransferID)
	}
	if transfers[0].Status != "completed" {
		t.Fatalf("expected completed transfer, got %q", transfers[0].Status)
	}
}

func depositClaimIdentityFromAnvilReceipt(t *testing.T, gateway string, receipt txReceipt) bridgetypes.ClaimIdentity {
	t.Helper()

	identity := bridgetypes.ClaimIdentity{
		Kind:           bridgetypes.ClaimKindDeposit,
		SourceChainID:  "11155111",
		SourceContract: gateway,
		SourceTxHash:   receipt.TransactionHash,
		SourceLogIndex: mustParseHexUint64(t, receipt.Logs[0].LogIndex),
		Nonce:          1,
	}
	identity.MessageID = identity.DerivedMessageID()
	return identity
}

func depositClaimForOsmosisRoute(identity bridgetypes.ClaimIdentity, amount string) bridgetypes.DepositClaim {
	return bridgetypes.DepositClaim{
		Identity:           identity,
		DestinationChainID: "aegislink-1",
		AssetID:            "eth.usdc",
		Amount:             mustBigAmountValue(amount),
		Recipient:          "cosmos1recipient",
		Deadline:           10000000000,
	}
}

func mustBigAmountValue(value string) *big.Int {
	amount, ok := new(big.Int).SetString(value, 10)
	if !ok {
		panic("invalid amount " + value)
	}
	return amount
}
