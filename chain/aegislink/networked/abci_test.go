package networked

import (
	"context"
	"path/filepath"
	"testing"

	aegisapp "github.com/ayushns01/aegislink/chain/aegislink/app"
	abcitypes "github.com/cometbft/cometbft/abci/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	proto "github.com/cosmos/gogoproto/proto"
	transfertypes "github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"
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
}
