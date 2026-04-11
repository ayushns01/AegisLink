package networked

import (
	"path/filepath"
	"testing"

	aegisapp "github.com/ayushns01/aegislink/chain/aegislink/app"
)

func TestNewChainAppBuildsBaseAppAndMountsCoreStoreKeys(t *testing.T) {
	t.Parallel()

	homeDir := filepath.Join(t.TempDir(), "networked-home")
	if _, err := aegisapp.InitHome(aegisapp.Config{
		HomeDir:     homeDir,
		ChainID:     "aegislink-networked-1",
		RuntimeMode: aegisapp.RuntimeModeSDKStore,
	}, false); err != nil {
		t.Fatalf("init home: %v", err)
	}

	app, err := NewChainApp(Config{HomeDir: homeDir})
	if err != nil {
		t.Fatalf("new chain app: %v", err)
	}
	t.Cleanup(func() {
		if err := app.Close(); err != nil {
			t.Fatalf("close chain app: %v", err)
		}
	})

	if app.BaseApp == nil {
		t.Fatal("expected BaseApp to be initialized")
	}
	if app.InterfaceRegistry == nil {
		t.Fatal("expected interface registry to be initialized")
	}
	if app.Codec == nil {
		t.Fatal("expected codec to be initialized")
	}
	if app.TxConfig == nil {
		t.Fatal("expected tx config to be initialized")
	}
	if app.DB == nil {
		t.Fatal("expected db to be initialized")
	}
	if app.MultiStore == nil {
		t.Fatal("expected multistore to be initialized")
	}
	if app.AppConfig.ChainID != "aegislink-networked-1" {
		t.Fatalf("expected chain id aegislink-networked-1, got %q", app.AppConfig.ChainID)
	}

	expectedKeys := []string{
		"auth",
		"bank",
		"bridge",
		"registry",
		"limits",
		"pauser",
		"governance",
		"ibcrouter",
		"capability",
		"ibc",
		"transfer",
	}
	for _, key := range expectedKeys {
		if app.StoreKeys[key] == nil {
			t.Fatalf("expected mounted store key %q", key)
		}
	}

	if err := app.MultiStore.LoadLatestVersion(); err != nil {
		t.Fatalf("load latest version: %v", err)
	}
}
