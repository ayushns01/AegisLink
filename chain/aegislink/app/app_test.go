package app

import "testing"

func TestNewAppRegistersBridgeAndSafetyModules(t *testing.T) {
	app := New()

	got := app.ModuleNames()
	want := []string{"bridge", "bank", "registry", "limits", "pauser", "ibcrouter", "governance"}

	if len(got) != len(want) {
		t.Fatalf("expected %d modules, got %d: %v", len(want), len(got), got)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected module %q at index %d, got %q", want[i], i, got[i])
		}
	}
}

func TestNewAppExposesStoreKeysForAllModules(t *testing.T) {
	app := New()

	storeKeys := app.StoreKeys()
	want := []string{"bridge", "bank", "registry", "limits", "pauser", "ibcrouter", "governance"}

	if len(storeKeys) != len(want) {
		t.Fatalf("expected %d store keys, got %d: %v", len(want), len(storeKeys), storeKeys)
	}

	for _, moduleName := range want {
		key, ok := storeKeys[moduleName]
		if !ok {
			t.Fatalf("missing store key for module %q", moduleName)
		}
		if key == "" {
			t.Fatalf("store key for module %q must not be empty", moduleName)
		}
	}
}

func TestEncodingConfigProvidesDefaultCodecAndTxConfig(t *testing.T) {
	app := New()

	encoding := app.EncodingConfig()
	if encoding.CodecName != "proto-json" {
		t.Fatalf("expected codec %q, got %q", "proto-json", encoding.CodecName)
	}
	if encoding.TxConfig.SignMode != "direct" {
		t.Fatalf("expected sign mode %q, got %q", "direct", encoding.TxConfig.SignMode)
	}
	if len(encoding.InterfaceRegistry) == 0 {
		t.Fatalf("expected interface registry entries, got none")
	}
}

func TestDefaultGenesisValidatesForNewApp(t *testing.T) {
	app := New()

	genesis := app.DefaultGenesis()
	if err := genesis.Validate(); err != nil {
		t.Fatalf("expected default genesis to validate, got error: %v", err)
	}
	if genesis.ChainID != app.Config.ChainID {
		t.Fatalf("expected chain id %q, got %q", app.Config.ChainID, genesis.ChainID)
	}
	if len(genesis.Modules) != len(app.ModuleNames()) {
		t.Fatalf("expected %d genesis modules, got %d", len(app.ModuleNames()), len(genesis.Modules))
	}
}
