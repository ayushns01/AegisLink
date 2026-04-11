package types

import (
	"fmt"
	"math/big"
	"strings"
)

type ClaimKind string

const (
	ClaimKindUnspecified ClaimKind = ""
	ClaimKindDeposit     ClaimKind = "deposit"
	ClaimKindWithdrawal  ClaimKind = "withdrawal"
)

const (
	SourceAssetKindUnspecified = ""
	SourceAssetKindNativeETH   = "native_eth"
	SourceAssetKindERC20       = "erc20"
)

type ClaimIdentity struct {
	Kind            ClaimKind
	SourceAssetKind string
	SourceChainID   string
	SourceContract  string
	SourceTxHash    string
	SourceLogIndex  uint64
	Nonce           uint64
	MessageID       string
}

func (c ClaimIdentity) ValidateBasic() error {
	sourceAssetKind := strings.TrimSpace(c.SourceAssetKind)
	if c.Kind != ClaimKindDeposit && c.Kind != ClaimKindWithdrawal {
		return fmt.Errorf("%w: invalid claim kind %q", ErrInvalidClaim, c.Kind)
	}
	if sourceAssetKind != SourceAssetKindNativeETH && sourceAssetKind != SourceAssetKindERC20 {
		return fmt.Errorf("%w: invalid source asset kind", ErrInvalidClaim)
	}
	if strings.TrimSpace(c.SourceChainID) == "" {
		return fmt.Errorf("%w: missing source chain id", ErrInvalidClaim)
	}
	if strings.TrimSpace(c.SourceTxHash) == "" {
		return fmt.Errorf("%w: missing source tx hash", ErrInvalidClaim)
	}
	if sourceAssetKind == SourceAssetKindERC20 && strings.TrimSpace(c.SourceContract) == "" {
		return fmt.Errorf("%w: missing source contract", ErrInvalidClaim)
	}
	if strings.TrimSpace(c.MessageID) == "" {
		return fmt.Errorf("%w: missing message id", ErrInvalidClaim)
	}
	if c.MessageID != c.DerivedMessageID() {
		return fmt.Errorf("%w: message id mismatch", ErrInvalidClaim)
	}
	return nil
}

func (c ClaimIdentity) ReplayKey() string {
	return c.DerivedMessageID()
}

func (c ClaimIdentity) DerivedMessageID() string {
	return ReplayKey(c.Kind, c.SourceAssetKind, c.SourceChainID, c.SourceContract, c.SourceTxHash, c.SourceLogIndex, c.Nonce)
}

type DepositClaim struct {
	Identity           ClaimIdentity
	DestinationChainID string
	AssetID            string
	Amount             *big.Int
	Recipient          string
	Deadline           uint64
}

func (c DepositClaim) ValidateBasic() error {
	if c.Identity.Kind != ClaimKindDeposit {
		return fmt.Errorf("%w: deposit claim must use deposit identity", ErrInvalidClaim)
	}
	if err := c.Identity.ValidateBasic(); err != nil {
		return err
	}
	if strings.TrimSpace(c.DestinationChainID) == "" {
		return fmt.Errorf("%w: missing destination chain id", ErrInvalidClaim)
	}
	if strings.TrimSpace(c.AssetID) == "" {
		return fmt.Errorf("%w: missing asset id", ErrInvalidClaim)
	}
	if c.Amount == nil || c.Amount.Sign() <= 0 {
		return fmt.Errorf("%w: amount must be positive", ErrInvalidClaim)
	}
	if strings.TrimSpace(c.Recipient) == "" {
		return fmt.Errorf("%w: missing recipient", ErrInvalidClaim)
	}
	if c.Deadline == 0 {
		return fmt.Errorf("%w: missing deadline", ErrInvalidClaim)
	}
	return nil
}

func (c DepositClaim) Digest() string {
	return ClaimDigest(
		c.Identity.Kind,
		c.Identity.SourceAssetKind,
		c.Identity.SourceChainID,
		c.Identity.SourceContract,
		c.Identity.SourceTxHash,
		c.Identity.SourceLogIndex,
		c.Identity.Nonce,
		c.DestinationChainID,
		c.AssetID,
		c.Amount,
		c.Recipient,
		c.Deadline,
	)
}

type WithdrawalClaim struct {
	Identity           ClaimIdentity
	DestinationChainID string
	AssetID            string
	Amount             *big.Int
	Recipient          string
	Deadline           uint64
}

func (c WithdrawalClaim) ValidateBasic() error {
	if c.Identity.Kind != ClaimKindWithdrawal {
		return fmt.Errorf("%w: withdrawal claim must use withdrawal identity", ErrInvalidClaim)
	}
	if err := c.Identity.ValidateBasic(); err != nil {
		return err
	}
	if strings.TrimSpace(c.DestinationChainID) == "" {
		return fmt.Errorf("%w: missing destination chain id", ErrInvalidClaim)
	}
	if strings.TrimSpace(c.AssetID) == "" {
		return fmt.Errorf("%w: missing asset id", ErrInvalidClaim)
	}
	if c.Amount == nil || c.Amount.Sign() <= 0 {
		return fmt.Errorf("%w: amount must be positive", ErrInvalidClaim)
	}
	if strings.TrimSpace(c.Recipient) == "" {
		return fmt.Errorf("%w: missing recipient", ErrInvalidClaim)
	}
	if c.Deadline == 0 {
		return fmt.Errorf("%w: missing deadline", ErrInvalidClaim)
	}
	return nil
}

func (c WithdrawalClaim) Digest() string {
	return ClaimDigest(
		c.Identity.Kind,
		c.Identity.SourceAssetKind,
		c.Identity.SourceChainID,
		c.Identity.SourceContract,
		c.Identity.SourceTxHash,
		c.Identity.SourceLogIndex,
		c.Identity.Nonce,
		c.DestinationChainID,
		c.AssetID,
		c.Amount,
		c.Recipient,
		c.Deadline,
	)
}
