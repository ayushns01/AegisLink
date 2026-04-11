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
	ErrAccountingInvariantBroken     = errors.New("bridge accounting invariant failed")
	ErrBridgeCircuitOpen             = errors.New("bridge circuit breaker is open")
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

	circuitBreakerTripped bool
	lastInvariantError    string

	nextWithdrawalNonce uint64
	prefixStore         *sdkstore.JSONPrefixStore
	legacyStore         *sdkstore.JSONStateStore
}

const (
	bridgeMetaPrefix       = "meta"
	bridgeSignerSetPrefix  = "signer_set"
	bridgeClaimPrefix      = "claim"
	bridgeSupplyPrefix     = "supply"
	bridgeWithdrawalPrefix = "withdrawal"
)

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
	prefixStore, err := sdkstore.NewJSONPrefixStore(multiStore, key)
	if err != nil {
		return nil, err
	}
	stateStore, err := sdkstore.NewJSONStateStore(multiStore, key)
	if err != nil {
		return nil, err
	}

	keeper := NewKeeper(registry, limits, pauser, allowedSigners, requiredThreshold)
	keeper.prefixStore = prefixStore
	keeper.legacyStore = stateStore

	if prefixStore.HasAny(bridgeMetaPrefix) || prefixStore.HasAny(bridgeSignerSetPrefix) || prefixStore.HasAny(bridgeClaimPrefix) || prefixStore.HasAny(bridgeSupplyPrefix) || prefixStore.HasAny(bridgeWithdrawalPrefix) {
		if err := keeper.loadFromPrefixStore(); err != nil {
			return nil, err
		}
		return keeper, nil
	}
	if stateStore.HasState() {
		var state StateSnapshot
		if err := stateStore.Load(&state); err != nil {
			return nil, err
		}
		if err := keeper.ImportState(state); err != nil {
			return nil, err
		}
	}

	return keeper, nil
}

func (k *Keeper) SetCurrentHeight(height uint64) {
	k.currentHeight = height
	_ = k.persist()
}

func (k *Keeper) ExecuteDepositClaim(claim bridgetypes.DepositClaim, attestation bridgetypes.Attestation) (ClaimResult, error) {
	if err := k.ensureCircuitHealthy(); err != nil {
		k.rejectedClaims++
		return ClaimResult{Status: ClaimStatusRejected, MessageID: claim.Identity.MessageID}, err
	}
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
	if err := k.limitsKeeper.CheckTransferAtHeight(claim.AssetID, claim.Amount, k.currentHeight); err != nil {
		k.rejectedClaims++
		return ClaimResult{Status: ClaimStatusRejected, MessageID: claim.Identity.MessageID}, err
	}
	if err := k.limitsKeeper.RecordTransferAtHeight(claim.AssetID, claim.Amount, k.currentHeight); err != nil {
		k.rejectedClaims++
		return ClaimResult{Status: ClaimStatusRejected, MessageID: claim.Identity.MessageID}, err
	}

	result := k.acceptDepositClaim(claimKey, claim, asset)
	if err := k.CheckAccountingInvariant(); err != nil {
		k.rejectedClaims++
		return ClaimResult{Status: ClaimStatusRejected, MessageID: claim.Identity.MessageID}, err
	}
	return result, k.persist()
}

func (k *Keeper) ExecuteWithdrawal(assetID string, amount *big.Int, recipient string, deadline uint64, signature []byte) (WithdrawalRecord, error) {
	if err := k.ensureCircuitHealthy(); err != nil {
		return WithdrawalRecord{}, err
	}
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
	if err := k.limitsKeeper.CheckTransferAtHeight(assetID, amount, k.currentHeight); err != nil {
		return WithdrawalRecord{}, err
	}
	if err := k.limitsKeeper.RecordTransferAtHeight(assetID, amount, k.currentHeight); err != nil {
		return WithdrawalRecord{}, err
	}

	denom := bridgeDenomForAsset(asset)
	currentSupply, ok := k.supplyByDenom[denom]
	if !ok || currentSupply.Cmp(amount) < 0 {
		return WithdrawalRecord{}, ErrInsufficientSupply
	}

	record := k.newWithdrawalRecord(assetID, sourceAssetAddressForWithdrawal(asset), amount, recipient, deadline, signature)
	if err := k.burnRepresentation(denom, amount); err != nil {
		return WithdrawalRecord{}, err
	}
	k.withdrawals = append(k.withdrawals, record)
	if err := k.CheckAccountingInvariant(); err != nil {
		return WithdrawalRecord{}, err
	}

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
	if k.prefixStore == nil {
		return nil
	}
	if err := k.prefixStore.ClearPrefix(bridgeMetaPrefix); err != nil {
		return err
	}
	if err := k.prefixStore.ClearPrefix(bridgeSignerSetPrefix); err != nil {
		return err
	}
	if err := k.prefixStore.ClearPrefix(bridgeClaimPrefix); err != nil {
		return err
	}
	if err := k.prefixStore.ClearPrefix(bridgeSupplyPrefix); err != nil {
		return err
	}
	if err := k.prefixStore.ClearPrefix(bridgeWithdrawalPrefix); err != nil {
		return err
	}
	if err := k.prefixStore.Save(bridgeMetaPrefix, "runtime", bridgeMetadataSnapshot{
		CurrentHeight:         k.currentHeight,
		NextWithdrawalNonce:   k.nextWithdrawalNonce,
		RejectedClaims:        k.rejectedClaims,
		CircuitBreakerTripped: k.circuitBreakerTripped,
		LastInvariantError:    k.lastInvariantError,
	}); err != nil {
		return err
	}
	for _, signerSet := range k.signerSets {
		if err := k.prefixStore.Save(bridgeSignerSetPrefix, fmt.Sprintf("%d", signerSet.Version), SignerSetSnapshot{
			Version:     signerSet.Version,
			Signers:     append([]string(nil), signerSet.Signers...),
			Threshold:   signerSet.Threshold,
			ActivatedAt: signerSet.ActivatedAt,
			ExpiresAt:   signerSet.ExpiresAt,
		}); err != nil {
			return err
		}
	}
	for claimKey, record := range k.processedClaims {
		if err := k.prefixStore.Save(bridgeClaimPrefix, claimKey, ClaimRecordSnapshot{
			ClaimKey:  claimKey,
			MessageID: record.MessageID,
			Denom:     record.Denom,
			AssetID:   record.AssetID,
			Amount:    record.Amount.String(),
			Status:    record.Status,
		}); err != nil {
			return err
		}
	}
	for denom, amount := range k.supplyByDenom {
		if err := k.prefixStore.Save(bridgeSupplyPrefix, denom, amount.String()); err != nil {
			return err
		}
	}
	for idx, withdrawal := range k.withdrawals {
		if err := k.prefixStore.Save(bridgeWithdrawalPrefix, fmt.Sprintf("%020d", idx), WithdrawalRecordSnapshot{
			BlockHeight:  withdrawal.BlockHeight,
			Identity:     withdrawal.Identity,
			AssetID:      withdrawal.AssetID,
			AssetAddress: withdrawal.AssetAddress,
			Amount:       withdrawal.Amount.String(),
			Recipient:    withdrawal.Recipient,
			Deadline:     withdrawal.Deadline,
			Signature:    append([]byte(nil), withdrawal.Signature...),
		}); err != nil {
			return err
		}
	}
	return k.prefixStore.Commit()
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
