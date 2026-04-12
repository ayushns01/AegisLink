package e2e

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

func TestPublicAegisLinkSeedScriptLoadsETHAssetRegistryIntoRuntime(t *testing.T) {
	repo := repoRoot(t)
	homeDir := filepath.Join(t.TempDir(), "public-seeded-home")
	bootstrapPublicAegisLinkTestnet(t, homeDir)

	tempDir := t.TempDir()
	deployOutputPath := filepath.Join(tempDir, "bridge-addresses.json")
	assetsOutputPath := filepath.Join(tempDir, "bridge-assets.json")
	deployFixture := `{
  "chain_id": "11155111",
  "deployer_address": "0x2977e40f9FD046840ED10c09fbf5F0DC63A09f1d",
  "verifier_address": "0xB44f06A0187D554f4d5847AD62014014962E73fc",
  "gateway_address": "0x37ecd127529B14253C8a858976e22c4671c6Bd1E"
}`
	if err := os.WriteFile(deployOutputPath, []byte(deployFixture), 0o644); err != nil {
		t.Fatalf("write deploy fixture: %v", err)
	}

	t.Setenv("AEGISLINK_SEPOLIA_DEPLOY_OUTPUT", deployOutputPath)
	t.Setenv("AEGISLINK_SEPOLIA_ASSET_REGISTRY", assetsOutputPath)
	assetsOutput := runShellScript(t, repo, "scripts/testnet/register_bridge_assets.sh")

	var registry struct {
		Assets []struct {
			AssetID string `json:"asset_id"`
		} `json:"assets"`
	}
	if err := json.Unmarshal([]byte(assetsOutput), &registry); err != nil {
		t.Fatalf("decode asset registry output: %v\n%s", err, assetsOutput)
	}
	if len(registry.Assets) != 1 || registry.Assets[0].AssetID != "eth" {
		t.Fatalf("expected ETH-only registry fixture, got %+v", registry.Assets)
	}

	runGoCommandWithLocalCache(
		t,
		repo,
		"run",
		"./scripts/testnet/seed_public_bridge_assets.go",
		"--home",
		homeDir,
		"--registry-file",
		assetsOutputPath,
	)

	statusOutput := runGoCommandWithLocalCache(
		t,
		repo,
		"run",
		"./chain/aegislink/cmd/aegislinkd",
		"query",
		"status",
		"--home",
		homeDir,
	)
	var status struct {
		Assets int `json:"assets"`
		Limits int `json:"limits"`
	}
	if err := decodeLastJSONObject(statusOutput, &status); err != nil {
		t.Fatalf("decode seeded status output: %v\n%s", err, statusOutput)
	}
	if status.Assets != 1 || status.Limits != 1 {
		t.Fatalf("expected seeded runtime with one asset and one limit, got %+v", status)
	}
}

func TestPublicAegisLinkSeedScriptCanSeedRunningDemoNode(t *testing.T) {
	repo := repoRoot(t)
	homeDir := filepath.Join(t.TempDir(), "public-networked-seed-home")
	bootstrapPublicAegisLinkTestnet(t, homeDir)

	readyPath := filepath.Join(t.TempDir(), "demo-node-ready.json")
	cmd, logs := startIBCDemoNodeProcess(t, homeDir, readyPath, nil)
	defer stopIBCDemoNodeProcess(t, cmd, logs)

	tempDir := t.TempDir()
	deployOutputPath := filepath.Join(tempDir, "bridge-addresses.json")
	assetsOutputPath := filepath.Join(tempDir, "bridge-assets.json")
	deployFixture := `{
  "chain_id": "11155111",
  "deployer_address": "0x2977e40f9FD046840ED10c09fbf5F0DC63A09f1d",
  "verifier_address": "0xB44f06A0187D554f4d5847AD62014014962E73fc",
  "gateway_address": "0x37ecd127529B14253C8a858976e22c4671c6Bd1E"
}`
	if err := os.WriteFile(deployOutputPath, []byte(deployFixture), 0o644); err != nil {
		t.Fatalf("write deploy fixture: %v", err)
	}

	t.Setenv("AEGISLINK_SEPOLIA_DEPLOY_OUTPUT", deployOutputPath)
	t.Setenv("AEGISLINK_SEPOLIA_ASSET_REGISTRY", assetsOutputPath)
	runShellScript(t, repo, "scripts/testnet/register_bridge_assets.sh")

	runGoCommandWithLocalCache(
		t,
		repo,
		"run",
		"./scripts/testnet/seed_public_bridge_assets.go",
		"--home",
		homeDir,
		"--registry-file",
		assetsOutputPath,
		"--demo-node-ready-file",
		readyPath,
	)

	summaryOutput := runGoCommandWithLocalCache(
		t,
		repo,
		"run",
		"./chain/aegislink/cmd/aegislinkd",
		"query",
		"summary",
		"--home",
		homeDir,
		"--demo-node-ready-file",
		readyPath,
	)
	var summary struct {
		Assets int `json:"assets"`
		Limits int `json:"limits"`
	}
	if err := decodeLastJSONObject(summaryOutput, &summary); err != nil {
		t.Fatalf("decode seeded demo-node summary output: %v\n%s", err, summaryOutput)
	}
	if summary.Assets != 1 || summary.Limits != 1 {
		t.Fatalf("expected running demo node to have one seeded asset and one limit, got %+v", summary)
	}
}

func TestPublicAegisLinkSeedScriptRejectsInvalidERC20RegistryEntry(t *testing.T) {
	repo := repoRoot(t)
	homeDir := filepath.Join(t.TempDir(), "public-invalid-seed-home")
	bootstrapPublicAegisLinkTestnet(t, homeDir)

	registryPath := filepath.Join(t.TempDir(), "bridge-assets.json")
	registryFixture := `{
  "chain_id": "11155111",
  "verifier_address": "0xB44f06A0187D554f4d5847AD62014014962E73fc",
  "gateway_address": "0x37ecd127529B14253C8a858976e22c4671c6Bd1E",
  "assets": [
    {
      "asset_id": "eth",
      "source_chain_id": "11155111",
      "source_asset_kind": "native_eth",
      "denom": "ueth",
      "decimals": 18,
      "display_name": "Ether",
      "display_symbol": "ETH",
      "enabled": true
    },
    {
      "asset_id": "eth.usdc",
      "source_chain_id": "11155111",
      "source_asset_kind": "erc20",
      "source_asset_address": "0xyour_test_erc20_address",
      "denom": "uethusdc",
      "decimals": 6,
      "display_name": "USD Coin",
      "display_symbol": "USDC",
      "enabled": true
    }
  ]
}`
	if err := os.WriteFile(registryPath, []byte(registryFixture), 0o644); err != nil {
		t.Fatalf("write invalid registry fixture: %v", err)
	}

	output := runGoCommandWithLocalCacheAllowError(
		t,
		repo,
		"run",
		"./scripts/testnet/seed_public_bridge_assets.go",
		"--home",
		homeDir,
		"--registry-file",
		registryPath,
	)
	if output.Err == nil {
		t.Fatalf("expected invalid ERC-20 registry entry to be rejected, got success:\n%s", output.Stdout)
	}
	if !strings.Contains(output.Stdout, "invalid ERC-20 source asset address") {
		t.Fatalf("expected invalid ERC-20 source asset address error, got:\n%s", output.Stdout)
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
