package cosmos

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
)

func TestCommandClaimSinkInvokesSubmitDepositClaimTx(t *testing.T) {
	t.Parallel()

	var called bool
	var gotName string
	var gotArgs []string
	var capturedClaim persistedDepositClaim
	runner := func(_ context.Context, name string, args ...string) ([]byte, error) {
		called = true
		gotName = name
		gotArgs = append([]string(nil), args...)
		for i := 0; i < len(args)-1; i++ {
			if args[i] != "--submission-file" {
				continue
			}
			data, err := os.ReadFile(args[i+1])
			if err != nil {
				t.Fatalf("read submission file: %v", err)
			}
			var payload persistedClaimSubmission
			if err := json.Unmarshal(data, &payload); err != nil {
				t.Fatalf("decode submission file: %v", err)
			}
			capturedClaim = payload.Claim
			break
		}
		return []byte(`{"status":"accepted","message_id":"message-1"}`), nil
	}

	sink := newCommandClaimSinkWithRunner(runner, "aegislinkd", []string{"--home", "/tmp/home"}, "/tmp/state.json")
	claim := validDepositClaim()
	attestation := validAttestationForClaim(claim)

	if err := sink.SubmitDepositClaim(context.Background(), claim, attestation); err != nil {
		t.Fatalf("submit claim: %v", err)
	}
	if !called {
		t.Fatalf("expected command runner to be invoked")
	}
	if gotName != "aegislinkd" {
		t.Fatalf("expected command name aegislinkd, got %q", gotName)
	}
	joined := strings.Join(gotArgs, " ")
	if !strings.Contains(joined, "tx submit-deposit-claim") {
		t.Fatalf("expected tx submit-deposit-claim invocation, got %q", joined)
	}
	if !strings.Contains(joined, "--home /tmp/home") {
		t.Fatalf("expected home flag to be preserved, got %q", joined)
	}
	if strings.Contains(joined, "--state-path /tmp/state.json") {
		t.Fatalf("expected state path flag to be omitted when --home is present, got %q", joined)
	}
	if !strings.Contains(joined, "--submission-file") {
		t.Fatalf("expected submission file flag, got %q", joined)
	}
	if capturedClaim.SourceAssetKind != claim.Identity.SourceAssetKind {
		t.Fatalf("expected source asset kind %q, got %q", claim.Identity.SourceAssetKind, capturedClaim.SourceAssetKind)
	}
}

func TestCommandWithdrawalSourceUsesRuntimeQueries(t *testing.T) {
	t.Parallel()

	var calls [][]string
	runner := func(_ context.Context, _ string, args ...string) ([]byte, error) {
		calls = append(calls, append([]string(nil), args...))
		joined := strings.Join(args, " ")
		switch {
		case strings.Contains(joined, "query summary"):
			return []byte(`{"current_height":61}`), nil
		case strings.Contains(joined, "query withdrawals"):
			return []byte(`[
  {
    "kind":"withdrawal",
    "source_chain_id":"aegislink-1",
    "source_contract":"aegislink.bridge",
    "source_tx_hash":"0xabc",
    "source_log_index":0,
    "nonce":1,
    "message_id":"message-1",
    "asset_id":"eth.usdc",
    "asset_address":"0xasset",
    "amount":"100000000",
    "recipient":"0xrecipient",
    "deadline":120,
    "block_height":60,
    "signature":"dGhyZXNob2xkLXByb29m"
  }
]`), nil
		default:
			return nil, errors.New("unexpected command")
		}
	}

	source := newCommandWithdrawalSourceWithRunner(runner, "aegislinkd", nil, "/tmp/state.json")

	latest, err := source.LatestHeight(context.Background())
	if err != nil {
		t.Fatalf("latest height: %v", err)
	}
	if latest != 61 {
		t.Fatalf("expected latest height 61, got %d", latest)
	}

	withdrawals, err := source.Withdrawals(context.Background(), 60, 60)
	if err != nil {
		t.Fatalf("withdrawals: %v", err)
	}
	if len(withdrawals) != 1 {
		t.Fatalf("expected one withdrawal, got %d", len(withdrawals))
	}
	if withdrawals[0].Identity.MessageID != "message-1" {
		t.Fatalf("expected message id message-1, got %q", withdrawals[0].Identity.MessageID)
	}
	if string(withdrawals[0].Signature) != "threshold-proof" {
		t.Fatalf("expected decoded signature threshold-proof, got %q", withdrawals[0].Signature)
	}
	if len(calls) != 2 {
		t.Fatalf("expected two command calls, got %d", len(calls))
	}
}

func TestCommandRuntimeTreatsDemoNodeReadyFileAsRuntimeFlag(t *testing.T) {
	t.Parallel()

	var calls [][]string
	runner := func(_ context.Context, _ string, args ...string) ([]byte, error) {
		calls = append(calls, append([]string(nil), args...))
		return []byte(`{"current_height":61}`), nil
	}

	source := newCommandWithdrawalSourceWithRunner(runner, "aegislinkd", []string{"--home", "/tmp/home", "--demo-node-ready-file", "/tmp/demo-node-ready.json"}, "/tmp/state.json")
	latest, err := source.LatestHeight(context.Background())
	if err != nil {
		t.Fatalf("latest height: %v", err)
	}
	if latest != 61 {
		t.Fatalf("expected latest height 61, got %d", latest)
	}
	if len(calls) != 1 {
		t.Fatalf("expected one command call, got %d", len(calls))
	}
	joined := strings.Join(calls[0], " ")
	if !strings.Contains(joined, "query summary") {
		t.Fatalf("expected query summary invocation, got %q", joined)
	}
	if !strings.Contains(joined, "--home /tmp/home") {
		t.Fatalf("expected preserved home flag, got %q", joined)
	}
	if !strings.Contains(joined, "--demo-node-ready-file /tmp/demo-node-ready.json") {
		t.Fatalf("expected preserved demo-node-ready-file flag, got %q", joined)
	}
	if strings.Contains(joined, "--state-path /tmp/state.json") {
		t.Fatalf("expected state path to be omitted when --home is present, got %q", joined)
	}
}
