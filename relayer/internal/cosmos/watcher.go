package cosmos

import (
	"context"
	"sort"
)

type Watcher struct {
	source        WithdrawalSource
	confirmations uint64
}

func NewWatcher(source WithdrawalSource, confirmations uint64) *Watcher {
	return &Watcher{source: source, confirmations: confirmations}
}

func (w *Watcher) Observe(ctx context.Context, fromHeight uint64) ([]Withdrawal, uint64, error) {
	if err := ctx.Err(); err != nil {
		return nil, fromHeight, err
	}
	if w == nil || w.source == nil {
		return nil, fromHeight, ErrSourceUnavailable
	}

	latest, err := w.source.LatestHeight(ctx)
	if err != nil {
		return nil, fromHeight, err
	}
	if latest < w.confirmations {
		return nil, fromHeight, nil
	}

	finalizedTip := latest - w.confirmations
	if fromHeight > finalizedTip {
		return nil, fromHeight, nil
	}

	withdrawals, err := w.source.Withdrawals(ctx, fromHeight, finalizedTip)
	if err != nil {
		return nil, fromHeight, err
	}

	sort.SliceStable(withdrawals, func(i, j int) bool {
		if withdrawals[i].BlockHeight != withdrawals[j].BlockHeight {
			return withdrawals[i].BlockHeight < withdrawals[j].BlockHeight
		}
		if withdrawals[i].Identity.SourceLogIndex != withdrawals[j].Identity.SourceLogIndex {
			return withdrawals[i].Identity.SourceLogIndex < withdrawals[j].Identity.SourceLogIndex
		}
		return withdrawals[i].Identity.SourceTxHash < withdrawals[j].Identity.SourceTxHash
	})

	if finalizedTip == ^uint64(0) {
		return withdrawals, finalizedTip, nil
	}
	return withdrawals, finalizedTip + 1, nil
}
