package cli

import (
	"encoding/base64"

	bridgev1 "github.com/ayushns01/aegislink/chain/aegislink/gen/go/aegislink/bridge/v1"
	bridgekeeper "github.com/ayushns01/aegislink/chain/aegislink/x/bridge/keeper"
)

func ClaimResponse(record bridgekeeper.ClaimRecordSnapshot) *bridgev1.ClaimResponse {
	return &bridgev1.ClaimResponse{
		ClaimKey:  record.ClaimKey,
		MessageId: record.MessageID,
		Denom:     record.Denom,
		AssetId:   record.AssetID,
		Amount:    record.Amount,
		Status:    string(record.Status),
	}
}

func WithdrawalsResponse(records []bridgekeeper.WithdrawalRecord) *bridgev1.WithdrawalsResponse {
	response := &bridgev1.WithdrawalsResponse{
		Withdrawals: make([]*bridgev1.Withdrawal, 0, len(records)),
	}
	for _, record := range records {
		response.Withdrawals = append(response.Withdrawals, &bridgev1.Withdrawal{
			Kind:            string(record.Identity.Kind),
			SourceChainId:   record.Identity.SourceChainID,
			SourceContract:  record.Identity.SourceContract,
			SourceTxHash:    record.Identity.SourceTxHash,
			SourceLogIndex:  record.Identity.SourceLogIndex,
			Nonce:           record.Identity.Nonce,
			MessageId:       record.Identity.MessageID,
			AssetId:         record.AssetID,
			AssetAddress:    record.AssetAddress,
			Amount:          record.Amount.String(),
			Recipient:       record.Recipient,
			Deadline:        record.Deadline,
			BlockHeight:     record.BlockHeight,
			SignatureBase64: base64.StdEncoding.EncodeToString(record.Signature),
		})
	}
	return response
}
