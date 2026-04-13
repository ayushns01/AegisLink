package autodelivery

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/ayushns01/aegislink/chain/aegislink/networked"
)

type submitInitiateTransferFunc func(context.Context, networked.Config, networked.InitiateIBCTransferPayload) (networked.TransferView, error)
type registerIntentFunc func(context.Context, networked.Config, networked.DeliveryIntent) (networked.DeliveryIntent, error)

type NetworkedIntentSource struct {
	Config networked.Config
}

func (s NetworkedIntentSource) PendingIntents(ctx context.Context) ([]Intent, error) {
	intents, err := networked.QueryDeliveryIntents(ctx, s.Config)
	if err != nil {
		return nil, err
	}

	result := make([]Intent, 0, len(intents))
	for _, intent := range intents {
		result = append(result, Intent{
			SourceTxHash: intent.SourceTxHash,
			Sender:       intent.Sender,
			RouteID:      intent.RouteID,
			AssetID:      intent.AssetID,
			Amount:       intent.Amount,
			Receiver:     intent.Receiver,
		})
	}
	return result, nil
}

type NetworkedStatusSource struct {
	Config networked.Config
}

func (s NetworkedStatusSource) QueryStatus(ctx context.Context, sourceTxHash string) (BridgeStatus, error) {
	view, err := networked.QueryBridgeSession(ctx, s.Config, sourceTxHash)
	if err != nil {
		return BridgeStatus{}, err
	}
	return BridgeStatus{
		Status:    view.Status,
		ChannelID: view.ChannelID,
	}, nil
}

type NetworkedTransferSubmitter struct {
	Config         networked.Config
	TimeoutHeight  uint64
	submit         submitInitiateTransferFunc
	registerIntent registerIntentFunc
}

func (s NetworkedTransferSubmitter) InitiateTransfer(ctx context.Context, intent Intent) (SubmittedTransfer, error) {
	transfer, err := s.submitTransfer(ctx, networked.InitiateIBCTransferPayload{
		Sender:        intent.Sender,
		RouteID:       intent.RouteID,
		AssetID:       intent.AssetID,
		Amount:        intent.Amount,
		Receiver:      intent.Receiver,
		TimeoutHeight: s.TimeoutHeight,
		Memo:          autoDeliveryMemo(intent.SourceTxHash),
	})
	if err != nil {
		return SubmittedTransfer{}, err
	}
	if _, err := s.recordIntentTransfer(ctx, intent, transfer); err != nil {
		return SubmittedTransfer{}, err
	}

	return SubmittedTransfer{
		TransferID: transfer.TransferID,
		ChannelID:  transfer.ChannelID,
		Status:     transfer.Status,
	}, nil
}

func autoDeliveryMemo(sourceTxHash string) string {
	sourceTxHash = strings.TrimSpace(sourceTxHash)
	if sourceTxHash == "" {
		return "bridge:auto"
	}
	return "bridge:" + sourceTxHash
}

func (s NetworkedTransferSubmitter) submitTransfer(ctx context.Context, payload networked.InitiateIBCTransferPayload) (networked.TransferView, error) {
	if s.submit != nil {
		return s.submit(ctx, s.Config, payload)
	}
	return networked.SubmitInitiateIBCTransfer(ctx, s.Config, payload)
}

func (s NetworkedTransferSubmitter) recordIntentTransfer(ctx context.Context, intent Intent, transfer networked.TransferView) (networked.DeliveryIntent, error) {
	record := networked.DeliveryIntent{
		SourceTxHash: intent.SourceTxHash,
		Sender:       intent.Sender,
		RouteID:      intent.RouteID,
		AssetID:      intent.AssetID,
		Amount:       intent.Amount,
		Receiver:     intent.Receiver,
		TransferID:   transfer.TransferID,
		ChannelID:    transfer.ChannelID,
	}
	if s.registerIntent != nil {
		return s.registerIntent(ctx, s.Config, record)
	}
	return networked.RegisterDeliveryIntentOverHTTP(ctx, s.Config, record)
}

type RlyFlusher struct {
	Command  string
	PathName string
	Home     string
}

func (f RlyFlusher) Flush(ctx context.Context, channelID string) error {
	command := strings.TrimSpace(f.Command)
	if command == "" {
		command = "./bin/relayer"
	}
	pathName := strings.TrimSpace(f.PathName)
	if pathName == "" {
		pathName = "osmosis-public-wallet"
	}
	home := strings.TrimSpace(f.Home)
	if home == "" {
		return fmt.Errorf("missing relayer home for auto delivery flush")
	}
	channelID = strings.TrimSpace(channelID)
	if channelID == "" {
		return fmt.Errorf("missing channel id for auto delivery flush")
	}

	cmd := exec.CommandContext(
		ctx,
		command,
		"transact",
		"flush",
		pathName,
		channelID,
		"--home",
		home,
		"--debug",
		"--log-level",
		"debug",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			return err
		}
		return fmt.Errorf("%w: %s", err, message)
	}
	return nil
}
