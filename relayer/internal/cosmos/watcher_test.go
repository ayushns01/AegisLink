package cosmos

import (
	"context"
	"math/big"
	"testing"

	bridgetypes "github.com/ayushns01/aegislink/chain/aegislink/x/bridge/types"
)

func TestWatcherObserveReturnsOnlyFinalizedWithdrawalsInOrder(t *testing.T) {
	t.Parallel()

	source := &stubWithdrawalSource{
		latestHeight: 9,
		withdrawals: []Withdrawal{
			newWithdrawal(7, 2, 11),
			newWithdrawal(5, 0, 7),
			newWithdrawal(8, 0, 13),
			newWithdrawal(7, 1, 10),
		},
	}

	watcher := NewWatcher(source, 1)

	withdrawals, nextCursor, err := watcher.Observe(context.Background(), 5)
	if err != nil {
		t.Fatalf("expected observe to succeed, got error: %v", err)
	}
	if nextCursor != 9 {
		t.Fatalf("expected next cursor 9, got %d", nextCursor)
	}
	if len(source.calls) != 1 {
		t.Fatalf("expected one source query, got %d", len(source.calls))
	}
	if source.calls[0].from != 5 || source.calls[0].to != 8 {
		t.Fatalf("expected query range [5,8], got [%d,%d]", source.calls[0].from, source.calls[0].to)
	}
	if len(withdrawals) != 4 {
		t.Fatalf("expected 4 finalized withdrawals, got %d", len(withdrawals))
	}
	if withdrawals[0].BlockHeight != 5 || withdrawals[0].Identity.SourceLogIndex != 0 {
		t.Fatalf("expected first withdrawal to be the lowest height/index, got height=%d index=%d", withdrawals[0].BlockHeight, withdrawals[0].Identity.SourceLogIndex)
	}
	if withdrawals[1].Identity.SourceLogIndex != 1 || withdrawals[2].Identity.SourceLogIndex != 2 {
		t.Fatalf("expected height-7 withdrawals to be sorted by log index, got %d then %d", withdrawals[1].Identity.SourceLogIndex, withdrawals[2].Identity.SourceLogIndex)
	}
}

func TestWatcherObserveWaitsForFinalizedHeightBeforeReturningWithdrawal(t *testing.T) {
	t.Parallel()

	source := &stubWithdrawalSource{
		latestHeight: 20,
		withdrawals:  []Withdrawal{newWithdrawal(20, 0, 3)},
	}
	watcher := NewWatcher(source, 1)

	withdrawals, nextCursor, err := watcher.Observe(context.Background(), 20)
	if err != nil {
		t.Fatalf("expected observe to succeed, got error: %v", err)
	}
	if len(withdrawals) != 0 {
		t.Fatalf("expected no finalized withdrawals yet, got %d", len(withdrawals))
	}
	if nextCursor != 20 {
		t.Fatalf("expected cursor to stay at 20 until finality, got %d", nextCursor)
	}

	source.latestHeight = 21
	withdrawals, nextCursor, err = watcher.Observe(context.Background(), 20)
	if err != nil {
		t.Fatalf("expected observe to succeed after finality, got error: %v", err)
	}
	if len(withdrawals) != 1 {
		t.Fatalf("expected finalized withdrawal after confirmation, got %d", len(withdrawals))
	}
	if nextCursor != 21 {
		t.Fatalf("expected cursor to advance to 21, got %d", nextCursor)
	}
}

type stubWithdrawalSource struct {
	latestHeight uint64
	withdrawals  []Withdrawal
	calls        []heightRange
}

type heightRange struct {
	from uint64
	to   uint64
}

func (s *stubWithdrawalSource) LatestHeight(context.Context) (uint64, error) {
	return s.latestHeight, nil
}

func (s *stubWithdrawalSource) Withdrawals(_ context.Context, fromHeight, toHeight uint64) ([]Withdrawal, error) {
	s.calls = append(s.calls, heightRange{from: fromHeight, to: toHeight})

	var withdrawals []Withdrawal
	for _, withdrawal := range s.withdrawals {
		if withdrawal.BlockHeight < fromHeight || withdrawal.BlockHeight > toHeight {
			continue
		}
		withdrawals = append(withdrawals, withdrawal)
	}
	return withdrawals, nil
}

func newWithdrawal(height, logIndex, nonce uint64) Withdrawal {
	identity := bridgetypes.ClaimIdentity{
		Kind:           bridgetypes.ClaimKindWithdrawal,
		SourceChainID:  "aegislink-1",
		SourceContract: "bridge",
		SourceTxHash:   "0xcosmos-tx",
		SourceLogIndex: logIndex,
		Nonce:          nonce,
	}
	identity.MessageID = identity.DerivedMessageID()

	return Withdrawal{
		BlockHeight:  height,
		Identity:     identity,
		AssetID:      "uusdc",
		AssetAddress: "0xasset",
		Amount:       big.NewInt(18),
		Recipient:    "0xrecipient",
		Deadline:     99,
		Signature:    []byte("proof"),
	}
}
