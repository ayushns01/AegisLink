package e2e

import (
	"encoding/json"
	"testing"

	aegisapp "github.com/ayushns01/aegislink/chain/aegislink/app"
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
