package evm

import (
	"context"
	"sort"
)

type Watcher struct {
	source        LogSource
	confirmations uint64
}

func NewWatcher(source LogSource, confirmations uint64) *Watcher {
	return &Watcher{source: source, confirmations: confirmations}
}

func (w *Watcher) Observe(ctx context.Context, fromBlock uint64) ([]DepositEvent, uint64, error) {
	if err := ctx.Err(); err != nil {
		return nil, fromBlock, err
	}
	if w == nil || w.source == nil {
		return nil, fromBlock, ErrSourceUnavailable
	}

	latest, err := w.source.LatestBlock(ctx)
	if err != nil {
		return nil, fromBlock, err
	}
	if latest < w.confirmations {
		return nil, fromBlock, nil
	}

	finalizedTip := latest - w.confirmations
	if fromBlock > finalizedTip {
		return nil, fromBlock, nil
	}

	events, err := w.source.DepositEvents(ctx, fromBlock, finalizedTip)
	if err != nil {
		return nil, fromBlock, err
	}

	sort.SliceStable(events, func(i, j int) bool {
		if events[i].BlockNumber != events[j].BlockNumber {
			return events[i].BlockNumber < events[j].BlockNumber
		}
		if events[i].LogIndex != events[j].LogIndex {
			return events[i].LogIndex < events[j].LogIndex
		}
		return events[i].TxHash < events[j].TxHash
	})

	if finalizedTip == ^uint64(0) {
		return events, finalizedTip, nil
	}
	return events, finalizedTip + 1, nil
}
