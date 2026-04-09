package route

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
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
	Transfer    Transfer     `json:"transfer"`
	Packet      Packet       `json:"packet"`
	DenomTrace  DenomTrace   `json:"denom_trace"`
	Action      *RouteAction `json:"action,omitempty"`
	ActionError string       `json:"action_error,omitempty"`
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
	Recipient   string `json:"recipient,omitempty"`
	Path        string `json:"path,omitempty"`
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

type LoopConfig struct {
	PollInterval       time.Duration
	FailureBackoff     time.Duration
	MaxConsecutiveRuns int
	OnResult           func(LoopEvent)
}

type LoopEvent struct {
	Iteration           int
	Summary             RunSummary
	Err                 error
	Temporary           bool
	ConsecutiveFailures int
}

type RunSummary struct {
	ReadyAcks         int `json:"ready_acks"`
	CompletedAcks     int `json:"completed_acks"`
	FailedAcks        int `json:"failed_acks"`
	TimedOutAcks      int `json:"timed_out_acks"`
	TransfersObserved int `json:"transfers_observed"`
	TransfersDelivered int `json:"transfers_delivered"`
	ReceivedDeliveries int `json:"received_deliveries"`
}

func NewRelayer(source PendingTransferSource, sink AckSink, target Target) *Relayer {
	return &Relayer{
		source: source,
		sink:   sink,
		target: target,
	}
}

type TemporaryError struct {
	Err error
}

func (e TemporaryError) Error() string {
	if e.Err == nil {
		return "temporary route error"
	}
	return e.Err.Error()
}

func (e TemporaryError) Unwrap() error { return e.Err }

func (e TemporaryError) Temporary() bool { return true }

func (r *Relayer) RunOnce(ctx context.Context) error {
	_, err := r.RunOnceWithSummary(ctx)
	return err
}

func (r *Relayer) RunOnceWithSummary(ctx context.Context) (RunSummary, error) {
	var summary RunSummary
	readyAcks, err := r.target.ReadyAcks(ctx)
	if err != nil {
		return summary, err
	}
	summary.ReadyAcks = len(readyAcks)

	for _, ack := range readyAcks {
		if err := r.applyAck(ctx, ack.TransferID, Ack{Status: ack.Status, Reason: ack.Reason}); err != nil {
			return summary, err
		}
		summary.recordAck(ack.Status)
		if err := r.target.ConfirmAck(ctx, ack.TransferID); err != nil {
			return summary, err
		}
	}

	transfers, err := r.source.PendingTransfers(ctx)
	if err != nil {
		return summary, err
	}
	summary.TransfersObserved = len(transfers)

	for _, transfer := range transfers {
		ack, err := r.target.SubmitTransfer(ctx, transfer)
		if err != nil {
			return summary, err
		}
		summary.TransfersDelivered++
		if ack.Status == "" || ack.Status == AckStatusReceived {
			summary.ReceivedDeliveries++
			continue
		}
		if err := r.applyAck(ctx, transfer.TransferID, ack); err != nil {
			return summary, err
		}
		summary.recordAck(ack.Status)
	}

	return summary, nil
}

func (r *Relayer) RunLoop(ctx context.Context, cfg LoopConfig) error {
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = time.Second
	}
	if cfg.FailureBackoff <= 0 {
		cfg.FailureBackoff = 5 * time.Second
	}

	consecutiveFailures := 0
	for iteration := 1; ; iteration++ {
		if err := ctx.Err(); err != nil {
			return nil
		}

		summary, err := r.RunOnceWithSummary(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			temporary := isTemporary(err)
			if temporary {
				consecutiveFailures++
			}
			emitLoopEvent(cfg, LoopEvent{
				Iteration:           iteration,
				Summary:             summary,
				Err:                 err,
				Temporary:           temporary,
				ConsecutiveFailures: consecutiveFailures,
			})
			if !temporary {
				return err
			}
			if cfg.MaxConsecutiveRuns > 0 && iteration >= cfg.MaxConsecutiveRuns {
				return nil
			}
			if waitForNextRouteRun(ctx, cfg.FailureBackoff) {
				return nil
			}
			continue
		}

		consecutiveFailures = 0
		emitLoopEvent(cfg, LoopEvent{
			Iteration: iteration,
			Summary:   summary,
		})
		if cfg.MaxConsecutiveRuns > 0 && iteration >= cfg.MaxConsecutiveRuns {
			return nil
		}
		if waitForNextRouteRun(ctx, cfg.PollInterval) {
			return nil
		}
	}
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

func (s *RunSummary) recordAck(status AckStatus) {
	switch status {
	case AckStatusCompleted:
		s.CompletedAcks++
	case AckStatusFailed:
		s.FailedAcks++
	case AckStatusTimedOut:
		s.TimedOutAcks++
	}
}

func emitLoopEvent(cfg LoopConfig, event LoopEvent) {
	if cfg.OnResult != nil {
		cfg.OnResult(event)
	}
}

func waitForNextRouteRun(ctx context.Context, delay time.Duration) bool {
	if delay <= 0 {
		select {
		case <-ctx.Done():
			return true
		default:
			return false
		}
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return true
	case <-timer.C:
		return false
	}
}

func isTemporary(err error) bool {
	var temporary interface{ Temporary() bool }
	return errors.As(err, &temporary) && temporary.Temporary()
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

	action, actionErr := parseRouteAction(transfer.Memo)

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
		Action:      action,
		ActionError: actionErr,
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

func parseRouteAction(memo string) (*RouteAction, string) {
	memo = strings.TrimSpace(memo)
	if memo == "" {
		return nil, ""
	}

	parts := strings.Split(memo, ":")
	if len(parts) == 1 {
		if strings.TrimSpace(parts[0]) == "swap" {
			return nil, "missing target denom for swap action"
		}
		return nil, ""
	}

	actionType := strings.TrimSpace(parts[0])
	switch actionType {
	case "swap":
		targetDenom := strings.TrimSpace(parts[1])
		if targetDenom == "" {
			return nil, "missing target denom for swap action"
		}
		action := &RouteAction{Type: "swap", TargetDenom: targetDenom}
		seen := make(map[string]struct{})
		for _, part := range parts[2:] {
			part = strings.TrimSpace(part)
			if part == "" {
				return nil, "empty swap option"
			}
			key, value, ok := strings.Cut(part, "=")
			if !ok {
				return nil, fmt.Sprintf("invalid swap option %q", part)
			}
			key = strings.TrimSpace(key)
			value = strings.TrimSpace(value)
			if key == "" || value == "" {
				return nil, fmt.Sprintf("invalid swap option %q", part)
			}
			if _, exists := seen[key]; exists {
				return nil, fmt.Sprintf("duplicate swap option %q", key)
			}
			seen[key] = struct{}{}

			switch key {
			case "min_out":
				action.MinOut = value
			case "recipient":
				action.Recipient = value
			case "path":
				action.Path = value
			default:
				return nil, fmt.Sprintf("unsupported swap option %q", key)
			}
		}
		return action, ""
	case "stake":
		targetDenom := strings.TrimSpace(parts[1])
		if targetDenom == "" {
			return nil, "missing target denom for stake action"
		}
		action := &RouteAction{Type: "stake", TargetDenom: targetDenom}
		seen := make(map[string]struct{})
		for _, part := range parts[2:] {
			part = strings.TrimSpace(part)
			if part == "" {
				return nil, "empty stake option"
			}
			key, value, ok := strings.Cut(part, "=")
			if !ok {
				return nil, fmt.Sprintf("invalid stake option %q", part)
			}
			key = strings.TrimSpace(key)
			value = strings.TrimSpace(value)
			if key == "" || value == "" {
				return nil, fmt.Sprintf("invalid stake option %q", part)
			}
			if _, exists := seen[key]; exists {
				return nil, fmt.Sprintf("duplicate stake option %q", key)
			}
			seen[key] = struct{}{}

			switch key {
			case "recipient":
				action.Recipient = value
			case "path":
				action.Path = value
			default:
				return nil, fmt.Sprintf("unsupported stake option %q", key)
			}
		}
		return action, ""
	default:
		return nil, fmt.Sprintf("unsupported route action %q", actionType)
	}
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
