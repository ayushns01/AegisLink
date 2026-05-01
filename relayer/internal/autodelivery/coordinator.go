package autodelivery

import (
	"context"
	"strings"
)

type Intent struct {
	SourceTxHash string
	Sender       string
	RouteID      string
	AssetID      string
	Amount       string
	Receiver     string
}

type BridgeStatus struct {
	Status    string
	ChannelID string
}

type SubmittedTransfer struct {
	TransferID string
	ChannelID  string
	Status     string
}

type IntentSource interface {
	PendingIntents(context.Context) ([]Intent, error)
}

type StatusSource interface {
	QueryStatus(context.Context, string) (BridgeStatus, error)
}

type TransferSubmitter interface {
	InitiateTransfer(context.Context, Intent) (SubmittedTransfer, error)
}

type Flusher interface {
	Flush(ctx context.Context, routeID, channelID string) error
}

type RunSummary struct {
	IntentsObserved     int
	IntentsWaiting      int
	TransfersInitiated  int
	FlushesTriggered    int
	CompletedDeliveries int
	FailedDeliveries    int
}

type Coordinator struct {
	intents   IntentSource
	statuses  StatusSource
	submitter TransferSubmitter
	flusher   Flusher
}

func NewCoordinator(intents IntentSource, statuses StatusSource, submitter TransferSubmitter, flusher Flusher) *Coordinator {
	return &Coordinator{
		intents:   intents,
		statuses:  statuses,
		submitter: submitter,
		flusher:   flusher,
	}
}

func (c *Coordinator) RunOnce(ctx context.Context) (RunSummary, error) {
	var summary RunSummary

	intents, err := c.intents.PendingIntents(ctx)
	if err != nil {
		return summary, err
	}
	summary.IntentsObserved = len(intents)

	for _, intent := range intents {
		status, err := c.statuses.QueryStatus(ctx, intent.SourceTxHash)
		if err != nil {
			return summary, err
		}

		switch strings.TrimSpace(status.Status) {
		case "", "deposit_submitted", "sepolia_confirming":
			summary.IntentsWaiting++
		case "aegislink_processing":
			transfer, err := c.submitter.InitiateTransfer(ctx, intent)
			if err != nil {
				return summary, err
			}
			summary.TransfersInitiated++
			if strings.TrimSpace(transfer.ChannelID) == "" {
				continue
			}
			if err := c.flusher.Flush(ctx, intent.RouteID, transfer.ChannelID); err != nil {
				return summary, err
			}
			summary.FlushesTriggered++
		case "osmosis_pending":
			channelID := strings.TrimSpace(status.ChannelID)
			if channelID == "" {
				summary.IntentsWaiting++
				continue
			}
			if err := c.flusher.Flush(ctx, intent.RouteID, channelID); err != nil {
				return summary, err
			}
			summary.FlushesTriggered++
		case "completed":
			summary.CompletedDeliveries++
		case "failed":
			summary.FailedDeliveries++
		default:
			summary.IntentsWaiting++
		}
	}

	return summary, nil
}
