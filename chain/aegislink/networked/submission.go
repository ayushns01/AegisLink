package networked

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"time"

	bridgekeeper "github.com/ayushns01/aegislink/chain/aegislink/x/bridge/keeper"
	bridgetypes "github.com/ayushns01/aegislink/chain/aegislink/x/bridge/types"
	rpchttp "github.com/cometbft/cometbft/rpc/client/http"
	cmttypes "github.com/cometbft/cometbft/types"
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

type InitiateIBCTransferPayload struct {
	Sender        string `json:"sender,omitempty"`
	RouteID       string `json:"route_id,omitempty"`
	AssetID       string `json:"asset_id"`
	Amount        string `json:"amount"`
	Receiver      string `json:"receiver"`
	TimeoutHeight uint64 `json:"timeout_height"`
	Memo          string `json:"memo,omitempty"`
}

type ExecuteWithdrawalPayload struct {
	OwnerAddress    string `json:"owner_address"`
	AssetID         string `json:"asset_id"`
	Amount          string `json:"amount"`
	Recipient       string `json:"recipient"`
	Deadline        uint64 `json:"deadline"`
	SignatureBase64 string `json:"signature_base64"`
	Height          uint64 `json:"height,omitempty"`
}

type FundAccountPayload struct {
	Address string `json:"address"`
	Denom   string `json:"denom"`
	Amount  string `json:"amount"`
}

type demoNodeTx struct {
	Type                string                      `json:"type"`
	QueueDepositClaim   *submissionPayload          `json:"queue_deposit_claim,omitempty"`
	InitiateIBCTransfer *InitiateIBCTransferPayload `json:"initiate_ibc_transfer,omitempty"`
	ExecuteWithdrawal   *ExecuteWithdrawalPayload   `json:"execute_withdrawal,omitempty"`
	FundAccount         *FundAccountPayload         `json:"fund_account,omitempty"`
}

func decodeSubmission(r *http.Request) (bridgetypes.DepositClaim, bridgetypes.Attestation, error) {
	var payload submissionPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		return bridgetypes.DepositClaim{}, bridgetypes.Attestation{}, err
	}
	return depositClaimAndAttestationFromPayload(payload)
}

func depositClaimAndAttestationFromPayload(payload submissionPayload) (bridgetypes.DepositClaim, bridgetypes.Attestation, error) {

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

func decodeInitiateIBCTransferPayload(payload InitiateIBCTransferPayload) (InitiateIBCTransferPayload, *big.Int, error) {
	payload.Sender = strings.TrimSpace(payload.Sender)
	if strings.TrimSpace(payload.AssetID) == "" {
		return InitiateIBCTransferPayload{}, nil, fmt.Errorf("missing asset id")
	}
	if strings.TrimSpace(payload.Receiver) == "" {
		return InitiateIBCTransferPayload{}, nil, fmt.Errorf("missing receiver")
	}
	if payload.TimeoutHeight == 0 {
		return InitiateIBCTransferPayload{}, nil, fmt.Errorf("missing timeout height")
	}
	amount, ok := new(big.Int).SetString(strings.TrimSpace(payload.Amount), 10)
	if !ok {
		return InitiateIBCTransferPayload{}, nil, fmt.Errorf("invalid amount %q", payload.Amount)
	}
	payload.Amount = amount.String()
	return payload, amount, nil
}

func decodeExecuteWithdrawalPayload(payload ExecuteWithdrawalPayload) (ExecuteWithdrawalPayload, *big.Int, []byte, error) {
	if strings.TrimSpace(payload.OwnerAddress) == "" {
		return ExecuteWithdrawalPayload{}, nil, nil, fmt.Errorf("missing owner address")
	}
	if strings.TrimSpace(payload.AssetID) == "" {
		return ExecuteWithdrawalPayload{}, nil, nil, fmt.Errorf("missing asset id")
	}
	if strings.TrimSpace(payload.Amount) == "" {
		return ExecuteWithdrawalPayload{}, nil, nil, fmt.Errorf("missing amount")
	}
	if strings.TrimSpace(payload.Recipient) == "" {
		return ExecuteWithdrawalPayload{}, nil, nil, fmt.Errorf("missing recipient")
	}
	if payload.Deadline == 0 {
		return ExecuteWithdrawalPayload{}, nil, nil, fmt.Errorf("missing deadline")
	}
	if strings.TrimSpace(payload.SignatureBase64) == "" {
		return ExecuteWithdrawalPayload{}, nil, nil, fmt.Errorf("missing signature")
	}

	amount, ok := new(big.Int).SetString(strings.TrimSpace(payload.Amount), 10)
	if !ok {
		return ExecuteWithdrawalPayload{}, nil, nil, fmt.Errorf("invalid amount %q", payload.Amount)
	}
	signature, err := base64.StdEncoding.DecodeString(strings.TrimSpace(payload.SignatureBase64))
	if err != nil {
		return ExecuteWithdrawalPayload{}, nil, nil, fmt.Errorf("decode signature: %w", err)
	}
	payload.Amount = amount.String()
	payload.SignatureBase64 = strings.TrimSpace(payload.SignatureBase64)
	return payload, amount, signature, nil
}

func decodeFundAccountPayload(payload FundAccountPayload) (FundAccountPayload, *big.Int, error) {
	if strings.TrimSpace(payload.Address) == "" {
		return FundAccountPayload{}, nil, fmt.Errorf("missing funded account address")
	}
	if strings.TrimSpace(payload.Denom) == "" {
		return FundAccountPayload{}, nil, fmt.Errorf("missing funded account denom")
	}
	amount, ok := new(big.Int).SetString(strings.TrimSpace(payload.Amount), 10)
	if !ok {
		return FundAccountPayload{}, nil, fmt.Errorf("invalid funded account amount %q", payload.Amount)
	}
	if amount.Sign() <= 0 {
		return FundAccountPayload{}, nil, fmt.Errorf("funded account amount must be positive")
	}
	payload.Address = strings.TrimSpace(payload.Address)
	payload.Denom = strings.TrimSpace(payload.Denom)
	payload.Amount = amount.String()
	return payload, amount, nil
}

func encodeQueueDepositClaimTx(claim bridgetypes.DepositClaim, attestation bridgetypes.Attestation) ([]byte, error) {
	payload := submissionPayload{}
	payload.Claim.Kind = string(claim.Identity.Kind)
	payload.Claim.SourceAssetKind = claim.Identity.SourceAssetKind
	payload.Claim.SourceChainID = claim.Identity.SourceChainID
	payload.Claim.SourceContract = claim.Identity.SourceContract
	payload.Claim.SourceTxHash = claim.Identity.SourceTxHash
	payload.Claim.SourceLogIndex = claim.Identity.SourceLogIndex
	payload.Claim.Nonce = claim.Identity.Nonce
	payload.Claim.MessageID = claim.Identity.MessageID
	payload.Claim.DestinationChainID = claim.DestinationChainID
	payload.Claim.AssetID = claim.AssetID
	payload.Claim.Amount = claim.Amount.String()
	payload.Claim.Recipient = claim.Recipient
	payload.Claim.Deadline = claim.Deadline

	payload.Attestation.MessageID = attestation.MessageID
	payload.Attestation.PayloadHash = attestation.PayloadHash
	payload.Attestation.Signers = append([]string(nil), attestation.Signers...)
	payload.Attestation.Proofs = append([]bridgetypes.AttestationProof(nil), attestation.Proofs...)
	payload.Attestation.Threshold = attestation.Threshold
	payload.Attestation.Expiry = attestation.Expiry
	payload.Attestation.SignerSetVersion = attestation.SignerSetVersion

	return json.Marshal(demoNodeTx{
		Type:              "queue_deposit_claim",
		QueueDepositClaim: &payload,
	})
}

func encodeInitiateIBCTransferTx(payload InitiateIBCTransferPayload) ([]byte, error) {
	payload, _, err := decodeInitiateIBCTransferPayload(payload)
	if err != nil {
		return nil, err
	}
	return json.Marshal(demoNodeTx{
		Type:                "initiate_ibc_transfer",
		InitiateIBCTransfer: &payload,
	})
}

func encodeExecuteWithdrawalTx(payload ExecuteWithdrawalPayload) ([]byte, error) {
	payload, _, _, err := decodeExecuteWithdrawalPayload(payload)
	if err != nil {
		return nil, err
	}
	return json.Marshal(demoNodeTx{
		Type:              "execute_withdrawal",
		ExecuteWithdrawal: &payload,
	})
}

func encodeFundAccountTx(payload FundAccountPayload) ([]byte, error) {
	payload, _, err := decodeFundAccountPayload(payload)
	if err != nil {
		return nil, err
	}
	return json.Marshal(demoNodeTx{
		Type:        "fund_account",
		FundAccount: &payload,
	})
}

func decodeDemoNodeTx(txBytes []byte) (demoNodeTx, error) {
	var tx demoNodeTx
	if err := json.Unmarshal(txBytes, &tx); err != nil {
		return demoNodeTx{}, fmt.Errorf("decode demo node tx: %w", err)
	}
	switch strings.TrimSpace(tx.Type) {
	case "queue_deposit_claim":
		if tx.QueueDepositClaim == nil {
			return demoNodeTx{}, fmt.Errorf("missing queue deposit claim payload")
		}
		if _, _, err := depositClaimAndAttestationFromPayload(*tx.QueueDepositClaim); err != nil {
			return demoNodeTx{}, err
		}
	case "initiate_ibc_transfer":
		if tx.InitiateIBCTransfer == nil {
			return demoNodeTx{}, fmt.Errorf("missing initiate ibc transfer payload")
		}
		if _, _, err := decodeInitiateIBCTransferPayload(*tx.InitiateIBCTransfer); err != nil {
			return demoNodeTx{}, err
		}
	case "execute_withdrawal":
		if tx.ExecuteWithdrawal == nil {
			return demoNodeTx{}, fmt.Errorf("missing execute withdrawal payload")
		}
		if _, _, _, err := decodeExecuteWithdrawalPayload(*tx.ExecuteWithdrawal); err != nil {
			return demoNodeTx{}, err
		}
	case "fund_account":
		if tx.FundAccount == nil {
			return demoNodeTx{}, fmt.Errorf("missing fund account payload")
		}
		if _, _, err := decodeFundAccountPayload(*tx.FundAccount); err != nil {
			return demoNodeTx{}, err
		}
	default:
		return demoNodeTx{}, fmt.Errorf("unsupported demo node tx type %q", tx.Type)
	}
	return tx, nil
}

func SubmitQueueDepositClaim(ctx context.Context, cfg Config, claim bridgetypes.DepositClaim, attestation bridgetypes.Attestation) (map[string]any, error) {
	txBytes, err := encodeQueueDepositClaimTx(claim, attestation)
	if err != nil {
		return nil, err
	}
	var result map[string]any
	if err := broadcastTxJSON(ctx, cfg, txBytes, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func SubmitDepositClaim(ctx context.Context, cfg Config, claim bridgetypes.DepositClaim, attestation bridgetypes.Attestation) (bridgekeeper.ClaimResult, error) {
	if _, err := SubmitQueueDepositClaim(ctx, cfg, claim, attestation); err != nil {
		return bridgekeeper.ClaimResult{}, err
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		record, ok, err := QueryClaim(ctx, cfg, claim.Identity.MessageID)
		if err != nil {
			return bridgekeeper.ClaimResult{}, err
		}
		if ok {
			amount, parsed := new(big.Int).SetString(record.Amount, 10)
			if !parsed {
				return bridgekeeper.ClaimResult{}, fmt.Errorf("invalid processed claim amount %q", record.Amount)
			}
			return bridgekeeper.ClaimResult{
				Status:    record.Status,
				MessageID: record.MessageID,
				Denom:     record.Denom,
				Amount:    amount,
			}, nil
		}

		select {
		case <-ctx.Done():
			return bridgekeeper.ClaimResult{}, ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}

	return bridgekeeper.ClaimResult{}, fmt.Errorf("timed out waiting for processed claim %s", claim.Identity.MessageID)
}

func SubmitInitiateIBCTransfer(ctx context.Context, cfg Config, payload InitiateIBCTransferPayload) (TransferView, error) {
	txBytes, err := encodeInitiateIBCTransferTx(payload)
	if err != nil {
		return TransferView{}, err
	}
	var transfer TransferView
	if err := broadcastTxJSON(ctx, cfg, txBytes, &transfer); err != nil {
		return TransferView{}, err
	}
	return transfer, nil
}

func SubmitExecuteWithdrawal(ctx context.Context, cfg Config, payload ExecuteWithdrawalPayload) (WithdrawalView, error) {
	txBytes, err := encodeExecuteWithdrawalTx(payload)
	if err != nil {
		return WithdrawalView{}, err
	}
	var withdrawal WithdrawalView
	if err := broadcastTxJSON(ctx, cfg, txBytes, &withdrawal); err != nil {
		return WithdrawalView{}, err
	}
	return withdrawal, nil
}

func SubmitFundAccount(ctx context.Context, cfg Config, address, denom, amount string) (FundAccountResult, error) {
	payload := FundAccountPayload{
		Address: address,
		Denom:   denom,
		Amount:  amount,
	}
	payload, _, err := decodeFundAccountPayload(payload)
	if err != nil {
		return FundAccountResult{}, err
	}
	ready, err := readReadyState(cfg)
	if err != nil {
		return FundAccountResult{}, err
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return FundAccountResult{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://"+strings.TrimSpace(ready.RPCAddress)+"/tx/fund-account", bytes.NewReader(body))
	if err != nil {
		return FundAccountResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return FundAccountResult{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		var failure map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&failure); err == nil {
			if message, ok := failure["error"].(string); ok && strings.TrimSpace(message) != "" {
				return FundAccountResult{}, fmt.Errorf("fund account request failed: %s", message)
			}
		}
		return FundAccountResult{}, fmt.Errorf("fund account request failed with status %s", resp.Status)
	}
	var result FundAccountResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return FundAccountResult{}, err
	}
	return result, nil
}

func broadcastTxJSON(ctx context.Context, cfg Config, txBytes []byte, target any) error {
	ready, err := readReadyState(cfg)
	if err != nil {
		return err
	}
	client, err := rpchttp.New("http://"+strings.TrimSpace(ready.CometRPCAddress), "/websocket")
	if err != nil {
		return err
	}
	resp, err := client.BroadcastTxCommit(ctx, cmttypes.Tx(txBytes))
	if err != nil {
		return err
	}
	if resp.CheckTx.Code != 0 {
		return fmt.Errorf("check tx failed: %s", resp.CheckTx.Log)
	}
	if resp.TxResult.Code != 0 {
		return fmt.Errorf("deliver tx failed: %s", resp.TxResult.Log)
	}
	if target == nil {
		return nil
	}
	if len(resp.TxResult.Data) == 0 {
		return nil
	}
	if err := json.Unmarshal(resp.TxResult.Data, target); err != nil {
		return fmt.Errorf("decode tx result data: %w", err)
	}
	return nil
}
