package keeper

import (
	"fmt"
	"math/big"

	bridgetypes "github.com/ayushns01/aegislink/chain/aegislink/x/bridge/types"
)

type StateSnapshot struct {
	CurrentHeight         uint64                     `json:"current_height"`
	NextWithdrawalNonce   uint64                     `json:"next_withdrawal_nonce"`
	RejectedClaims        uint64                     `json:"rejected_claims"`
	CircuitBreakerTripped bool                       `json:"circuit_breaker_tripped"`
	LastInvariantError    string                     `json:"last_invariant_error"`
	SignerSets            []SignerSetSnapshot        `json:"signer_sets"`
	ProcessedClaims       []ClaimRecordSnapshot      `json:"processed_claims"`
	SupplyByDenom         map[string]string          `json:"supply_by_denom"`
	Withdrawals           []WithdrawalRecordSnapshot `json:"withdrawals"`
}

type bridgeMetadataSnapshot struct {
	CurrentHeight         uint64 `json:"current_height"`
	NextWithdrawalNonce   uint64 `json:"next_withdrawal_nonce"`
	RejectedClaims        uint64 `json:"rejected_claims"`
	CircuitBreakerTripped bool   `json:"circuit_breaker_tripped"`
	LastInvariantError    string `json:"last_invariant_error"`
}

type SignerSetSnapshot struct {
	Version     uint64   `json:"version"`
	Signers     []string `json:"signers"`
	Threshold   uint32   `json:"threshold"`
	ActivatedAt uint64   `json:"activated_at"`
	ExpiresAt   uint64   `json:"expires_at"`
}

type ClaimRecordSnapshot struct {
	ClaimKey     string      `json:"claim_key"`
	MessageID    string      `json:"message_id"`
	SourceTxHash string      `json:"source_tx_hash"`
	Denom        string      `json:"denom"`
	AssetID      string      `json:"asset_id"`
	Recipient    string      `json:"recipient,omitempty"`
	Amount       string      `json:"amount"`
	Status       ClaimStatus `json:"status"`
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
		CurrentHeight:         k.currentHeight,
		NextWithdrawalNonce:   k.nextWithdrawalNonce,
		RejectedClaims:        k.rejectedClaims,
		CircuitBreakerTripped: k.circuitBreakerTripped,
		LastInvariantError:    k.lastInvariantError,
		SignerSets:            make([]SignerSetSnapshot, 0, len(k.signerSets)),
		ProcessedClaims:       make([]ClaimRecordSnapshot, 0, len(k.processedClaims)),
		SupplyByDenom:         make(map[string]string, len(k.supplyByDenom)),
		Withdrawals:           make([]WithdrawalRecordSnapshot, 0, len(k.withdrawals)),
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
			ClaimKey:     claimKey,
			MessageID:    record.MessageID,
			SourceTxHash: record.SourceTxHash,
			Denom:        record.Denom,
			AssetID:      record.AssetID,
			Recipient:    record.Recipient,
			Amount:       record.Amount.String(),
			Status:       record.Status,
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
	k.circuitBreakerTripped = state.CircuitBreakerTripped
	k.lastInvariantError = state.LastInvariantError
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
			MessageID:    claim.MessageID,
			SourceTxHash: claim.SourceTxHash,
			Denom:        claim.Denom,
			AssetID:      claim.AssetID,
			Recipient:    claim.Recipient,
			Amount:       amount,
			Status:       claim.Status,
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

func (k *Keeper) loadFromPrefixStore() error {
	defaultSignerSets := k.signerSets
	k.signerSets = make(map[uint64]SignerSet)
	k.processedClaims = make(map[string]ClaimRecord)
	k.supplyByDenom = make(map[string]*big.Int)
	k.withdrawals = make([]WithdrawalRecord, 0)
	k.nextWithdrawalNonce = 1

	var meta bridgeMetadataSnapshot
	if found, err := k.prefixStore.Load(bridgeMetaPrefix, "runtime", &meta); err != nil {
		return err
	} else if found {
		k.currentHeight = meta.CurrentHeight
		k.nextWithdrawalNonce = meta.NextWithdrawalNonce
		k.rejectedClaims = meta.RejectedClaims
		k.circuitBreakerTripped = meta.CircuitBreakerTripped
		k.lastInvariantError = meta.LastInvariantError
		if k.nextWithdrawalNonce == 0 {
			k.nextWithdrawalNonce = 1
		}
	}

	if err := k.prefixStore.LoadAll(bridgeSignerSetPrefix, func() any {
		return &SignerSetSnapshot{}
	}, func(_ string, value any) error {
		signerSet := *(value.(*SignerSetSnapshot))
		set := normalizeSignerSet(SignerSet{
			Version:     signerSet.Version,
			Signers:     append([]string(nil), signerSet.Signers...),
			Threshold:   signerSet.Threshold,
			ActivatedAt: signerSet.ActivatedAt,
			ExpiresAt:   signerSet.ExpiresAt,
		})
		k.signerSets[set.Version] = set
		return nil
	}); err != nil {
		return err
	}
	if len(k.signerSets) == 0 {
		k.signerSets = defaultSignerSets
	}

	if err := k.prefixStore.LoadAll(bridgeClaimPrefix, func() any {
		return &ClaimRecordSnapshot{}
	}, func(_ string, value any) error {
		claim := *(value.(*ClaimRecordSnapshot))
		amount, ok := new(big.Int).SetString(claim.Amount, 10)
		if !ok {
			return fmt.Errorf("invalid processed claim amount %q", claim.Amount)
		}
		k.processedClaims[claim.ClaimKey] = ClaimRecord{
			MessageID:    claim.MessageID,
			SourceTxHash: claim.SourceTxHash,
			Denom:        claim.Denom,
			AssetID:      claim.AssetID,
			Recipient:    claim.Recipient,
			Amount:       amount,
			Status:       claim.Status,
		}
		return nil
	}); err != nil {
		return err
	}

	if err := k.prefixStore.LoadAll(bridgeSupplyPrefix, func() any {
		value := ""
		return &value
	}, func(id string, value any) error {
		parsed, ok := new(big.Int).SetString(*(value.(*string)), 10)
		if !ok {
			return fmt.Errorf("invalid supply amount %q", *(value.(*string)))
		}
		k.supplyByDenom[id] = parsed
		return nil
	}); err != nil {
		return err
	}

	return k.prefixStore.LoadAll(bridgeWithdrawalPrefix, func() any {
		return &WithdrawalRecordSnapshot{}
	}, func(_ string, value any) error {
		withdrawal := *(value.(*WithdrawalRecordSnapshot))
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
		return nil
	})
}
