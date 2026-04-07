package cosmos

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"path/filepath"

	bridgetypes "github.com/ayushns01/aegislink/chain/aegislink/x/bridge/types"
)

type FileWithdrawalSource struct {
	path string
}

type FileClaimSink struct {
	path string
}

type persistedWithdrawalState struct {
	LatestHeight uint64                `json:"latest_height"`
	Withdrawals  []persistedWithdrawal `json:"withdrawals"`
}

type persistedWithdrawal struct {
	BlockHeight     uint64 `json:"block_height"`
	Kind            string `json:"kind"`
	SourceChainID   string `json:"source_chain_id"`
	SourceContract  string `json:"source_contract"`
	SourceTxHash    string `json:"source_tx_hash"`
	SourceLogIndex  uint64 `json:"source_log_index"`
	Nonce           uint64 `json:"nonce"`
	MessageID       string `json:"message_id"`
	AssetID         string `json:"asset_id"`
	AssetAddress    string `json:"asset_address"`
	Amount          string `json:"amount"`
	Recipient       string `json:"recipient"`
	Deadline        uint64 `json:"deadline"`
	Signature       string `json:"signature"`
	SignatureBase64 string `json:"signature_base64"`
}

type persistedClaimOutbox struct {
	Submissions []persistedClaimSubmission `json:"submissions"`
}

type persistedClaimSubmission struct {
	Claim       persistedDepositClaim `json:"claim"`
	Attestation persistedAttestation  `json:"attestation"`
}

type persistedDepositClaim struct {
	Kind               string `json:"kind"`
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
}

type persistedAttestation struct {
	MessageID        string   `json:"message_id"`
	PayloadHash      string   `json:"payload_hash"`
	Signers          []string `json:"signers"`
	Threshold        uint32   `json:"threshold"`
	Expiry           uint64   `json:"expiry"`
	SignerSetVersion uint64   `json:"signer_set_version"`
}

func NewFileWithdrawalSource(path string) *FileWithdrawalSource {
	return &FileWithdrawalSource{path: path}
}

func NewFileClaimSink(path string) *FileClaimSink {
	return &FileClaimSink{path: path}
}

func (s *FileWithdrawalSource) LatestHeight(ctx context.Context) (uint64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	state, err := s.load()
	if err != nil {
		return 0, err
	}
	return state.LatestHeight, nil
}

func (s *FileWithdrawalSource) Withdrawals(ctx context.Context, fromHeight, toHeight uint64) ([]Withdrawal, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	state, err := s.load()
	if err != nil {
		return nil, err
	}

	withdrawals := make([]Withdrawal, 0, len(state.Withdrawals))
	for _, withdrawal := range state.Withdrawals {
		if withdrawal.BlockHeight < fromHeight || withdrawal.BlockHeight > toHeight {
			continue
		}
		amount, ok := new(big.Int).SetString(withdrawal.Amount, 10)
		if !ok {
			return nil, fmt.Errorf("invalid withdrawal amount %q", withdrawal.Amount)
		}
		signature, err := base64.StdEncoding.DecodeString(withdrawal.Signature)
		if err != nil {
			return nil, fmt.Errorf("decode withdrawal signature: %w", err)
		}
		withdrawals = append(withdrawals, Withdrawal{
			BlockHeight: withdrawal.BlockHeight,
			Identity: bridgetypes.ClaimIdentity{
				Kind:           bridgetypes.ClaimKind(withdrawal.Kind),
				SourceChainID:  withdrawal.SourceChainID,
				SourceContract: withdrawal.SourceContract,
				SourceTxHash:   withdrawal.SourceTxHash,
				SourceLogIndex: withdrawal.SourceLogIndex,
				Nonce:          withdrawal.Nonce,
				MessageID:      withdrawal.MessageID,
			},
			AssetID:      withdrawal.AssetID,
			AssetAddress: withdrawal.AssetAddress,
			Amount:       amount,
			Recipient:    withdrawal.Recipient,
			Deadline:     withdrawal.Deadline,
			Signature:    signature,
		})
	}
	return withdrawals, nil
}

func (s *FileClaimSink) SubmitDepositClaim(ctx context.Context, claim bridgetypes.DepositClaim, attestation bridgetypes.Attestation) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	outbox, err := s.loadOutbox()
	if err != nil {
		return err
	}

	outbox.Submissions = append(outbox.Submissions, persistedClaimSubmission{
		Claim: persistedDepositClaim{
			Kind:               string(claim.Identity.Kind),
			SourceChainID:      claim.Identity.SourceChainID,
			SourceContract:     claim.Identity.SourceContract,
			SourceTxHash:       claim.Identity.SourceTxHash,
			SourceLogIndex:     claim.Identity.SourceLogIndex,
			Nonce:              claim.Identity.Nonce,
			MessageID:          claim.Identity.MessageID,
			DestinationChainID: claim.DestinationChainID,
			AssetID:            claim.AssetID,
			Amount:             claim.Amount.String(),
			Recipient:          claim.Recipient,
			Deadline:           claim.Deadline,
		},
		Attestation: persistedAttestation{
			MessageID:        attestation.MessageID,
			PayloadHash:      attestation.PayloadHash,
			Signers:          append([]string(nil), attestation.Signers...),
			Threshold:        attestation.Threshold,
			Expiry:           attestation.Expiry,
			SignerSetVersion: attestation.SignerSetVersion,
		},
	})
	return persistJSON(s.path, outbox)
}

func (s *FileWithdrawalSource) load() (persistedWithdrawalState, error) {
	if s == nil || s.path == "" {
		return persistedWithdrawalState{}, nil
	}

	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return persistedWithdrawalState{}, nil
		}
		return persistedWithdrawalState{}, err
	}

	var state persistedWithdrawalState
	if err := json.Unmarshal(data, &state); err != nil {
		return persistedWithdrawalState{}, err
	}
	return state, nil
}

func (s *FileClaimSink) loadOutbox() (persistedClaimOutbox, error) {
	if s == nil || s.path == "" {
		return persistedClaimOutbox{}, nil
	}

	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return persistedClaimOutbox{}, nil
		}
		return persistedClaimOutbox{}, err
	}

	var outbox persistedClaimOutbox
	if err := json.Unmarshal(data, &outbox); err != nil {
		return persistedClaimOutbox{}, err
	}
	return outbox, nil
}

func persistJSON(path string, value any) error {
	if path == "" {
		return nil
	}

	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "runtime-*.json")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.Write(encoded); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
