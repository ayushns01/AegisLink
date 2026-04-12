package networked

import (
	"context"
	"encoding/hex"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math/big"
	"sort"
	"strings"

	sdkmath "cosmossdk.io/math"
	bridgecli "github.com/ayushns01/aegislink/chain/aegislink/x/bridge/client/cli"
	abcitypes "github.com/cometbft/cometbft/abci/types"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	proto "github.com/cosmos/gogoproto/proto"
	clienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types"
	transfertypes "github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"

	aegisapp "github.com/ayushns01/aegislink/chain/aegislink/app"
	ibcrouterkeeper "github.com/ayushns01/aegislink/chain/aegislink/x/ibcrouter/keeper"
)

type ABCIApplication struct {
	abcitypes.BaseApplication
	appConfig               aegisapp.Config
	app                     *aegisapp.App
	chainApp                *ChainApp
	appHash                 []byte
	lastFinalizeUsedBaseApp bool
	pendingBaseAppCommit    bool
}

func NewABCIApplication(appCfg aegisapp.Config, app *aegisapp.App, chainApp *ChainApp) *ABCIApplication {
	abciApp := &ABCIApplication{
		BaseApplication: *abcitypes.NewBaseApplication(),
		appConfig:       appCfg,
		app:             app,
		chainApp:        chainApp,
	}
	if app != nil {
		if hash, err := hashABCIStatus(app.Status()); err == nil {
			abciApp.appHash = hash
		}
	}
	return abciApp
}

func (a *ABCIApplication) Info(_ context.Context, _ *abcitypes.RequestInfo) (*abcitypes.ResponseInfo, error) {
	if a.chainApp != nil && a.chainApp.BaseApp != nil {
		return a.chainApp.BaseApp.Info(&abcitypes.RequestInfo{})
	}
	status := a.app.Status()
	return &abcitypes.ResponseInfo{
		Data:             a.appConfig.AppName,
		Version:          "aegislink-demo-abci",
		AppVersion:       1,
		LastBlockHeight:  int64(status.CurrentHeight),
		LastBlockAppHash: append([]byte(nil), a.appHash...),
	}, nil
}

func (a *ABCIApplication) Query(_ context.Context, req *abcitypes.RequestQuery) (*abcitypes.ResponseQuery, error) {
	status := a.app.Status()
	height := int64(status.CurrentHeight)

	switch strings.TrimSpace(req.Path) {
	case "/status":
		encoded, err := json.Marshal(status)
		if err != nil {
			return nil, err
		}
		return &abcitypes.ResponseQuery{
			Height: height,
			Value:  encoded,
		}, nil
	case "/summary":
		encoded, err := json.Marshal(summaryViewFromApp(a.app))
		if err != nil {
			return nil, err
		}
		return &abcitypes.ResponseQuery{
			Height: height,
			Value:  encoded,
		}, nil
	case "/balances":
		address := strings.TrimSpace(string(req.Data))
		if address == "" {
			return &abcitypes.ResponseQuery{
				Code:   1,
				Log:    "missing balance query address",
				Height: height,
			}, nil
		}
		balances, err := a.app.WalletBalances(address)
		if err != nil {
			return &abcitypes.ResponseQuery{
				Code:   1,
				Log:    err.Error(),
				Height: height,
			}, nil
		}
		encoded, err := json.Marshal(balances)
		if err != nil {
			return nil, err
		}
		return &abcitypes.ResponseQuery{
			Height: height,
			Value:  encoded,
		}, nil
	case "/transfers":
		transfers := a.app.Transfers()
		sort.Slice(transfers, func(i, j int) bool {
			return transfers[i].TransferID < transfers[j].TransferID
		})
		encoded, err := json.Marshal(transferViewsFromRecords(transfers))
		if err != nil {
			return nil, err
		}
		return &abcitypes.ResponseQuery{
			Height: height,
			Value:  encoded,
		}, nil
	case "/withdrawals":
		var query struct {
			FromHeight uint64 `json:"from_height"`
			ToHeight   uint64 `json:"to_height"`
		}
		if len(req.Data) > 0 {
			if err := json.Unmarshal(req.Data, &query); err != nil {
				return &abcitypes.ResponseQuery{
					Code:   1,
					Log:    fmt.Sprintf("decode withdrawals query: %v", err),
					Height: height,
				}, nil
			}
		}
		withdrawals := aegisapp.NewBridgeQueryService(a.app).ListWithdrawals(query.FromHeight, query.ToHeight)
		encoded, err := json.Marshal(bridgecli.WithdrawalsResponse(withdrawals).Withdrawals)
		if err != nil {
			return nil, err
		}
		return &abcitypes.ResponseQuery{
			Height: height,
			Value:  encoded,
		}, nil
	case "/claim":
		messageID := strings.TrimSpace(string(req.Data))
		if messageID == "" {
			return &abcitypes.ResponseQuery{
				Code:   1,
				Log:    "missing claim query message id",
				Height: height,
			}, nil
		}
		record, ok := aegisapp.NewBridgeQueryService(a.app).GetClaim(messageID)
		if !ok {
			return &abcitypes.ResponseQuery{
				Code:   1,
				Log:    "claim not found",
				Height: height,
			}, nil
		}
		encoded, err := json.Marshal(record)
		if err != nil {
			return nil, err
		}
		return &abcitypes.ResponseQuery{
			Height: height,
			Value:  encoded,
		}, nil
	case "/ibc.applications.transfer.v1.Query/DenomTraces":
		return a.queryLegacyTransferDenomTraces(height, req)
	case "/ibc.applications.transfer.v1.Query/DenomTrace":
		return a.queryLegacyTransferDenomTrace(height, req)
	default:
		return a.queryBaseApp(height, req)
	}
}

func (a *ABCIApplication) CheckTx(_ context.Context, req *abcitypes.RequestCheckTx) (*abcitypes.ResponseCheckTx, error) {
	if _, err := decodeDemoNodeTx(req.Tx); err == nil {
		return &abcitypes.ResponseCheckTx{Code: 0}, nil
	}
	if a.chainApp != nil && a.chainApp.BaseApp != nil {
		return a.chainApp.BaseApp.CheckTx(req)
	}
	return &abcitypes.ResponseCheckTx{
		Code: 1,
		Log:  "unsupported tx format",
	}, nil
}

func (a *ABCIApplication) FinalizeBlock(_ context.Context, req *abcitypes.RequestFinalizeBlock) (*abcitypes.ResponseFinalizeBlock, error) {
	hasDemoTx, hasSDKTx := false, false
	for _, tx := range req.Txs {
		if _, err := decodeDemoNodeTx(tx); err == nil {
			hasDemoTx = true
			continue
		}
		hasSDKTx = true
	}
	if hasDemoTx && hasSDKTx {
		return &abcitypes.ResponseFinalizeBlock{
			TxResults: []*abcitypes.ExecTxResult{{
				Code: 1,
				Log:  "mixed demo-node and sdk txs are not supported in the same block",
			}},
		}, nil
	}
	if !hasDemoTx && a.chainApp != nil && a.chainApp.BaseApp != nil {
		resp, err := a.chainApp.BaseApp.FinalizeBlock(req)
		if err != nil {
			return nil, err
		}
		a.lastFinalizeUsedBaseApp = true
		a.appHash = append([]byte(nil), resp.AppHash...)
		if a.app != nil && req.Height > 0 {
			a.app.SetCurrentHeight(uint64(req.Height))
		}
		return resp, nil
	}

	a.lastFinalizeUsedBaseApp = false
	a.pendingBaseAppCommit = false
	targetHeight := uint64(req.Height)
	current := a.app.Status().CurrentHeight

	txResults := make([]*abcitypes.ExecTxResult, 0, len(req.Txs))
	for _, tx := range req.Txs {
		result, err := a.applyDemoNodeTx(tx)
		if err != nil {
			txResults = append(txResults, &abcitypes.ExecTxResult{
				Code: 1,
				Log:  err.Error(),
			})
			continue
		}
		txResults = append(txResults, result)
	}

	if a.chainApp != nil && a.chainApp.BaseApp != nil {
		nextBaseAppHeight := a.chainApp.BaseApp.LastBlockHeight() + 1
		if req.Height > 0 && nextBaseAppHeight == req.Height {
			resp, err := a.chainApp.BaseApp.FinalizeBlock(&abcitypes.RequestFinalizeBlock{
				Height: req.Height,
				Hash:   req.Hash,
				Time:   req.Time,
			})
			if err != nil {
				return nil, err
			}
			a.lastFinalizeUsedBaseApp = true
			a.pendingBaseAppCommit = true
			if len(resp.AppHash) > 0 {
				a.appHash = append([]byte(nil), resp.AppHash...)
			}
		}
	}

	switch {
	case targetHeight == 0:
		if err := a.advanceAppAndSyncChain(); err != nil {
			return nil, err
		}
	case targetHeight > current:
		for a.app.Status().CurrentHeight < targetHeight {
			if err := a.advanceAppAndSyncChain(); err != nil {
				return nil, err
			}
		}
	}

	status := a.app.Status()
	hash, err := hashABCIStatus(status)
	if err != nil {
		return nil, err
	}
	if !a.pendingBaseAppCommit {
		a.appHash = hash
	}

	return &abcitypes.ResponseFinalizeBlock{
		AppHash:   append([]byte(nil), a.appHash...),
		TxResults: txResults,
	}, nil
}

func (a *ABCIApplication) advanceAppAndSyncChain() error {
	progress := a.app.AdvanceBlock()
	if a.chainApp == nil {
		return nil
	}
	for _, balance := range progress.AppliedClaimBalances {
		amount, ok := new(big.Int).SetString(strings.TrimSpace(balance.Amount), 10)
		if !ok {
			return fmt.Errorf("invalid applied claim balance amount %q", balance.Amount)
		}
		if err := a.chainApp.SyncAccountBalance(balance.Address, sdk.NewCoin(balance.Denom, sdkmath.NewIntFromBigInt(amount))); err != nil {
			return fmt.Errorf("sync claim balance for %s/%s: %w", balance.Address, balance.Denom, err)
		}
	}
	return nil
}

func (a *ABCIApplication) Commit(_ context.Context, _ *abcitypes.RequestCommit) (*abcitypes.ResponseCommit, error) {
	if err := a.app.Save(); err != nil {
		return nil, err
	}
	if a.lastFinalizeUsedBaseApp && a.chainApp != nil && a.chainApp.BaseApp != nil {
		resp, err := a.chainApp.BaseApp.Commit()
		if err != nil {
			return nil, err
		}
		a.lastFinalizeUsedBaseApp = false
		a.pendingBaseAppCommit = false
		a.appHash = append([]byte(nil), a.chainApp.BaseApp.LastCommitID().Hash...)
		return resp, nil
	}
	return &abcitypes.ResponseCommit{
		RetainHeight: 0,
	}, nil
}

func hashABCIStatus(status aegisapp.Status) ([]byte, error) {
	encoded, err := json.Marshal(status)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(encoded)
	return sum[:], nil
}

func (a *ABCIApplication) queryBaseApp(height int64, req *abcitypes.RequestQuery) (*abcitypes.ResponseQuery, error) {
	if a.chainApp == nil || a.chainApp.BaseApp == nil {
		return &abcitypes.ResponseQuery{
			Code:   1,
			Log:    fmt.Sprintf("unknown query path %q", req.Path),
			Height: height,
		}, nil
	}

	queryReq := *req
	if queryReq.Height == 0 {
		queryReq.Height = height
	}
	if grpcHandler := a.chainApp.BaseApp.GRPCQueryRouter().Route(queryReq.Path); grpcHandler != nil {
		resp, err := grpcHandler(
			a.chainApp.BaseApp.NewUncachedContext(false, cmtproto.Header{
				ChainID: a.appConfig.ChainID,
				Height:  queryReq.Height,
			}),
			&queryReq,
		)
		if err != nil {
			return &abcitypes.ResponseQuery{
				Code:   1,
				Log:    err.Error(),
				Height: queryReq.Height,
			}, nil
		}
		return resp, nil
	}
	resp, err := a.chainApp.BaseApp.Query(context.Background(), &queryReq)
	if err != nil {
		return &abcitypes.ResponseQuery{
			Code:   1,
			Log:    err.Error(),
			Height: queryReq.Height,
		}, nil
	}
	return resp, nil
}

func (a *ABCIApplication) queryLegacyTransferDenomTraces(height int64, req *abcitypes.RequestQuery) (*abcitypes.ResponseQuery, error) {
	var legacyReq legacyQueryDenomTracesRequest
	if len(req.Data) > 0 {
		if err := proto.Unmarshal(req.Data, &legacyReq); err != nil {
			return &abcitypes.ResponseQuery{
				Code:   1,
				Log:    fmt.Sprintf("decode legacy denom traces query: %v", err),
				Height: height,
			}, nil
		}
	}

	modernReqBz, err := proto.Marshal(&transfertypes.QueryDenomsRequest{
		Pagination: legacyReq.Pagination,
	})
	if err != nil {
		return nil, err
	}
	resp, err := a.queryBaseApp(height, &abcitypes.RequestQuery{
		Path:   "/ibc.applications.transfer.v1.Query/Denoms",
		Data:   modernReqBz,
		Height: req.Height,
		Prove:  req.Prove,
	})
	if err != nil || resp.Code != 0 {
		return resp, err
	}

	var modernResp transfertypes.QueryDenomsResponse
	if err := proto.Unmarshal(resp.Value, &modernResp); err != nil {
		return nil, err
	}
	legacyResp := legacyQueryDenomTracesResponse{
		Pagination: modernResp.Pagination,
	}
	for _, denom := range modernResp.Denoms {
		legacyResp.DenomTraces = append(legacyResp.DenomTraces, legacyTransferTraceFromDenom(denom))
	}
	encoded, err := proto.Marshal(&legacyResp)
	if err != nil {
		return nil, err
	}
	return &abcitypes.ResponseQuery{
		Code:   resp.Code,
		Log:    resp.Log,
		Info:   resp.Info,
		Index:  resp.Index,
		Key:    resp.Key,
		Value:  encoded,
		ProofOps: resp.ProofOps,
		Height: resp.Height,
		Codespace: resp.Codespace,
	}, nil
}

func (a *ABCIApplication) queryLegacyTransferDenomTrace(height int64, req *abcitypes.RequestQuery) (*abcitypes.ResponseQuery, error) {
	var legacyReq legacyQueryDenomTraceRequest
	if len(req.Data) > 0 {
		if err := proto.Unmarshal(req.Data, &legacyReq); err != nil {
			return &abcitypes.ResponseQuery{
				Code:   1,
				Log:    fmt.Sprintf("decode legacy denom trace query: %v", err),
				Height: height,
			}, nil
		}
	}

	if legacyReq.Hash != "" && !strings.HasPrefix(legacyReq.Hash, "ibc/") {
		if _, err := hex.DecodeString(legacyReq.Hash); err != nil {
			legacyResp := legacyQueryDenomTraceResponse{
				DenomTrace: legacyTransferTraceFromDenom(transfertypes.NewDenom(legacyReq.Hash)),
			}
			encoded, marshalErr := proto.Marshal(&legacyResp)
			if marshalErr != nil {
				return nil, marshalErr
			}
			return &abcitypes.ResponseQuery{
				Height: height,
				Value:  encoded,
			}, nil
		}
	}

	modernReqBz, err := proto.Marshal(&transfertypes.QueryDenomRequest{Hash: legacyReq.Hash})
	if err != nil {
		return nil, err
	}
	resp, err := a.queryBaseApp(height, &abcitypes.RequestQuery{
		Path:   "/ibc.applications.transfer.v1.Query/Denom",
		Data:   modernReqBz,
		Height: req.Height,
		Prove:  req.Prove,
	})
	if err != nil || resp.Code != 0 {
		return resp, err
	}

	var modernResp transfertypes.QueryDenomResponse
	if err := proto.Unmarshal(resp.Value, &modernResp); err != nil {
		return nil, err
	}
	legacyResp := legacyQueryDenomTraceResponse{}
	if modernResp.Denom != nil {
		legacyResp.DenomTrace = legacyTransferTraceFromDenom(*modernResp.Denom)
	}
	encoded, err := proto.Marshal(&legacyResp)
	if err != nil {
		return nil, err
	}
	return &abcitypes.ResponseQuery{
		Code:   resp.Code,
		Log:    resp.Log,
		Info:   resp.Info,
		Index:  resp.Index,
		Key:    resp.Key,
		Value:  encoded,
		ProofOps: resp.ProofOps,
		Height: resp.Height,
		Codespace: resp.Codespace,
	}, nil
}

func (a *ABCIApplication) applyDemoNodeTx(txBytes []byte) (*abcitypes.ExecTxResult, error) {
	tx, err := decodeDemoNodeTx(txBytes)
	if err != nil {
		return nil, err
	}

	var result any
	switch tx.Type {
	case "queue_deposit_claim":
		claim, attestation, err := depositClaimAndAttestationFromPayload(*tx.QueueDepositClaim)
		if err != nil {
			return nil, err
		}
		if err := a.app.QueueDepositClaim(claim, attestation); err != nil {
			return nil, err
		}
		result = map[string]any{
			"status":     "queued",
			"message_id": claim.Identity.MessageID,
			"asset_id":   claim.AssetID,
			"amount":     claim.Amount.String(),
		}
	case "initiate_ibc_transfer":
		payload, amount, err := decodeInitiateIBCTransferPayload(*tx.InitiateIBCTransfer)
		if err != nil {
			return nil, err
		}
		transfer, err := a.applyIBCTransferPayload(payload, amount)
		if err != nil {
			return nil, err
		}
		result = transferJSONResponse(transfer)
	case "execute_withdrawal":
		payload, amount, signature, err := decodeExecuteWithdrawalPayload(*tx.ExecuteWithdrawal)
		if err != nil {
			return nil, err
		}
		if payload.Height > 0 {
			a.app.SetCurrentHeight(payload.Height)
		}
		withdrawal, err := a.app.ExecuteWithdrawal(payload.OwnerAddress, payload.AssetID, amount, payload.Recipient, payload.Deadline, signature)
		if err != nil {
			return nil, err
		}
		result = bridgecli.ExecuteWithdrawalResponse(withdrawal)
	case "fund_account":
		if a.chainApp == nil {
			return nil, fmt.Errorf("fund account requires chain app")
		}
		payload, amount, err := decodeFundAccountPayload(*tx.FundAccount)
		if err != nil {
			return nil, err
		}
		if err := a.chainApp.FundAccount(payload.Address, sdk.NewCoin(payload.Denom, sdkmath.NewIntFromBigInt(amount))); err != nil {
			return nil, err
		}
		result = FundAccountResult{
			Address: payload.Address,
			Denom:   payload.Denom,
			Amount:  amount.String(),
		}
	default:
		return nil, fmt.Errorf("unsupported demo node tx type %q", tx.Type)
	}

	encoded, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	return &abcitypes.ExecTxResult{
		Code: 0,
		Data: encoded,
	}, nil
}

func (a *ABCIApplication) applyIBCTransferPayload(payload InitiateIBCTransferPayload, amount *big.Int) (ibcrouterkeeper.TransferRecord, error) {
	var (
		transfer ibcrouterkeeper.TransferRecord
		err      error
	)
	sender := strings.TrimSpace(payload.Sender)
	asset, ok := a.app.RegistryKeeper.GetAsset(payload.AssetID)
	if !ok {
		return ibcrouterkeeper.TransferRecord{}, fmt.Errorf("networked ibc asset %q is not registered", payload.AssetID)
	}
	preTransferBalance := a.app.WalletBalance(sender, asset.Denom)
	if sender != "" {
		if preTransferBalance.Cmp(amount) < 0 {
			return ibcrouterkeeper.TransferRecord{}, fmt.Errorf("wallet %s has insufficient %s balance", sender, asset.Denom)
		}
		if a.chainApp != nil {
			if err := a.chainApp.SyncAccountBalance(sender, sdk.NewCoin(asset.Denom, sdkmath.NewIntFromBigInt(preTransferBalance))); err != nil {
				return ibcrouterkeeper.TransferRecord{}, err
			}
		}
		if err := a.app.DebitWallet(sender, asset.Denom, amount); err != nil {
			return ibcrouterkeeper.TransferRecord{}, err
		}
	}
	if strings.TrimSpace(payload.RouteID) != "" {
		transfer, err = a.app.InitiateIBCTransferWithProfile(payload.RouteID, payload.AssetID, amount, payload.Receiver, payload.TimeoutHeight, payload.Memo)
	} else {
		transfer, err = a.app.InitiateIBCTransfer(payload.AssetID, amount, payload.Receiver, payload.TimeoutHeight, payload.Memo)
	}
	if err != nil {
		if sender != "" {
			_ = a.app.CreditWallet(sender, asset.Denom, amount)
			if a.chainApp != nil {
				_ = a.chainApp.SyncAccountBalance(sender, sdk.NewCoin(asset.Denom, sdkmath.NewIntFromBigInt(preTransferBalance)))
			}
		}
		return ibcrouterkeeper.TransferRecord{}, err
	}
	if a.chainApp == nil {
		return transfer, nil
	}
	_, err = a.chainApp.ExecuteLocalhostTransfer(LocalhostTransferRequest{
		Sender:        sender,
		Coin:          sdk.NewCoin(asset.Denom, sdkmath.NewIntFromBigInt(amount)),
		Receiver:      payload.Receiver,
		TimeoutHeight: clienttypes.NewHeight(1, payload.TimeoutHeight),
		Memo:          payload.Memo,
	})
	if err != nil {
		if sender != "" {
			_ = a.app.CreditWallet(sender, asset.Denom, amount)
			_ = a.chainApp.SyncAccountBalance(sender, sdk.NewCoin(asset.Denom, sdkmath.NewIntFromBigInt(preTransferBalance)))
		}
		if _, failErr := a.app.FailIBCTransfer(transfer.TransferID, "localhost sdk transfer failed: "+err.Error()); failErr != nil {
			return ibcrouterkeeper.TransferRecord{}, fmt.Errorf("execute localhost transfer: %w (also failed to mark transfer failed: %v)", err, failErr)
		}
		return ibcrouterkeeper.TransferRecord{}, err
	}
	return transfer, nil
}

func transferViewsFromRecords(transfers []ibcrouterkeeper.TransferRecord) []TransferView {
	items := make([]TransferView, 0, len(transfers))
	for _, transfer := range transfers {
		items = append(items, TransferView{
			TransferID:         transfer.TransferID,
			AssetID:            transfer.AssetID,
			Amount:             transfer.Amount.String(),
			Receiver:           transfer.Receiver,
			DestinationChainID: transfer.DestinationChainID,
			ChannelID:          transfer.ChannelID,
			DestinationDenom:   transfer.DestinationDenom,
			TimeoutHeight:      transfer.TimeoutHeight,
			Memo:               transfer.Memo,
			Status:             string(transfer.Status),
			FailureReason:      transfer.FailureReason,
		})
	}
	return items
}

func summaryViewFromApp(app *aegisapp.App) SummaryView {
	bridgeState := app.BridgeKeeper.ExportState()
	return SummaryView{
		AppName:       app.Config.AppName,
		Modules:       app.ModuleNames(),
		Assets:        len(app.RegistryKeeper.ExportAssets()),
		Limits:        len(app.LimitsKeeper.ExportLimits()),
		PausedFlows:   len(app.PauserKeeper.ExportPausedFlows()),
		CurrentHeight: bridgeState.CurrentHeight,
		Withdrawals:   len(bridgeState.Withdrawals),
		SupplyByDenom: bridgeState.SupplyByDenom,
	}
}
