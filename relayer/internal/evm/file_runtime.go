package evm

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
)

type FileLogSource struct {
	path string
}

type FileReleaseTarget struct {
	path string
}

type persistedDepositState struct {
	LatestBlock   uint64                  `json:"latest_block"`
	DepositEvents []persistedDepositEvent `json:"deposit_events"`
}

type persistedDepositEvent struct {
	BlockNumber    uint64 `json:"block_number"`
	SourceChainID  string `json:"source_chain_id"`
	SourceContract string `json:"source_contract"`
	TxHash         string `json:"tx_hash"`
	LogIndex       uint64 `json:"log_index"`
	Nonce          uint64 `json:"nonce"`
	DepositID      string `json:"deposit_id"`
	MessageID      string `json:"message_id"`
	AssetAddress   string `json:"asset_address"`
	AssetID        string `json:"asset_id"`
	Amount         string `json:"amount"`
	Recipient      string `json:"recipient"`
	Expiry         uint64 `json:"expiry"`
}

type persistedReleaseOutbox struct {
	Requests []persistedReleaseRequest `json:"requests"`
}

type persistedReleaseRequest struct {
	MessageID    string `json:"message_id"`
	AssetAddress string `json:"asset_address"`
	Amount       string `json:"amount"`
	Recipient    string `json:"recipient"`
	Deadline     uint64 `json:"deadline"`
	Signature    string `json:"signature"`
}

func NewFileLogSource(path string) *FileLogSource {
	return &FileLogSource{path: path}
}

func NewFileReleaseTarget(path string) *FileReleaseTarget {
	return &FileReleaseTarget{path: path}
}

func (s *FileLogSource) LatestBlock(ctx context.Context) (uint64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	state, err := s.load()
	if err != nil {
		return 0, err
	}
	return state.LatestBlock, nil
}

func (s *FileLogSource) DepositEvents(ctx context.Context, fromBlock, toBlock uint64) ([]DepositEvent, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	state, err := s.load()
	if err != nil {
		return nil, err
	}

	events := make([]DepositEvent, 0, len(state.DepositEvents))
	for _, event := range state.DepositEvents {
		if event.BlockNumber < fromBlock || event.BlockNumber > toBlock {
			continue
		}
		amount, ok := new(big.Int).SetString(event.Amount, 10)
		if !ok {
			return nil, fmt.Errorf("invalid deposit amount %q", event.Amount)
		}
		events = append(events, DepositEvent{
			BlockNumber:    event.BlockNumber,
			SourceChainID:  event.SourceChainID,
			SourceContract: event.SourceContract,
			TxHash:         event.TxHash,
			LogIndex:       event.LogIndex,
			Nonce:          event.Nonce,
			DepositID:      event.DepositID,
			MessageID:      event.MessageID,
			AssetAddress:   event.AssetAddress,
			AssetID:        event.AssetID,
			Amount:         amount,
			Recipient:      event.Recipient,
			Expiry:         event.Expiry,
		})
	}
	return events, nil
}

func (t *FileReleaseTarget) ReleaseWithdrawal(ctx context.Context, request ReleaseRequest) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	outbox, err := t.loadOutbox()
	if err != nil {
		return "", err
	}

	outbox.Requests = append(outbox.Requests, persistedReleaseRequest{
		MessageID:    request.MessageID,
		AssetAddress: request.AssetAddress,
		Amount:       request.Amount.String(),
		Recipient:    request.Recipient,
		Deadline:     request.Deadline,
		Signature:    base64.StdEncoding.EncodeToString(request.Signature),
	})
	if err := persistJSON(t.path, outbox); err != nil {
		return "", err
	}
	return request.MessageID, nil
}

func (s *FileLogSource) load() (persistedDepositState, error) {
	if s == nil || s.path == "" {
		return persistedDepositState{}, nil
	}

	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return persistedDepositState{}, nil
		}
		return persistedDepositState{}, err
	}

	var state persistedDepositState
	if err := json.Unmarshal(data, &state); err != nil {
		return persistedDepositState{}, err
	}
	return state, nil
}

func (t *FileReleaseTarget) loadOutbox() (persistedReleaseOutbox, error) {
	if t == nil || t.path == "" {
		return persistedReleaseOutbox{}, nil
	}

	data, err := os.ReadFile(t.path)
	if err != nil {
		if os.IsNotExist(err) {
			return persistedReleaseOutbox{}, nil
		}
		return persistedReleaseOutbox{}, err
	}

	var outbox persistedReleaseOutbox
	if err := json.Unmarshal(data, &outbox); err != nil {
		return persistedReleaseOutbox{}, err
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
