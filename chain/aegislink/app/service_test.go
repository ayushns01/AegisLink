package app

import (
	"errors"
	"math/big"
	"testing"

	bankkeeper "github.com/ayushns01/aegislink/chain/aegislink/x/bank/keeper"
	bridgekeeper "github.com/ayushns01/aegislink/chain/aegislink/x/bridge/keeper"
	bridgetypes "github.com/ayushns01/aegislink/chain/aegislink/x/bridge/types"
	governancekeeper "github.com/ayushns01/aegislink/chain/aegislink/x/governance/keeper"
	ibcrouterkeeper "github.com/ayushns01/aegislink/chain/aegislink/x/ibcrouter/keeper"
	limittypes "github.com/ayushns01/aegislink/chain/aegislink/x/limits/types"
	registrytypes "github.com/ayushns01/aegislink/chain/aegislink/x/registry/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func TestBridgeQueryServiceReturnsStoredClaim(t *testing.T) {
	app := New()
	seedBridgeRuntime(t, app)

	claim := sampleDepositClaim()
	attestation := sampleAttestation(claim)
	if _, err := app.SubmitDepositClaim(claim, attestation); err != nil {
		t.Fatalf("submit deposit claim: %v", err)
	}

	service := NewBridgeQueryService(app)
	record, ok := service.GetClaim(claim.Identity.MessageID)
	if !ok {
		t.Fatalf("expected stored claim %q", claim.Identity.MessageID)
	}
	if record.MessageID != claim.Identity.MessageID {
		t.Fatalf("expected message id %q, got %q", claim.Identity.MessageID, record.MessageID)
	}
}

func TestBridgeTxServiceSubmitsDepositClaim(t *testing.T) {
	app := New()
	seedBridgeRuntime(t, app)

	claim := sampleDepositClaim()
	attestation := sampleAttestation(claim)

	service := NewBridgeTxService(app)
	result, err := service.SubmitDepositClaim(claim, attestation)
	if err != nil {
		t.Fatalf("submit deposit claim: %v", err)
	}
	if result.Status != "accepted" {
		t.Fatalf("expected accepted claim result, got %+v", result)
	}
}

func TestBridgeTxServiceRejectsInvalidRecipientBeforeBridgeMutation(t *testing.T) {
	app := New()
	seedBridgeRuntime(t, app)

	claim := sampleDepositClaim()
	claim.Recipient = "not-a-bech32-address"
	attestation := sampleAttestation(claim)

	service := NewBridgeTxService(app)
	if _, err := service.SubmitDepositClaim(claim, attestation); !errors.Is(err, bankkeeper.ErrInvalidAddress) {
		t.Fatalf("expected invalid address error, got %v", err)
	}

	query := NewBridgeQueryService(app)
	if _, ok := query.GetClaim(claim.Identity.MessageID); ok {
		t.Fatalf("expected invalid-recipient claim %q to remain unprocessed", claim.Identity.MessageID)
	}

	if supply := app.BridgeKeeper.SupplyForDenom("uethusdc"); supply.Sign() != 0 {
		t.Fatalf("expected zero bridge supply after invalid recipient rejection, got %s", supply.String())
	}
}

func TestBridgeTxServiceRollsBackDepositWhenWalletCreditFails(t *testing.T) {
	app := New()
	seedBridgeRuntime(t, app)

	claim := sampleDepositClaim()
	attestation := sampleAttestation(claim)
	failed := false
	app.BankKeeper.SetPersistHookForTesting(func() error {
		if failed {
			return nil
		}
		failed = true
		return errors.New("forced bank persist failure")
	})

	service := NewBridgeTxService(app)
	if _, err := service.SubmitDepositClaim(claim, attestation); err == nil {
		t.Fatalf("expected wallet credit failure")
	}

	query := NewBridgeQueryService(app)
	if _, ok := query.GetClaim(claim.Identity.MessageID); ok {
		t.Fatalf("expected failed wallet credit to roll back processed claim %q", claim.Identity.MessageID)
	}
	if supply := app.BridgeKeeper.SupplyForDenom("uethusdc"); supply.Sign() != 0 {
		t.Fatalf("expected zero bridge supply after rollback, got %s", supply.String())
	}
	if usage, ok := app.LimitsKeeper.CurrentUsage("eth.usdc", 0); ok {
		t.Fatalf("expected no persisted usage after rollback, got %+v", usage)
	}
}

func TestIBCRouterQueryServiceListsRoutesAndTransfers(t *testing.T) {
	app := New()
	seedBridgeRuntime(t, app)

	if _, err := app.InitiateIBCTransfer("eth.usdc", big.NewInt(25000000), "osmo1receiver", 120, "swap:uosmo"); err != nil {
		t.Fatalf("initiate ibc transfer: %v", err)
	}

	service := NewIBCRouterQueryService(app)
	routes := service.ListRoutes()
	transfers := service.ListTransfers()

	if len(routes) != 1 {
		t.Fatalf("expected one route, got %d", len(routes))
	}
	if len(transfers) != 1 {
		t.Fatalf("expected one transfer, got %d", len(transfers))
	}
	if transfers[0].Status != ibcrouterkeeper.TransferStatusPending {
		t.Fatalf("expected pending transfer, got %q", transfers[0].Status)
	}
}

func TestBridgeQueryServiceReturnsActiveSignerSetAndHistory(t *testing.T) {
	app := New()
	seedBridgeRuntime(t, app)
	app.SetCurrentHeight(90)

	if err := app.BridgeKeeper.UpsertSignerSet(bridgekeeper.SignerSet{
		Version:     2,
		Signers:     bridgetypes.DefaultHarnessSignerAddresses()[1:4],
		Threshold:   2,
		ActivatedAt: 80,
	}); err != nil {
		t.Fatalf("upsert signer set: %v", err)
	}

	service := NewBridgeQueryService(app)
	active, err := service.ActiveSignerSet()
	if err != nil {
		t.Fatalf("active signer set: %v", err)
	}
	if active.Version != 2 {
		t.Fatalf("expected active signer set version 2, got %d", active.Version)
	}

	sets := service.ListSignerSets()
	if len(sets) != 2 {
		t.Fatalf("expected two signer sets, got %d", len(sets))
	}
	if sets[0].Version != 1 || sets[1].Version != 2 {
		t.Fatalf("expected signer set history [1 2], got %+v", sets)
	}
}

func TestGovernanceTxServiceRequiresAuthorizedAuthority(t *testing.T) {
	app := New()
	seedBridgeRuntime(t, app)

	service := NewGovernanceTxService(app)
	err := service.ApplyAssetStatusProposal("intruder", governancekeeper.AssetStatusProposal{
		ProposalID: "asset-disable-unauthorized",
		AssetID:    "eth.usdc",
		Enabled:    false,
	})
	if !errors.Is(err, governancekeeper.ErrUnauthorizedProposal) {
		t.Fatalf("expected unauthorized governance proposal error, got %v", err)
	}

	if err := service.ApplyAssetStatusProposal("guardian-1", governancekeeper.AssetStatusProposal{
		ProposalID: "asset-disable-authorized",
		AssetID:    "eth.usdc",
		Enabled:    false,
	}); err != nil {
		t.Fatalf("apply authorized asset status proposal: %v", err)
	}

	asset, ok := app.RegistryKeeper.GetAsset("eth.usdc")
	if !ok || asset.Enabled {
		t.Fatalf("expected authorized governance proposal to disable asset, got %+v exists=%t", asset, ok)
	}
}

func seedBridgeRuntime(t *testing.T, app *App) {
	t.Helper()

	if err := app.RegisterAsset(registrytypes.Asset{
		AssetID:        "eth.usdc",
		SourceChainID:  "ethereum-sepolia",
		SourceContract: "0xabc123",
		Denom:          "uethusdc",
		Decimals:       6,
		DisplayName:    "Ethereum USDC",
		Enabled:        true,
	}); err != nil {
		t.Fatalf("register asset: %v", err)
	}

	if err := app.SetLimit(limittypes.RateLimit{
		AssetID:       "eth.usdc",
		WindowSeconds: 600,
		MaxAmount:     big.NewInt(250000000),
	}); err != nil {
		t.Fatalf("set limit: %v", err)
	}

	if err := app.IBCRouterKeeper.SetRoute(ibcrouterkeeper.Route{
		AssetID:            "eth.usdc",
		DestinationChainID: "osmosis-local-1",
		ChannelID:          "channel-0",
		DestinationDenom:   "ibc/uethusdc",
		Enabled:            true,
	}); err != nil {
		t.Fatalf("set route: %v", err)
	}
}

func sampleDepositClaim() bridgetypes.DepositClaim {
	identity := bridgetypes.ClaimIdentity{
		Kind:           bridgetypes.ClaimKindDeposit,
		SourceChainID:  "ethereum-sepolia",
		SourceContract: "0xbridge",
		SourceTxHash:   "0xdeadbeef",
		SourceLogIndex: 1,
		Nonce:          1,
	}
	identity.MessageID = identity.DerivedMessageID()

	return bridgetypes.DepositClaim{
		Identity:           identity,
		DestinationChainID: "aegislink-local-1",
		AssetID:            "eth.usdc",
		Amount:             big.NewInt(25000000),
		Recipient:          sdk.AccAddress([]byte("service-test-wallet")).String(),
		Deadline:           100,
	}
}

func sampleAttestation(claim bridgetypes.DepositClaim) bridgetypes.Attestation {
	attestation := bridgetypes.Attestation{
		MessageID:        claim.Identity.MessageID,
		PayloadHash:      claim.Digest(),
		Signers:          bridgetypes.DefaultHarnessSignerAddresses()[:2],
		Threshold:        2,
		Expiry:           200,
		SignerSetVersion: 1,
	}
	for _, key := range bridgetypes.DefaultHarnessSignerPrivateKeys()[:2] {
		proof, err := bridgetypes.SignAttestationWithPrivateKeyHex(attestation, key)
		if err != nil {
			panic(err)
		}
		attestation.Proofs = append(attestation.Proofs, proof)
	}
	return attestation
}
