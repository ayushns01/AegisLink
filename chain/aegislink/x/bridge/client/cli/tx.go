package cli

import (
	"encoding/base64"

	bridgev1 "github.com/ayushns01/aegislink/chain/aegislink/gen/go/aegislink/bridge/v1"
	bridgekeeper "github.com/ayushns01/aegislink/chain/aegislink/x/bridge/keeper"
)

func SubmitDepositClaimResponse(result bridgekeeper.ClaimResult) *bridgev1.SubmitDepositClaimResponse {
	amount := ""
	if result.Amount != nil {
		amount = result.Amount.String()
	}
	return &bridgev1.SubmitDepositClaimResponse{
		Status:    string(result.Status),
		MessageId: result.MessageID,
		Denom:     result.Denom,
		Amount:    amount,
	}
}

func ExecuteWithdrawalResponse(record bridgekeeper.WithdrawalRecord) *bridgev1.ExecuteWithdrawalResponse {
	return &bridgev1.ExecuteWithdrawalResponse{
		MessageId:       record.Identity.MessageID,
		AssetId:         record.AssetID,
		AssetAddress:    record.AssetAddress,
		Amount:          record.Amount.String(),
		Recipient:       record.Recipient,
		Deadline:        record.Deadline,
		BlockHeight:     record.BlockHeight,
		SignatureBase64: base64.StdEncoding.EncodeToString(record.Signature),
	}
}
