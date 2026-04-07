package keeper

import (
	"fmt"
	"math/big"

	bridgetypes "github.com/ayushns01/aegislink/chain/aegislink/x/bridge/types"
)

type StateSnapshot struct {
	CurrentHeight       uint64                     `json:"current_height"`
	NextWithdrawalNonce uint64                     `json:"next_withdrawal_nonce"`
	RejectedClaims      uint64                     `json:"rejected_claims"`
	SignerSets          []SignerSetSnapshot        `json:"signer_sets"`
	ProcessedClaims     []ClaimRecordSnapshot      `json:"processed_claims"`
	SupplyByDenom       map[string]string          `json:"supply_by_denom"`
	Withdrawals         []WithdrawalRecordSnapshot `json:"withdrawals"`
}

type SignerSetSnapshot struct {
	Version     uint64   `json:"version"`
	Signers     []string `json:"signers"`
	Threshold   uint32   `json:"threshold"`
	ActivatedAt uint64   `json:"activated_at"`
	ExpiresAt   uint64   `json:"expires_at"`
}

type ClaimRecordSnapshot struct {
	ClaimKey  string      `json:"claim_key"`
	MessageID string      `json:"message_id"`
	Denom     string      `json:"denom"`
	AssetID   string      `json:"asset_id"`
	Amount    string      `json:"amount"`
	Status    ClaimStatus `json:"status"`
}

type WithdrawalRecordSnapshot struct {
	BlockHeight  uint64                    `json:"block_height"`
	Identity     bridgetypes.ClaimIdentity `json:"identity"`
	AssetID      string                    `json:"asset_id"`
	AssetAddress string                    `json:"asset_address"`
	Amount       string                    `json:"amount"`
	Recipient    string                    `json:"recipient"`
	Deadline     uint64                    `json:"deadline"`
	Signature    []byte                    `json:"signature"`
}

func (k *Keeper) ExportState() StateSnapshot {
	state := StateSnapshot{
		CurrentHeight:       k.currentHeight,
		NextWithdrawalNonce: k.nextWithdrawalNonce,
		RejectedClaims:      k.rejectedClaims,
		SignerSets:          make([]SignerSetSnapshot, 0, len(k.signerSets)),
		ProcessedClaims:     make([]ClaimRecordSnapshot, 0, len(k.processedClaims)),
		SupplyByDenom:       make(map[string]string, len(k.supplyByDenom)),
		Withdrawals:         make([]WithdrawalRecordSnapshot, 0, len(k.withdrawals)),
	}

	for _, signerSet := range k.signerSets {
		state.SignerSets = append(state.SignerSets, SignerSetSnapshot{
			Version:     signerSet.Version,
			Signers:     append([]string(nil), signerSet.Signers...),
			Threshold:   signerSet.Threshold,
			ActivatedAt: signerSet.ActivatedAt,
			ExpiresAt:   signerSet.ExpiresAt,
		})
	}
	for claimKey, record := range k.processedClaims {
		state.ProcessedClaims = append(state.ProcessedClaims, ClaimRecordSnapshot{
			ClaimKey:  claimKey,
			MessageID: record.MessageID,
			Denom:     record.Denom,
			AssetID:   record.AssetID,
			Amount:    record.Amount.String(),
			Status:    record.Status,
		})
	}
	for denom, amount := range k.supplyByDenom {
		state.SupplyByDenom[denom] = amount.String()
	}
	for _, withdrawal := range k.withdrawals {
		state.Withdrawals = append(state.Withdrawals, WithdrawalRecordSnapshot{
			BlockHeight:  withdrawal.BlockHeight,
			Identity:     withdrawal.Identity,
			AssetID:      withdrawal.AssetID,
			AssetAddress: withdrawal.AssetAddress,
			Amount:       withdrawal.Amount.String(),
			Recipient:    withdrawal.Recipient,
			Deadline:     withdrawal.Deadline,
			Signature:    append([]byte(nil), withdrawal.Signature...),
		})
	}

	return state
}

func (k *Keeper) ImportState(state StateSnapshot) error {
	k.currentHeight = state.CurrentHeight
	k.nextWithdrawalNonce = state.NextWithdrawalNonce
	k.rejectedClaims = state.RejectedClaims
	if k.nextWithdrawalNonce == 0 {
		k.nextWithdrawalNonce = 1
	}

	if len(state.SignerSets) > 0 {
		k.signerSets = make(map[uint64]SignerSet, len(state.SignerSets))
		for _, signerSet := range state.SignerSets {
			set := normalizeSignerSet(SignerSet{
				Version:     signerSet.Version,
				Signers:     append([]string(nil), signerSet.Signers...),
				Threshold:   signerSet.Threshold,
				ActivatedAt: signerSet.ActivatedAt,
				ExpiresAt:   signerSet.ExpiresAt,
			})
			k.signerSets[set.Version] = set
		}
	}

	k.processedClaims = make(map[string]ClaimRecord, len(state.ProcessedClaims))
	for _, claim := range state.ProcessedClaims {
		amount, ok := new(big.Int).SetString(claim.Amount, 10)
		if !ok {
			return fmt.Errorf("invalid processed claim amount %q", claim.Amount)
		}
		k.processedClaims[claim.ClaimKey] = ClaimRecord{
			MessageID: claim.MessageID,
			Denom:     claim.Denom,
			AssetID:   claim.AssetID,
			Amount:    amount,
			Status:    claim.Status,
		}
	}

	k.supplyByDenom = make(map[string]*big.Int, len(state.SupplyByDenom))
	for denom, amount := range state.SupplyByDenom {
		parsed, ok := new(big.Int).SetString(amount, 10)
		if !ok {
			return fmt.Errorf("invalid supply amount %q", amount)
		}
		k.supplyByDenom[denom] = parsed
	}

	k.withdrawals = make([]WithdrawalRecord, 0, len(state.Withdrawals))
	for _, withdrawal := range state.Withdrawals {
		amount, ok := new(big.Int).SetString(withdrawal.Amount, 10)
		if !ok {
			return fmt.Errorf("invalid withdrawal amount %q", withdrawal.Amount)
		}
		k.withdrawals = append(k.withdrawals, WithdrawalRecord{
			BlockHeight:  withdrawal.BlockHeight,
			Identity:     withdrawal.Identity,
			AssetID:      withdrawal.AssetID,
			AssetAddress: withdrawal.AssetAddress,
			Amount:       amount,
			Recipient:    withdrawal.Recipient,
			Deadline:     withdrawal.Deadline,
			Signature:    append([]byte(nil), withdrawal.Signature...),
		})
	}

	return nil
}
