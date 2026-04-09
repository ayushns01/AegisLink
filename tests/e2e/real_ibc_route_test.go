package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	aegisapp "github.com/ayushns01/aegislink/chain/aegislink/app"
	ibcrouterkeeper "github.com/ayushns01/aegislink/chain/aegislink/x/ibcrouter/keeper"
	limittypes "github.com/ayushns01/aegislink/chain/aegislink/x/limits/types"
	registrytypes "github.com/ayushns01/aegislink/chain/aegislink/x/registry/types"
)

func TestRealDestinationChainBootstrap(t *testing.T) {
	t.Parallel()

	homeDir := filepath.Join(t.TempDir(), "osmo-local-home")
	runShellScript(t, repoRoot(t), "scripts/localnet/bootstrap_destination_chain.sh", homeDir)

	output := runGoCommand(
		t,
		repoRoot(t),
		nil,
		"run",
		"./relayer/cmd/osmo-locald",
		"query",
		"status",
		"--home",
		homeDir,
	)

	var status struct {
		ChainID     string `json:"chain_id"`
		Initialized bool   `json:"initialized"`
		Pools       int    `json:"pools"`
	}
	if err := decodeLastJSONObject(output, &status); err != nil {
		t.Fatalf("decode destination status: %v\n%s", err, output)
	}
	if status.ChainID != "osmo-local-1" {
		t.Fatalf("expected chain id osmo-local-1, got %q", status.ChainID)
	}
	if !status.Initialized {
		t.Fatal("expected initialized destination runtime")
	}
	if status.Pools == 0 {
		t.Fatal("expected seeded pools")
	}
}

func TestRealIBCRoute(t *testing.T) {
	t.Parallel()

	aegisHome := filepath.Join(t.TempDir(), "aegis-home")
	runShellScript(t, repoRoot(t), "scripts/localnet/bootstrap_real_chain.sh", aegisHome)
	seedRealIBCAegisLinkRuntime(t, aegisHome)

	destinationHome := filepath.Join(t.TempDir(), "osmo-local-home")
	runShellScript(t, repoRoot(t), "scripts/localnet/bootstrap_destination_chain.sh", destinationHome)
	runShellScript(t, repoRoot(t), "scripts/localnet/bootstrap_ibc.sh", aegisHome, destinationHome)

	initiateOutput := runGoCommand(
		t,
		filepath.Join(repoRoot(t), "chain", "aegislink"),
		nil,
		"run",
		"./cmd/aegislinkd",
		"tx",
		"initiate-ibc-transfer",
		"--home",
		aegisHome,
		"--runtime-mode",
		aegisapp.RuntimeModeSDKStore,
		"--asset-id",
		"eth.usdc",
		"--amount",
		"25000000",
		"--receiver",
		"osmo1recipient",
		"--timeout-height",
		"140",
		"--memo",
		"swap:uosmo:min_out=1",
	)

	var transfer struct {
		TransferID string `json:"transfer_id"`
		Status     string `json:"status"`
	}
	if err := decodeLastJSONObject(initiateOutput, &transfer); err != nil {
		t.Fatalf("decode transfer output: %v\n%s", err, initiateOutput)
	}
	if transfer.Status != "pending" {
		t.Fatalf("expected pending transfer, got %+v", transfer)
	}

	env := map[string]string{
		"AEGISLINK_ROUTE_RELAYER_AEGISLINK_CMD":            "go",
		"AEGISLINK_ROUTE_RELAYER_AEGISLINK_CMD_ARGS":       "run ./chain/aegislink/cmd/aegislinkd",
		"AEGISLINK_ROUTE_RELAYER_AEGISLINK_HOME":           aegisHome,
		"AEGISLINK_ROUTE_RELAYER_AEGISLINK_RUNTIME_MODE":   aegisapp.RuntimeModeSDKStore,
		"AEGISLINK_ROUTE_RELAYER_DESTINATION_CMD":          "go",
		"AEGISLINK_ROUTE_RELAYER_DESTINATION_CMD_ARGS":     "run ./relayer/cmd/osmo-locald",
		"AEGISLINK_ROUTE_RELAYER_DESTINATION_HOME":         destinationHome,
		"AEGISLINK_ROUTE_RELAYER_DESTINATION_RUNTIME_MODE": "osmo-local-runtime",
	}

	firstRouteRun := runGoCommand(t, repoRoot(t), env, "run", "./relayer/cmd/route-relayer")

	transferQuery := runGoCommand(
		t,
		filepath.Join(repoRoot(t), "chain", "aegislink"),
		nil,
		"run",
		"./cmd/aegislinkd",
		"query",
		"transfers",
		"--home",
		aegisHome,
		"--runtime-mode",
		aegisapp.RuntimeModeSDKStore,
	)
	var transfers []struct {
		TransferID string `json:"transfer_id"`
		Status     string `json:"status"`
	}
	if err := decodeLastJSONObject(transferQuery, &transfers); err != nil {
		t.Fatalf("decode transfers after first relay: %v\n%s", err, transferQuery)
	}
	if len(transfers) != 1 || transfers[0].Status != "pending" {
		t.Fatalf("expected pending transfer after first packet relay, got %+v", transfers)
	}

	packetOutput := runGoCommand(
		t,
		repoRoot(t),
		nil,
		"run",
		"./relayer/cmd/osmo-locald",
		"query",
		"packets",
		"--home",
		destinationHome,
	)
	var packets []struct {
		TransferID  string `json:"transfer_id"`
		PacketState string `json:"packet_state"`
		AckState    string `json:"ack_state"`
	}
	if err := decodeLastJSONObject(packetOutput, &packets); err != nil {
		t.Fatalf("decode packets after first relay: %v\n%s", err, packetOutput)
	}
	if len(packets) != 1 || packets[0].PacketState == "" {
		t.Fatalf("expected destination packet receipt after first relay, got %+v", packets)
	}

	ackOutput := runGoCommand(
		t,
		repoRoot(t),
		nil,
		"run",
		"./relayer/cmd/osmo-locald",
		"query",
		"packet-acks",
		"--home",
		destinationHome,
	)
	var pendingAcks []struct {
		TransferID string `json:"transfer_id"`
		Status     string `json:"status"`
	}
	if err := decodeLastJSONObject(ackOutput, &pendingAcks); err != nil {
		t.Fatalf("decode packet acks after first relay: %v\n%s", err, ackOutput)
	}
	if len(pendingAcks) != 1 {
		t.Fatalf("expected one pending ack after first relay, got %+v", pendingAcks)
	}

	secondRouteRun := runGoCommand(t, repoRoot(t), env, "run", "./relayer/cmd/route-relayer")

	transferQuery = runGoCommand(
		t,
		filepath.Join(repoRoot(t), "chain", "aegislink"),
		nil,
		"run",
		"./cmd/aegislinkd",
		"query",
		"transfers",
		"--home",
		aegisHome,
		"--runtime-mode",
		aegisapp.RuntimeModeSDKStore,
	)
	if err := decodeLastJSONObject(transferQuery, &transfers); err != nil {
		t.Fatalf("decode transfers: %v\n%s", err, transferQuery)
	}
	if len(transfers) != 1 || transfers[0].Status != "completed" {
		readyAckOutput := runGoCommand(
			t,
			repoRoot(t),
			nil,
			"run",
			"./relayer/cmd/osmo-locald",
			"query",
			"packet-acks",
			"--home",
			destinationHome,
		)
		packetOutput := runGoCommand(
			t,
			repoRoot(t),
			nil,
			"run",
			"./relayer/cmd/osmo-locald",
			"query",
			"packets",
			"--home",
			destinationHome,
		)
		t.Fatalf(
			"expected completed transfer, got %+v\nfirst route run:\n%s\nsecond route run:\n%s\nready acks:\n%s\npackets:\n%s",
			transfers,
			firstRouteRun,
			secondRouteRun,
			readyAckOutput,
			packetOutput,
		)
	}

	balanceOutput := runGoCommand(
		t,
		repoRoot(t),
		nil,
		"run",
		"./relayer/cmd/osmo-locald",
		"query",
		"balances",
		"--home",
		destinationHome,
	)
	var balances []struct {
		Address string `json:"address"`
		Denom   string `json:"denom"`
		Amount  string `json:"amount"`
	}
	if err := decodeLastJSONObject(balanceOutput, &balances); err != nil {
		t.Fatalf("decode balances: %v\n%s", err, balanceOutput)
	}
	if len(balances) == 0 {
		t.Fatal("expected destination balance state")
	}
}

func TestRealHermesIBC(t *testing.T) {
	t.Parallel()

	aegisHome := filepath.Join(t.TempDir(), "aegis-home")
	destinationHome := filepath.Join(t.TempDir(), "osmo-local-home")
	runShellScript(t, repoRoot(t), "scripts/localnet/bootstrap_real_chain.sh", aegisHome)
	runShellScript(t, repoRoot(t), "scripts/localnet/bootstrap_destination_chain.sh", destinationHome)
	runShellScript(t, repoRoot(t), "scripts/localnet/bootstrap_ibc.sh", aegisHome, destinationHome)

	type linkMetadata struct {
		RelayMode          string `json:"relay_mode"`
		SourceChainID      string `json:"source_chain_id"`
		DestinationChainID string `json:"destination_chain_id"`
		SourcePort         string `json:"source_port"`
		SourceChannel      string `json:"source_channel"`
		ConnectionID       string `json:"connection_id"`
	}

	readLink := func(path string) linkMetadata {
		t.Helper()
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read ibc link metadata: %v", err)
		}
		var link linkMetadata
		if err := json.Unmarshal(data, &link); err != nil {
			t.Fatalf("decode ibc link metadata: %v", err)
		}
		return link
	}

	aegisLink := readLink(filepath.Join(aegisHome, "data", "ibc-link.json"))
	destLink := readLink(filepath.Join(destinationHome, "data", "ibc-link.json"))

	if aegisLink.RelayMode != "hermes-local" || destLink.RelayMode != "hermes-local" {
		t.Fatalf("expected hermes-local relay metadata, got aegis=%+v dest=%+v", aegisLink, destLink)
	}
	if aegisLink.SourcePort != "transfer" || aegisLink.SourceChannel != "channel-0" {
		t.Fatalf("unexpected source packet metadata: %+v", aegisLink)
	}
	if aegisLink.ConnectionID == "" || destLink.ConnectionID == "" {
		t.Fatalf("expected connection metadata, got aegis=%+v dest=%+v", aegisLink, destLink)
	}
}

func TestRouteExtensionsStakeAction(t *testing.T) {
	t.Parallel()

	aegisHome := filepath.Join(t.TempDir(), "aegis-home")
	runShellScript(t, repoRoot(t), "scripts/localnet/bootstrap_real_chain.sh", aegisHome)
	seedRealIBCAegisLinkRuntime(t, aegisHome)

	destinationHome := filepath.Join(t.TempDir(), "osmo-local-home")
	runShellScript(t, repoRoot(t), "scripts/localnet/bootstrap_destination_chain.sh", destinationHome)

	initiateOutput := runGoCommand(
		t,
		filepath.Join(repoRoot(t), "chain", "aegislink"),
		nil,
		"run",
		"./cmd/aegislinkd",
		"tx",
		"initiate-ibc-transfer",
		"--home",
		aegisHome,
		"--runtime-mode",
		aegisapp.RuntimeModeSDKStore,
		"--asset-id",
		"eth.usdc",
		"--amount",
		"25000000",
		"--receiver",
		"osmo1recipient",
		"--timeout-height",
		"141",
		"--memo",
		"stake:ibc/uethusdc:recipient=osmo1staker:path=validator-7",
	)

	var transfer struct {
		TransferID string `json:"transfer_id"`
		Status     string `json:"status"`
	}
	if err := decodeLastJSONObject(initiateOutput, &transfer); err != nil {
		t.Fatalf("decode transfer output: %v\n%s", err, initiateOutput)
	}
	if transfer.Status != "pending" {
		t.Fatalf("expected pending transfer, got %+v", transfer)
	}

	env := map[string]string{
		"AEGISLINK_ROUTE_RELAYER_AEGISLINK_CMD":            "go",
		"AEGISLINK_ROUTE_RELAYER_AEGISLINK_CMD_ARGS":       "run ./chain/aegislink/cmd/aegislinkd",
		"AEGISLINK_ROUTE_RELAYER_AEGISLINK_HOME":           aegisHome,
		"AEGISLINK_ROUTE_RELAYER_AEGISLINK_RUNTIME_MODE":   aegisapp.RuntimeModeSDKStore,
		"AEGISLINK_ROUTE_RELAYER_DESTINATION_CMD":          "go",
		"AEGISLINK_ROUTE_RELAYER_DESTINATION_CMD_ARGS":     "run ./relayer/cmd/osmo-locald",
		"AEGISLINK_ROUTE_RELAYER_DESTINATION_HOME":         destinationHome,
		"AEGISLINK_ROUTE_RELAYER_DESTINATION_RUNTIME_MODE": "osmo-local-runtime",
	}
	runGoCommand(t, repoRoot(t), env, "run", "./relayer/cmd/route-relayer")
	runGoCommand(t, repoRoot(t), env, "run", "./relayer/cmd/route-relayer")

	executionsOutput := runGoCommand(
		t,
		repoRoot(t),
		nil,
		"run",
		"./relayer/cmd/osmo-locald",
		"query",
		"executions",
		"--home",
		destinationHome,
	)
	var executions []struct {
		TransferID string `json:"transfer_id"`
		Result     string `json:"result"`
		Recipient  string `json:"recipient"`
		RoutePath  string `json:"route_path"`
	}
	if err := decodeLastJSONObject(executionsOutput, &executions); err != nil {
		t.Fatalf("decode executions: %v\n%s", err, executionsOutput)
	}
	if len(executions) != 1 || executions[0].Result != "stake_success" {
		t.Fatalf("expected one stake_success execution, got %+v", executions)
	}
	if executions[0].Recipient != "osmo1staker" || executions[0].RoutePath != "validator-7" {
		t.Fatalf("unexpected execution payload: %+v", executions[0])
	}
}

func seedRealIBCAegisLinkRuntime(t *testing.T, homeDir string) {
	t.Helper()

	cfg, err := aegisapp.ResolveConfig(aegisapp.Config{
		HomeDir:     homeDir,
		RuntimeMode: aegisapp.RuntimeModeSDKStore,
	})
	if err != nil {
		t.Fatalf("resolve aegis runtime config: %v", err)
	}
	app, err := aegisapp.LoadWithConfig(cfg)
	if err != nil {
		t.Fatalf("load aegis runtime: %v", err)
	}
	defer func() {
		if err := app.Close(); err != nil {
			t.Fatalf("close aegis runtime: %v", err)
		}
	}()

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
		MaxAmount:     mustBigAmount(t, "1000000000000000000"),
	}); err != nil {
		t.Fatalf("set limit: %v", err)
	}
	if err := app.SetRoute(ibcrouterkeeper.Route{
		AssetID:            "eth.usdc",
		DestinationChainID: "osmo-local-1",
		ChannelID:          "channel-0",
		DestinationDenom:   "ibc/uethusdc",
		Enabled:            true,
	}); err != nil {
		t.Fatalf("set ibc route: %v", err)
	}
	if err := app.Save(); err != nil {
		t.Fatalf("save aegis runtime: %v", err)
	}
}
