package evm

import (
	"context"
	"errors"
	"math/big"
	"strings"

	bridgetypes "github.com/ayushns01/aegislink/chain/aegislink/x/bridge/types"
)

var ErrSourceUnavailable = errors.New("evm source unavailable")
var ErrReleaseUnavailable = errors.New("evm release target unavailable")

type TemporaryError struct {
	Err error
}

func (e TemporaryError) Error() string {
	if e.Err == nil {
		return "temporary error"
	}
	return e.Err.Error()
}

func (e TemporaryError) Unwrap() error { return e.Err }

func (e TemporaryError) Temporary() bool { return true }

type DepositEvent struct {
	BlockNumber    uint64
	SourceChainID  string
	SourceContract string
	TxHash         string
	LogIndex       uint64
	Nonce          uint64
	DepositID      string
	MessageID      string
	AssetAddress   string
	AssetID        string
	Amount         *big.Int
	Recipient      string
	Expiry         uint64
}

func (e DepositEvent) ReplayKey() string {
	return bridgetypes.ReplayKey(
		bridgetypes.ClaimKindDeposit,
		e.SourceChainID,
		e.SourceContract,
		e.TxHash,
		e.LogIndex,
		e.Nonce,
	)
}

func (e DepositEvent) Claim(destinationChainID string) bridgetypes.DepositClaim {
	identity := bridgetypes.ClaimIdentity{
		Kind:           bridgetypes.ClaimKindDeposit,
		SourceChainID:  e.SourceChainID,
		SourceContract: e.SourceContract,
		SourceTxHash:   e.TxHash,
		SourceLogIndex: e.LogIndex,
		Nonce:          e.Nonce,
	}
	identity.MessageID = identity.DerivedMessageID()

	return bridgetypes.DepositClaim{
		Identity:           identity,
		DestinationChainID: destinationChainID,
		AssetID:            e.AssetID,
		Amount:             cloneBigInt(e.Amount),
		Recipient:          e.Recipient,
		Deadline:           e.Expiry,
	}
}

type ReleaseRequest struct {
	MessageID    string
	AssetAddress string
	Amount       *big.Int
	Recipient    string
	Deadline     uint64
	Signature    []byte
}

func (r ReleaseRequest) Validate() error {
	if strings.TrimSpace(r.MessageID) == "" {
		return errors.New("missing message id")
	}
	if strings.TrimSpace(r.AssetAddress) == "" {
		return errors.New("missing asset address")
	}
	if r.Amount == nil || r.Amount.Sign() <= 0 {
		return errors.New("amount must be positive")
	}
	if strings.TrimSpace(r.Recipient) == "" {
		return errors.New("missing recipient")
	}
	if r.Deadline == 0 {
		return errors.New("missing deadline")
	}
	if len(r.Signature) == 0 {
		return errors.New("missing signature")
	}
	return nil
}

type LogSource interface {
	LatestBlock(context.Context) (uint64, error)
	DepositEvents(context.Context, uint64, uint64) ([]DepositEvent, error)
}

type ReleaseTarget interface {
	ReleaseWithdrawal(context.Context, ReleaseRequest) (string, error)
}

type Client struct {
	source LogSource
}

func NewClient(source LogSource) *Client {
	return &Client{source: source}
}

func (c *Client) LatestBlock(ctx context.Context) (uint64, error) {
	if c == nil || c.source == nil {
		return 0, ErrSourceUnavailable
	}
	return c.source.LatestBlock(ctx)
}

func (c *Client) DepositEvents(ctx context.Context, fromBlock, toBlock uint64) ([]DepositEvent, error) {
	if c == nil || c.source == nil {
		return nil, ErrSourceUnavailable
	}
	return c.source.DepositEvents(ctx, fromBlock, toBlock)
}

type Releaser struct {
	target ReleaseTarget
}

func NewReleaser(target ReleaseTarget) *Releaser {
	return &Releaser{target: target}
}

func (r *Releaser) ReleaseWithdrawal(ctx context.Context, request ReleaseRequest) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if r == nil || r.target == nil {
		return "", ErrReleaseUnavailable
	}
	if err := request.Validate(); err != nil {
		return "", err
	}

	cloned := ReleaseRequest{
		MessageID:    request.MessageID,
		AssetAddress: request.AssetAddress,
		Amount:       cloneBigInt(request.Amount),
		Recipient:    request.Recipient,
		Deadline:     request.Deadline,
		Signature:    append([]byte(nil), request.Signature...),
	}
	return r.target.ReleaseWithdrawal(ctx, cloned)
}

func cloneBigInt(value *big.Int) *big.Int {
	if value == nil {
		return nil
	}
	return new(big.Int).Set(value)
}
