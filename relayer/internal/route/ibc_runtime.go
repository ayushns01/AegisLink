package route

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type CommandIBCTarget struct {
	run      commandRunner
	command  string
	baseArgs []string
	locator  RuntimeLocator
}

func NewCommandIBCTarget(command string, baseArgs []string, locator RuntimeLocator) *CommandIBCTarget {
	return &CommandIBCTarget{
		run:      runCommand,
		command:  command,
		baseArgs: append([]string(nil), baseArgs...),
		locator:  locator,
	}
}

func (t *CommandIBCTarget) SubmitTransfer(ctx context.Context, transfer Transfer) (Ack, error) {
	envelope, err := buildDeliveryEnvelope(transfer)
	if err != nil {
		return Ack{}, err
	}

	args := append(append([]string(nil), t.baseArgs...),
		"relay", "recv-packet",
		"--transfer-id", transfer.TransferID,
		"--sequence", fmt.Sprintf("%d", envelope.Packet.Sequence),
		"--source-port", envelope.Packet.SourcePort,
		"--source-channel", envelope.Packet.SourceChannel,
		"--destination-port", envelope.Packet.DestinationPort,
		"--asset-id", transfer.AssetID,
		"--amount", transfer.Amount,
		"--receiver", transfer.Receiver,
		"--destination-chain-id", transfer.DestinationChainID,
		"--channel-id", transfer.ChannelID,
		"--destination-denom", transfer.DestinationDenom,
		"--timeout-height", fmt.Sprintf("%d", transfer.TimeoutHeight),
		"--memo", transfer.Memo,
	)
	args = appendRuntimeLocatorArgs(args, t.locator)
	output, err := t.run(ctx, t.command, args...)
	if err != nil {
		return Ack{}, err
	}

	var ack Ack
	if err := json.Unmarshal(output, &ack); err != nil {
		return Ack{}, err
	}
	return ack, nil
}

func (t *CommandIBCTarget) ReadyAcks(ctx context.Context) ([]AckRecord, error) {
	args := append(append([]string(nil), t.baseArgs...),
		"query", "packet-acks",
	)
	args = appendRuntimeLocatorArgs(args, t.locator)
	output, err := t.run(ctx, t.command, args...)
	if err != nil {
		return nil, err
	}

	var acks []AckRecord
	if err := json.Unmarshal(output, &acks); err != nil {
		return nil, err
	}
	return acks, nil
}

func (t *CommandIBCTarget) ConfirmAck(ctx context.Context, transferID string) error {
	args := append(append([]string(nil), t.baseArgs...),
		"relay", "acknowledge-packet",
		"--transfer-id", strings.TrimSpace(transferID),
	)
	args = appendRuntimeLocatorArgs(args, t.locator)
	_, err := t.run(ctx, t.command, args...)
	return err
}

func newCommandIBCTargetWithRunner(run commandRunner, command string, baseArgs []string, locator RuntimeLocator) *CommandIBCTarget {
	return &CommandIBCTarget{
		run:      run,
		command:  command,
		baseArgs: append([]string(nil), baseArgs...),
		locator:  locator,
	}
}
