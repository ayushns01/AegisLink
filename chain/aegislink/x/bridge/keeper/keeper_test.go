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

func TestExecuteDepositClaimMintsNativeETHCanonicalDenom(t *testing.T) {
	keeper, claim, attestation, _, _, _ := newNativeKeeperFixture(t)

	result, err := keeper.ExecuteDepositClaim(claim, attestation)
	if err != nil {
		t.Fatalf("expected native ETH claim to succeed, got %v", err)
	}
	if result.Status != ClaimStatusAccepted {
		t.Fatalf("expected accepted status, got %q", result.Status)
	}
	if result.Denom != "ueth" {
		t.Fatalf("expected denom ueth, got %q", result.Denom)
	}

	supply := keeper.SupplyForDenom("ueth")
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
	attestation.Proofs = signAttestationForTests(t, attestation, 0)
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
	attestation.Proofs = signAttestationForTestsFromHelpers(attestation, 0, 1)

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

func TestExecuteWithdrawalForNativeETHUsesCanonicalZeroAddress(t *testing.T) {
	keeper, claim, attestation, _, _, _ := newNativeKeeperFixture(t)

	if _, err := keeper.ExecuteDepositClaim(claim, attestation); err != nil {
		t.Fatalf("expected native ETH deposit claim to succeed, got %v", err)
	}

	keeper.SetCurrentHeight(75)
	withdrawal, err := keeper.ExecuteWithdrawal("eth", mustAmount("200000000000000000"), "0xrecipient", 160, []byte("proof"))
	if err != nil {
		t.Fatalf("expected native ETH withdrawal to succeed, got %v", err)
	}
	if withdrawal.AssetAddress != "0x0000000000000000000000000000000000000000" {
		t.Fatalf("expected canonical native ETH zero address, got %q", withdrawal.AssetAddress)
	}
	if supply := keeper.SupplyForDenom("ueth"); supply.Cmp(mustAmount("800000000000000000")) != 0 {
		t.Fatalf("expected remaining native ETH supply 800000000000000000, got %s", supply.String())
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

func TestBridgeAccountingSeparatesNativeETHAndERC20Supply(t *testing.T) {
	t.Parallel()

	keeper, erc20Claim, erc20Attestation, registry, limits, _ := newKeeperFixture(t)
	nativeClaim := validNativeDepositClaim()
	nativeAttestation := validAttestation(nativeClaim)

	if err := registry.RegisterAsset(registrytypes.Asset{
		AssetID:         "eth",
		SourceChainID:   "ethereum-11155111",
		SourceAssetKind: registrytypes.SourceAssetKindNativeETH,
		DisplayName:     "Ether",
		DisplaySymbol:   "ETH",
		Decimals:        18,
		Enabled:         true,
	}); err != nil {
		t.Fatalf("expected native asset registration to succeed, got %v", err)
	}
	if err := limits.SetLimit(limittypes.RateLimit{
		AssetID:       "eth",
		WindowSeconds: 600,
		MaxAmount:     mustAmount("100000000000000000000"),
	}); err != nil {
		t.Fatalf("expected native rate limit registration to succeed, got %v", err)
	}

	if _, err := keeper.ExecuteDepositClaim(erc20Claim, erc20Attestation); err != nil {
		t.Fatalf("expected ERC-20 deposit claim to succeed, got %v", err)
	}
	if _, err := keeper.ExecuteDepositClaim(nativeClaim, nativeAttestation); err != nil {
		t.Fatalf("expected native ETH deposit claim to succeed, got %v", err)
	}

	if supply := keeper.SupplyForDenom("uethusdc"); supply.Cmp(erc20Claim.Amount) != 0 {
		t.Fatalf("expected ERC-20 supply %s, got %s", erc20Claim.Amount.String(), supply.String())
	}
	if supply := keeper.SupplyForDenom("ueth"); supply.Cmp(nativeClaim.Amount) != 0 {
		t.Fatalf("expected native ETH supply %s, got %s", nativeClaim.Amount.String(), supply.String())
	}
}

func TestBridgeAccountingInvariantRejectsTamperedSupplyAndTripsCircuitBreaker(t *testing.T) {
	t.Parallel()

	keeper, claim, attestation, _, _, _ := newKeeperFixture(t)
	if _, err := keeper.ExecuteDepositClaim(claim, attestation); err != nil {
		t.Fatalf("expected first deposit claim to succeed, got %v", err)
	}

	keeper.supplyByDenom["uethusdc"] = mustAmount("999999999")
	if err := keeper.CheckAccountingInvariant(); !errors.Is(err, ErrAccountingInvariantBroken) {
		t.Fatalf("expected accounting invariant error after tamper, got %v", err)
	}

	secondClaim := claim
	secondClaim.Identity.SourceTxHash = "0xbeadcafe"
	secondClaim.Identity.SourceLogIndex = 8
	secondClaim.Identity.Nonce = 2
	secondClaim.Identity.MessageID = secondClaim.Identity.DerivedMessageID()
	secondAttestation := validAttestation(secondClaim)

	_, err := keeper.ExecuteDepositClaim(secondClaim, secondAttestation)
	if !errors.Is(err, ErrBridgeCircuitOpen) {
		t.Fatalf("expected circuit breaker rejection after invariant failure, got %v", err)
	}
	if !keeper.CircuitBreakerTripped() {
		t.Fatal("expected circuit breaker to remain tripped")
	}
}

func TestBurnRepresentationRejectsUnderflow(t *testing.T) {
	t.Parallel()

	keeper, _, _, _, _, _ := newKeeperFixture(t)
	err := keeper.burnRepresentation("uethusdc", mustAmount("1"))
	if !errors.Is(err, ErrInsufficientSupply) {
		t.Fatalf("expected insufficient supply from direct underflow burn, got %v", err)
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

	keeper := NewKeeper(registry, limits, pauser, bridgetypes.DefaultHarnessSignerAddresses()[:3], 2)
	keeper.SetCurrentHeight(50)

	claim := validDepositClaim()
	attestation := validAttestation(claim)

	return keeper, claim, attestation, registry, limits, pauser
}

func newNativeKeeperFixture(t *testing.T) (*Keeper, bridgetypes.DepositClaim, bridgetypes.Attestation, *registrykeeper.Keeper, *limitskeeper.Keeper, *pauserkeeper.Keeper) {
	t.Helper()

	registry := registrykeeper.NewKeeper()
	limits := limitskeeper.NewKeeper()
	pauser := pauserkeeper.NewKeeper()

	asset := registrytypes.Asset{
		AssetID:         "eth",
		SourceChainID:   "ethereum-11155111",
		SourceAssetKind: registrytypes.SourceAssetKindNativeETH,
		DisplayName:     "Ether",
		DisplaySymbol:   "ETH",
		Decimals:        18,
		Enabled:         true,
	}
	if err := registry.RegisterAsset(asset); err != nil {
		t.Fatalf("expected native asset registration to succeed, got %v", err)
	}
	if err := limits.SetLimit(limittypes.RateLimit{
		AssetID:       asset.AssetID,
		WindowSeconds: 600,
		MaxAmount:     mustAmount("100000000000000000000"),
	}); err != nil {
		t.Fatalf("expected native rate limit registration to succeed, got %v", err)
	}

	keeper := NewKeeper(registry, limits, pauser, bridgetypes.DefaultHarnessSignerAddresses()[:3], 2)
	keeper.SetCurrentHeight(50)

	claim := validNativeDepositClaim()
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

func validNativeDepositClaim() bridgetypes.DepositClaim {
	identity := bridgetypes.ClaimIdentity{
		Kind:            bridgetypes.ClaimKindDeposit,
		SourceAssetKind: bridgetypes.SourceAssetKindNativeETH,
		SourceChainID:   "ethereum-11155111",
		SourceTxHash:    "0xfeedbeef",
		SourceLogIndex:  9,
		Nonce:           2,
	}
	identity.MessageID = identity.DerivedMessageID()

	return bridgetypes.DepositClaim{
		Identity:           identity,
		DestinationChainID: "aegislink-1",
		AssetID:            "eth",
		Amount:             mustAmount("1000000000000000000"),
		Recipient:          "cosmos1nativeeth",
		Deadline:           100,
	}
}

func validAttestation(claim bridgetypes.DepositClaim) bridgetypes.Attestation {
	attestation := bridgetypes.Attestation{
		MessageID:        claim.Identity.MessageID,
		PayloadHash:      claim.Digest(),
		Signers:          bridgetypes.DefaultHarnessSignerAddresses()[:2],
		Threshold:        2,
		Expiry:           100,
		SignerSetVersion: 1,
	}
	attestation.Proofs = signAttestationForTestsFromHelpers(attestation, 0, 1)
	return attestation
}

func signAttestationForTestsFromHelpers(attestation bridgetypes.Attestation, signerIndexes ...int) []bridgetypes.AttestationProof {
	signers := bridgetypes.DefaultHarnessAttestationSigners()
	proofs := make([]bridgetypes.AttestationProof, 0, len(signerIndexes))
	for _, idx := range signerIndexes {
		proof, err := bridgetypes.SignAttestationWithPrivateKeyHex(attestation, signers[idx].PrivateKeyHex)
		if err != nil {
			panic(err)
		}
		proofs = append(proofs, proof)
	}
	return proofs
}

func mustAmount(value string) *big.Int {
	amount, ok := new(big.Int).SetString(value, 10)
	if !ok {
		panic("invalid test amount")
	}
	return amount
}
