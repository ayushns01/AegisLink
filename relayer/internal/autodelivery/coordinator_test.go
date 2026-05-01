package autodelivery

import (
	"context"
	"errors"
	"testing"
)

func TestCoordinatorRunOnceInitiatesAndFlushesWhenClaimedIntentIsReady(t *testing.T) {
	t.Parallel()

	intents := &stubIntentSource{
		intents: []Intent{
			{
				SourceTxHash: "0x422d075a86656b27694780b3ad553abee1dded6f3fb5bfa805137a3da64f30b8",
				Sender:       "cosmos1q5nq6v24qq0584nf00wuhqrku4anlxaq80aqy4",
				RouteID:      "osmosis-public-wallet",
				AssetID:      "eth",
				Amount:       "1000000000000000",
				Receiver:     "osmo1q5nq6v24qq0584nf00wuhqrku4anlxaq05wsj8",
			},
		},
	}
	statuses := &stubStatusSource{
		byTxHash: map[string]BridgeStatus{
			"0x422d075a86656b27694780b3ad553abee1dded6f3fb5bfa805137a3da64f30b8": {
				Status: "aegislink_processing",
			},
		},
	}
	submitter := &stubTransferSubmitter{
		result: SubmittedTransfer{
			TransferID: "ibc/eth/1",
			ChannelID:  "channel-0",
			Status:     "pending",
		},
	}
	flusher := &stubFlusher{}

	coordinator := NewCoordinator(intents, statuses, submitter, flusher)

	summary, err := coordinator.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("run once: %v", err)
	}
	if summary.IntentsObserved != 1 || summary.TransfersInitiated != 1 || summary.FlushesTriggered != 1 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
	if len(submitter.calls) != 1 {
		t.Fatalf("expected one transfer initiation, got %d", len(submitter.calls))
	}
	if submitter.calls[0].Receiver != "osmo1q5nq6v24qq0584nf00wuhqrku4anlxaq05wsj8" {
		t.Fatalf("unexpected initiation payload: %+v", submitter.calls[0])
	}
	if len(flusher.calls) != 1 || flusher.calls[0].channelID != "channel-0" || flusher.calls[0].routeID != "osmosis-public-wallet" {
		t.Fatalf("expected one flush for channel-0 with osmosis route, got %v", flusher.calls)
	}
}

func TestCoordinatorRunOnceWaitsWhileSepoliaConfirmationIsPending(t *testing.T) {
	t.Parallel()

	intents := &stubIntentSource{
		intents: []Intent{
			{
				SourceTxHash: "0xstill-pending",
				Sender:       "cosmos1sender",
				RouteID:      "osmosis-public-wallet",
				AssetID:      "eth",
				Amount:       "42",
				Receiver:     "osmo1pending",
			},
		},
	}
	statuses := &stubStatusSource{
		byTxHash: map[string]BridgeStatus{
			"0xstill-pending": {Status: "sepolia_confirming"},
		},
	}
	submitter := &stubTransferSubmitter{}
	flusher := &stubFlusher{}

	coordinator := NewCoordinator(intents, statuses, submitter, flusher)

	summary, err := coordinator.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("run once: %v", err)
	}
	if summary.IntentsObserved != 1 || summary.IntentsWaiting != 1 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
	if len(submitter.calls) != 0 {
		t.Fatalf("expected no initiation calls, got %d", len(submitter.calls))
	}
	if len(flusher.calls) != 0 {
		t.Fatalf("expected no flushes, got %v", flusher.calls)
	}
}

func TestCoordinatorRunOnceReturnsFlushErrors(t *testing.T) {
	t.Parallel()

	coordinator := NewCoordinator(
		&stubIntentSource{
			intents: []Intent{{
				SourceTxHash: "0xready",
				Sender:       "cosmos1sender",
				RouteID:      "osmosis-public-wallet",
				AssetID:      "eth",
				Amount:       "42",
				Receiver:     "osmo1ready",
			}},
		},
		&stubStatusSource{
			byTxHash: map[string]BridgeStatus{
				"0xready": {Status: "aegislink_processing"},
			},
		},
		&stubTransferSubmitter{
			result: SubmittedTransfer{TransferID: "ibc/eth/2", ChannelID: "channel-7", Status: "pending"},
		},
		&stubFlusher{err: errors.New("flush failed")},
	)

	if _, err := coordinator.RunOnce(context.Background()); err == nil {
		t.Fatal("expected flush failure")
	}
}

type stubIntentSource struct {
	intents []Intent
	err     error
}

func (s *stubIntentSource) PendingIntents(context.Context) ([]Intent, error) {
	if s.err != nil {
		return nil, s.err
	}
	return append([]Intent(nil), s.intents...), nil
}

type stubStatusSource struct {
	byTxHash map[string]BridgeStatus
	err      error
}

func (s *stubStatusSource) QueryStatus(_ context.Context, sourceTxHash string) (BridgeStatus, error) {
	if s.err != nil {
		return BridgeStatus{}, s.err
	}
	return s.byTxHash[sourceTxHash], nil
}

type stubTransferSubmitter struct {
	calls  []Intent
	result SubmittedTransfer
	err    error
}

func (s *stubTransferSubmitter) InitiateTransfer(_ context.Context, intent Intent) (SubmittedTransfer, error) {
	s.calls = append(s.calls, intent)
	if s.err != nil {
		return SubmittedTransfer{}, s.err
	}
	return s.result, nil
}

type stubFlusher struct {
	calls []struct{ routeID, channelID string }
	err   error
}

func (s *stubFlusher) Flush(_ context.Context, routeID, channelID string) error {
	s.calls = append(s.calls, struct{ routeID, channelID string }{routeID, channelID})
	return s.err
}
