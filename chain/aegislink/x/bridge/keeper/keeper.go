package keeper

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"math/big"
	"strings"

	storetypes "cosmossdk.io/store/types"
	"github.com/ayushns01/aegislink/chain/aegislink/internal/sdkstore"
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
	ErrSignerSetVersionMismatch      = errors.New("signer set version mismatch")
	ErrSignerSetInactive             = errors.New("signer set inactive")
	ErrInvalidAttestationSignature   = errors.New("invalid attestation signature")
	ErrUnknownAsset                  = errors.New("unknown asset")
	ErrAssetDisabled                 = errors.New("asset disabled")
	ErrAssetPaused                   = errors.New("asset paused")
	ErrInsufficientSupply            = errors.New("insufficient supply")
	ErrInvalidWithdrawal             = errors.New("invalid withdrawal")
)

type ClaimStatus string

const (
	ClaimStatusAccepted ClaimStatus = "accepted"
	ClaimStatusRejected ClaimStatus = "rejected"

	withdrawalSourceChainID = "aegislink-1"
	withdrawalSourceModule  = "aegislink.bridge"
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

	signerSets    map[uint64]SignerSet
	currentHeight uint64

	processedClaims map[string]ClaimRecord
	rejectedClaims  uint64
	supplyByDenom   map[string]*big.Int
	withdrawals     []WithdrawalRecord

	nextWithdrawalNonce uint64
	stateStore          *sdkstore.JSONStateStore
}

func NewKeeper(registry *registrykeeper.Keeper, limits *limitskeeper.Keeper, pauser *pauserkeeper.Keeper, allowedSigners []string, requiredThreshold uint32) *Keeper {
	if requiredThreshold == 0 {
		requiredThreshold = 1
	}

	keeper := &Keeper{
		registryKeeper:      registry,
		limitsKeeper:        limits,
		pauserKeeper:        pauser,
		signerSets:          make(map[uint64]SignerSet, 1),
		processedClaims:     make(map[string]ClaimRecord),
		supplyByDenom:       make(map[string]*big.Int),
		nextWithdrawalNonce: 1,
	}
	keeper.signerSets[1] = normalizeSignerSet(SignerSet{
		Version:     1,
		Signers:     append([]string(nil), allowedSigners...),
		Threshold:   requiredThreshold,
		ActivatedAt: 0,
	})
	return keeper
}

func NewStoreKeeper(
	multiStore storetypes.CommitMultiStore,
	key *storetypes.KVStoreKey,
	registry *registrykeeper.Keeper,
	limits *limitskeeper.Keeper,
	pauser *pauserkeeper.Keeper,
	allowedSigners []string,
	requiredThreshold uint32,
) (*Keeper, error) {
	stateStore, err := sdkstore.NewJSONStateStore(multiStore, key)
	if err != nil {
		return nil, err
	}

	keeper := NewKeeper(registry, limits, pauser, allowedSigners, requiredThreshold)
	keeper.stateStore = stateStore

	var state StateSnapshot
	if err := stateStore.Load(&state); err != nil {
		return nil, err
	}
	if err := keeper.ImportState(state); err != nil {
		return nil, err
	}

	return keeper, nil
}

func (k *Keeper) SetCurrentHeight(height uint64) {
	k.currentHeight = height
	_ = k.persist()
}

func (k *Keeper) ExecuteDepositClaim(claim bridgetypes.DepositClaim, attestation bridgetypes.Attestation) (ClaimResult, error) {
	if err := claim.ValidateBasic(); err != nil {
		k.rejectedClaims++
		return ClaimResult{Status: ClaimStatusRejected}, err
	}

	claimKey := claim.Identity.ReplayKey()
	if _, exists := k.processedClaims[claimKey]; exists {
		k.rejectedClaims++
		return ClaimResult{Status: ClaimStatusRejected, MessageID: claim.Identity.MessageID}, ErrDuplicateClaim
	}

	if err := k.verifyDepositClaim(claim, attestation); err != nil {
		k.rejectedClaims++
		return ClaimResult{Status: ClaimStatusRejected, MessageID: claim.Identity.MessageID}, err
	}

	asset, ok := k.registryKeeper.GetAsset(claim.AssetID)
	if !ok {
		k.rejectedClaims++
		return ClaimResult{Status: ClaimStatusRejected, MessageID: claim.Identity.MessageID}, ErrUnknownAsset
	}
	if !asset.Enabled {
		k.rejectedClaims++
		return ClaimResult{Status: ClaimStatusRejected, MessageID: claim.Identity.MessageID}, ErrAssetDisabled
	}
	if err := k.pauserKeeper.AssertNotPaused(claim.AssetID); err != nil {
		k.rejectedClaims++
		return ClaimResult{Status: ClaimStatusRejected, MessageID: claim.Identity.MessageID}, fmt.Errorf("%w: %s", ErrAssetPaused, claim.AssetID)
	}
	if err := k.limitsKeeper.CheckTransfer(claim.AssetID, claim.Amount); err != nil {
		k.rejectedClaims++
		return ClaimResult{Status: ClaimStatusRejected, MessageID: claim.Identity.MessageID}, err
	}

	result := k.acceptDepositClaim(claimKey, claim, asset)
	return result, k.persist()
}

func (k *Keeper) ExecuteWithdrawal(assetID string, amount *big.Int, recipient string, deadline uint64, signature []byte) (WithdrawalRecord, error) {
	if amount == nil || amount.Sign() <= 0 {
		return WithdrawalRecord{}, fmt.Errorf("%w: amount must be positive", ErrInvalidWithdrawal)
	}
	if strings.TrimSpace(assetID) == "" {
		return WithdrawalRecord{}, fmt.Errorf("%w: missing asset id", ErrInvalidWithdrawal)
	}
	if strings.TrimSpace(recipient) == "" || isZeroHexAddress(recipient) {
		return WithdrawalRecord{}, fmt.Errorf("%w: invalid recipient", ErrInvalidWithdrawal)
	}
	if deadline == 0 {
		return WithdrawalRecord{}, fmt.Errorf("%w: missing deadline", ErrInvalidWithdrawal)
	}
	if len(signature) == 0 {
		return WithdrawalRecord{}, fmt.Errorf("%w: missing signature", ErrInvalidWithdrawal)
	}

	asset, ok := k.registryKeeper.GetAsset(assetID)
	if !ok {
		return WithdrawalRecord{}, ErrUnknownAsset
	}
	if !asset.Enabled {
		return WithdrawalRecord{}, ErrAssetDisabled
	}
	if err := k.pauserKeeper.AssertNotPaused(assetID); err != nil {
		return WithdrawalRecord{}, fmt.Errorf("%w: %s", ErrAssetPaused, assetID)
	}
	if err := k.limitsKeeper.CheckTransfer(assetID, amount); err != nil {
		return WithdrawalRecord{}, err
	}

	currentSupply, ok := k.supplyByDenom[asset.Denom]
	if !ok || currentSupply.Cmp(amount) < 0 {
		return WithdrawalRecord{}, ErrInsufficientSupply
	}

	record := k.newWithdrawalRecord(assetID, asset.SourceContract, amount, recipient, deadline, signature)
	k.burnRepresentation(asset.Denom, amount)
	k.withdrawals = append(k.withdrawals, record)

	return cloneWithdrawalRecord(record), k.persist()
}

func (k *Keeper) SupplyForDenom(denom string) *big.Int {
	current, ok := k.supplyByDenom[denom]
	if !ok {
		return big.NewInt(0)
	}
	return new(big.Int).Set(current)
}

func (k *Keeper) Withdrawals(fromHeight, toHeight uint64) []WithdrawalRecord {
	if fromHeight > toHeight {
		return nil
	}

	withdrawals := make([]WithdrawalRecord, 0, len(k.withdrawals))
	for _, withdrawal := range k.withdrawals {
		if withdrawal.BlockHeight < fromHeight || withdrawal.BlockHeight > toHeight {
			continue
		}
		withdrawals = append(withdrawals, cloneWithdrawalRecord(withdrawal))
	}
	return withdrawals
}

func (k *Keeper) RejectedClaims() uint64 {
	return k.rejectedClaims
}

func (k *Keeper) persist() error {
	if k.stateStore == nil {
		return nil
	}
	return k.stateStore.Save(k.ExportState())
}

func (k *Keeper) Flush() error {
	return k.persist()
}

func (k *Keeper) newWithdrawalRecord(assetID, assetAddress string, amount *big.Int, recipient string, deadline uint64, signature []byte) WithdrawalRecord {
	nonce := k.nextWithdrawalNonce
	k.nextWithdrawalNonce++

	txHash := fmt.Sprintf("0x%x", sha256.Sum256([]byte(
		fmt.Sprintf("%d:%d:%s:%s:%s", k.currentHeight, nonce, assetID, strings.TrimSpace(recipient), amount.String()),
	)))
	identity := bridgetypes.ClaimIdentity{
		Kind:           bridgetypes.ClaimKindWithdrawal,
		SourceChainID:  withdrawalSourceChainID,
		SourceContract: withdrawalSourceModule,
		SourceTxHash:   txHash,
		SourceLogIndex: 0,
		Nonce:          nonce,
	}
	identity.MessageID = identity.DerivedMessageID()

	return WithdrawalRecord{
		BlockHeight:  k.currentHeight,
		Identity:     identity,
		AssetID:      strings.TrimSpace(assetID),
		AssetAddress: strings.TrimSpace(assetAddress),
		Amount:       cloneAmount(amount),
		Recipient:    strings.TrimSpace(recipient),
		Deadline:     deadline,
		Signature:    append([]byte(nil), signature...),
	}
}

func isZeroHexAddress(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return false
	}
	return strings.EqualFold(trimmed, "0x0000000000000000000000000000000000000000")
}
