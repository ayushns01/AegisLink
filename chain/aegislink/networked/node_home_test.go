package networked

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	aegisapp "github.com/ayushns01/aegislink/chain/aegislink/app"
	cmttypes "github.com/cometbft/cometbft/types"
)

func TestEnsureCometNodeHomeCreatesCometArtifactsAndConfig(t *testing.T) {
	t.Parallel()

	homeDir := filepath.Join(t.TempDir(), "networked-home")
	if _, err := aegisapp.InitHome(aegisapp.Config{
		HomeDir:     homeDir,
		ChainID:     "aegislink-networked-1",
		RuntimeMode: aegisapp.RuntimeModeSDKStore,
	}, false); err != nil {
		t.Fatalf("init home: %v", err)
	}

	artifacts, err := EnsureCometNodeHome(Config{
		HomeDir:         homeDir,
		CometRPCAddress: "127.0.0.1:27657",
		GRPCAddress:     "127.0.0.1:9190",
	})
	if err != nil {
		t.Fatalf("ensure comet node home: %v", err)
	}

	if artifacts.RootDir != homeDir {
		t.Fatalf("expected root dir %q, got %q", homeDir, artifacts.RootDir)
	}
	if artifacts.Config == nil {
		t.Fatal("expected comet config to be returned")
	}
	if got := artifacts.Config.RPC.ListenAddress; got != "tcp://127.0.0.1:27657" {
		t.Fatalf("expected rpc listen address tcp://127.0.0.1:27657, got %q", got)
	}

	for _, path := range []string{
		artifacts.ConfigPath,
		artifacts.CometGenesisPath,
		artifacts.NodeKeyPath,
		artifacts.PrivValidatorKeyPath,
		artifacts.PrivValidatorStatePath,
	} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("expected %s to exist: %v", path, err)
		}
		if info.IsDir() {
			t.Fatalf("expected %s to be a file", path)
		}
	}

	configBytes, err := os.ReadFile(artifacts.ConfigPath)
	if err != nil {
		t.Fatalf("read config.toml: %v", err)
	}
	configText := string(configBytes)
	if !strings.Contains(configText, `laddr = "tcp://127.0.0.1:27657"`) {
		t.Fatalf("expected config.toml to contain rpc listen address, got:\n%s", configText)
	}
	if !strings.Contains(configText, `moniker = "aegislink"`) {
		t.Fatalf("expected config.toml to contain moniker, got:\n%s", configText)
	}
	if !strings.Contains(configText, `genesis_file = "config/comet-genesis.json"`) {
		t.Fatalf("expected config.toml to point at comet genesis file, got:\n%s", configText)
	}

	cometGenesis, err := cmttypes.GenesisDocFromFile(artifacts.CometGenesisPath)
	if err != nil {
		t.Fatalf("load comet genesis: %v", err)
	}
	if cometGenesis.ChainID != "aegislink-networked-1" {
		t.Fatalf("expected comet genesis chain id aegislink-networked-1, got %q", cometGenesis.ChainID)
	}
	if len(cometGenesis.Validators) != 1 {
		t.Fatalf("expected one comet genesis validator, got %d", len(cometGenesis.Validators))
	}
	if cometGenesis.Validators[0].Power != 1 {
		t.Fatalf("expected comet genesis validator power 1, got %d", cometGenesis.Validators[0].Power)
	}
}

func TestEnsureCometNodeHomeIsStableAcrossRepeatedCalls(t *testing.T) {
	t.Parallel()

	homeDir := filepath.Join(t.TempDir(), "networked-home")
	if _, err := aegisapp.InitHome(aegisapp.Config{
		HomeDir:     homeDir,
		ChainID:     "aegislink-networked-1",
		RuntimeMode: aegisapp.RuntimeModeSDKStore,
	}, false); err != nil {
		t.Fatalf("init home: %v", err)
	}

	artifacts, err := EnsureCometNodeHome(Config{HomeDir: homeDir})
	if err != nil {
		t.Fatalf("first ensure comet node home: %v", err)
	}
	nodeKeyBefore, err := os.ReadFile(artifacts.NodeKeyPath)
	if err != nil {
		t.Fatalf("read node key before: %v", err)
	}
	privValBefore, err := os.ReadFile(artifacts.PrivValidatorKeyPath)
	if err != nil {
		t.Fatalf("read priv validator key before: %v", err)
	}

	artifacts, err = EnsureCometNodeHome(Config{HomeDir: homeDir})
	if err != nil {
		t.Fatalf("second ensure comet node home: %v", err)
	}
	nodeKeyAfter, err := os.ReadFile(artifacts.NodeKeyPath)
	if err != nil {
		t.Fatalf("read node key after: %v", err)
	}
	privValAfter, err := os.ReadFile(artifacts.PrivValidatorKeyPath)
	if err != nil {
		t.Fatalf("read priv validator key after: %v", err)
	}

	if string(nodeKeyBefore) != string(nodeKeyAfter) {
		t.Fatal("expected node key to remain stable across repeated calls")
	}
	if string(privValBefore) != string(privValAfter) {
		t.Fatal("expected priv validator key to remain stable across repeated calls")
	}
}
