package route

import (
	"context"
	"testing"
)

func TestRelayerRunOnceCompletesReadyAckBeforeSubmittingTransfers(t *testing.T) {
	t.Parallel()

	source := &stubSource{}
	sink := &stubSink{}
	target := &stubTarget{
		readyAcks: []AckRecord{
			{TransferID: "ibc/eth.usdc/1", Status: AckStatusCompleted},
		},
	}

	relayer := NewRelayer(source, sink, target)
	if err := relayer.RunOnce(context.Background()); err != nil {
		t.Fatalf("run once: %v", err)
	}
	if len(sink.completed) != 1 || sink.completed[0] != "ibc/eth.usdc/1" {
		t.Fatalf("expected completed transfer ibc/eth.usdc/1, got %v", sink.completed)
	}
	if len(target.confirmed) != 1 || target.confirmed[0] != "ibc/eth.usdc/1" {
		t.Fatalf("expected confirmed ack for ibc/eth.usdc/1, got %v", target.confirmed)
	}
	if len(target.calls) != 0 {
		t.Fatalf("expected no transfer submissions, got %d", len(target.calls))
	}
}

func TestRelayerRunOnceMarksFailedTransfersFromReadyAck(t *testing.T) {
	t.Parallel()

	source := &stubSource{}
	sink := &stubSink{}
	target := &stubTarget{
		readyAcks: []AckRecord{
			{TransferID: "ibc/eth.usdc/1", Status: AckStatusFailed, Reason: "mock ack failed"},
		},
	}

	relayer := NewRelayer(source, sink, target)
	if err := relayer.RunOnce(context.Background()); err != nil {
		t.Fatalf("run once: %v", err)
	}
	if len(sink.failed) != 1 {
		t.Fatalf("expected one failed transfer, got %v", sink.failed)
	}
	if sink.failed[0].TransferID != "ibc/eth.usdc/1" || sink.failed[0].Reason != "mock ack failed" {
		t.Fatalf("unexpected failed transfer payload: %+v", sink.failed[0])
	}
	if len(target.confirmed) != 1 || target.confirmed[0] != "ibc/eth.usdc/1" {
		t.Fatalf("expected confirmed failed ack, got %v", target.confirmed)
	}
}

func TestRelayerRunOnceMarksTimedOutTransfersFromReadyAck(t *testing.T) {
	t.Parallel()

	source := &stubSource{}
	sink := &stubSink{}
	target := &stubTarget{
		readyAcks: []AckRecord{
			{TransferID: "ibc/eth.usdc/1", Status: AckStatusTimedOut, Reason: "mock timeout"},
		},
	}

	relayer := NewRelayer(source, sink, target)
	if err := relayer.RunOnce(context.Background()); err != nil {
		t.Fatalf("run once: %v", err)
	}
	if len(sink.timedOut) != 1 || sink.timedOut[0] != "ibc/eth.usdc/1" {
		t.Fatalf("expected timed out transfer ibc/eth.usdc/1, got %v", sink.timedOut)
	}
	if len(target.confirmed) != 1 || target.confirmed[0] != "ibc/eth.usdc/1" {
		t.Fatalf("expected confirmed timed-out ack, got %v", target.confirmed)
	}
}

func TestRelayerRunOnceLeavesTransferPendingAfterDeliveryReceipt(t *testing.T) {
	t.Parallel()

	source := &stubSource{
		transfers: []Transfer{
			{
				TransferID:         "ibc/eth.usdc/1",
				AssetID:            "eth.usdc",
				Amount:             "25000000",
				Receiver:           "osmo1recipient",
				DestinationChainID: "osmosis-1",
				ChannelID:          "channel-0",
				DestinationDenom:   "ibc/uatom-usdc",
				TimeoutHeight:      140,
				Memo:               "swap:uosmo",
				Status:             "pending",
			},
		},
	}
	sink := &stubSink{}
	target := &stubTarget{
		submitAcks: map[string]Ack{
			"ibc/eth.usdc/1": {Status: AckStatusReceived},
		},
	}

	relayer := NewRelayer(source, sink, target)
	if err := relayer.RunOnce(context.Background()); err != nil {
		t.Fatalf("run once: %v", err)
	}
	if len(target.calls) != 1 || target.calls[0].TransferID != "ibc/eth.usdc/1" {
		t.Fatalf("expected one submitted transfer, got %+v", target.calls)
	}
	if len(sink.completed) != 0 || len(sink.failed) != 0 || len(sink.timedOut) != 0 {
		t.Fatalf("expected no sink actions on delivery receipt, got completed=%v failed=%v timedOut=%v", sink.completed, sink.failed, sink.timedOut)
	}
	if len(target.confirmed) != 0 {
		t.Fatalf("expected no confirmed acks, got %v", target.confirmed)
	}
}

type stubSource struct {
	transfers []Transfer
}

func (s *stubSource) PendingTransfers(context.Context) ([]Transfer, error) {
	return append([]Transfer(nil), s.transfers...), nil
}

type failedTransfer struct {
	TransferID string
	Reason     string
}

type stubSink struct {
	completed []string
	failed    []failedTransfer
	timedOut  []string
}

func (s *stubSink) CompleteTransfer(_ context.Context, transferID string) error {
	s.completed = append(s.completed, transferID)
	return nil
}

func (s *stubSink) FailTransfer(_ context.Context, transferID, reason string) error {
	s.failed = append(s.failed, failedTransfer{TransferID: transferID, Reason: reason})
	return nil
}

func (s *stubSink) TimeoutTransfer(_ context.Context, transferID string) error {
	s.timedOut = append(s.timedOut, transferID)
	return nil
}

type stubTarget struct {
	submitAcks map[string]Ack
	readyAcks  []AckRecord
	calls      []Transfer
	confirmed  []string
}

func (s *stubTarget) SubmitTransfer(_ context.Context, transfer Transfer) (Ack, error) {
	s.calls = append(s.calls, transfer)
	return s.submitAcks[transfer.TransferID], nil
}

func (s *stubTarget) ReadyAcks(context.Context) ([]AckRecord, error) {
	return append([]AckRecord(nil), s.readyAcks...), nil
}

func (s *stubTarget) ConfirmAck(_ context.Context, transferID string) error {
	s.confirmed = append(s.confirmed, transferID)
	return nil
}
