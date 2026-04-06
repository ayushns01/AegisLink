package route

import (
	"context"
	"strings"
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
	summary, err := relayer.RunOnceWithSummary(context.Background())
	if err != nil {
		t.Fatalf("run once: %v", err)
	}
	if summary.TransfersObserved != 1 || summary.TransfersDelivered != 1 {
		t.Fatalf("unexpected transfer summary: %+v", summary)
	}
	if summary.ReceivedDeliveries != 1 {
		t.Fatalf("expected one received delivery summary, got %+v", summary)
	}
	if summary.ReadyAcks != 0 || summary.CompletedAcks != 0 || summary.FailedAcks != 0 || summary.TimedOutAcks != 0 {
		t.Fatalf("unexpected ack summary: %+v", summary)
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

func TestParseRouteActionSupportsRecipientPathAndMinOut(t *testing.T) {
	t.Parallel()

	action, actionErr := parseRouteAction("swap:uosmo:min_out=50000000:recipient=osmo1override:path=pool-7")
	if actionErr != "" {
		t.Fatalf("expected no action error, got %q", actionErr)
	}
	if action == nil {
		t.Fatal("expected parsed action")
	}
	if action.Type != "swap" {
		t.Fatalf("expected swap action type, got %q", action.Type)
	}
	if action.TargetDenom != "uosmo" {
		t.Fatalf("expected target denom uosmo, got %q", action.TargetDenom)
	}
	if action.MinOut != "50000000" {
		t.Fatalf("expected min_out 50000000, got %q", action.MinOut)
	}
	if action.Recipient != "osmo1override" {
		t.Fatalf("expected recipient override osmo1override, got %q", action.Recipient)
	}
	if action.Path != "pool-7" {
		t.Fatalf("expected route path pool-7, got %q", action.Path)
	}
}

func TestParseRouteActionReportsMalformedOrUnsupportedAction(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		memo       string
		wantErrSub string
	}{
		{
			name:       "missing target denom",
			memo:       "swap:",
			wantErrSub: "missing target denom",
		},
		{
			name:       "unsupported option",
			memo:       "swap:uosmo:slippage=10",
			wantErrSub: "unsupported swap option",
		},
		{
			name:       "unsupported action",
			memo:       "stake:uosmo",
			wantErrSub: "unsupported route action",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			action, actionErr := parseRouteAction(tc.memo)
			if action != nil {
				t.Fatalf("expected nil action for %q, got %+v", tc.memo, action)
			}
			if actionErr == "" {
				t.Fatalf("expected action error for %q", tc.memo)
			}
			if want := tc.wantErrSub; want != "" && !strings.Contains(actionErr, want) {
				t.Fatalf("expected action error containing %q, got %q", want, actionErr)
			}
		})
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
