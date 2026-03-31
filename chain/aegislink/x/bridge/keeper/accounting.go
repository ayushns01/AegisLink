package keeper

import (
	"math/big"

	bridgetypes "github.com/ayushns01/aegislink/chain/aegislink/x/bridge/types"
	registrytypes "github.com/ayushns01/aegislink/chain/aegislink/x/registry/types"
)

type ClaimRecord struct {
	MessageID string
	Denom     string
	AssetID   string
	Amount    *big.Int
	Status    ClaimStatus
}

func (k *Keeper) acceptDepositClaim(claimKey string, claim bridgetypes.DepositClaim, asset registrytypes.Asset) ClaimResult {
	k.mintRepresentation(asset.Denom, claim.Amount)
	k.processedClaims[claimKey] = ClaimRecord{
		MessageID: claim.Identity.MessageID,
		Denom:     asset.Denom,
		AssetID:   claim.AssetID,
		Amount:    cloneAmount(claim.Amount),
		Status:    ClaimStatusAccepted,
	}

	return ClaimResult{
		Status:    ClaimStatusAccepted,
		MessageID: claim.Identity.MessageID,
		Denom:     asset.Denom,
		Amount:    cloneAmount(claim.Amount),
	}
}

func (k *Keeper) mintRepresentation(denom string, amount *big.Int) {
	if _, ok := k.supplyByDenom[denom]; !ok {
		k.supplyByDenom[denom] = big.NewInt(0)
	}
	k.supplyByDenom[denom].Add(k.supplyByDenom[denom], amount)
}

func cloneAmount(amount *big.Int) *big.Int {
	if amount == nil {
		return big.NewInt(0)
	}
	return new(big.Int).Set(amount)
}
