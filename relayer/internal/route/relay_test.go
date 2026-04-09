package route

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
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

func TestParseRouteActionSupportsStakeAction(t *testing.T) {
	t.Parallel()

	action, actionErr := parseRouteAction("stake:ibc/uethusdc:recipient=osmo1validator:path=validator-1")
	if actionErr != "" {
		t.Fatalf("expected no action error, got %q", actionErr)
	}
	if action == nil {
		t.Fatal("expected parsed stake action")
	}
	if action.Type != "stake" {
		t.Fatalf("expected stake action type, got %q", action.Type)
	}
	if action.TargetDenom != "ibc/uethusdc" {
		t.Fatalf("expected target denom ibc/uethusdc, got %q", action.TargetDenom)
	}
	if action.Recipient != "osmo1validator" {
		t.Fatalf("expected recipient osmo1validator, got %q", action.Recipient)
	}
	if action.Path != "validator-1" {
		t.Fatalf("expected route path validator-1, got %q", action.Path)
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
			memo:       "farm:uosmo",
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

func TestRelayerRunLoopPollsWithoutDuplicateRedelivery(t *testing.T) {
	t.Parallel()

	source := &statefulLoopSource{
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
	sink := &statefulLoopSink{
		stubSink: &stubSink{},
		source:   source,
	}
	target := &loopTarget{
		submitAck: Ack{Status: AckStatusReceived},
		readyAck:  AckRecord{TransferID: "ibc/eth.usdc/1", Status: AckStatusCompleted},
	}

	relayer := NewRelayer(source, sink, target)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- relayer.RunLoop(ctx, LoopConfig{PollInterval: time.Millisecond})
	}()

	deadline := time.Now().Add(250 * time.Millisecond)
	for time.Now().Before(deadline) {
		if len(target.calls) == 1 && len(sink.completed) == 1 && source.calls.Load() >= 2 {
			break
		}
		time.Sleep(time.Millisecond)
	}
	cancel()

	if err := <-done; err != nil {
		t.Fatalf("run loop: %v", err)
	}
	if len(target.calls) != 1 {
		t.Fatalf("expected one transfer delivery across loop, got %d", len(target.calls))
	}
	if len(sink.completed) != 1 || sink.completed[0] != "ibc/eth.usdc/1" {
		t.Fatalf("expected completed transfer once, got %v", sink.completed)
	}
	if got := source.calls.Load(); got < 2 {
		t.Fatalf("expected repeated polling, got %d source calls", got)
	}
}

func TestRelayerRunLoopBacksOffAfterTemporaryFailure(t *testing.T) {
	t.Parallel()

	source := &erroringLoopSource{
		errs: []error{
			TemporaryError{Err: errors.New("target unavailable")},
			nil,
			nil,
		},
	}
	relayer := NewRelayer(source, &stubSink{}, &stubTarget{})

	start := time.Now()
	if err := relayer.RunLoop(context.Background(), LoopConfig{
		PollInterval:       time.Millisecond,
		FailureBackoff:     40 * time.Millisecond,
		MaxConsecutiveRuns: 3,
	}); err != nil {
		t.Fatalf("run loop: %v", err)
	}
	if got := source.calls.Load(); got < 2 {
		t.Fatalf("expected retry after temporary failure, got %d calls", got)
	}
	if elapsed := time.Since(start); elapsed < 35*time.Millisecond {
		t.Fatalf("expected backoff delay, run finished too quickly: %s", elapsed)
	}
}

func TestRelayerRunLoopStopsGracefullyOnContextCancel(t *testing.T) {
	t.Parallel()

	relayer := NewRelayer(&blockingLoopSource{}, &stubSink{}, &stubTarget{})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- relayer.RunLoop(ctx, LoopConfig{PollInterval: 5 * time.Millisecond})
	}()

	time.Sleep(10 * time.Millisecond)
	cancel()

	if err := <-done; err != nil {
		t.Fatalf("expected graceful shutdown, got %v", err)
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

type statefulLoopSource struct {
	mu        sync.Mutex
	transfers []Transfer
	calls     atomic.Int32
}

func (s *statefulLoopSource) PendingTransfers(context.Context) ([]Transfer, error) {
	s.calls.Add(1)
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Transfer, len(s.transfers))
	copy(out, s.transfers)
	return out, nil
}

func (s *statefulLoopSource) complete(transferID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	filtered := s.transfers[:0]
	for _, transfer := range s.transfers {
		if transfer.TransferID == transferID {
			continue
		}
		filtered = append(filtered, transfer)
	}
	s.transfers = filtered
}

type statefulLoopSink struct {
	*stubSink
	source *statefulLoopSource
}

func (s *statefulLoopSink) CompleteTransfer(ctx context.Context, transferID string) error {
	if s.stubSink == nil {
		s.stubSink = &stubSink{}
	}
	if err := s.stubSink.CompleteTransfer(ctx, transferID); err != nil {
		return err
	}
	s.source.complete(transferID)
	return nil
}

type loopTarget struct {
	mu        sync.Mutex
	submitAck Ack
	readyAck  AckRecord
	delivered bool
	confirmed []string
	calls     []Transfer
}

func (t *loopTarget) SubmitTransfer(_ context.Context, transfer Transfer) (Ack, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.calls = append(t.calls, transfer)
	t.delivered = true
	return t.submitAck, nil
}

func (t *loopTarget) ReadyAcks(context.Context) ([]AckRecord, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.delivered || t.readyAck.TransferID == "" {
		return nil, nil
	}
	if len(t.confirmed) > 0 {
		return nil, nil
	}
	return []AckRecord{t.readyAck}, nil
}

func (t *loopTarget) ConfirmAck(_ context.Context, transferID string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.confirmed = append(t.confirmed, transferID)
	return nil
}

type erroringLoopSource struct {
	errs  []error
	calls atomic.Int32
}

func (s *erroringLoopSource) PendingTransfers(context.Context) ([]Transfer, error) {
	call := int(s.calls.Add(1))
	if call <= len(s.errs) && s.errs[call-1] != nil {
		return nil, s.errs[call-1]
	}
	return nil, nil
}

type blockingLoopSource struct {
	once sync.Once
}

func (s *blockingLoopSource) PendingTransfers(ctx context.Context) ([]Transfer, error) {
	s.once.Do(func() {
		<-ctx.Done()
	})
	return nil, ctx.Err()
}
