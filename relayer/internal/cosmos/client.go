package cosmos

import (
	"context"
	"errors"
	"math/big"
	"strings"

	bridgetypes "github.com/ayushns01/aegislink/chain/aegislink/x/bridge/types"
	"github.com/ayushns01/aegislink/relayer/internal/evm"
)

var ErrSourceUnavailable = errors.New("cosmos source unavailable")
var ErrSubmitterUnavailable = errors.New("cosmos submitter unavailable")

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

type Withdrawal struct {
	BlockHeight  uint64
	Identity     bridgetypes.ClaimIdentity
	AssetID      string
	AssetAddress string
	Amount       *big.Int
	Recipient    string
	Deadline     uint64
	Signature    []byte
}

func (w Withdrawal) Validate() error {
	if err := w.Identity.ValidateBasic(); err != nil {
		return err
	}
	if w.Identity.Kind != bridgetypes.ClaimKindWithdrawal {
		return errors.New("withdrawal must use withdrawal claim identity")
	}
	if strings.TrimSpace(w.AssetID) == "" {
		return errors.New("missing asset id")
	}
	if strings.TrimSpace(w.AssetAddress) == "" {
		return errors.New("missing asset address")
	}
	if w.Amount == nil || w.Amount.Sign() <= 0 {
		return errors.New("amount must be positive")
	}
	if strings.TrimSpace(w.Recipient) == "" {
		return errors.New("missing recipient")
	}
	if w.Deadline == 0 {
		return errors.New("missing deadline")
	}
	if len(w.Signature) == 0 {
		return errors.New("missing signature")
	}
	return nil
}

func (w Withdrawal) ReplayKey() string {
	return w.Identity.ReplayKey()
}

func (w Withdrawal) ReleaseRequest() evm.ReleaseRequest {
	return evm.ReleaseRequest{
		MessageID:    w.Identity.MessageID,
		AssetAddress: w.AssetAddress,
		Amount:       cloneBigInt(w.Amount),
		Recipient:    w.Recipient,
		Deadline:     w.Deadline,
		Signature:    cloneBytes(w.Signature),
	}
}

type WithdrawalSource interface {
	LatestHeight(context.Context) (uint64, error)
	Withdrawals(context.Context, uint64, uint64) ([]Withdrawal, error)
}

type ClaimSink interface {
	SubmitDepositClaim(context.Context, bridgetypes.DepositClaim, bridgetypes.Attestation) error
}

type Client struct {
	source WithdrawalSource
}

func NewClient(source WithdrawalSource) *Client {
	return &Client{source: source}
}

func (c *Client) LatestHeight(ctx context.Context) (uint64, error) {
	if c == nil || c.source == nil {
		return 0, ErrSourceUnavailable
	}
	return c.source.LatestHeight(ctx)
}

func (c *Client) Withdrawals(ctx context.Context, fromHeight, toHeight uint64) ([]Withdrawal, error) {
	if c == nil || c.source == nil {
		return nil, ErrSourceUnavailable
	}
	return c.source.Withdrawals(ctx, fromHeight, toHeight)
}

type Submitter struct {
	sink ClaimSink
}

func NewSubmitter(sink ClaimSink) *Submitter {
	return &Submitter{sink: sink}
}

func (s *Submitter) SubmitDepositClaim(ctx context.Context, claim bridgetypes.DepositClaim, attestation bridgetypes.Attestation) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.sink == nil {
		return ErrSubmitterUnavailable
	}
	if err := claim.ValidateBasic(); err != nil {
		return err
	}
	if err := attestation.ValidateBasic(); err != nil {
		return err
	}
	if attestation.MessageID != claim.Identity.MessageID {
		return errors.New("attestation message id mismatch")
	}
	if attestation.PayloadHash != claim.Digest() {
		return errors.New("attestation payload hash mismatch")
	}
	return s.sink.SubmitDepositClaim(ctx, claim, attestation)
}

func cloneBigInt(value *big.Int) *big.Int {
	if value == nil {
		return nil
	}
	return new(big.Int).Set(value)
}

func cloneBytes(value []byte) []byte {
	if value == nil {
		return nil
	}
	return append([]byte(nil), value...)
}
