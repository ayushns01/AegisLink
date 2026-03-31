package keeper

import (
	"errors"
	"fmt"
	"math/big"

	bridgetypes "github.com/ayushns01/aegislink/chain/aegislink/x/bridge/types"
	limitskeeper "github.com/ayushns01/aegislink/chain/aegislink/x/limits/keeper"
	pauserkeeper "github.com/ayushns01/aegislink/chain/aegislink/x/pauser/keeper"
	registrykeeper "github.com/ayushns01/aegislink/chain/aegislink/x/registry/keeper"
)

var (
	ErrDuplicateClaim                = errors.New("duplicate claim")
	ErrInsufficientAttestationQuorum = errors.New("insufficient attestation quorum")
	ErrFinalityWindowExpired         = errors.New("finality window expired")
	ErrAttestationExpired            = errors.New("attestation expired")
	ErrMessageIDMismatch             = errors.New("message id mismatch")
	ErrPayloadHashMismatch           = errors.New("payload hash mismatch")
	ErrUnknownAsset                  = errors.New("unknown asset")
	ErrAssetDisabled                 = errors.New("asset disabled")
	ErrAssetPaused                   = errors.New("asset paused")
)

type ClaimStatus string

const (
	ClaimStatusAccepted ClaimStatus = "accepted"
	ClaimStatusRejected ClaimStatus = "rejected"
)

type ClaimResult struct {
	Status    ClaimStatus
	MessageID string
	Denom     string
	Amount    *big.Int
}

type Keeper struct {
	registryKeeper *registrykeeper.Keeper
	limitsKeeper   *limitskeeper.Keeper
	pauserKeeper   *pauserkeeper.Keeper

	allowedSigners    map[string]struct{}
	requiredThreshold uint32
	currentHeight     uint64

	processedClaims map[string]ClaimRecord
	supplyByDenom   map[string]*big.Int
}

func NewKeeper(registry *registrykeeper.Keeper, limits *limitskeeper.Keeper, pauser *pauserkeeper.Keeper, allowedSigners []string, requiredThreshold uint32) *Keeper {
	if requiredThreshold == 0 {
		requiredThreshold = 1
	}

	signerSet := make(map[string]struct{}, len(allowedSigners))
	for _, signer := range allowedSigners {
		signerSet[signer] = struct{}{}
	}

	return &Keeper{
		registryKeeper:    registry,
		limitsKeeper:      limits,
		pauserKeeper:      pauser,
		allowedSigners:    signerSet,
		requiredThreshold: requiredThreshold,
		processedClaims:   make(map[string]ClaimRecord),
		supplyByDenom:     make(map[string]*big.Int),
	}
}

func (k *Keeper) SetCurrentHeight(height uint64) {
	k.currentHeight = height
}

func (k *Keeper) ExecuteDepositClaim(claim bridgetypes.DepositClaim, attestation bridgetypes.Attestation) (ClaimResult, error) {
	if err := claim.ValidateBasic(); err != nil {
		return ClaimResult{Status: ClaimStatusRejected}, err
	}

	claimKey := claim.Identity.ReplayKey()
	if _, exists := k.processedClaims[claimKey]; exists {
		return ClaimResult{Status: ClaimStatusRejected, MessageID: claim.Identity.MessageID}, ErrDuplicateClaim
	}

	if err := k.verifyDepositClaim(claim, attestation); err != nil {
		return ClaimResult{Status: ClaimStatusRejected, MessageID: claim.Identity.MessageID}, err
	}

	asset, ok := k.registryKeeper.GetAsset(claim.AssetID)
	if !ok {
		return ClaimResult{Status: ClaimStatusRejected, MessageID: claim.Identity.MessageID}, ErrUnknownAsset
	}
	if !asset.Enabled {
		return ClaimResult{Status: ClaimStatusRejected, MessageID: claim.Identity.MessageID}, ErrAssetDisabled
	}
	if err := k.pauserKeeper.AssertNotPaused(claim.AssetID); err != nil {
		return ClaimResult{Status: ClaimStatusRejected, MessageID: claim.Identity.MessageID}, fmt.Errorf("%w: %s", ErrAssetPaused, claim.AssetID)
	}
	if err := k.limitsKeeper.CheckTransfer(claim.AssetID, claim.Amount); err != nil {
		return ClaimResult{Status: ClaimStatusRejected, MessageID: claim.Identity.MessageID}, err
	}

	return k.acceptDepositClaim(claimKey, claim, asset), nil
}

func (k *Keeper) SupplyForDenom(denom string) *big.Int {
	current, ok := k.supplyByDenom[denom]
	if !ok {
		return big.NewInt(0)
	}
	return new(big.Int).Set(current)
}
