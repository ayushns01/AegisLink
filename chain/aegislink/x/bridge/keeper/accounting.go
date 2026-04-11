package keeper

import (
	"math/big"
	"strings"

	bridgetypes "github.com/ayushns01/aegislink/chain/aegislink/x/bridge/types"
	registrytypes "github.com/ayushns01/aegislink/chain/aegislink/x/registry/types"
)

const canonicalNativeETHAddress = "0x0000000000000000000000000000000000000000"

type ClaimRecord struct {
	MessageID string
	Denom     string
	AssetID   string
	Amount    *big.Int
	Status    ClaimStatus
}

type WithdrawalRecord struct {
	BlockHeight  uint64
	Identity     bridgetypes.ClaimIdentity
	AssetID      string
	AssetAddress string
	Amount       *big.Int
	Recipient    string
	Deadline     uint64
	Signature    []byte
}

func (k *Keeper) acceptDepositClaim(claimKey string, claim bridgetypes.DepositClaim, asset registrytypes.Asset) ClaimResult {
	denom := bridgeDenomForAsset(asset)
	k.mintRepresentation(denom, claim.Amount)
	k.processedClaims[claimKey] = ClaimRecord{
		MessageID: claim.Identity.MessageID,
		Denom:     denom,
		AssetID:   claim.AssetID,
		Amount:    cloneAmount(claim.Amount),
		Status:    ClaimStatusAccepted,
	}

	return ClaimResult{
		Status:    ClaimStatusAccepted,
		MessageID: claim.Identity.MessageID,
		Denom:     denom,
		Amount:    cloneAmount(claim.Amount),
	}
}

func (k *Keeper) mintRepresentation(denom string, amount *big.Int) {
	if _, ok := k.supplyByDenom[denom]; !ok {
		k.supplyByDenom[denom] = big.NewInt(0)
	}
	k.supplyByDenom[denom].Add(k.supplyByDenom[denom], amount)
}

func (k *Keeper) burnRepresentation(denom string, amount *big.Int) error {
	current, ok := k.supplyByDenom[denom]
	if !ok {
		return ErrInsufficientSupply
	}
	if current.Cmp(amount) < 0 {
		return ErrInsufficientSupply
	}
	current.Sub(current, amount)
	return nil
}

func cloneWithdrawalRecord(record WithdrawalRecord) WithdrawalRecord {
	return WithdrawalRecord{
		BlockHeight:  record.BlockHeight,
		Identity:     record.Identity,
		AssetID:      record.AssetID,
		AssetAddress: record.AssetAddress,
		Amount:       cloneAmount(record.Amount),
		Recipient:    record.Recipient,
		Deadline:     record.Deadline,
		Signature:    append([]byte(nil), record.Signature...),
	}
}

func bridgeDenomForAsset(asset registrytypes.Asset) string {
	if denom := strings.TrimSpace(asset.DestinationDenom); denom != "" {
		return denom
	}
	return strings.TrimSpace(asset.Denom)
}

func sourceAssetAddressForWithdrawal(asset registrytypes.Asset) string {
	if address := strings.TrimSpace(asset.SourceAssetAddress); address != "" {
		return address
	}
	if address := strings.TrimSpace(asset.SourceContract); address != "" {
		return address
	}
	if asset.SourceAssetKind == registrytypes.SourceAssetKindNativeETH {
		return canonicalNativeETHAddress
	}
	return ""
}

func cloneAmount(amount *big.Int) *big.Int {
	if amount == nil {
		return big.NewInt(0)
	}
	return new(big.Int).Set(amount)
}
