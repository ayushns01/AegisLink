package route

import (
	"context"
	"strings"
	"testing"
)

func TestCommandTransferSourceFiltersPendingTransfers(t *testing.T) {
	t.Parallel()

	var called bool
	source := newCommandTransferSourceWithRunner(func(_ context.Context, _ string, args ...string) ([]byte, error) {
		called = true
		if !strings.Contains(strings.Join(args, " "), "query transfers") {
			t.Fatalf("expected query transfers command, got %v", args)
		}
		if !strings.Contains(strings.Join(args, " "), "--home /tmp/home") {
			t.Fatalf("expected runtime home flag, got %v", args)
		}
		if !strings.Contains(strings.Join(args, " "), "--runtime-mode sdk-store-runtime") {
			t.Fatalf("expected runtime mode flag, got %v", args)
		}
		return []byte(`[
  {"transfer_id":"ibc/eth.usdc/1","asset_id":"eth.usdc","amount":"25000000","receiver":"osmo1recipient","status":"pending"},
  {"transfer_id":"ibc/eth.usdc/2","asset_id":"eth.usdc","amount":"25000000","receiver":"osmo1recipient","status":"completed"}
]`), nil
	}, "aegislinkd", nil, RuntimeLocator{
		Home:        "/tmp/home",
		StatePath:   "/tmp/state.json",
		RuntimeMode: "sdk-store-runtime",
	})

	transfers, err := source.PendingTransfers(context.Background())
	if err != nil {
		t.Fatalf("pending transfers: %v", err)
	}
	if !called {
		t.Fatal("expected command runner to be called")
	}
	if len(transfers) != 1 {
		t.Fatalf("expected one pending transfer, got %d", len(transfers))
	}
	if transfers[0].TransferID != "ibc/eth.usdc/1" {
		t.Fatalf("expected pending transfer id ibc/eth.usdc/1, got %q", transfers[0].TransferID)
	}
}

func TestCommandAckSinkInvokesExpectedTransferCommands(t *testing.T) {
	t.Parallel()

	var commands []string
	runner := func(_ context.Context, _ string, args ...string) ([]byte, error) {
		commands = append(commands, strings.Join(args, " "))
		return []byte(`{"status":"ok"}`), nil
	}
	sink := newCommandAckSinkWithRunner(runner, "aegislinkd", nil, RuntimeLocator{
		Home:        "/tmp/home",
		StatePath:   "/tmp/state.json",
		RuntimeMode: "sdk-store-runtime",
	})

	if err := sink.CompleteTransfer(context.Background(), "ibc/eth.usdc/1"); err != nil {
		t.Fatalf("complete transfer: %v", err)
	}
	if err := sink.FailTransfer(context.Background(), "ibc/eth.usdc/2", "ack failed"); err != nil {
		t.Fatalf("fail transfer: %v", err)
	}
	if err := sink.TimeoutTransfer(context.Background(), "ibc/eth.usdc/3"); err != nil {
		t.Fatalf("timeout transfer: %v", err)
	}

	if len(commands) != 3 {
		t.Fatalf("expected 3 commands, got %d", len(commands))
	}
	if !strings.Contains(commands[0], "tx complete-ibc-transfer") {
		t.Fatalf("expected complete command, got %q", commands[0])
	}
	if !strings.Contains(commands[1], "tx fail-ibc-transfer") || !strings.Contains(commands[1], "--reason ack failed") {
		t.Fatalf("expected fail command with reason, got %q", commands[1])
	}
	if !strings.Contains(commands[2], "tx timeout-ibc-transfer") {
		t.Fatalf("expected timeout command, got %q", commands[2])
	}
	if !strings.Contains(commands[0], "--home /tmp/home") || !strings.Contains(commands[0], "--runtime-mode sdk-store-runtime") {
		t.Fatalf("expected runtime locator flags in command, got %q", commands[0])
	}
}
