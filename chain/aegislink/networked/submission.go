package networked

import (
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"

	bridgetypes "github.com/ayushns01/aegislink/chain/aegislink/x/bridge/types"
)

type submissionPayload struct {
	Claim struct {
		Kind               string `json:"kind"`
		SourceAssetKind    string `json:"source_asset_kind,omitempty"`
		SourceChainID      string `json:"source_chain_id"`
		SourceContract     string `json:"source_contract"`
		SourceTxHash       string `json:"source_tx_hash"`
		SourceLogIndex     uint64 `json:"source_log_index"`
		Nonce              uint64 `json:"nonce"`
		MessageID          string `json:"message_id"`
		DestinationChainID string `json:"destination_chain_id"`
		AssetID            string `json:"asset_id"`
		Amount             string `json:"amount"`
		Recipient          string `json:"recipient"`
		Deadline           uint64 `json:"deadline"`
	} `json:"claim"`
	Attestation struct {
		MessageID        string                         `json:"message_id"`
		PayloadHash      string                         `json:"payload_hash"`
		Signers          []string                       `json:"signers"`
		Proofs           []bridgetypes.AttestationProof `json:"proofs"`
		Threshold        uint32                         `json:"threshold"`
		Expiry           uint64                         `json:"expiry"`
		SignerSetVersion uint64                         `json:"signer_set_version"`
	} `json:"attestation"`
}

func decodeSubmission(r *http.Request) (bridgetypes.DepositClaim, bridgetypes.Attestation, error) {
	var payload submissionPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		return bridgetypes.DepositClaim{}, bridgetypes.Attestation{}, err
	}

	amount, ok := new(big.Int).SetString(payload.Claim.Amount, 10)
	if !ok {
		return bridgetypes.DepositClaim{}, bridgetypes.Attestation{}, fmt.Errorf("invalid claim amount %q", payload.Claim.Amount)
	}

	claim := bridgetypes.DepositClaim{
		Identity: bridgetypes.ClaimIdentity{
			Kind:            bridgetypes.ClaimKind(payload.Claim.Kind),
			SourceAssetKind: payload.Claim.SourceAssetKind,
			SourceChainID:   payload.Claim.SourceChainID,
			SourceContract:  payload.Claim.SourceContract,
			SourceTxHash:    payload.Claim.SourceTxHash,
			SourceLogIndex:  payload.Claim.SourceLogIndex,
			Nonce:           payload.Claim.Nonce,
			MessageID:       payload.Claim.MessageID,
		},
		DestinationChainID: payload.Claim.DestinationChainID,
		AssetID:            payload.Claim.AssetID,
		Amount:             amount,
		Recipient:          payload.Claim.Recipient,
		Deadline:           payload.Claim.Deadline,
	}
	attestation := bridgetypes.Attestation{
		MessageID:        payload.Attestation.MessageID,
		PayloadHash:      payload.Attestation.PayloadHash,
		Signers:          append([]string(nil), payload.Attestation.Signers...),
		Proofs:           append([]bridgetypes.AttestationProof(nil), payload.Attestation.Proofs...),
		Threshold:        payload.Attestation.Threshold,
		Expiry:           payload.Attestation.Expiry,
		SignerSetVersion: payload.Attestation.SignerSetVersion,
	}
	return claim, attestation, nil
}
