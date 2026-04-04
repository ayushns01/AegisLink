package route

import (
	"context"
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
	MinOut      string `json:"min_out,omitempty"`
}

const routePacketSender = "aegislink1ibcrouter"

type AckStatus string

const (
	AckStatusReceived  AckStatus = "received"
	AckStatusCompleted AckStatus = "completed"
	AckStatusFailed    AckStatus = "ack_failed"
	AckStatusTimedOut  AckStatus = "timed_out"
)

type Ack struct {
	Status AckStatus `json:"status"`
	Reason string    `json:"reason,omitempty"`
}

type AckRecord struct {
	TransferID string    `json:"transfer_id"`
	Status     AckStatus `json:"status"`
	Reason     string    `json:"reason,omitempty"`
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
	ReadyAcks(context.Context) ([]AckRecord, error)
	ConfirmAck(context.Context, string) error
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
	readyAcks, err := r.target.ReadyAcks(ctx)
	if err != nil {
		return err
	}

	for _, ack := range readyAcks {
		if err := r.applyAck(ctx, ack.TransferID, Ack{Status: ack.Status, Reason: ack.Reason}); err != nil {
			return err
		}
		if err := r.target.ConfirmAck(ctx, ack.TransferID); err != nil {
			return err
		}
	}

	transfers, err := r.source.PendingTransfers(ctx)
	if err != nil {
		return err
	}

	for _, transfer := range transfers {
		ack, err := r.target.SubmitTransfer(ctx, transfer)
		if err != nil {
			return err
		}
		if ack.Status == "" || ack.Status == AckStatusReceived {
			continue
		}
		if err := r.applyAck(ctx, transfer.TransferID, ack); err != nil {
			return err
		}
	}

	return nil
}

func (r *Relayer) applyAck(ctx context.Context, transferID string, ack Ack) error {
	switch ack.Status {
	case AckStatusCompleted:
		return r.sink.CompleteTransfer(ctx, transferID)
	case AckStatusFailed:
		return r.sink.FailTransfer(ctx, transferID, ack.Reason)
	case AckStatusTimedOut:
		return r.sink.TimeoutTransfer(ctx, transferID)
	default:
		return fmt.Errorf("unexpected ack status %q", ack.Status)
	}
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
		parts := strings.Split(memo, ":")
		if len(parts) < 2 {
			return nil
		}
		targetDenom := strings.TrimSpace(parts[1])
		if targetDenom == "" {
			return nil
		}
		action := &RouteAction{Type: "swap", TargetDenom: targetDenom}
		for _, part := range parts[2:] {
			part = strings.TrimSpace(part)
			if strings.HasPrefix(part, "min_out=") {
				action.MinOut = strings.TrimSpace(strings.TrimPrefix(part, "min_out="))
			}
		}
		return action
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
