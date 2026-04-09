package route

import (
	"context"
	"strings"
	"testing"
)

func TestCommandIBCTargetInvokesExpectedCommands(t *testing.T) {
	t.Parallel()

	var commands []string
	runner := func(_ context.Context, _ string, args ...string) ([]byte, error) {
		command := strings.Join(args, " ")
		commands = append(commands, command)
		switch {
		case strings.Contains(command, "relay recv-packet"):
			return []byte(`{"status":"received"}`), nil
		case strings.Contains(command, "query packet-acks"):
			return []byte(`[{"transfer_id":"ibc/eth.usdc/1","status":"completed"}]`), nil
		case strings.Contains(command, "relay acknowledge-packet"):
			return []byte(`{"status":"confirmed"}`), nil
		default:
			return nil, errUnexpectedCommand
		}
	}

	target := newCommandIBCTargetWithRunner(runner, "osmo-locald", nil, RuntimeLocator{
		Home:        "/tmp/osmo-home",
		RuntimeMode: "osmo-local-runtime",
	})

	ack, err := target.SubmitTransfer(context.Background(), Transfer{
		TransferID:         "ibc/eth.usdc/1",
		AssetID:            "eth.usdc",
		Amount:             "25000000",
		Receiver:           "osmo1receiver",
		DestinationChainID: "osmo-local-1",
		ChannelID:          "channel-0",
		DestinationDenom:   "ibc/uethusdc",
		TimeoutHeight:      140,
		Memo:               "swap:uosmo",
	})
	if err != nil {
		t.Fatalf("submit transfer: %v", err)
	}
	if ack.Status != AckStatusReceived {
		t.Fatalf("expected received ack, got %+v", ack)
	}

	acks, err := target.ReadyAcks(context.Background())
	if err != nil {
		t.Fatalf("ready acks: %v", err)
	}
	if len(acks) != 1 || acks[0].Status != AckStatusCompleted {
		t.Fatalf("unexpected ready acks: %+v", acks)
	}

	if err := target.ConfirmAck(context.Background(), "ibc/eth.usdc/1"); err != nil {
		t.Fatalf("confirm ack: %v", err)
	}

	if len(commands) != 3 {
		t.Fatalf("expected 3 commands, got %d", len(commands))
	}
	if !strings.Contains(commands[0], "relay recv-packet") {
		t.Fatalf("expected recv-packet command, got %q", commands[0])
	}
	if !strings.Contains(commands[1], "query packet-acks") {
		t.Fatalf("expected packet-acks query, got %q", commands[1])
	}
	if !strings.Contains(commands[2], "relay acknowledge-packet") {
		t.Fatalf("expected acknowledge-packet relay command, got %q", commands[2])
	}
}
