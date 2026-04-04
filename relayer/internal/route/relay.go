package route

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
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

type DeliveryEnvelope struct {
	Transfer   Transfer     `json:"transfer"`
	Packet     Packet       `json:"packet"`
	DenomTrace DenomTrace   `json:"denom_trace"`
	Action     *RouteAction `json:"action,omitempty"`
}

type Packet struct {
	Sequence        uint64     `json:"sequence"`
	SourcePort      string     `json:"source_port"`
	SourceChannel   string     `json:"source_channel"`
	DestinationPort string     `json:"destination_port"`
	TimeoutHeight   uint64     `json:"timeout_height"`
	Data            PacketData `json:"data"`
}

type PacketData struct {
	Denom    string `json:"denom"`
	Amount   string `json:"amount"`
	Sender   string `json:"sender"`
	Receiver string `json:"receiver"`
	Memo     string `json:"memo"`
}

type DenomTrace struct {
	Path      string `json:"path"`
	BaseDenom string `json:"base_denom"`
	IBCDenom  string `json:"ibc_denom"`
}

type RouteAction struct {
	Type        string `json:"type"`
	TargetDenom string `json:"target_denom,omitempty"`
}

const routePacketSender = "aegislink1ibcrouter"

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

func buildDeliveryEnvelope(transfer Transfer) (DeliveryEnvelope, error) {
	sequence, err := parseTransferSequence(transfer.TransferID)
	if err != nil {
		return DeliveryEnvelope{}, err
	}
	channelID := strings.TrimSpace(transfer.ChannelID)
	if channelID == "" {
		return DeliveryEnvelope{}, fmt.Errorf("missing channel id for transfer %s", transfer.TransferID)
	}
	assetID := strings.TrimSpace(transfer.AssetID)
	if assetID == "" {
		return DeliveryEnvelope{}, fmt.Errorf("missing asset id for transfer %s", transfer.TransferID)
	}
	receiver := strings.TrimSpace(transfer.Receiver)
	if receiver == "" {
		return DeliveryEnvelope{}, fmt.Errorf("missing receiver for transfer %s", transfer.TransferID)
	}

	envelope := DeliveryEnvelope{
		Transfer: transfer,
		Packet: Packet{
			Sequence:        sequence,
			SourcePort:      "transfer",
			SourceChannel:   channelID,
			DestinationPort: "transfer",
			TimeoutHeight:   transfer.TimeoutHeight,
			Data: PacketData{
				Denom:    assetID,
				Amount:   strings.TrimSpace(transfer.Amount),
				Sender:   routePacketSender,
				Receiver: receiver,
				Memo:     strings.TrimSpace(transfer.Memo),
			},
		},
		DenomTrace: DenomTrace{
			Path:      fmt.Sprintf("transfer/%s", channelID),
			BaseDenom: assetID,
			IBCDenom:  strings.TrimSpace(transfer.DestinationDenom),
		},
		Action: parseRouteAction(transfer.Memo),
	}
	return envelope, nil
}

func parseTransferSequence(transferID string) (uint64, error) {
	parts := strings.Split(strings.TrimSpace(transferID), "/")
	if len(parts) == 0 {
		return 0, fmt.Errorf("invalid transfer id %q", transferID)
	}
	sequence, err := strconv.ParseUint(parts[len(parts)-1], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid transfer id %q: %w", transferID, err)
	}
	return sequence, nil
}

func parseRouteAction(memo string) *RouteAction {
	memo = strings.TrimSpace(memo)
	if strings.HasPrefix(memo, "swap:") {
		targetDenom := strings.TrimSpace(strings.TrimPrefix(memo, "swap:"))
		if targetDenom != "" {
			return &RouteAction{Type: "swap", TargetDenom: targetDenom}
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
