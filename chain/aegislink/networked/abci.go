package networked

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math/big"
	"sort"
	"strings"

	abcitypes "github.com/cometbft/cometbft/abci/types"

	aegisapp "github.com/ayushns01/aegislink/chain/aegislink/app"
	ibcrouterkeeper "github.com/ayushns01/aegislink/chain/aegislink/x/ibcrouter/keeper"
)

type ABCIApplication struct {
	abcitypes.BaseApplication
	appConfig aegisapp.Config
	app       *aegisapp.App
	appHash   []byte
}

func NewABCIApplication(appCfg aegisapp.Config, app *aegisapp.App) *ABCIApplication {
	abciApp := &ABCIApplication{
		BaseApplication: *abcitypes.NewBaseApplication(),
		appConfig:       appCfg,
		app:             app,
	}
	if app != nil {
		if hash, err := hashABCIStatus(app.Status()); err == nil {
			abciApp.appHash = hash
		}
	}
	return abciApp
}

func (a *ABCIApplication) Info(_ context.Context, _ *abcitypes.RequestInfo) (*abcitypes.ResponseInfo, error) {
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
	default:
		return &abcitypes.ResponseQuery{
			Code:   1,
			Log:    fmt.Sprintf("unknown query path %q", req.Path),
			Height: height,
		}, nil
	}
}

func (a *ABCIApplication) CheckTx(_ context.Context, req *abcitypes.RequestCheckTx) (*abcitypes.ResponseCheckTx, error) {
	if _, err := decodeDemoNodeTx(req.Tx); err != nil {
		return &abcitypes.ResponseCheckTx{
			Code: 1,
			Log:  err.Error(),
		}, nil
	}
	return &abcitypes.ResponseCheckTx{Code: 0}, nil
}

func (a *ABCIApplication) FinalizeBlock(_ context.Context, req *abcitypes.RequestFinalizeBlock) (*abcitypes.ResponseFinalizeBlock, error) {
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

	switch {
	case targetHeight == 0:
		a.app.AdvanceBlock()
	case targetHeight > current:
		for a.app.Status().CurrentHeight < targetHeight {
			a.app.AdvanceBlock()
		}
	}

	status := a.app.Status()
	hash, err := hashABCIStatus(status)
	if err != nil {
		return nil, err
	}
	a.appHash = hash

	return &abcitypes.ResponseFinalizeBlock{
		AppHash:   append([]byte(nil), hash...),
		TxResults: txResults,
	}, nil
}

func (a *ABCIApplication) Commit(_ context.Context, _ *abcitypes.RequestCommit) (*abcitypes.ResponseCommit, error) {
	if err := a.app.Save(); err != nil {
		return nil, err
	}
	return &abcitypes.ResponseCommit{
		RetainHeight: int64(a.app.Status().CurrentHeight),
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
	if strings.TrimSpace(payload.RouteID) != "" {
		return a.app.InitiateIBCTransferWithProfile(payload.RouteID, payload.AssetID, amount, payload.Receiver, payload.TimeoutHeight, payload.Memo)
	}
	return a.app.InitiateIBCTransfer(payload.AssetID, amount, payload.Receiver, payload.TimeoutHeight, payload.Memo)
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
