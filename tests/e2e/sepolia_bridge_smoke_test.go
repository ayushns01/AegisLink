package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestSepoliaBridgeSmoke(t *testing.T) {
	repo := repoRoot(t)
	tempDir := t.TempDir()
	anvil := startAnvilRuntime(t)
	deployer := rpcAccounts(t, anvil.rpcURL)[0]

	deployOutputPath := filepath.Join(tempDir, "bridge-addresses.json")
	assetsOutputPath := filepath.Join(tempDir, "bridge-assets.json")
	erc20Address := deployContract(
		t,
		anvil.rpcURL,
		deployer,
		filepath.Join(repo, "contracts/ethereum/out/BridgeGateway.t.sol/TestToken.json"),
		"constructor(string,string,uint8)",
		"USD Coin",
		"USDC",
		"6",
	)

	t.Setenv("AEGISLINK_SEPOLIA_DEPLOY_OUTPUT", deployOutputPath)
	t.Setenv("AEGISLINK_SEPOLIA_ASSET_REGISTRY", assetsOutputPath)
	t.Setenv("AEGISLINK_SEPOLIA_RPC_URL", anvil.rpcURL)
	t.Setenv("AEGISLINK_SEPOLIA_PRIVATE_KEY", anvilFirstAccountPrivateKey)
	t.Setenv("AEGISLINK_SEPOLIA_DEPLOYER_ADDRESS", deployer)
	t.Setenv("AEGISLINK_SEPOLIA_ATTESTER_ADDRESS", deployer)
	t.Setenv("AEGISLINK_SEPOLIA_ERC20_ADDRESS", erc20Address)

	deployOutput := runShellScript(t, repo, "scripts/testnet/deploy_sepolia_bridge.sh")
	deployed := decodeSepoliaBridgeDeployment(t, deployOutput)
	if deployed.DeployerAddress != deployer {
		t.Fatalf("expected deployer address %q, got %q", deployer, deployed.DeployerAddress)
	}
	if deployed.VerifierAddress == "" || deployed.GatewayAddress == "" {
		t.Fatalf("expected deployed verifier and gateway addresses, got %+v", deployed)
	}
	if deployed.VerifierAddress == deployed.GatewayAddress {
		t.Fatalf("expected distinct verifier and gateway addresses, got %+v", deployed)
	}
	if code := rpcCallResult[string](t, anvil.rpcURL, "eth_getCode", []any{deployed.VerifierAddress, "latest"}); code == "" || code == "0x" {
		t.Fatalf("expected deployed verifier code at %s", deployed.VerifierAddress)
	}
	if code := rpcCallResult[string](t, anvil.rpcURL, "eth_getCode", []any{deployed.GatewayAddress, "latest"}); code == "" || code == "0x" {
		t.Fatalf("expected deployed gateway code at %s", deployed.GatewayAddress)
	}

	configCheckOutput := runGoCommand(t, repo, map[string]string{
		"AEGISLINK_RELAYER_EVM_RPC_URL":          anvil.rpcURL,
		"AEGISLINK_RELAYER_EVM_VERIFIER_ADDRESS": deployed.VerifierAddress,
		"AEGISLINK_RELAYER_EVM_GATEWAY_ADDRESS":  deployed.GatewayAddress,
	}, "test", "./relayer/internal/config", "-run", "TestLoadFromEnvParsesEVMBridgeAddresses", "-count=1")
	if configCheckOutput == "" {
		t.Fatal("expected relayer config acceptance check output")
	}

	firstDeployFile := mustReadFile(t, deployOutputPath)
	secondDeployOutput := runShellScript(t, repo, "scripts/testnet/deploy_sepolia_bridge.sh")
	secondDeployFile := mustReadFile(t, deployOutputPath)
	if firstDeployFile != secondDeployFile {
		t.Fatalf("expected deployment output to be stable across repeated runs")
	}
	if secondDeployOutput == "" {
		t.Fatal("expected repeated deployment run to emit JSON")
	}

	assetsOutput := runShellScript(t, repo, "scripts/testnet/register_bridge_assets.sh")
	registered := decodeSepoliaBridgeRegistry(t, assetsOutput)
	if registered.VerifierAddress != deployed.VerifierAddress || registered.GatewayAddress != deployed.GatewayAddress {
		t.Fatalf("expected registry to bind to deployed bridge addresses, got %+v", registered)
	}
	if len(registered.Assets) != 2 {
		t.Fatalf("expected two bridge registry entries, got %d", len(registered.Assets))
	}
	if registered.Assets[0].AssetID != "eth" && registered.Assets[1].AssetID != "eth" {
		t.Fatalf("expected native ETH registry entry, got %+v", registered.Assets)
	}
	if registered.Assets[0].AssetID != "eth.usdc" && registered.Assets[1].AssetID != "eth.usdc" {
		t.Fatalf("expected ERC-20 registry entry, got %+v", registered.Assets)
	}
	for _, asset := range registered.Assets {
		if asset.AssetID == "eth.usdc" && asset.SourceAssetAddress != erc20Address {
			t.Fatalf("expected ERC-20 source address %q, got %q", erc20Address, asset.SourceAssetAddress)
		}
	}

	firstAssetsFile := mustReadFile(t, assetsOutputPath)
	_ = runShellScript(t, repo, "scripts/testnet/register_bridge_assets.sh")
	secondAssetsFile := mustReadFile(t, assetsOutputPath)
	if firstAssetsFile != secondAssetsFile {
		t.Fatalf("expected bridge asset registry to be stable across repeated runs")
	}
}

type sepoliaBridgeDeployment struct {
	ChainID         string `json:"chain_id"`
	DeployerAddress string `json:"deployer_address"`
	VerifierAddress string `json:"verifier_address"`
	GatewayAddress  string `json:"gateway_address"`
	OutputPath      string `json:"output_path"`
}

type sepoliaBridgeRegistry struct {
	ChainID         string               `json:"chain_id"`
	VerifierAddress string               `json:"verifier_address"`
	GatewayAddress  string               `json:"gateway_address"`
	Assets          []sepoliaBridgeAsset `json:"assets"`
}

type sepoliaBridgeAsset struct {
	AssetID            string `json:"asset_id"`
	SourceChainID      string `json:"source_chain_id"`
	SourceAssetKind    string `json:"source_asset_kind"`
	SourceAssetAddress string `json:"source_asset_address,omitempty"`
	Denom              string `json:"denom"`
	Decimals           uint32 `json:"decimals"`
	DisplayName        string `json:"display_name"`
	DisplaySymbol      string `json:"display_symbol"`
	Enabled            bool   `json:"enabled"`
}

func decodeSepoliaBridgeDeployment(t *testing.T, raw string) sepoliaBridgeDeployment {
	t.Helper()

	var payload sepoliaBridgeDeployment
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatalf("decode sepolia deployment output: %v\n%s", err, raw)
	}
	return payload
}

func decodeSepoliaBridgeRegistry(t *testing.T, raw string) sepoliaBridgeRegistry {
	t.Helper()

	var payload sepoliaBridgeRegistry
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatalf("decode sepolia registry output: %v\n%s", err, raw)
	}
	return payload
}

func mustReadFile(t *testing.T, path string) string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}
