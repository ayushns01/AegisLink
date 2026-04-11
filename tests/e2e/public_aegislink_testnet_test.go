package e2e

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	aegisapp "github.com/ayushns01/aegislink/chain/aegislink/app"
)

func TestPublicAegisLinkTestnet(t *testing.T) {
	t.Parallel()

	homeDir := filepath.Join(t.TempDir(), "public-testnet-home")
	bootstrapPublicAegisLinkTestnet(t, homeDir)

	operatorConfigPath := filepath.Join(repoRoot(t), "deploy", "testnet", "aegislink", "operator.json")
	data, err := os.ReadFile(operatorConfigPath)
	if err != nil {
		t.Fatalf("read operator config: %v", err)
	}

	var operatorConfig struct {
		ChainID               string   `json:"chain_id"`
		RuntimeMode           string   `json:"runtime_mode"`
		AllowedSigners        []string `json:"allowed_signers"`
		GovernanceAuthorities []string `json:"governance_authorities"`
		RequiredThreshold     uint32   `json:"required_threshold"`
	}
	if err := json.Unmarshal(data, &operatorConfig); err != nil {
		t.Fatalf("decode operator config: %v", err)
	}
	if operatorConfig.ChainID == "" || operatorConfig.RuntimeMode == "" {
		t.Fatalf("expected operator config to include chain and runtime mode, got %+v", operatorConfig)
	}
	if len(operatorConfig.AllowedSigners) == 0 || len(operatorConfig.GovernanceAuthorities) == 0 || operatorConfig.RequiredThreshold == 0 {
		t.Fatalf("expected operator config to include bridge settings, got %+v", operatorConfig)
	}

	cfg, err := aegisapp.ResolveConfig(aegisapp.Config{
		HomeDir:     homeDir,
		RuntimeMode: aegisapp.RuntimeModeSDKStore,
	})
	if err != nil {
		t.Fatalf("resolve public testnet runtime: %v", err)
	}
	app, err := aegisapp.LoadWithConfig(cfg)
	if err != nil {
		t.Fatalf("load public testnet runtime: %v", err)
	}

	recipient := testCosmosWalletAddress()
	if err := seedPublicWalletAssets(t, app, recipient); err != nil {
		t.Fatalf("seed public wallet assets: %v", err)
	}
	if err := app.Save(); err != nil {
		t.Fatalf("save public testnet runtime: %v", err)
	}
	if err := app.Close(); err != nil {
		t.Fatalf("close public testnet runtime: %v", err)
	}

	startOutput := runGoCommandWithLocalCache(
		t,
		repoRoot(t),
		"run",
		"./chain/aegislink/cmd/aegislinkd",
		"start",
		"--home",
		homeDir,
	)
	var started struct {
		Status      string `json:"status"`
		RuntimeMode string `json:"runtime_mode"`
		Initialized bool   `json:"initialized"`
	}
	if err := decodeLastJSONObject(startOutput, &started); err != nil {
		t.Fatalf("decode start output: %v\n%s", err, startOutput)
	}
	if started.Status != "started" || !started.Initialized {
		t.Fatalf("expected started initialized runtime, got %+v", started)
	}

	balanceOutput := runGoCommandWithLocalCache(
		t,
		repoRoot(t),
		"run",
		"./chain/aegislink/cmd/aegislinkd",
		"query",
		"balances",
		"--home",
		homeDir,
		"--address",
		recipient,
	)
	balances := decodeWalletBalances(t, balanceOutput)
	if len(balances) != 2 {
		t.Fatalf("expected two public testnet wallet balances, got %d (%+v)", len(balances), balances)
	}
}

func bootstrapPublicAegisLinkTestnet(t *testing.T, homeDir string) {
	t.Helper()

	cmd := exec.Command("bash", "scripts/testnet/bootstrap_aegislink_testnet.sh", homeDir)
	cmd.Dir = repoRoot(t)
	cmd.Env = append([]string{}, os.Environ()...)
	cmd.Env = append(cmd.Env,
		"GOCACHE=/tmp/aegislink-gocache",
		"GOMODCACHE=/Users/ayushns01/go/pkg/mod",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bootstrap public aegislink testnet: %v\n%s", err, output)
	}
}
