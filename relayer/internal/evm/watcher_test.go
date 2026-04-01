package evm

import (
	"context"
	"math/big"
	"testing"
)

func TestWatcherObserveReturnsOnlyFinalizedDepositsInOrder(t *testing.T) {
	t.Parallel()

	source := &stubDepositLogSource{
		latestBlock: 12,
		logs: []DepositEvent{
			newDepositEvent(7, 2, 11),
			newDepositEvent(5, 0, 7),
			newDepositEvent(11, 0, 13),
			newDepositEvent(7, 1, 10),
		},
	}

	watcher := NewWatcher(source, 2)

	events, nextCursor, err := watcher.Observe(context.Background(), 5)
	if err != nil {
		t.Fatalf("expected observe to succeed, got error: %v", err)
	}

	if nextCursor != 11 {
		t.Fatalf("expected next cursor 11, got %d", nextCursor)
	}

	if len(source.calls) != 1 {
		t.Fatalf("expected one source query, got %d", len(source.calls))
	}
	if source.calls[0].from != 5 || source.calls[0].to != 10 {
		t.Fatalf("expected query range [5,10], got [%d,%d]", source.calls[0].from, source.calls[0].to)
	}

	if len(events) != 3 {
		t.Fatalf("expected 3 finalized events, got %d", len(events))
	}

	got := []struct {
		block uint64
		index uint64
	}{
		{events[0].BlockNumber, events[0].LogIndex},
		{events[1].BlockNumber, events[1].LogIndex},
		{events[2].BlockNumber, events[2].LogIndex},
	}
	want := []struct {
		block uint64
		index uint64
	}{
		{5, 0},
		{7, 1},
		{7, 2},
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("event %d mismatch: want %+v, got %+v", i, want[i], got[i])
		}
	}
}

func TestWatcherObserveWaitsForConfirmationsBeforeReturningDeposit(t *testing.T) {
	t.Parallel()

	source := &stubDepositLogSource{
		latestBlock: 10,
		logs:        []DepositEvent{newDepositEvent(9, 0, 1)},
	}
	watcher := NewWatcher(source, 2)

	events, nextCursor, err := watcher.Observe(context.Background(), 9)
	if err != nil {
		t.Fatalf("expected observe to succeed, got error: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("expected no finalized events yet, got %d", len(events))
	}
	if nextCursor != 9 {
		t.Fatalf("expected cursor to stay at 9 until event finalizes, got %d", nextCursor)
	}
	if len(source.calls) != 0 {
		t.Fatalf("expected no log query before finality, got %d", len(source.calls))
	}

	source.latestBlock = 11
	events, nextCursor, err = watcher.Observe(context.Background(), 9)
	if err != nil {
		t.Fatalf("expected observe to succeed after finality, got error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected finalized event after confirmation, got %d", len(events))
	}
	if nextCursor != 10 {
		t.Fatalf("expected cursor to advance to 10, got %d", nextCursor)
	}
}

type stubDepositLogSource struct {
	latestBlock uint64
	logs        []DepositEvent
	calls       []depositRange
}

type depositRange struct {
	from uint64
	to   uint64
}

func (s *stubDepositLogSource) LatestBlock(context.Context) (uint64, error) {
	return s.latestBlock, nil
}

func (s *stubDepositLogSource) DepositEvents(_ context.Context, fromBlock, toBlock uint64) ([]DepositEvent, error) {
	s.calls = append(s.calls, depositRange{from: fromBlock, to: toBlock})

	var events []DepositEvent
	for _, event := range s.logs {
		if event.BlockNumber < fromBlock || event.BlockNumber > toBlock {
			continue
		}
		events = append(events, event)
	}
	return events, nil
}

func newDepositEvent(blockNumber, logIndex, nonce uint64) DepositEvent {
	return DepositEvent{
		BlockNumber:    blockNumber,
		SourceChainID:  "11155111",
		SourceContract: "0xgateway",
		TxHash:         "0xtx",
		LogIndex:       logIndex,
		Nonce:          nonce,
		DepositID:      "deposit-id",
		MessageID:      "message-id",
		AssetAddress:   "0xasset",
		AssetID:        "uusdc",
		Amount:         big.NewInt(25),
		Recipient:      "aegis1recipient",
		Expiry:         99,
	}
}
