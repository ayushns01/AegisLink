package evm

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"os"
	"path/filepath"
	"testing"
)

func TestFileLogSourceLoadsLatestBlockAndFiltersRange(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "evm.json")
	payload := `{
  "latest_block": 18,
  "deposit_events": [
    {
      "block_number": 10,
      "source_chain_id": "11155111",
      "source_contract": "0xgateway",
      "tx_hash": "0xtx-1",
      "log_index": 1,
      "nonce": 7,
      "deposit_id": "deposit-7",
      "message_id": "message-7",
      "asset_address": "0xasset",
      "asset_id": "uusdc",
      "amount": "42",
      "recipient": "aegis1recipient",
      "expiry": 120
    },
    {
      "block_number": 21,
      "source_chain_id": "11155111",
      "source_contract": "0xgateway",
      "tx_hash": "0xtx-2",
      "log_index": 2,
      "nonce": 8,
      "deposit_id": "deposit-8",
      "message_id": "message-8",
      "asset_address": "0xasset",
      "asset_id": "uusdc",
      "amount": "84",
      "recipient": "aegis1recipient",
      "expiry": 140
    }
  ]
}`
	if err := os.WriteFile(path, []byte(payload), 0o644); err != nil {
		t.Fatalf("write evm fixture: %v", err)
	}

	source := NewFileLogSource(path)
	latest, err := source.LatestBlock(context.Background())
	if err != nil {
		t.Fatalf("latest block: %v", err)
	}
	if latest != 18 {
		t.Fatalf("expected latest block 18, got %d", latest)
	}

	events, err := source.DepositEvents(context.Background(), 9, 18)
	if err != nil {
		t.Fatalf("deposit events: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event in range, got %d", len(events))
	}
	if events[0].Amount.String() != "42" {
		t.Fatalf("expected amount 42, got %s", events[0].Amount)
	}
}

func TestFileReleaseTargetPersistsRequests(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "release-outbox.json")
	target := NewFileReleaseTarget(path)
	signature := []byte{0x00, 0x01, 0xff}

	releaseID, err := target.ReleaseWithdrawal(context.Background(), ReleaseRequest{
		MessageID:    "message-9",
		AssetAddress: "0xasset",
		Amount:       big.NewInt(99),
		Recipient:    "0xrecipient",
		Deadline:     200,
		Signature:    signature,
	})
	if err != nil {
		t.Fatalf("persist release: %v", err)
	}
	if releaseID != "message-9" {
		t.Fatalf("expected release id message-9, got %q", releaseID)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read outbox: %v", err)
	}
	var outbox persistedReleaseOutbox
	if err := json.Unmarshal(data, &outbox); err != nil {
		t.Fatalf("decode outbox: %v", err)
	}
	if len(outbox.Requests) != 1 {
		t.Fatalf("expected 1 persisted request, got %d", len(outbox.Requests))
	}
	if outbox.Requests[0].Signature != base64.StdEncoding.EncodeToString(signature) {
		t.Fatalf("expected base64 signature %q, got %q", base64.StdEncoding.EncodeToString(signature), outbox.Requests[0].Signature)
	}
}
