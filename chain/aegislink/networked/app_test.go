package networked

import (
	"bytes"
	"path/filepath"
	"testing"

	aegisapp "github.com/ayushns01/aegislink/chain/aegislink/app"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	signingtypes "github.com/cosmos/cosmos-sdk/types/tx/signing"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	transfertypes "github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"
	clienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types"
	ibcexported "github.com/cosmos/ibc-go/v10/modules/core/exported"
	ibctypes "github.com/cosmos/ibc-go/v10/modules/core/types"
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
		"staking",
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
		stakingtypes.ModuleName,
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
	if moduleAddr := app.AccountKeeper.GetModuleAddress(stakingtypes.BondedPoolName); moduleAddr == nil {
		t.Fatalf("expected bonded staking pool module address to be derived from auth keeper permissions")
	}
	if moduleAddr := app.AccountKeeper.GetModuleAddress(stakingtypes.NotBondedPoolName); moduleAddr == nil {
		t.Fatalf("expected not bonded staking pool module address to be derived from auth keeper permissions")
	}
}

func TestBuildChainAppConstructsStakingKeeperAndDefaultParams(t *testing.T) {
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

	if app.StakingKeeper == nil {
		t.Fatal("expected staking keeper to be initialized")
	}

	ctx := app.BaseApp.NewUncachedContext(false, cmtproto.Header{
		ChainID: app.AppConfig.ChainID,
		Height:  app.BaseApp.LastBlockHeight(),
	})
	params, err := app.StakingKeeper.GetParams(ctx)
	if err != nil {
		t.Fatalf("get staking params: %v", err)
	}
	if params.UnbondingTime <= 0 {
		t.Fatalf("expected positive staking unbonding time, got %+v", params)
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

func TestBuildChainAppRegistersSDKModuleServicesAndInitializesGenesis(t *testing.T) {
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

	if app.ModuleManager == nil {
		t.Fatal("expected module manager to be initialized")
	}
	if app.Configurator == nil {
		t.Fatal("expected module configurator to be initialized")
	}

	for _, msg := range []sdk.Msg{
		&authtypes.MsgUpdateParams{},
		&banktypes.MsgSend{},
		&clienttypes.MsgUpdateParams{},
		&transfertypes.MsgTransfer{},
	} {
		if handler := app.BaseApp.MsgServiceRouter().HandlerByTypeURL(sdk.MsgTypeURL(msg)); handler == nil {
			t.Fatalf("expected msg service handler for %T", msg)
		}
	}

	for _, path := range []string{
		"/cosmos.auth.v1beta1.Query/Accounts",
		"/cosmos.auth.v1beta1.Query/Account",
		"/cosmos.bank.v1beta1.Query/Params",
		"/ibc.core.client.v1.Query/ClientStates",
		"/ibc.applications.transfer.v1.Query/Params",
		"/ibc.applications.transfer.v1.Query/Denoms",
	} {
		if route := app.BaseApp.GRPCQueryRouter().Route(path); route == nil {
			t.Fatalf("expected grpc query route %q to be registered", path)
		}
	}

	ctx := app.BaseApp.NewUncachedContext(false, cmtproto.Header{ChainID: "aegislink-networked-1"})
	if got := app.TransferKeeper.GetPort(ctx); got != transfertypes.PortID {
		t.Fatalf("expected initialized transfer port %q, got %q", transfertypes.PortID, got)
	}
}

func TestBuildChainAppRegistersSDKTxService(t *testing.T) {
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

	if handler := app.BaseApp.GRPCQueryRouter().Route("/cosmos.tx.v1beta1.Service/Simulate"); handler == nil {
		t.Fatal("expected tx simulate service to be registered on the BaseApp gRPC query router")
	}
}

func TestBuildChainAppTxDecoderHandlesSecp256k1SignerPubKeys(t *testing.T) {
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

	privKey := secp256k1.GenPrivKey()
	from := sdk.AccAddress(privKey.PubKey().Address()).String()

	txBuilder := app.TxConfig.NewTxBuilder()
	if err := txBuilder.SetMsgs(&banktypes.MsgSend{
		FromAddress: from,
		ToAddress:   "cosmos1q5nq6v24qq0584nf00wuhqrku4anlxaq80aqy4",
		Amount:      sdk.NewCoins(sdk.NewInt64Coin("stake", 1)),
	}); err != nil {
		t.Fatalf("set bank send msg: %v", err)
	}
	if err := txBuilder.SetSignatures(signingtypes.SignatureV2{
		PubKey: privKey.PubKey(),
		Data: &signingtypes.SingleSignatureData{
			SignMode:  signingtypes.SignMode_SIGN_MODE_DIRECT,
			Signature: []byte("demo-signature"),
		},
		Sequence: 0,
	}); err != nil {
		t.Fatalf("set signature with secp256k1 pubkey: %v", err)
	}

	txBytes, err := app.TxConfig.TxEncoder()(txBuilder.GetTx())
	if err != nil {
		t.Fatalf("encode signed tx: %v", err)
	}
	if _, err := app.TxConfig.TxDecoder()(txBytes); err != nil {
		t.Fatalf("decode signed tx with secp256k1 pubkey: %v", err)
	}
}

func TestChainAppExecuteLocalhostTransferCreatesPacketCommitment(t *testing.T) {
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

	sender := sdk.AccAddress(bytes.Repeat([]byte{0x11}, 20))
	coin := sdk.NewInt64Coin("ueth", 1_000_000)

	result, err := app.ExecuteLocalhostTransfer(LocalhostTransferRequest{
		Sender:        sender.String(),
		Coin:          coin,
		Receiver:      "osmo1q5nq6v24qq0584nf00wuhqrku4anlxaq05wsj8",
		TimeoutHeight: clienttypes.NewHeight(1, 50),
		Memo:          "localhost-demo",
	})
	if err != nil {
		t.Fatalf("execute localhost transfer: %v", err)
	}

	if result.PortID != transfertypes.PortID {
		t.Fatalf("expected source port %q, got %q", transfertypes.PortID, result.PortID)
	}
	if result.ChannelID == "" {
		t.Fatal("expected localhost transfer channel id")
	}
	if result.Sequence != 1 {
		t.Fatalf("expected first localhost packet sequence 1, got %d", result.Sequence)
	}
	if len(result.PacketCommitment) == 0 {
		t.Fatal("expected packet commitment to be recorded")
	}

	ctx := app.BaseApp.NewUncachedContext(false, cmtproto.Header{
		ChainID: app.AppConfig.ChainID,
		Height:  app.BaseApp.LastBlockHeight(),
	})
	nextSeq, found := app.IBCKeeper.ChannelKeeper.GetNextSequenceSend(ctx, result.PortID, result.ChannelID)
	if !found {
		t.Fatalf("expected next send sequence for %s/%s", result.PortID, result.ChannelID)
	}
	if nextSeq != 2 {
		t.Fatalf("expected next send sequence 2, got %d", nextSeq)
	}

	commitment := app.IBCKeeper.ChannelKeeper.GetPacketCommitment(ctx, result.PortID, result.ChannelID, result.Sequence)
	if len(commitment) == 0 {
		t.Fatal("expected stored packet commitment")
	}
	if !bytes.Equal(commitment, result.PacketCommitment) {
		t.Fatalf("expected stored packet commitment %X, got %X", result.PacketCommitment, commitment)
	}
}

func TestChainAppFundAccountCommitsBalance(t *testing.T) {
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

	address, err := app.AccountKeeper.AddressCodec().BytesToString(bytes.Repeat([]byte{0x33}, 20))
	if err != nil {
		t.Fatalf("encode test account address: %v", err)
	}
	coin := sdk.NewInt64Coin("ueth", 42_000)

	if err := app.FundAccount(address, coin); err != nil {
		t.Fatalf("fund account: %v", err)
	}

	ctx := app.BaseApp.NewUncachedContext(false, cmtproto.Header{
		ChainID: app.AppConfig.ChainID,
		Height:  app.BaseApp.LastBlockHeight(),
	})
	accountBytes, err := app.AccountKeeper.AddressCodec().StringToBytes(address)
	if err != nil {
		t.Fatalf("decode funded account address: %v", err)
	}
	balance := app.BankKeeper.GetBalance(ctx, sdk.AccAddress(accountBytes), coin.Denom)
	if !balance.IsEqual(coin) {
		t.Fatalf("expected funded balance %s, got %s", coin, balance)
	}
	if app.BaseApp.LastBlockHeight() < 2 {
		t.Fatalf("expected baseapp height to advance after funding, got %d", app.BaseApp.LastBlockHeight())
	}
}
