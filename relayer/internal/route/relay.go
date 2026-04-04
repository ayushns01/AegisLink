package route

import (
	"context"
	"errors"
	"fmt"
)

type Transfer struct {
	TransferID         string `json:"transfer_id"`
	AssetID            string `json:"asset_id"`
	Amount             string `json:"amount"`
	Receiver           string `json:"receiver"`
	DestinationChainID string `json:"destination_chain_id"`
	ChannelID          string `json:"channel_id"`
	DestinationDenom   string `json:"destination_denom"`
	TimeoutHeight      uint64 `json:"timeout_height"`
	Memo               string `json:"memo"`
	Status             string `json:"status"`
	FailureReason      string `json:"failure_reason"`
}

type AckStatus string

const (
	AckStatusCompleted AckStatus = "completed"
	AckStatusFailed    AckStatus = "ack_failed"
)

type Ack struct {
	Status AckStatus `json:"status"`
	Reason string    `json:"reason,omitempty"`
}

type PendingTransferSource interface {
	PendingTransfers(context.Context) ([]Transfer, error)
}

type AckSink interface {
	CompleteTransfer(context.Context, string) error
	FailTransfer(context.Context, string, string) error
	TimeoutTransfer(context.Context, string) error
}

type Target interface {
	SubmitTransfer(context.Context, Transfer) (Ack, error)
}

type Relayer struct {
	source PendingTransferSource
	sink   AckSink
	target Target
}

func NewRelayer(source PendingTransferSource, sink AckSink, target Target) *Relayer {
	return &Relayer{
		source: source,
		sink:   sink,
		target: target,
	}
}

func (r *Relayer) RunOnce(ctx context.Context) error {
	transfers, err := r.source.PendingTransfers(ctx)
	if err != nil {
		return err
	}

	for _, transfer := range transfers {
		ack, err := r.target.SubmitTransfer(ctx, transfer)
		if err != nil {
			var timeout TimeoutError
			if errors.As(err, &timeout) {
				if err := r.sink.TimeoutTransfer(ctx, transfer.TransferID); err != nil {
					return err
				}
				continue
			}
			return err
		}

		switch ack.Status {
		case AckStatusCompleted:
			if err := r.sink.CompleteTransfer(ctx, transfer.TransferID); err != nil {
				return err
			}
		case AckStatusFailed:
			if err := r.sink.FailTransfer(ctx, transfer.TransferID, ack.Reason); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unexpected ack status %q", ack.Status)
		}
	}

	return nil
}

type TimeoutError struct {
	Err error
}

func (e TimeoutError) Error() string {
	if e.Err == nil {
		return "route target timeout"
	}
	return e.Err.Error()
}

func (e TimeoutError) Unwrap() error {
	return e.Err
}
