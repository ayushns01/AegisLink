package networked

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"
	aegisapp "github.com/ayushns01/aegislink/chain/aegislink/app"
	abcitypes "github.com/cometbft/cometbft/abci/types"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	txtypes "github.com/cosmos/cosmos-sdk/types/tx"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	proto "github.com/cosmos/gogoproto/proto"
	transfertypes "github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"
	clienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types"
	commitmenttypes "github.com/cosmos/ibc-go/v10/modules/core/23-commitment/types"
	ibctm "github.com/cosmos/ibc-go/v10/modules/light-clients/07-tendermint"
)

func TestABCIApplicationRoutesSDKGRPCQueriesToBaseApp(t *testing.T) {
	t.Parallel()

	homeDir := filepath.Join(t.TempDir(), "networked-home")
	if _, err := aegisapp.InitHome(aegisapp.Config{
		HomeDir:     homeDir,
		ChainID:     "aegislink-networked-1",
		RuntimeMode: aegisapp.RuntimeModeSDKStore,
	}, false); err != nil {
		t.Fatalf("init home: %v", err)
	}

	chainApp, err := NewChainApp(Config{HomeDir: homeDir})
	if err != nil {
		t.Fatalf("new chain app: %v", err)
	}
	t.Cleanup(func() {
		if err := chainApp.Close(); err != nil {
			t.Fatalf("close chain app: %v", err)
		}
	})

	bridgeApp, err := aegisapp.LoadWithConfig(chainApp.AppConfig)
	if err != nil {
		t.Fatalf("load bridge app: %v", err)
	}
	t.Cleanup(func() {
		if err := bridgeApp.Close(); err != nil {
			t.Fatalf("close bridge app: %v", err)
		}
	})

	abciApp := NewABCIApplication(chainApp.AppConfig, bridgeApp, chainApp)

	bankReqBz, err := proto.Marshal(&banktypes.QueryParamsRequest{})
	if err != nil {
		t.Fatalf("marshal bank params request: %v", err)
	}
	bankResp, err := abciApp.Query(context.Background(), &abcitypes.RequestQuery{
		Path: "/cosmos.bank.v1beta1.Query/Params",
		Data: bankReqBz,
	})
	if err != nil {
		t.Fatalf("query bank params: %v", err)
	}
	if bankResp.Code != 0 {
		t.Fatalf("expected bank params query success, got %+v", bankResp)
	}
	var bankParams banktypes.QueryParamsResponse
	if err := proto.Unmarshal(bankResp.Value, &bankParams); err != nil {
		t.Fatalf("unmarshal bank params response: %v", err)
	}
	if len(bankResp.Value) == 0 {
		t.Fatalf("expected bank params in response, got %+v", bankParams)
	}

	transferReqBz, err := proto.Marshal(&transfertypes.QueryParamsRequest{})
	if err != nil {
		t.Fatalf("marshal transfer params request: %v", err)
	}
	transferResp, err := abciApp.Query(context.Background(), &abcitypes.RequestQuery{
		Path: "/ibc.applications.transfer.v1.Query/Params",
		Data: transferReqBz,
	})
	if err != nil {
		t.Fatalf("query transfer params: %v", err)
	}
	if transferResp.Code != 0 {
		t.Fatalf("expected transfer params query success, got %+v", transferResp)
	}
	var transferParams transfertypes.QueryParamsResponse
	if err := proto.Unmarshal(transferResp.Value, &transferParams); err != nil {
		t.Fatalf("unmarshal transfer params response: %v", err)
	}
	if transferParams.Params == nil {
		t.Fatalf("expected transfer params in response, got %+v", transferParams)
	}

	signer := "cosmos1q5nq6v24qq0584nf00wuhqrku4anlxaq80aqy4"
	if err := chainApp.FundAccount(signer, sdk.NewCoin(sdk.DefaultBondDenom, sdkmath.NewInt(1_000_000))); err != nil {
		t.Fatalf("fund signer account: %v", err)
	}

	accountReqBz, err := proto.Marshal(&authtypes.QueryAccountRequest{Address: signer})
	if err != nil {
		t.Fatalf("marshal auth account request: %v", err)
	}
	accountResp, err := abciApp.Query(context.Background(), &abcitypes.RequestQuery{
		Path: "/cosmos.auth.v1beta1.Query/Account",
		Data: accountReqBz,
	})
	if err != nil {
		t.Fatalf("query auth account: %v", err)
	}
	if accountResp.Code != 0 {
		t.Fatalf("expected auth account query success, got %+v", accountResp)
	}
	var accountQuery authtypes.QueryAccountResponse
	if err := proto.Unmarshal(accountResp.Value, &accountQuery); err != nil {
		t.Fatalf("unmarshal auth account response: %v", err)
	}
	if accountQuery.Account == nil {
		t.Fatalf("expected auth account query payload, got %+v", accountQuery)
	}

	denomTracesReqBz, err := proto.Marshal(&transfertypes.QueryDenomsRequest{})
	if err != nil {
		t.Fatalf("marshal denom traces request: %v", err)
	}
	denomTracesResp, err := abciApp.Query(context.Background(), &abcitypes.RequestQuery{
		Path: "/ibc.applications.transfer.v1.Query/Denoms",
		Data: denomTracesReqBz,
	})
	if err != nil {
		t.Fatalf("query transfer denom traces: %v", err)
	}
	if denomTracesResp.Code != 0 {
		t.Fatalf("expected denom traces query success, got %+v", denomTracesResp)
	}
	var denomTraceQuery transfertypes.QueryDenomsResponse
	if err := proto.Unmarshal(denomTracesResp.Value, &denomTraceQuery); err != nil {
		t.Fatalf("unmarshal denom traces response: %v", err)
	}

	legacyDenomTracesReqBz, err := proto.Marshal(&legacyQueryDenomTracesRequest{})
	if err != nil {
		t.Fatalf("marshal legacy denom traces request: %v", err)
	}
	legacyDenomTracesResp, err := abciApp.Query(context.Background(), &abcitypes.RequestQuery{
		Path: "/ibc.applications.transfer.v1.Query/DenomTraces",
		Data: legacyDenomTracesReqBz,
	})
	if err != nil {
		t.Fatalf("query legacy transfer denom traces: %v", err)
	}
	if legacyDenomTracesResp.Code != 0 {
		t.Fatalf("expected legacy denom traces query success, got %+v", legacyDenomTracesResp)
	}
	var legacyDenomTraceQuery legacyQueryDenomTracesResponse
	if err := proto.Unmarshal(legacyDenomTracesResp.Value, &legacyDenomTraceQuery); err != nil {
		t.Fatalf("unmarshal legacy denom traces response: %v", err)
	}

	legacyDenomTraceReqBz, err := proto.Marshal(&legacyQueryDenomTraceRequest{Hash: "ueth"})
	if err != nil {
		t.Fatalf("marshal legacy denom trace request: %v", err)
	}
	legacyDenomTraceResp, err := abciApp.Query(context.Background(), &abcitypes.RequestQuery{
		Path: "/ibc.applications.transfer.v1.Query/DenomTrace",
		Data: legacyDenomTraceReqBz,
	})
	if err != nil {
		t.Fatalf("query legacy transfer denom trace: %v", err)
	}
	if legacyDenomTraceResp.Code != 0 {
		t.Fatalf("expected legacy denom trace query success, got %+v", legacyDenomTraceResp)
	}
	var legacyDenomQuery legacyQueryDenomTraceResponse
	if err := proto.Unmarshal(legacyDenomTraceResp.Value, &legacyDenomQuery); err != nil {
		t.Fatalf("unmarshal legacy denom trace response: %v", err)
	}
	if legacyDenomQuery.DenomTrace == nil || legacyDenomQuery.DenomTrace.BaseDenom != "ueth" {
		t.Fatalf("expected legacy denom trace for ueth, got %+v", legacyDenomQuery.DenomTrace)
	}
}

func TestABCIApplicationRoutesSDKTransferTxsToBaseApp(t *testing.T) {
	t.Parallel()

	homeDir := filepath.Join(t.TempDir(), "networked-home")
	if _, err := aegisapp.InitHome(aegisapp.Config{
		HomeDir:     homeDir,
		ChainID:     "aegislink-networked-1",
		RuntimeMode: aegisapp.RuntimeModeSDKStore,
	}, false); err != nil {
		t.Fatalf("init home: %v", err)
	}

	chainApp, err := NewChainApp(Config{HomeDir: homeDir})
	if err != nil {
		t.Fatalf("new chain app: %v", err)
	}
	t.Cleanup(func() {
		if err := chainApp.Close(); err != nil {
			t.Fatalf("close chain app: %v", err)
		}
	})

	bridgeApp, err := aegisapp.LoadWithConfig(chainApp.AppConfig)
	if err != nil {
		t.Fatalf("load bridge app: %v", err)
	}
	t.Cleanup(func() {
		if err := bridgeApp.Close(); err != nil {
			t.Fatalf("close bridge app: %v", err)
		}
	})

	abciApp := NewABCIApplication(chainApp.AppConfig, bridgeApp, chainApp)

	transferReq, sender, err := chainApp.normalizeLocalhostTransferRequest(LocalhostTransferRequest{
		Coin:          sdk.NewCoin("ueth", sdkmath.NewInt(1000)),
		Receiver:      "cosmos1q5nq6v24qq0584nf00wuhqrku4anlxaq80aqy4",
		TimeoutHeight: clienttypes.NewHeight(1, 120),
		Memo:          "sdk-abci-transfer",
	})
	if err != nil {
		t.Fatalf("normalize localhost transfer request: %v", err)
	}

	ctx := chainApp.BaseApp.NewUncachedContext(false, cmtproto.Header{
		ChainID: chainApp.AppConfig.ChainID,
		Height:  chainApp.BaseApp.LastBlockHeight(),
		Time:    time.Now().UTC(),
	})
	if err := chainApp.ensureLocalhostTransferPath(ctx, transferReq.SourcePort, transferReq.SourceChannel, transferReq.CounterpartyChannel); err != nil {
		t.Fatalf("ensure localhost transfer path: %v", err)
	}
	if err := chainApp.ensureTransferSenderBalance(ctx, sender, transferReq.Coin); err != nil {
		t.Fatalf("ensure localhost transfer sender balance: %v", err)
	}
	chainApp.MultiStore.Commit()

	msg := transfertypes.NewMsgTransfer(
		transferReq.SourcePort,
		transferReq.SourceChannel,
		transferReq.Coin,
		transferReq.Sender,
		transferReq.Receiver,
		transferReq.TimeoutHeight,
		transferReq.TimeoutTimestamp,
		transferReq.Memo,
	)
	txBuilder := chainApp.TxConfig.NewTxBuilder()
	if err := txBuilder.SetMsgs(msg); err != nil {
		t.Fatalf("set sdk transfer msg: %v", err)
	}
	txBytes, err := chainApp.TxConfig.TxEncoder()(txBuilder.GetTx())
	if err != nil {
		t.Fatalf("encode sdk transfer tx: %v", err)
	}

	checkResp, err := abciApp.CheckTx(context.Background(), &abcitypes.RequestCheckTx{
		Type: abcitypes.CheckTxType_New,
		Tx:   txBytes,
	})
	if err != nil {
		t.Fatalf("check sdk transfer tx: %v", err)
	}
	if checkResp.Code != 0 {
		t.Fatalf("expected sdk transfer checktx success, got %+v", checkResp)
	}

	finalizeResp, err := abciApp.FinalizeBlock(context.Background(), &abcitypes.RequestFinalizeBlock{
		Height: chainApp.BaseApp.LastBlockHeight() + 1,
		Hash:   chainApp.BaseApp.LastCommitID().Hash,
		Time:   time.Now().UTC(),
		Txs:    [][]byte{txBytes},
	})
	if err != nil {
		t.Fatalf("finalize sdk transfer tx: %v", err)
	}
	if len(finalizeResp.TxResults) != 1 || finalizeResp.TxResults[0].Code != 0 {
		t.Fatalf("expected sdk transfer finalize success, got %+v", finalizeResp.TxResults)
	}

	if _, err := abciApp.Commit(context.Background(), &abcitypes.RequestCommit{}); err != nil {
		t.Fatalf("commit sdk transfer tx: %v", err)
	}

	info, err := abciApp.Info(context.Background(), &abcitypes.RequestInfo{})
	if err != nil {
		t.Fatalf("info after sdk transfer tx: %v", err)
	}
	if info.LastBlockHeight < 2 {
		t.Fatalf("expected sdk block height to advance, got %+v", info)
	}
	if len(info.LastBlockAppHash) == 0 {
		t.Fatalf("expected sdk app hash after commit, got %+v", info)
	}

	verifyCtx := chainApp.BaseApp.NewUncachedContext(false, cmtproto.Header{
		ChainID: chainApp.AppConfig.ChainID,
		Height:  chainApp.BaseApp.LastBlockHeight(),
		Time:    time.Now().UTC(),
	})
	nextSequence, found := chainApp.IBCKeeper.ChannelKeeper.GetNextSequenceSend(verifyCtx, transferReq.SourcePort, transferReq.SourceChannel)
	if !found || nextSequence <= 1 {
		t.Fatalf("expected next send sequence to advance, found=%t next=%d", found, nextSequence)
	}
	commitment := chainApp.IBCKeeper.ChannelKeeper.GetPacketCommitment(verifyCtx, transferReq.SourcePort, transferReq.SourceChannel, nextSequence-1)
	if len(commitment) == 0 {
		t.Fatal("expected packet commitment after sdk transfer tx")
	}
}

func TestABCIApplicationRoutesSDKSimulateQueriesForIBCClientTxs(t *testing.T) {
	t.Parallel()

	homeDir := filepath.Join(t.TempDir(), "networked-home")
	if _, err := aegisapp.InitHome(aegisapp.Config{
		HomeDir:     homeDir,
		ChainID:     "aegislink-networked-1",
		RuntimeMode: aegisapp.RuntimeModeSDKStore,
	}, false); err != nil {
		t.Fatalf("init home: %v", err)
	}

	chainApp, err := NewChainApp(Config{HomeDir: homeDir})
	if err != nil {
		t.Fatalf("new chain app: %v", err)
	}
	t.Cleanup(func() {
		if err := chainApp.Close(); err != nil {
			t.Fatalf("close chain app: %v", err)
		}
	})

	bridgeApp, err := aegisapp.LoadWithConfig(chainApp.AppConfig)
	if err != nil {
		t.Fatalf("load bridge app: %v", err)
	}
	t.Cleanup(func() {
		if err := bridgeApp.Close(); err != nil {
			t.Fatalf("close bridge app: %v", err)
		}
	})
	abciApp := NewABCIApplication(chainApp.AppConfig, bridgeApp, chainApp)

	signer := "cosmos1q5nq6v24qq0584nf00wuhqrku4anlxaq80aqy4"
	if err := chainApp.FundAccount(signer, sdk.NewCoin(sdk.DefaultBondDenom, sdkmath.NewInt(1_000_000))); err != nil {
		t.Fatalf("fund signer account: %v", err)
	}

	msg, err := clienttypes.NewMsgCreateClient(
		ibctm.NewClientState(
			"counterparty-1",
			ibctm.DefaultTrustLevel,
			14*24*time.Hour,
			21*24*time.Hour,
			10*time.Minute,
			clienttypes.NewHeight(1, 1),
			commitmenttypes.GetSDKSpecs(),
			[]string{"upgrade", "upgradedIBCState"},
		),
		ibctm.NewConsensusState(
			time.Now().UTC(),
			commitmenttypes.NewMerkleRoot([]byte("counterparty-root")),
			bytes.Repeat([]byte{1}, 32),
		),
		signer,
	)
	if err != nil {
		t.Fatalf("new msg create client: %v", err)
	}

	txBuilder := chainApp.TxConfig.NewTxBuilder()
	if err := txBuilder.SetMsgs(msg); err != nil {
		t.Fatalf("set msg create client: %v", err)
	}
	txBytes, err := chainApp.TxConfig.TxEncoder()(txBuilder.GetTx())
	if err != nil {
		t.Fatalf("encode msg create client tx: %v", err)
	}
	simReqBz, err := proto.Marshal(&txtypes.SimulateRequest{TxBytes: txBytes})
	if err != nil {
		t.Fatalf("marshal simulate request: %v", err)
	}

	resp, err := abciApp.Query(context.Background(), &abcitypes.RequestQuery{
		Path: "/cosmos.tx.v1beta1.Service/Simulate",
		Data: simReqBz,
	})
	if err != nil {
		t.Fatalf("simulate create-client query: %v", err)
	}
	if resp.Code != 0 {
		if strings.Contains(resp.Log, "unknown query path") {
			t.Fatalf("expected tx simulate route to be registered, got %+v", resp)
		}
		if len(resp.Value) == 0 {
			t.Fatalf("expected tx simulate response payload or success code, got %+v", resp)
		}
	}
}
