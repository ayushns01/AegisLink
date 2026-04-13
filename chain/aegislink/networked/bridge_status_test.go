package networked

import (
	"context"
	"math/big"
	"path/filepath"
	"testing"

	aegisapp "github.com/ayushns01/aegislink/chain/aegislink/app"
	bridgetypes "github.com/ayushns01/aegislink/chain/aegislink/x/bridge/types"
	ibcrouterkeeper "github.com/ayushns01/aegislink/chain/aegislink/x/ibcrouter/keeper"
	limittypes "github.com/ayushns01/aegislink/chain/aegislink/x/limits/types"
	registrytypes "github.com/ayushns01/aegislink/chain/aegislink/x/registry/types"
)

func TestResolveBridgeSessionViewReturnsSepoliaConfirmingWhenClaimIsNotProcessed(t *testing.T) {
	t.Parallel()

	app := newBridgeStatusTestApp(t)

	view, err := ResolveBridgeSessionView(context.Background(), app, "0xmissing-source-tx", nil)
	if err != nil {
		t.Fatalf("resolve bridge session view: %v", err)
	}
	if view.Status != "sepolia_confirming" {
		t.Fatalf("expected sepolia_confirming status, got %+v", view)
	}
}

func TestResolveBridgeSessionViewReturnsAegisLinkProcessingAfterClaimAcceptance(t *testing.T) {
	t.Parallel()

	app := newBridgeStatusTestApp(t)
	recipient := "cosmos1q5nq6v24qq0584nf00wuhqrku4anlxaq80aqy4"
	claim := bridgeStatusClaim(t, "0xaccepted-source-tx", recipient, "1000000000000000")
	if _, err := app.SubmitDepositClaim(claim, bridgeStatusAttestation(t, claim)); err != nil {
		t.Fatalf("submit deposit claim: %v", err)
	}

	view, err := ResolveBridgeSessionView(context.Background(), app, claim.Identity.SourceTxHash, nil)
	if err != nil {
		t.Fatalf("resolve bridge session view: %v", err)
	}
	if view.Status != "aegislink_processing" {
		t.Fatalf("expected aegislink_processing status, got %+v", view)
	}
	if view.MessageID != claim.Identity.MessageID {
		t.Fatalf("expected message id %q, got %+v", claim.Identity.MessageID, view)
	}
}

func TestResolveBridgeSessionViewReturnsCompletedDestinationTxHash(t *testing.T) {
	t.Parallel()

	app := newBridgeStatusTestApp(t)
	recipient := "cosmos1q5nq6v24qq0584nf00wuhqrku4anlxaq80aqy4"
	claim := bridgeStatusClaim(t, "0xcompleted-source-tx", recipient, "1000000000000000")
	if _, err := app.SubmitDepositClaim(claim, bridgeStatusAttestation(t, claim)); err != nil {
		t.Fatalf("submit deposit claim: %v", err)
	}
	transfer, err := app.InitiateIBCTransfer("eth", big.NewInt(1_000_000_000_000_000), recipient, 120, "bridge-status-test")
	if err != nil {
		t.Fatalf("initiate ibc transfer: %v", err)
	}
	if _, err := app.CompleteIBCTransfer(transfer.TransferID); err != nil {
		t.Fatalf("complete ibc transfer: %v", err)
	}

	view, err := ResolveBridgeSessionView(context.Background(), app, claim.Identity.SourceTxHash, stubDestinationTxResolver{
		result: DestinationTxResult{
			TxHash: "5E40ED4BF5B065DA159D66785534EAAEEE376876749DADAF639F6A51524B2F7D",
			TxURL:  "https://www.mintscan.io/osmosis-testnet/txs/5E40ED4BF5B065DA159D66785534EAAEEE376876749DADAF639F6A51524B2F7D",
		},
		found: true,
	})
	if err != nil {
		t.Fatalf("resolve bridge session view: %v", err)
	}
	if view.Status != "completed" {
		t.Fatalf("expected completed status, got %+v", view)
	}
	if view.TransferID != transfer.TransferID {
		t.Fatalf("expected transfer id %q, got %+v", transfer.TransferID, view)
	}
	if view.DestinationTxHash == "" || view.DestinationTxURL == "" {
		t.Fatalf("expected destination tx data, got %+v", view)
	}
}

func newBridgeStatusTestApp(t *testing.T) *aegisapp.App {
	t.Helper()

	homeDir := filepath.Join(t.TempDir(), "bridge-status-home")
	cfg, err := aegisapp.InitHome(aegisapp.Config{
		HomeDir:     homeDir,
		ChainID:     "aegislink-public-testnet-1",
		RuntimeMode: aegisapp.RuntimeModeSDKStore,
	}, false)
	if err != nil {
		t.Fatalf("init home: %v", err)
	}
	app, err := aegisapp.LoadWithConfig(cfg)
	if err != nil {
		t.Fatalf("load app: %v", err)
	}
	t.Cleanup(func() {
		if err := app.Close(); err != nil {
			t.Fatalf("close app: %v", err)
		}
	})
	if err := app.RegisterAsset(registrytypes.Asset{
		AssetID:         "eth",
		SourceChainID:   "11155111",
		SourceAssetKind: registrytypes.SourceAssetKindNativeETH,
		Denom:           "ueth",
		Decimals:        18,
		DisplayName:     "Ether",
		DisplaySymbol:   "ETH",
		Enabled:         true,
	}); err != nil {
		t.Fatalf("register asset: %v", err)
	}
	if err := app.SetLimit(limittypes.RateLimit{
		AssetID:       "eth",
		WindowSeconds: 600,
		MaxAmount:     big.NewInt(2_000_000_000_000_000_000),
	}); err != nil {
		t.Fatalf("set limit: %v", err)
	}
	if err := app.SetRoute(ibcrouterkeeper.Route{
		AssetID:            "eth",
		DestinationChainID: "osmo-test-5",
		ChannelID:          "channel-0",
		DestinationDenom:   "ibc/ueth",
		Enabled:            true,
	}); err != nil {
		t.Fatalf("set route: %v", err)
	}
	return app
}

func bridgeStatusClaim(t *testing.T, sourceTxHash, recipient, amount string) bridgetypes.DepositClaim {
	t.Helper()

	parsedAmount, ok := new(big.Int).SetString(amount, 10)
	if !ok {
		t.Fatalf("invalid amount %q", amount)
	}
	identity := bridgetypes.ClaimIdentity{
		Kind:            bridgetypes.ClaimKindDeposit,
		SourceAssetKind: bridgetypes.SourceAssetKindNativeETH,
		SourceChainID:   "11155111",
		SourceTxHash:    sourceTxHash,
		SourceLogIndex:  1,
		Nonce:           1,
	}
	identity.MessageID = identity.DerivedMessageID()
	return bridgetypes.DepositClaim{
		Identity:           identity,
		DestinationChainID: "aegislink-public-testnet-1",
		AssetID:            "eth",
		Amount:             parsedAmount,
		Recipient:          recipient,
		Deadline:           120,
	}
}

func bridgeStatusAttestation(t *testing.T, claim bridgetypes.DepositClaim) bridgetypes.Attestation {
	t.Helper()

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
			t.Fatalf("sign attestation: %v", err)
		}
		attestation.Proofs = append(attestation.Proofs, proof)
	}
	return attestation
}

type stubDestinationTxResolver struct {
	result DestinationTxResult
	found  bool
	err    error
}

func (s stubDestinationTxResolver) Resolve(_ context.Context, _ DestinationTxLookup) (DestinationTxResult, bool, error) {
	return s.result, s.found, s.err
}
