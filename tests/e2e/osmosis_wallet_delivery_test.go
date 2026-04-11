package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
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
		Enabled            bool   `json:"enabled"`
		SourceChainID      string `json:"source_chain_id"`
		DestinationChainID string `json:"destination_chain_id"`
		Provider           string `json:"provider"`
		WalletPrefix       string `json:"wallet_prefix"`
		PortID             string `json:"port_id"`
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
}
