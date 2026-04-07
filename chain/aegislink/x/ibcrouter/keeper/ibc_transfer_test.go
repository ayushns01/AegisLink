package keeper

import (
	"math/big"
	"testing"
)

func TestIBCSendReceiveAckAndTimeoutLifecycle(t *testing.T) {
	t.Parallel()

	keeper := NewKeeper()
	if err := keeper.SetRoute(Route{
		AssetID:            "eth.usdc",
		DestinationChainID: "osmo-local-1",
		ChannelID:          "channel-0",
		DestinationDenom:   "ibc/uethusdc",
		Enabled:            true,
	}); err != nil {
		t.Fatalf("set route: %v", err)
	}

	packet, err := keeper.SendIBCPacket("eth.usdc", big.NewInt(25000000), "osmo1receiver", 120, "swap:uosmo")
	if err != nil {
		t.Fatalf("send ibc packet: %v", err)
	}
	if packet.TransferID == "" {
		t.Fatal("expected transfer id")
	}
	if packet.Sequence == 0 {
		t.Fatal("expected packet sequence")
	}

	completed, err := keeper.ReceiveIBCAck(packet.TransferID, true, "")
	if err != nil {
		t.Fatalf("receive successful ack: %v", err)
	}
	if completed.Status != TransferStatusCompleted {
		t.Fatalf("expected completed transfer, got %q", completed.Status)
	}

	packet, err = keeper.SendIBCPacket("eth.usdc", big.NewInt(25000000), "osmo1receiver", 140, "")
	if err != nil {
		t.Fatalf("send ibc packet: %v", err)
	}
	failed, err := keeper.ReceiveIBCAck(packet.TransferID, false, "destination failure")
	if err != nil {
		t.Fatalf("receive failed ack: %v", err)
	}
	if failed.Status != TransferStatusAckFailed {
		t.Fatalf("expected ack_failed transfer, got %q", failed.Status)
	}

	packet, err = keeper.SendIBCPacket("eth.usdc", big.NewInt(25000000), "osmo1receiver", 160, "")
	if err != nil {
		t.Fatalf("send ibc packet: %v", err)
	}
	timedOut, err := keeper.HandleIBCTimeout(packet.TransferID)
	if err != nil {
		t.Fatalf("timeout packet: %v", err)
	}
	if timedOut.Status != TransferStatusTimedOut {
		t.Fatalf("expected timed_out transfer, got %q", timedOut.Status)
	}

	refunded, err := keeper.MarkRefunded(packet.TransferID)
	if err != nil {
		t.Fatalf("refund timed out transfer: %v", err)
	}
	if refunded.Status != TransferStatusRefunded {
		t.Fatalf("expected refunded transfer, got %q", refunded.Status)
	}
}
