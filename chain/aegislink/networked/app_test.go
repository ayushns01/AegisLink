package networked

import (
	"path/filepath"
	"testing"

	aegisapp "github.com/ayushns01/aegislink/chain/aegislink/app"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	ibcexported "github.com/cosmos/ibc-go/v10/modules/core/exported"
	ibctypes "github.com/cosmos/ibc-go/v10/modules/core/types"
	transfertypes "github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"
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

func TestBuildChainAppRegistersIBCModuleBasicsAndDefaultGenesis(t *testing.T) {
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

	if len(app.BasicModuleManager) == 0 {
		t.Fatal("expected basic module manager to be initialized")
	}

	genesisState := app.DefaultGenesis()
	for _, moduleName := range []string{
		authtypes.ModuleName,
		banktypes.ModuleName,
		ibcexported.ModuleName,
		transfertypes.ModuleName,
	} {
		if len(genesisState[moduleName]) == 0 {
			t.Fatalf("expected default genesis for module %q", moduleName)
		}
	}

	if err := app.BasicModuleManager.ValidateGenesis(app.Codec, app.TxConfig, genesisState); err != nil {
		t.Fatalf("validate default genesis: %v", err)
	}

	var transferGenesis transfertypes.GenesisState
	if err := app.Codec.UnmarshalJSON(genesisState[transfertypes.ModuleName], &transferGenesis); err != nil {
		t.Fatalf("decode transfer genesis: %v", err)
	}
	if transferGenesis.PortId != transfertypes.PortID {
		t.Fatalf("expected default transfer port %q, got %q", transfertypes.PortID, transferGenesis.PortId)
	}

	var ibcGenesis ibctypes.GenesisState
	if err := app.Codec.UnmarshalJSON(genesisState[ibcexported.ModuleName], &ibcGenesis); err != nil {
		t.Fatalf("decode ibc genesis: %v", err)
	}
	if len(ibcGenesis.ClientGenesis.Clients) != 0 {
		t.Fatalf("expected default ibc client list to start empty, got %+v", ibcGenesis.ClientGenesis.Clients)
	}
}

func TestBuildChainAppConstructsIBCCoreKeeperAndUpgradeStore(t *testing.T) {
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

	if app.UpgradeKeeper == nil {
		t.Fatal("expected upgrade keeper to be initialized")
	}
	if app.IBCKeeper == nil {
		t.Fatal("expected ibc keeper to be initialized")
	}
	if app.IBCModule.Name() != ibcexported.ModuleName {
		t.Fatalf("expected ibc app module name %q, got %q", ibcexported.ModuleName, app.IBCModule.Name())
	}
	if app.StoreKeys["upgrade"] == nil {
		t.Fatal("expected upgrade store key to be mounted")
	}
	if got := app.IBCKeeper.GetAuthority(); got == "" {
		t.Fatal("expected ibc keeper authority to be set")
	}
}

func TestBuildChainAppConstructsSDKAccountAndBankKeepers(t *testing.T) {
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

	if app.AccountKeeper.AddressCodec() == nil {
		t.Fatal("expected account keeper address codec to be initialized")
	}
	if got := app.AccountKeeper.GetAuthority(); got != app.Authority {
		t.Fatalf("expected account keeper authority %q, got %q", app.Authority, got)
	}
	if got := app.BankKeeper.GetAuthority(); got != app.Authority {
		t.Fatalf("expected bank keeper authority %q, got %q", app.Authority, got)
	}
	if moduleAddr := app.AccountKeeper.GetModuleAddress(transfertypes.ModuleName); moduleAddr == nil {
		t.Fatalf("expected transfer module address to be derived from auth keeper permissions")
	}
}

func TestBuildChainAppConstructsTransferKeeperAndSealsIBCRouter(t *testing.T) {
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

	if got := app.TransferKeeper.GetAuthority(); got != app.Authority {
		t.Fatalf("expected transfer keeper authority %q, got %q", app.Authority, got)
	}
	if app.TransferModule.Name() != transfertypes.ModuleName {
		t.Fatalf("expected transfer app module name %q, got %q", transfertypes.ModuleName, app.TransferModule.Name())
	}
	if app.IBCKeeper.PortKeeper.Router == nil {
		t.Fatal("expected ibc port router to be initialized")
	}
	if !app.IBCKeeper.PortKeeper.Router.HasRoute(transfertypes.ModuleName) {
		t.Fatalf("expected ibc port router to include %q route", transfertypes.ModuleName)
	}
	if !app.IBCKeeper.PortKeeper.Router.Sealed() {
		t.Fatal("expected ibc port router to be sealed after transfer route registration")
	}
}
