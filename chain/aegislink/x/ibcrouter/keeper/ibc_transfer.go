package keeper

import (
	"fmt"
	"math/big"
	"strings"
)

type IBCPacket struct {
	TransferID         string `json:"transfer_id"`
	Sequence           uint64 `json:"sequence"`
	SourcePort         string `json:"source_port"`
	SourceChannel      string `json:"source_channel"`
	DestinationPort    string `json:"destination_port"`
	DestinationChainID string `json:"destination_chain_id"`
	DestinationDenom   string `json:"destination_denom"`
	TimeoutHeight      uint64 `json:"timeout_height"`
	Memo               string `json:"memo"`
}

func (k *Keeper) SendIBCPacket(assetID string, amount *big.Int, receiver string, timeoutHeight uint64, memo string) (IBCPacket, error) {
	record, err := k.InitiateTransfer(assetID, amount, receiver, timeoutHeight, memo)
	if err != nil {
		return IBCPacket{}, err
	}

	sequence, err := ibcTransferSequence(record.TransferID)
	if err != nil {
		return IBCPacket{}, err
	}

	return IBCPacket{
		TransferID:         record.TransferID,
		Sequence:           sequence,
		SourcePort:         "transfer",
		SourceChannel:      record.ChannelID,
		DestinationPort:    "transfer",
		DestinationChainID: record.DestinationChainID,
		DestinationDenom:   record.DestinationDenom,
		TimeoutHeight:      record.TimeoutHeight,
		Memo:               record.Memo,
	}, nil
}

func ibcTransferSequence(transferID string) (uint64, error) {
	parts := strings.Split(strings.TrimSpace(transferID), "/")
	if len(parts) == 0 {
		return 0, fmt.Errorf("%w: invalid transfer id %q", ErrInvalidTransfer, transferID)
	}
	var sequence uint64
	if _, err := fmt.Sscanf(parts[len(parts)-1], "%d", &sequence); err != nil {
		return 0, fmt.Errorf("%w: invalid transfer id %q", ErrInvalidTransfer, transferID)
	}
	return sequence, nil
}
