package cosmos

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestFileWithdrawalSourceLoadsLatestHeightAndFiltersRange(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "cosmos.json")
	payload := `{
  "latest_height": 30,
  "withdrawals": [
    {
      "block_height": 21,
      "kind": "withdrawal",
      "source_chain_id": "aegislink-1",
      "source_contract": "bridge",
      "source_tx_hash": "0xtx-1",
      "source_log_index": 4,
      "nonce": 9,
      "message_id": "message-9",
      "asset_id": "uusdc",
      "asset_address": "0xasset",
      "amount": "75",
      "recipient": "0xrecipient",
      "deadline": 300,
      "signature": "cHJvb2Y="
    },
    {
      "block_height": 40,
      "kind": "withdrawal",
      "source_chain_id": "aegislink-1",
      "source_contract": "bridge",
      "source_tx_hash": "0xtx-2",
      "source_log_index": 5,
      "nonce": 10,
      "message_id": "message-10",
      "asset_id": "uusdc",
      "asset_address": "0xasset",
      "amount": "25",
      "recipient": "0xrecipient",
      "deadline": 320,
      "signature": "cHJvb2YtMg=="
    }
  ]
}`
	if err := os.WriteFile(path, []byte(payload), 0o644); err != nil {
		t.Fatalf("write cosmos fixture: %v", err)
	}

	source := NewFileWithdrawalSource(path)
	latest, err := source.LatestHeight(context.Background())
	if err != nil {
		t.Fatalf("latest height: %v", err)
	}
	if latest != 30 {
		t.Fatalf("expected latest height 30, got %d", latest)
	}

	withdrawals, err := source.Withdrawals(context.Background(), 20, 30)
	if err != nil {
		t.Fatalf("withdrawals: %v", err)
	}
	if len(withdrawals) != 1 {
		t.Fatalf("expected 1 withdrawal in range, got %d", len(withdrawals))
	}
	if withdrawals[0].Amount.String() != "75" {
		t.Fatalf("expected amount 75, got %s", withdrawals[0].Amount)
	}
	if string(withdrawals[0].Signature) != "proof" {
		t.Fatalf("expected decoded signature %q, got %q", "proof", withdrawals[0].Signature)
	}
}

func TestFileClaimSinkPersistsClaimAndAttestation(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "cosmos-outbox.json")
	sink := NewFileClaimSink(path)
	claim := validDepositClaim()
	attestation := validAttestationForClaim(claim)

	if err := sink.SubmitDepositClaim(context.Background(), claim, attestation); err != nil {
		t.Fatalf("persist claim: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read outbox: %v", err)
	}
	if len(data) == 0 {
		t.Fatalf("expected persisted claim submission")
	}
}
