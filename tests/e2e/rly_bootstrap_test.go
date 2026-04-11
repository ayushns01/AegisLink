package e2e

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRlyBootstrapGeneratesPathConfig(t *testing.T) {
	t.Parallel()

	repo := repoRoot(t)
	outputDir := filepath.Join(t.TempDir(), "rly")
	manifestPath := filepath.Join(t.TempDir(), "osmosis-wallet-delivery.json")
	destinationMetadataPath := filepath.Join(t.TempDir(), "osmosis-testnet.chain.json")

	manifestFixture := `{
  "enabled": false,
  "source_chain_id": "aegislink-public-testnet-1",
  "destination_chain_id": "osmo-test-5",
  "provider": "rly",
  "wallet_prefix": "osmo",
  "channel_id": "channel-42",
  "port_id": "transfer",
  "route_id": "osmosis-public-wallet",
  "allowed_memo_prefixes": ["swap:", "stake:"],
  "allowed_action_types": ["swap", "stake"],
  "assets": [
    {
      "asset_id": "eth",
      "source_denom": "ueth",
      "destination_denom": "ibc/ueth"
    }
  ]
}`
	if err := os.WriteFile(manifestPath, []byte(manifestFixture), 0o644); err != nil {
		t.Fatalf("write manifest fixture: %v", err)
	}

	destinationMetadataFixture := `{
  "chain_name": "osmosistestnet",
  "chain_id": "osmo-test-5",
  "bech32_prefix": "osmo",
  "fees": {
    "fee_tokens": [
      {
        "denom": "uosmo",
        "fixed_min_gas_price": 0.025
      }
    ]
  },
  "apis": {
    "rpc": [
      {
        "address": "https://rpc.osmotest5.osmosis.zone:443"
      }
    ],
    "grpc": [
      {
        "address": "https://grpc.osmotest5.osmosis.zone:443"
      }
    ]
  }
}`
	if err := os.WriteFile(destinationMetadataPath, []byte(destinationMetadataFixture), 0o644); err != nil {
		t.Fatalf("write destination metadata fixture: %v", err)
	}

	output := runShellScriptWithEnv(
		t,
		repo,
		"scripts/testnet/bootstrap_rly_path.sh",
		map[string]string{
			"AEGISLINK_PUBLIC_IBC_MANIFEST_PATH":         manifestPath,
			"AEGISLINK_RLY_DESTINATION_METADATA_PATH":    destinationMetadataPath,
			"AEGISLINK_RLY_OUTPUT_DIR":                   outputDir,
			"AEGISLINK_RLY_SOURCE_RPC_ADDR":              "http://127.0.0.1:26657",
			"AEGISLINK_RLY_SOURCE_RPC_WS_ADDR":           "ws://127.0.0.1:26657/websocket",
			"AEGISLINK_RLY_SOURCE_GRPC_ADDR":             "http://127.0.0.1:9090",
			"AEGISLINK_RLY_SOURCE_KEY_NAME":              "aegislink-demo",
			"AEGISLINK_RLY_DESTINATION_KEY_NAME":         "osmosis-demo",
			"AEGISLINK_RLY_DESTINATION_ACCOUNT_PREFIX":   "osmo",
			"AEGISLINK_RLY_DESTINATION_GAS_PRICE_DENOM":  "uosmo",
			"AEGISLINK_RLY_DESTINATION_GAS_PRICE_AMOUNT": "0.025",
			"AEGISLINK_RLY_PATH_NAME":                    "aegislink-osmosis-demo",
		},
	)

	var result struct {
		Status          string `json:"status"`
		PathName        string `json:"path_name"`
		ConfigPath      string `json:"config_path"`
		PathFile        string `json:"path_file"`
		SourceChainID   string `json:"source_chain_id"`
		DestChainID     string `json:"destination_chain_id"`
		DestinationRPC  string `json:"destination_rpc"`
		DestinationGRPC string `json:"destination_grpc"`
	}
	if err := decodeLastJSONObject(output, &result); err != nil {
		t.Fatalf("decode bootstrap output: %v\n%s", err, output)
	}
	if result.Status != "bootstrapped" || result.PathName != "aegislink-osmosis-demo" {
		t.Fatalf("unexpected bootstrap result: %+v", result)
	}

	configData, err := os.ReadFile(result.ConfigPath)
	if err != nil {
		t.Fatalf("read generated rly config: %v", err)
	}
	configText := string(configData)
	if !strings.Contains(configText, "id: aegislink-public-testnet-1") {
		t.Fatalf("expected source chain id in config, got:\n%s", configText)
	}
	if !strings.Contains(configText, "rpc-addr: http://127.0.0.1:26657") {
		t.Fatalf("expected source rpc in config, got:\n%s", configText)
	}
	if !strings.Contains(configText, "id: osmo-test-5") {
		t.Fatalf("expected destination chain id in config, got:\n%s", configText)
	}
	if !strings.Contains(configText, "grpc-addr: https://grpc.osmotest5.osmosis.zone:443") {
		t.Fatalf("expected destination grpc in config, got:\n%s", configText)
	}

	pathData, err := os.ReadFile(result.PathFile)
	if err != nil {
		t.Fatalf("read generated rly path file: %v", err)
	}
	var path struct {
		Src struct {
			ChainID   string `json:"chain_id"`
			ClientID  string `json:"client_id"`
			PortID    string `json:"port_id"`
			ChannelID string `json:"channel_id"`
		} `json:"src"`
		Dst struct {
			ChainID   string `json:"chain_id"`
			PortID    string `json:"port_id"`
			ChannelID string `json:"channel_id"`
		} `json:"dst"`
		Manifest struct {
			RouteID string `json:"route_id"`
			Assets  []struct {
				AssetID string `json:"asset_id"`
			} `json:"assets"`
		} `json:"manifest"`
	}
	if err := json.Unmarshal(pathData, &path); err != nil {
		t.Fatalf("decode generated rly path file: %v\n%s", err, string(pathData))
	}
	if path.Src.ChainID != "aegislink-public-testnet-1" || path.Dst.ChainID != "osmo-test-5" {
		t.Fatalf("unexpected generated path: %+v", path)
	}
	if path.Src.PortID != "transfer" || path.Dst.PortID != "transfer" {
		t.Fatalf("expected transfer port ids, got %+v", path)
	}
	if path.Src.ChannelID != "channel-42" || path.Dst.ChannelID != "channel-42" {
		t.Fatalf("expected channel-42 in generated path, got %+v", path)
	}
	if path.Manifest.RouteID != "osmosis-public-wallet" || len(path.Manifest.Assets) != 1 || path.Manifest.Assets[0].AssetID != "eth" {
		t.Fatalf("unexpected manifest wiring in generated path: %+v", path.Manifest)
	}
}

func TestRlyBootstrapCanReadSourceEndpointsFromDemoNodeReadyFile(t *testing.T) {
	t.Parallel()

	repo := repoRoot(t)
	outputDir := filepath.Join(t.TempDir(), "rly")
	manifestPath := filepath.Join(t.TempDir(), "osmosis-wallet-delivery.json")
	destinationMetadataPath := filepath.Join(t.TempDir(), "osmosis-testnet.chain.json")
	readyPath := filepath.Join(t.TempDir(), "demo-node-ready.json")

	manifestFixture := `{
  "enabled": false,
  "source_chain_id": "aegislink-public-testnet-1",
  "destination_chain_id": "osmo-test-5",
  "provider": "rly",
  "wallet_prefix": "osmo",
  "channel_id": "channel-42",
  "port_id": "transfer",
  "route_id": "osmosis-public-wallet",
  "assets": [
    {
      "asset_id": "eth",
      "source_denom": "ueth",
      "destination_denom": "ibc/ueth"
    }
  ]
}`
	if err := os.WriteFile(manifestPath, []byte(manifestFixture), 0o644); err != nil {
		t.Fatalf("write manifest fixture: %v", err)
	}

	destinationMetadataFixture := `{
  "chain_name": "osmosistestnet",
  "chain_id": "osmo-test-5",
  "bech32_prefix": "osmo",
  "fees": {
    "fee_tokens": [
      {
        "denom": "uosmo",
        "fixed_min_gas_price": 0.025
      }
    ]
  },
  "apis": {
    "rpc": [
      {
        "address": "https://rpc.osmotest5.osmosis.zone:443"
      }
    ],
    "grpc": [
      {
        "address": "https://grpc.osmotest5.osmosis.zone:443"
      }
    ]
  }
}`
	if err := os.WriteFile(destinationMetadataPath, []byte(destinationMetadataFixture), 0o644); err != nil {
		t.Fatalf("write destination metadata fixture: %v", err)
	}

	readyFixture := `{
  "status": "ready",
  "chain_id": "aegislink-public-testnet-1",
  "node_id": "aegislink-demo-node",
  "rpc_address": "127.0.0.1:28657",
  "comet_rpc_address": "127.0.0.1:29657",
  "grpc_address": "127.0.0.1:29090",
  "config_path": "/tmp/aegislink-demo/config/config.toml",
  "node_key_path": "/tmp/aegislink-demo/config/node_key.json",
  "priv_validator_key_path": "/tmp/aegislink-demo/config/priv_validator_key.json",
  "priv_validator_state_path": "/tmp/aegislink-demo/data/priv_validator_state.json",
  "core_store_keys": ["auth", "bank", "bridge", "ibc", "transfer"]
}`
	if err := os.WriteFile(readyPath, []byte(readyFixture), 0o644); err != nil {
		t.Fatalf("write ready fixture: %v", err)
	}

	output := runShellScriptWithEnv(
		t,
		repo,
		"scripts/testnet/bootstrap_rly_path.sh",
		map[string]string{
			"AEGISLINK_PUBLIC_IBC_MANIFEST_PATH":         manifestPath,
			"AEGISLINK_RLY_DESTINATION_METADATA_PATH":    destinationMetadataPath,
			"AEGISLINK_RLY_OUTPUT_DIR":                   outputDir,
			"AEGISLINK_RLY_SOURCE_READY_FILE":            readyPath,
			"AEGISLINK_RLY_SOURCE_KEY_NAME":              "aegislink-demo",
			"AEGISLINK_RLY_DESTINATION_KEY_NAME":         "osmosis-demo",
			"AEGISLINK_RLY_DESTINATION_ACCOUNT_PREFIX":   "osmo",
			"AEGISLINK_RLY_DESTINATION_GAS_PRICE_DENOM":  "uosmo",
			"AEGISLINK_RLY_DESTINATION_GAS_PRICE_AMOUNT": "0.025",
			"AEGISLINK_RLY_PATH_NAME":                    "aegislink-osmosis-demo",
		},
	)

	var result struct {
		Status     string `json:"status"`
		ConfigPath string `json:"config_path"`
	}
	if err := decodeLastJSONObject(output, &result); err != nil {
		t.Fatalf("decode bootstrap output: %v\n%s", err, output)
	}
	if result.Status != "bootstrapped" {
		t.Fatalf("unexpected bootstrap result: %+v", result)
	}

	configData, err := os.ReadFile(result.ConfigPath)
	if err != nil {
		t.Fatalf("read generated rly config: %v", err)
	}
	configText := string(configData)
	if !strings.Contains(configText, "rpc-addr: http://127.0.0.1:29657") {
		t.Fatalf("expected ready-file rpc in config, got:\n%s", configText)
	}
	if !strings.Contains(configText, "websocket-addr: ws://127.0.0.1:29657/websocket") {
		t.Fatalf("expected derived websocket addr in config, got:\n%s", configText)
	}
	if !strings.Contains(configText, "grpc-addr: http://127.0.0.1:29090") {
		t.Fatalf("expected ready-file grpc in config, got:\n%s", configText)
	}
}

func TestRlyBootstrapRejectsReadyFileWithoutIBCCoreStoreKeys(t *testing.T) {
	t.Parallel()

	repo := repoRoot(t)
	outputDir := filepath.Join(t.TempDir(), "rly")
	manifestPath := filepath.Join(t.TempDir(), "osmosis-wallet-delivery.json")
	destinationMetadataPath := filepath.Join(t.TempDir(), "osmosis-testnet.chain.json")
	readyPath := filepath.Join(t.TempDir(), "demo-node-ready.json")

	manifestFixture := `{
  "source_chain_id": "aegislink-public-testnet-1",
  "destination_chain_id": "osmo-test-5",
  "provider": "rly",
  "wallet_prefix": "osmo",
  "channel_id": "channel-42",
  "port_id": "transfer",
  "route_id": "osmosis-public-wallet",
  "assets": [{"asset_id":"eth","source_denom":"ueth","destination_denom":"ibc/ueth"}]
}`
	if err := os.WriteFile(manifestPath, []byte(manifestFixture), 0o644); err != nil {
		t.Fatalf("write manifest fixture: %v", err)
	}

	destinationMetadataFixture := `{
  "chain_name": "osmosistestnet",
  "chain_id": "osmo-test-5",
  "bech32_prefix": "osmo",
  "fees": {"fee_tokens":[{"denom":"uosmo","fixed_min_gas_price":0.025}]},
  "apis": {
    "rpc":[{"address":"https://rpc.osmotest5.osmosis.zone:443"}],
    "grpc":[{"address":"https://grpc.osmotest5.osmosis.zone:443"}]
  }
}`
	if err := os.WriteFile(destinationMetadataPath, []byte(destinationMetadataFixture), 0o644); err != nil {
		t.Fatalf("write destination metadata fixture: %v", err)
	}

	readyFixture := `{
  "status": "ready",
  "chain_id": "aegislink-public-testnet-1",
  "node_id": "aegislink-demo-node",
  "rpc_address": "127.0.0.1:28657",
  "grpc_address": "127.0.0.1:29090",
  "core_store_keys": ["auth", "bank", "bridge"]
}`
	if err := os.WriteFile(readyPath, []byte(readyFixture), 0o644); err != nil {
		t.Fatalf("write ready fixture: %v", err)
	}

	err := runShellScriptExpectError(
		t,
		repo,
		"scripts/testnet/bootstrap_rly_path.sh",
		map[string]string{
			"AEGISLINK_PUBLIC_IBC_MANIFEST_PATH":         manifestPath,
			"AEGISLINK_RLY_DESTINATION_METADATA_PATH":    destinationMetadataPath,
			"AEGISLINK_RLY_OUTPUT_DIR":                   outputDir,
			"AEGISLINK_RLY_SOURCE_READY_FILE":            readyPath,
			"AEGISLINK_RLY_SOURCE_KEY_NAME":              "aegislink-demo",
			"AEGISLINK_RLY_DESTINATION_KEY_NAME":         "osmosis-demo",
			"AEGISLINK_RLY_DESTINATION_ACCOUNT_PREFIX":   "osmo",
			"AEGISLINK_RLY_DESTINATION_GAS_PRICE_DENOM":  "uosmo",
			"AEGISLINK_RLY_DESTINATION_GAS_PRICE_AMOUNT": "0.025",
			"AEGISLINK_RLY_PATH_NAME":                    "aegislink-osmosis-demo",
		},
	)
	if !strings.Contains(err, "missing required source core store keys") {
		t.Fatalf("expected missing core store keys error, got:\n%s", err)
	}
}

func runShellScriptExpectError(t *testing.T, dir string, script string, extraEnv map[string]string, args ...string) string {
	t.Helper()

	cmdArgs := append([]string{script}, args...)
	cmd := exec.Command("bash", cmdArgs...)
	cmd.Dir = dir
	cmd.Env = append([]string{}, os.Environ()...)

	cacheRoot := filepath.Join(os.TempDir(), "aegislink-e2e-go-cache")
	if err := os.MkdirAll(cacheRoot, 0o755); err != nil {
		t.Fatalf("create e2e go cache root: %v", err)
	}
	cmd.Env = append(cmd.Env,
		"GOCACHE="+filepath.Join(cacheRoot, "gocache"),
		"GOMODCACHE=/Users/ayushns01/go/pkg/mod",
	)
	for key, value := range extraEnv {
		cmd.Env = append(cmd.Env, key+"="+value)
	}

	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected script to fail: bash %v\n%s", cmdArgs, output)
	}
	return string(output)
}
