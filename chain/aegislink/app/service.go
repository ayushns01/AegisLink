package app

import (
	"math/big"
	"strings"

	bridgekeeper "github.com/ayushns01/aegislink/chain/aegislink/x/bridge/keeper"
	bridgetypes "github.com/ayushns01/aegislink/chain/aegislink/x/bridge/types"
	ibcrouterkeeper "github.com/ayushns01/aegislink/chain/aegislink/x/ibcrouter/keeper"
)

type BridgeQueryService struct {
	app *App
}

func NewBridgeQueryService(app *App) *BridgeQueryService {
	return &BridgeQueryService{app: app}
}

func (s *BridgeQueryService) GetClaim(messageID string) (bridgekeeper.ClaimRecordSnapshot, bool) {
	messageID = strings.TrimSpace(messageID)
	if messageID == "" {
		return bridgekeeper.ClaimRecordSnapshot{}, false
	}

	for _, record := range s.app.BridgeKeeper.ExportState().ProcessedClaims {
		if record.MessageID == messageID {
			return record, true
		}
	}

	return bridgekeeper.ClaimRecordSnapshot{}, false
}

func (s *BridgeQueryService) ListWithdrawals(fromHeight, toHeight uint64) []bridgekeeper.WithdrawalRecord {
	return s.app.Withdrawals(fromHeight, toHeight)
}

func (s *BridgeQueryService) ActiveSignerSet() (bridgekeeper.SignerSet, error) {
	return s.app.ActiveSignerSet()
}

func (s *BridgeQueryService) GetSignerSet(version uint64) (bridgekeeper.SignerSet, bool) {
	return s.app.BridgeKeeper.SignerSet(version)
}

func (s *BridgeQueryService) ListSignerSets() []bridgekeeper.SignerSet {
	return s.app.SignerSets()
}

type BridgeTxService struct {
	app *App
}

func NewBridgeTxService(app *App) *BridgeTxService {
	return &BridgeTxService{app: app}
}

func (s *BridgeTxService) SubmitDepositClaim(claim bridgetypes.DepositClaim, attestation bridgetypes.Attestation) (bridgekeeper.ClaimResult, error) {
	return s.app.SubmitDepositClaim(claim, attestation)
}

func (s *BridgeTxService) ExecuteWithdrawal(assetID string, amount *big.Int, recipient string, deadline uint64, signature []byte) (bridgekeeper.WithdrawalRecord, error) {
	return s.app.ExecuteWithdrawal(assetID, amount, recipient, deadline, signature)
}

type IBCRouterQueryService struct {
	app *App
}

func NewIBCRouterQueryService(app *App) *IBCRouterQueryService {
	return &IBCRouterQueryService{app: app}
}

func (s *IBCRouterQueryService) ListRoutes() []ibcrouterkeeper.Route {
	return s.app.Routes()
}

func (s *IBCRouterQueryService) ListTransfers() []ibcrouterkeeper.TransferRecord {
	return s.app.Transfers()
}
