package e2e

import (
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
		"AEGISLINK_ROUTE_RELAYER_AEGISLINK_CMD":                "go",
		"AEGISLINK_ROUTE_RELAYER_AEGISLINK_CMD_ARGS":           "run ./chain/aegislink/cmd/aegislinkd",
		"AEGISLINK_ROUTE_RELAYER_AEGISLINK_HOME":               aegisHome,
		"AEGISLINK_ROUTE_RELAYER_AEGISLINK_RUNTIME_MODE":       aegisapp.RuntimeModeSDKStore,
		"AEGISLINK_ROUTE_RELAYER_DESTINATION_CMD":              "go",
		"AEGISLINK_ROUTE_RELAYER_DESTINATION_CMD_ARGS":         "run ./relayer/cmd/osmo-locald",
		"AEGISLINK_ROUTE_RELAYER_DESTINATION_HOME":             destinationHome,
		"AEGISLINK_ROUTE_RELAYER_DESTINATION_RUNTIME_MODE":     "osmo-local-runtime",
	}

	firstRouteRun := runGoCommand(t, repoRoot(t), env, "run", "./relayer/cmd/route-relayer")
	secondRouteRun := runGoCommand(t, repoRoot(t), env, "run", "./relayer/cmd/route-relayer")

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
			"ready-acks",
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
