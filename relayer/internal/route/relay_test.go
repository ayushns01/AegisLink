package route

import (
	"context"
	"errors"
	"testing"
)

func TestRelayerRunOnceCompletesPendingTransfersOnSuccessfulAck(t *testing.T) {
	t.Parallel()

	source := &stubSource{
		transfers: []Transfer{
			{TransferID: "ibc/eth.usdc/1", AssetID: "eth.usdc", Amount: "25000000", Receiver: "osmo1recipient", Status: "pending"},
		},
	}
	sink := &stubSink{}
	target := &stubTarget{
		acks: map[string]Ack{
			"ibc/eth.usdc/1": {Status: AckStatusCompleted},
		},
	}

	relayer := NewRelayer(source, sink, target)
	if err := relayer.RunOnce(context.Background()); err != nil {
		t.Fatalf("run once: %v", err)
	}
	if len(target.calls) != 1 {
		t.Fatalf("expected one target call, got %d", len(target.calls))
	}
	if len(sink.completed) != 1 || sink.completed[0] != "ibc/eth.usdc/1" {
		t.Fatalf("expected completed transfer ibc/eth.usdc/1, got %v", sink.completed)
	}
	if len(sink.failed) != 0 {
		t.Fatalf("expected no failed transfers, got %v", sink.failed)
	}
	if len(sink.timedOut) != 0 {
		t.Fatalf("expected no timed out transfers, got %v", sink.timedOut)
	}
}

func TestRelayerRunOnceMarksFailedTransfersOnNegativeAck(t *testing.T) {
	t.Parallel()

	source := &stubSource{
		transfers: []Transfer{
			{TransferID: "ibc/eth.usdc/1", AssetID: "eth.usdc", Amount: "25000000", Receiver: "osmo1recipient", Status: "pending"},
		},
	}
	sink := &stubSink{}
	target := &stubTarget{
		acks: map[string]Ack{
			"ibc/eth.usdc/1": {Status: AckStatusFailed, Reason: "mock ack failed"},
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
}

func TestRelayerRunOnceMarksTimedOutTransfersOnTargetTimeout(t *testing.T) {
	t.Parallel()

	source := &stubSource{
		transfers: []Transfer{
			{TransferID: "ibc/eth.usdc/1", AssetID: "eth.usdc", Amount: "25000000", Receiver: "osmo1recipient", Status: "pending"},
		},
	}
	sink := &stubSink{}
	target := &stubTarget{
		errs: map[string]error{
			"ibc/eth.usdc/1": TimeoutError{Err: errors.New("mock timeout")},
		},
	}

	relayer := NewRelayer(source, sink, target)
	if err := relayer.RunOnce(context.Background()); err != nil {
		t.Fatalf("run once: %v", err)
	}
	if len(sink.timedOut) != 1 || sink.timedOut[0] != "ibc/eth.usdc/1" {
		t.Fatalf("expected timed out transfer ibc/eth.usdc/1, got %v", sink.timedOut)
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
	acks  map[string]Ack
	errs  map[string]error
	calls []Transfer
}

func (s *stubTarget) SubmitTransfer(_ context.Context, transfer Transfer) (Ack, error) {
	s.calls = append(s.calls, transfer)
	if err := s.errs[transfer.TransferID]; err != nil {
		return Ack{}, err
	}
	return s.acks[transfer.TransferID], nil
}
