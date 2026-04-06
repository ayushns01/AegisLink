package keeper

import (
	"errors"
	"math/big"
	"testing"

	bridgetypes "github.com/ayushns01/aegislink/chain/aegislink/x/bridge/types"
	limitskeeper "github.com/ayushns01/aegislink/chain/aegislink/x/limits/keeper"
	limittypes "github.com/ayushns01/aegislink/chain/aegislink/x/limits/types"
	pauserkeeper "github.com/ayushns01/aegislink/chain/aegislink/x/pauser/keeper"
	registrykeeper "github.com/ayushns01/aegislink/chain/aegislink/x/registry/keeper"
	registrytypes "github.com/ayushns01/aegislink/chain/aegislink/x/registry/types"
)

func TestExecuteDepositClaimAcceptsValidInboundClaimOnce(t *testing.T) {
	keeper, claim, attestation, _, _, _ := newKeeperFixture(t)

	result, err := keeper.ExecuteDepositClaim(claim, attestation)
	if err != nil {
		t.Fatalf("expected valid claim to succeed, got %v", err)
	}
	if result.Status != ClaimStatusAccepted {
		t.Fatalf("expected accepted status, got %q", result.Status)
	}
	if result.Denom != "uethusdc" {
		t.Fatalf("expected denom uethusdc, got %q", result.Denom)
	}

	supply := keeper.SupplyForDenom("uethusdc")
	if supply.Cmp(claim.Amount) != 0 {
		t.Fatalf("expected supply %s, got %s", claim.Amount.String(), supply.String())
	}
}

func TestExecuteDepositClaimRejectsDuplicateClaim(t *testing.T) {
	keeper, claim, attestation, _, _, _ := newKeeperFixture(t)

	if _, err := keeper.ExecuteDepositClaim(claim, attestation); err != nil {
		t.Fatalf("expected first claim to succeed, got %v", err)
	}

	_, err := keeper.ExecuteDepositClaim(claim, attestation)
	if !errors.Is(err, ErrDuplicateClaim) {
		t.Fatalf("expected duplicate claim error, got %v", err)
	}
}

func TestExecuteDepositClaimRejectsInsufficientAttesterQuorum(t *testing.T) {
	keeper, claim, attestation, _, _, _ := newKeeperFixture(t)
	attestation.Signers = []string{"relayer-1"}
	attestation.Threshold = 1

	_, err := keeper.ExecuteDepositClaim(claim, attestation)
	if !errors.Is(err, ErrInsufficientAttestationQuorum) {
		t.Fatalf("expected insufficient quorum error, got %v", err)
	}
}

func TestExecuteDepositClaimRejectsFinalityWindowViolation(t *testing.T) {
	keeper, claim, attestation, _, _, _ := newKeeperFixture(t)
	keeper.SetCurrentHeight(claim.Deadline + 1)

	_, err := keeper.ExecuteDepositClaim(claim, attestation)
	if !errors.Is(err, ErrFinalityWindowExpired) {
		t.Fatalf("expected finality window error, got %v", err)
	}
}

func TestExecuteDepositClaimRejectsPausedAsset(t *testing.T) {
	keeper, claim, attestation, _, _, pauser := newKeeperFixture(t)

	if err := pauser.Pause(claim.AssetID); err != nil {
		t.Fatalf("expected asset pause to succeed, got %v", err)
	}

	_, err := keeper.ExecuteDepositClaim(claim, attestation)
	if !errors.Is(err, ErrAssetPaused) {
		t.Fatalf("expected paused asset error, got %v", err)
	}
}

func TestExecuteDepositClaimRejectsOverLimitClaim(t *testing.T) {
	keeper, claim, attestation, _, limits, _ := newKeeperFixture(t)

	if err := limits.SetLimit(limittypes.RateLimit{
		AssetID:       claim.AssetID,
		WindowSeconds: 600,
		MaxAmount:     mustAmount("1"),
	}); err != nil {
		t.Fatalf("expected limit update to succeed, got %v", err)
	}

	_, err := keeper.ExecuteDepositClaim(claim, attestation)
	if !errors.Is(err, limitskeeper.ErrRateLimitExceeded) {
		t.Fatalf("expected rate limit exceeded error, got %v", err)
	}
}

func TestExecuteDepositClaimRejectsUnknownAsset(t *testing.T) {
	keeper, claim, attestation, _, _, _ := newKeeperFixture(t)
	claim.AssetID = "eth.unknown"
	attestation.PayloadHash = claim.Digest()

	_, err := keeper.ExecuteDepositClaim(claim, attestation)
	if !errors.Is(err, ErrUnknownAsset) {
		t.Fatalf("expected unknown asset error, got %v", err)
	}
}

func TestExecuteDepositClaimUpdatesAccountingExactlyOnce(t *testing.T) {
	keeper, claim, attestation, _, _, _ := newKeeperFixture(t)

	if _, err := keeper.ExecuteDepositClaim(claim, attestation); err != nil {
		t.Fatalf("expected first claim to succeed, got %v", err)
	}
	if _, err := keeper.ExecuteDepositClaim(claim, attestation); !errors.Is(err, ErrDuplicateClaim) {
		t.Fatalf("expected duplicate claim error, got %v", err)
	}

	supply := keeper.SupplyForDenom("uethusdc")
	if supply.Cmp(claim.Amount) != 0 {
		t.Fatalf("expected supply to remain %s after duplicate attempt, got %s", claim.Amount.String(), supply.String())
	}
}

func TestExecuteWithdrawalBurnsSupplyAndRecordsWithdrawal(t *testing.T) {
	keeper, claim, attestation, _, _, _ := newKeeperFixture(t)

	if _, err := keeper.ExecuteDepositClaim(claim, attestation); err != nil {
		t.Fatalf("expected deposit claim to succeed, got %v", err)
	}

	keeper.SetCurrentHeight(60)
	withdrawal, err := keeper.ExecuteWithdrawal("eth.usdc", mustAmount("40000000"), "0xrecipient", 120, []byte("proof"))
	if err != nil {
		t.Fatalf("expected withdrawal to succeed, got %v", err)
	}
	if withdrawal.BlockHeight != 60 {
		t.Fatalf("expected withdrawal height 60, got %d", withdrawal.BlockHeight)
	}
	if withdrawal.AssetAddress != "0xabc123" {
		t.Fatalf("expected source contract 0xabc123, got %q", withdrawal.AssetAddress)
	}
	if withdrawal.Amount.Cmp(mustAmount("40000000")) != 0 {
		t.Fatalf("expected withdrawal amount 40000000, got %s", withdrawal.Amount.String())
	}

	supply := keeper.SupplyForDenom("uethusdc")
	if supply.Cmp(mustAmount("60000000")) != 0 {
		t.Fatalf("expected remaining supply 60000000, got %s", supply.String())
	}

	withdrawals := keeper.Withdrawals(60, 60)
	if len(withdrawals) != 1 {
		t.Fatalf("expected one stored withdrawal, got %d", len(withdrawals))
	}
	if withdrawals[0].Identity.MessageID != withdrawal.Identity.MessageID {
		t.Fatalf("expected stored withdrawal message id %q, got %q", withdrawal.Identity.MessageID, withdrawals[0].Identity.MessageID)
	}
}

func TestExecuteWithdrawalRejectsWithoutMintedSupply(t *testing.T) {
	keeper, _, _, _, _, _ := newKeeperFixture(t)
	keeper.SetCurrentHeight(60)

	_, err := keeper.ExecuteWithdrawal("eth.usdc", mustAmount("1"), "0xrecipient", 120, []byte("proof"))
	if !errors.Is(err, ErrInsufficientSupply) {
		t.Fatalf("expected insufficient supply error, got %v", err)
	}
}

func TestExecuteWithdrawalRejectsOverLimitClaim(t *testing.T) {
	keeper, claim, attestation, _, limits, _ := newKeeperFixture(t)

	if _, err := keeper.ExecuteDepositClaim(claim, attestation); err != nil {
		t.Fatalf("expected deposit claim to succeed, got %v", err)
	}
	if err := limits.SetLimit(limittypes.RateLimit{
		AssetID:       claim.AssetID,
		WindowSeconds: 600,
		MaxAmount:     mustAmount("1"),
	}); err != nil {
		t.Fatalf("expected limit update to succeed, got %v", err)
	}

	keeper.SetCurrentHeight(60)
	_, err := keeper.ExecuteWithdrawal(claim.AssetID, mustAmount("2"), "0xrecipient", 120, []byte("proof"))
	if !errors.Is(err, limitskeeper.ErrRateLimitExceeded) {
		t.Fatalf("expected rate limit exceeded error, got %v", err)
	}
}

func TestBridgeReplayResistanceCountsOnlyUniqueClaims(t *testing.T) {
	t.Parallel()

	keeper, baseClaim, _, _, _, _ := newKeeperFixture(t)

	totalAccepted := big.NewInt(0)
	for i := 0; i < 5; i++ {
		claim := baseClaim
		claim.Identity.SourceTxHash = "0xdeadbeef" + string(rune('a'+i))
		claim.Identity.SourceLogIndex = uint64(i + 1)
		claim.Identity.Nonce = uint64(i + 1)
		claim.Identity.MessageID = claim.Identity.DerivedMessageID()
		claim.Amount = mustAmount("10000000")
		attestation := validAttestation(claim)

		if _, err := keeper.ExecuteDepositClaim(claim, attestation); err != nil {
			t.Fatalf("expected unique claim %d to succeed, got %v", i, err)
		}
		if _, err := keeper.ExecuteDepositClaim(claim, attestation); !errors.Is(err, ErrDuplicateClaim) {
			t.Fatalf("expected duplicate rejection for claim %d, got %v", i, err)
		}
		totalAccepted.Add(totalAccepted, claim.Amount)
	}

	state := keeper.ExportState()
	if len(state.ProcessedClaims) != 5 {
		t.Fatalf("expected 5 processed claims, got %d", len(state.ProcessedClaims))
	}
	if state.RejectedClaims != 5 {
		t.Fatalf("expected 5 rejected duplicate claims, got %d", state.RejectedClaims)
	}
	if supply := keeper.SupplyForDenom("uethusdc"); supply.Cmp(totalAccepted) != 0 {
		t.Fatalf("expected supply %s after duplicates, got %s", totalAccepted.String(), supply.String())
	}
}

func TestBridgeSupplyConservationAcrossDepositAndWithdrawalSequence(t *testing.T) {
	t.Parallel()

	keeper, baseClaim, _, _, _, _ := newKeeperFixture(t)

	accepted := big.NewInt(0)
	withdrawn := big.NewInt(0)
	withdrawAmounts := []string{"5000000", "7000000", "3000000"}
	depositAmounts := []string{"10000000", "12000000", "8000000"}

	for i, amount := range depositAmounts {
		claim := baseClaim
		claim.Identity.SourceTxHash = "0xbeefcafe" + string(rune('a'+i))
		claim.Identity.SourceLogIndex = uint64(i + 10)
		claim.Identity.Nonce = uint64(i + 10)
		claim.Identity.MessageID = claim.Identity.DerivedMessageID()
		claim.Amount = mustAmount(amount)
		attestation := validAttestation(claim)

		if _, err := keeper.ExecuteDepositClaim(claim, attestation); err != nil {
			t.Fatalf("deposit %d failed: %v", i, err)
		}
		accepted.Add(accepted, claim.Amount)
	}

	for i, amount := range withdrawAmounts {
		keeper.SetCurrentHeight(uint64(60 + i))
		withdrawAmount := mustAmount(amount)
		if _, err := keeper.ExecuteWithdrawal("eth.usdc", withdrawAmount, "0xrecipient", 200, []byte("proof")); err != nil {
			t.Fatalf("withdrawal %d failed: %v", i, err)
		}
		withdrawn.Add(withdrawn, withdrawAmount)
	}

	expectedSupply := new(big.Int).Sub(accepted, withdrawn)
	if supply := keeper.SupplyForDenom("uethusdc"); supply.Cmp(expectedSupply) != 0 {
		t.Fatalf("expected conserved supply %s, got %s", expectedSupply.String(), supply.String())
	}
	if len(keeper.Withdrawals(60, 62)) != len(withdrawAmounts) {
		t.Fatalf("expected %d withdrawals recorded, got %d", len(withdrawAmounts), len(keeper.Withdrawals(60, 62)))
	}
}

func newKeeperFixture(t *testing.T) (*Keeper, bridgetypes.DepositClaim, bridgetypes.Attestation, *registrykeeper.Keeper, *limitskeeper.Keeper, *pauserkeeper.Keeper) {
	t.Helper()

	registry := registrykeeper.NewKeeper()
	limits := limitskeeper.NewKeeper()
	pauser := pauserkeeper.NewKeeper()

	asset := registrytypes.Asset{
		AssetID:        "eth.usdc",
		SourceChainID:  "ethereum-1",
		SourceContract: "0xabc123",
		Denom:          "uethusdc",
		Decimals:       6,
		DisplayName:    "USDC",
		Enabled:        true,
	}
	if err := registry.RegisterAsset(asset); err != nil {
		t.Fatalf("expected asset registration to succeed, got %v", err)
	}
	if err := limits.SetLimit(limittypes.RateLimit{
		AssetID:       asset.AssetID,
		WindowSeconds: 600,
		MaxAmount:     mustAmount("100000000000000000000"),
	}); err != nil {
		t.Fatalf("expected rate limit registration to succeed, got %v", err)
	}

	keeper := NewKeeper(registry, limits, pauser, []string{"relayer-1", "relayer-2", "relayer-3"}, 2)
	keeper.SetCurrentHeight(50)

	claim := validDepositClaim()
	attestation := validAttestation(claim)

	return keeper, claim, attestation, registry, limits, pauser
}

func validDepositClaim() bridgetypes.DepositClaim {
	identity := bridgetypes.ClaimIdentity{
		Kind:           bridgetypes.ClaimKindDeposit,
		SourceChainID:  "ethereum-1",
		SourceContract: "0xabc123",
		SourceTxHash:   "0xdeadbeef",
		SourceLogIndex: 7,
		Nonce:          1,
	}
	identity.MessageID = identity.DerivedMessageID()

	return bridgetypes.DepositClaim{
		Identity:           identity,
		DestinationChainID: "aegislink-1",
		AssetID:            "eth.usdc",
		Amount:             mustAmount("100000000"),
		Recipient:          "cosmos1recipient",
		Deadline:           100,
	}
}

func validAttestation(claim bridgetypes.DepositClaim) bridgetypes.Attestation {
	return bridgetypes.Attestation{
		MessageID:   claim.Identity.MessageID,
		PayloadHash: claim.Digest(),
		Signers:     []string{"relayer-1", "relayer-2"},
		Threshold:   2,
		Expiry:      100,
	}
}

func mustAmount(value string) *big.Int {
	amount, ok := new(big.Int).SetString(value, 10)
	if !ok {
		panic("invalid test amount")
	}
	return amount
}
