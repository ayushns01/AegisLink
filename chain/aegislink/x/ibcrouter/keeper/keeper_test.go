package keeper

import (
	"errors"
	"math/big"
	"testing"
)

func TestInitiateTransferCreatesPendingRequestForEnabledRoute(t *testing.T) {
	t.Parallel()

	k := NewKeeper()
	if err := k.SetRoute(Route{
		AssetID:            "eth.usdc",
		DestinationChainID: "osmosis-1",
		ChannelID:          "channel-0",
		DestinationDenom:   "ibc/usdc",
		Enabled:            true,
	}); err != nil {
		t.Fatalf("set route: %v", err)
	}

	transfer, err := k.InitiateTransfer("eth.usdc", mustAmount("25000000"), "osmo1recipient", 120, "swap")
	if err != nil {
		t.Fatalf("initiate transfer: %v", err)
	}
	if transfer.Status != TransferStatusPending {
		t.Fatalf("expected pending status, got %q", transfer.Status)
	}
	if transfer.ChannelID != "channel-0" {
		t.Fatalf("expected channel-0, got %q", transfer.ChannelID)
	}
	if transfer.DestinationChainID != "osmosis-1" {
		t.Fatalf("expected osmosis-1, got %q", transfer.DestinationChainID)
	}
	if transfer.DestinationDenom != "ibc/usdc" {
		t.Fatalf("expected ibc/usdc, got %q", transfer.DestinationDenom)
	}
}

func TestInitiateTransferRejectsDisabledRoute(t *testing.T) {
	t.Parallel()

	k := NewKeeper()
	if err := k.SetRoute(Route{
		AssetID:            "eth.usdc",
		DestinationChainID: "osmosis-1",
		ChannelID:          "channel-0",
		DestinationDenom:   "ibc/usdc",
		Enabled:            false,
	}); err != nil {
		t.Fatalf("set route: %v", err)
	}

	_, err := k.InitiateTransfer("eth.usdc", mustAmount("1"), "osmo1recipient", 120, "")
	if !errors.Is(err, ErrRouteDisabled) {
		t.Fatalf("expected disabled route error, got %v", err)
	}
}

func TestAcknowledgeFailureAndRefundExposeRecoverableState(t *testing.T) {
	t.Parallel()

	k := seededRouterKeeper(t)
	transfer, err := k.InitiateTransfer("eth.usdc", mustAmount("25000000"), "osmo1recipient", 120, "swap")
	if err != nil {
		t.Fatalf("initiate transfer: %v", err)
	}

	failed, err := k.AcknowledgeFailure(transfer.TransferID, "ack failed")
	if err != nil {
		t.Fatalf("ack failure: %v", err)
	}
	if failed.Status != TransferStatusAckFailed {
		t.Fatalf("expected ack_failed status, got %q", failed.Status)
	}
	if failed.FailureReason != "ack failed" {
		t.Fatalf("expected failure reason ack failed, got %q", failed.FailureReason)
	}

	refunded, err := k.MarkRefunded(transfer.TransferID)
	if err != nil {
		t.Fatalf("mark refunded: %v", err)
	}
	if refunded.Status != TransferStatusRefunded {
		t.Fatalf("expected refunded status, got %q", refunded.Status)
	}
}

func TestTimeoutTransferMarksRecoverableState(t *testing.T) {
	t.Parallel()

	k := seededRouterKeeper(t)
	transfer, err := k.InitiateTransfer("eth.usdc", mustAmount("25000000"), "osmo1recipient", 120, "")
	if err != nil {
		t.Fatalf("initiate transfer: %v", err)
	}

	timedOut, err := k.TimeoutTransfer(transfer.TransferID)
	if err != nil {
		t.Fatalf("timeout transfer: %v", err)
	}
	if timedOut.Status != TransferStatusTimedOut {
		t.Fatalf("expected timed_out status, got %q", timedOut.Status)
	}
}

func TestAcknowledgeSuccessMarksTransferCompleted(t *testing.T) {
	t.Parallel()

	k := seededRouterKeeper(t)
	transfer, err := k.InitiateTransfer("eth.usdc", mustAmount("25000000"), "osmo1recipient", 120, "")
	if err != nil {
		t.Fatalf("initiate transfer: %v", err)
	}

	completed, err := k.AcknowledgeSuccess(transfer.TransferID)
	if err != nil {
		t.Fatalf("ack success: %v", err)
	}
	if completed.Status != TransferStatusCompleted {
		t.Fatalf("expected completed status, got %q", completed.Status)
	}
}

func TestTransportSessionTracksHandshakePacketAndAckStages(t *testing.T) {
	t.Parallel()

	k := seededRouterKeeper(t)
	transfer, err := k.InitiateTransfer("eth.usdc", mustAmount("25000000"), "osmo1recipient", 120, "swap:uosmo")
	if err != nil {
		t.Fatalf("initiate transfer: %v", err)
	}

	session, ok := k.TransportSession(transfer.TransferID)
	if !ok {
		t.Fatal("expected transport session to exist")
	}
	if session.Stage != TransportStageHandshakePending {
		t.Fatalf("expected handshake_pending stage, got %q", session.Stage)
	}

	session, err = k.OpenTransportSession(transfer.TransferID)
	if err != nil {
		t.Fatalf("open transport session: %v", err)
	}
	if session.Stage != TransportStageHandshakeOpen {
		t.Fatalf("expected handshake_open stage, got %q", session.Stage)
	}

	session, err = k.MarkPacketRelayed(transfer.TransferID)
	if err != nil {
		t.Fatalf("mark packet relayed: %v", err)
	}
	if session.Stage != TransportStagePacketRelayed {
		t.Fatalf("expected packet_relayed stage, got %q", session.Stage)
	}

	if _, err := k.AcknowledgeSuccess(transfer.TransferID); err != nil {
		t.Fatalf("ack success: %v", err)
	}
	session, ok = k.TransportSession(transfer.TransferID)
	if !ok {
		t.Fatal("expected transport session after ack success")
	}
	if session.Stage != TransportStageAckCompleted {
		t.Fatalf("expected ack_completed stage, got %q", session.Stage)
	}
}

func TestRouteStateMachineAllowsOnlyRecoverableRefunds(t *testing.T) {
	t.Parallel()

	k := seededRouterKeeper(t)
	transfer, err := k.InitiateTransfer("eth.usdc", mustAmount("25000000"), "osmo1recipient", 120, "swap")
	if err != nil {
		t.Fatalf("initiate transfer: %v", err)
	}
	if _, err := k.MarkRefunded(transfer.TransferID); !errors.Is(err, ErrTransferNotRecoverable) {
		t.Fatalf("expected pending transfer refund to fail, got %v", err)
	}

	failed, err := k.AcknowledgeFailure(transfer.TransferID, "swap failed")
	if err != nil {
		t.Fatalf("ack failure: %v", err)
	}
	if failed.Status != TransferStatusAckFailed {
		t.Fatalf("expected ack_failed status, got %q", failed.Status)
	}

	refunded, err := k.MarkRefunded(transfer.TransferID)
	if err != nil {
		t.Fatalf("refund failed transfer: %v", err)
	}
	if refunded.Status != TransferStatusRefunded {
		t.Fatalf("expected refunded status, got %q", refunded.Status)
	}
	if _, err := k.MarkRefunded(transfer.TransferID); !errors.Is(err, ErrTransferNotRecoverable) {
		t.Fatalf("expected second refund to fail, got %v", err)
	}
}

func seededRouterKeeper(t *testing.T) *Keeper {
	t.Helper()

	k := NewKeeper()
	if err := k.SetRoute(Route{
		AssetID:            "eth.usdc",
		DestinationChainID: "osmosis-1",
		ChannelID:          "channel-0",
		DestinationDenom:   "ibc/usdc",
		Enabled:            true,
	}); err != nil {
		t.Fatalf("set route: %v", err)
	}
	return k
}

func mustAmount(value string) *big.Int {
	amount, ok := new(big.Int).SetString(value, 10)
	if !ok {
		panic("invalid amount " + value)
	}
	return amount
}
