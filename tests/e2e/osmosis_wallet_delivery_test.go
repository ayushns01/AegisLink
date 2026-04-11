package e2e

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	aegisapp "github.com/ayushns01/aegislink/chain/aegislink/app"
)

func TestOsmosisWalletDeliveryScaffold(t *testing.T) {
	t.Parallel()

	if os.Getenv("AEGISLINK_ENABLE_REAL_IBC") != "1" {
		t.Skip("real public IBC is optional and disabled by default in this repo")
	}

	repo := repoRoot(t)
	scaffoldDir := filepath.Join(repo, "deploy", "testnet", "ibc")
	readmePath := filepath.Join(scaffoldDir, "README.md")
	manifestPath := filepath.Join(scaffoldDir, "osmosis-wallet-delivery.example.json")

	if _, err := os.Stat(readmePath); err != nil {
		t.Fatalf("expected IBC scaffold README at %s: %v", readmePath, err)
	}

	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read IBC scaffold manifest: %v", err)
	}

	var manifest struct {
		Enabled             bool     `json:"enabled"`
		SourceChainID       string   `json:"source_chain_id"`
		DestinationChainID  string   `json:"destination_chain_id"`
		Provider            string   `json:"provider"`
		WalletPrefix        string   `json:"wallet_prefix"`
		PortID              string   `json:"port_id"`
		RouteID             string   `json:"route_id"`
		AllowedMemoPrefixes []string `json:"allowed_memo_prefixes"`
		AllowedActionTypes  []string `json:"allowed_action_types"`
		Assets              []struct {
			AssetID          string `json:"asset_id"`
			SourceDenom      string `json:"source_denom"`
			DestinationDenom string `json:"destination_denom"`
		} `json:"assets"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("decode IBC scaffold manifest: %v", err)
	}

	if manifest.Enabled {
		t.Fatal("expected public IBC scaffold to remain disabled until real Osmosis delivery is wired")
	}
	if manifest.SourceChainID == "" || manifest.DestinationChainID == "" {
		t.Fatalf("expected scaffold chain ids to be populated, got %+v", manifest)
	}
	if manifest.Provider != "hermes" {
		t.Fatalf("expected hermes provider scaffold, got %q", manifest.Provider)
	}
	if manifest.WalletPrefix != "osmo" {
		t.Fatalf("expected osmo wallet prefix scaffold, got %q", manifest.WalletPrefix)
	}
	if manifest.PortID != "transfer" {
		t.Fatalf("expected transfer port scaffold, got %q", manifest.PortID)
	}
	if manifest.RouteID != "osmosis-public-wallet" {
		t.Fatalf("expected osmosis-public-wallet route scaffold, got %q", manifest.RouteID)
	}
	if len(manifest.Assets) != 1 || manifest.Assets[0].AssetID != "eth" {
		t.Fatalf("expected ETH scaffold asset, got %+v", manifest.Assets)
	}
	if manifest.Assets[0].DestinationDenom != "ibc/ueth" {
		t.Fatalf("expected ibc/ueth scaffold destination denom, got %q", manifest.Assets[0].DestinationDenom)
	}
	if len(manifest.AllowedActionTypes) == 0 || manifest.AllowedActionTypes[0] != "swap" {
		t.Fatalf("expected action-type scaffold, got %+v", manifest.AllowedActionTypes)
	}
}

func TestPublicIBCBootstrapScriptBuildsManifestFromBridgeRegistry(t *testing.T) {
	repo := repoRoot(t)
	registryPath := filepath.Join(t.TempDir(), "bridge-assets.json")
	manifestPath := filepath.Join(t.TempDir(), "osmosis-wallet-delivery.json")
	registryFixture := `{
  "assets": [
    {
      "asset_id": "eth",
      "denom": "ueth"
    },
    {
      "asset_id": "eth.usdc",
      "denom": "uethusdc"
    }
  ]
}`
	if err := os.WriteFile(registryPath, []byte(registryFixture), 0o644); err != nil {
		t.Fatalf("write bridge registry fixture: %v", err)
	}

	output := runShellScriptWithEnv(t, repo, "scripts/testnet/bootstrap_public_ibc.sh", map[string]string{
		"AEGISLINK_SEPOLIA_ASSET_REGISTRY":           registryPath,
		"AEGISLINK_PUBLIC_IBC_MANIFEST_PATH":         manifestPath,
		"AEGISLINK_PUBLIC_IBC_SOURCE_CHAIN_ID":       "aegislink-public-testnet-1",
		"AEGISLINK_PUBLIC_IBC_DESTINATION_CHAIN_ID":  "osmosis-testnet",
		"AEGISLINK_PUBLIC_IBC_CHANNEL_ID":            "channel-public-osmosis",
		"AEGISLINK_PUBLIC_IBC_ROUTE_ID":              "osmosis-public-wallet",
		"AEGISLINK_PUBLIC_IBC_ALLOWED_MEMO_PREFIXES": "swap:,stake:",
		"AEGISLINK_PUBLIC_IBC_ALLOWED_ACTION_TYPES":  "swap,stake",
		"AEGISLINK_ENABLE_REAL_IBC":                  "0",
	})

	var result struct {
		Status     string `json:"status"`
		RouteID    string `json:"route_id"`
		AssetCount int    `json:"asset_count"`
		Enabled    bool   `json:"enabled"`
	}
	if err := decodeLastJSONObject(output, &result); err != nil {
		t.Fatalf("decode bootstrap output: %v\n%s", err, output)
	}
	if result.Status != "bootstrapped" || result.RouteID != "osmosis-public-wallet" {
		t.Fatalf("unexpected bootstrap output: %+v", result)
	}
	if result.AssetCount != 2 {
		t.Fatalf("expected two scaffolded assets, got %+v", result)
	}
	if result.Enabled {
		t.Fatalf("expected disabled scaffold by default, got %+v", result)
	}

	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read generated manifest: %v", err)
	}
	var manifest struct {
		Enabled   bool   `json:"enabled"`
		RouteID   string `json:"route_id"`
		ChannelID string `json:"channel_id"`
		Assets    []struct {
			AssetID          string `json:"asset_id"`
			DestinationDenom string `json:"destination_denom"`
		} `json:"assets"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("decode generated manifest: %v", err)
	}
	if manifest.RouteID != "osmosis-public-wallet" || manifest.ChannelID != "channel-public-osmosis" {
		t.Fatalf("unexpected manifest: %+v", manifest)
	}
	if len(manifest.Assets) != 2 {
		t.Fatalf("expected two generated assets, got %+v", manifest.Assets)
	}
	if manifest.Assets[0].DestinationDenom != "ibc/ueth" || manifest.Assets[1].DestinationDenom != "ibc/uethusdc" {
		t.Fatalf("expected derived destination denoms, got %+v", manifest.Assets)
	}
}

func TestPublicIBCSeedScriptLoadsRouteProfileIntoRuntime(t *testing.T) {
	repo := repoRoot(t)
	homeDir := filepath.Join(t.TempDir(), "public-ibc-home")
	bootstrapPublicAegisLinkTestnet(t, homeDir)

	app, err := aegisapp.LoadWithConfig(aegisapp.Config{
		HomeDir: homeDir,
	})
	if err != nil {
		t.Fatalf("load public runtime: %v", err)
	}
	if err := registerPublicBridgeAssets(t, app); err != nil {
		t.Fatalf("register bridge assets: %v", err)
	}
	if err := app.Save(); err != nil {
		t.Fatalf("save public runtime: %v", err)
	}
	if err := app.Close(); err != nil {
		t.Fatalf("close public runtime: %v", err)
	}

	manifestPath := filepath.Join(t.TempDir(), "osmosis-wallet-delivery.json")
	manifestFixture := `{
  "enabled": true,
  "source_chain_id": "aegislink-public-testnet-1",
  "destination_chain_id": "osmosis-testnet",
  "provider": "hermes",
  "wallet_prefix": "osmo",
  "channel_id": "channel-public-osmosis",
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

	output := runShellScriptWithEnv(
		t,
		repo,
		"scripts/testnet/seed_public_ibc_route.sh",
		map[string]string{
			"AEGISLINK_PUBLIC_IBC_MANIFEST_PATH": manifestPath,
		},
		homeDir,
	)
	var result struct {
		Status  string `json:"status"`
		RouteID string `json:"route_id"`
		Enabled bool   `json:"enabled"`
	}
	if err := decodeLastJSONObject(output, &result); err != nil {
		t.Fatalf("decode route seed output: %v\n%s", err, output)
	}
	if result.Status != "seeded" || result.RouteID != "osmosis-public-wallet" || !result.Enabled {
		t.Fatalf("unexpected route seed result: %+v", result)
	}

	queryOutput := runGoCommandWithLocalCache(
		t,
		repo,
		"run",
		"./chain/aegislink/cmd/aegislinkd",
		"query",
		"route-profiles",
		"--home",
		homeDir,
	)
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
	if err := decodeLastJSONObject(queryOutput, &profiles); err != nil {
		t.Fatalf("decode route profile query output: %v\n%s", err, queryOutput)
	}
	if len(profiles) != 1 {
		t.Fatalf("expected one seeded route profile, got %+v", profiles)
	}
	if profiles[0].RouteID != "osmosis-public-wallet" || profiles[0].DestinationChainID != "osmosis-testnet" || profiles[0].ChannelID != "channel-public-osmosis" {
		t.Fatalf("unexpected route profile: %+v", profiles[0])
	}
	if !profiles[0].Enabled {
		t.Fatalf("expected seeded route profile to be enabled, got %+v", profiles[0])
	}
	if len(profiles[0].Assets) != 1 || profiles[0].Assets[0].AssetID != "eth" || profiles[0].Assets[0].DestinationDenom != "ibc/ueth" {
		t.Fatalf("unexpected seeded route profile assets: %+v", profiles[0].Assets)
	}
}

func runShellScriptWithEnv(t *testing.T, dir string, script string, extraEnv map[string]string, args ...string) string {
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
	if err != nil {
		t.Fatalf("script failed: bash %v\n%s", cmdArgs, output)
	}
	return string(output)
}
