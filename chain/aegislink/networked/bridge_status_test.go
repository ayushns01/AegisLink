package networked

import (
	"context"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
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

func TestResolveBridgeSessionViewUsesDeliveryIntentReceiverAndMarksDestinationReceiptComplete(t *testing.T) {
	t.Parallel()

	app := newBridgeStatusTestApp(t)
	claimRecipient := "cosmos1q5nq6v24qq0584nf00wuhqrku4anlxaq80aqy4"
	finalReceiver := "osmo1q5nq6v24qq0584nf00wuhqrku4anlxaq05wsj8"
	claim := bridgeStatusClaim(t, "0xintent-backed-source-tx", claimRecipient, "1000000000000000")
	if _, err := app.SubmitDepositClaim(claim, bridgeStatusAttestation(t, claim)); err != nil {
		t.Fatalf("submit deposit claim: %v", err)
	}
	if _, err := RegisterDeliveryIntent(app.Config, DeliveryIntent{
		SourceTxHash: claim.Identity.SourceTxHash,
		Sender:       claimRecipient,
		RouteID:      "osmosis-public-wallet",
		AssetID:      "eth",
		Amount:       "1000000000000000",
		Receiver:     finalReceiver,
	}); err != nil {
		t.Fatalf("register delivery intent: %v", err)
	}
	transfer, err := app.InitiateIBCTransfer("eth", big.NewInt(1_000_000_000_000_000), finalReceiver, 120, "bridge-status-intent-test")
	if err != nil {
		t.Fatalf("initiate ibc transfer: %v", err)
	}

	view, err := ResolveBridgeSessionView(context.Background(), app, claim.Identity.SourceTxHash, stubDestinationTxResolver{
		result: DestinationTxResult{
			TxHash: "705C76DFC240723143C2D48BC36D9A835D8377F9E9664039FEFEF7D24FD01FA8",
			TxURL:  "https://www.mintscan.io/osmosis-testnet/txs/705C76DFC240723143C2D48BC36D9A835D8377F9E9664039FEFEF7D24FD01FA8",
		},
		found: true,
	})
	if err != nil {
		t.Fatalf("resolve bridge session view: %v", err)
	}
	if view.Status != "completed" {
		t.Fatalf("expected completed status from destination receipt, got %+v", view)
	}
	if view.TransferID != transfer.TransferID {
		t.Fatalf("expected transfer id %q, got %+v", transfer.TransferID, view)
	}
	if view.DestinationTxHash == "" || view.DestinationTxURL == "" {
		t.Fatalf("expected destination tx data, got %+v", view)
	}
}

func TestResolveBridgeSessionViewUsesIntentTransferIDForDuplicateAmountAndReceiver(t *testing.T) {
	t.Parallel()

	app := newBridgeStatusTestApp(t)
	claimRecipient := "cosmos1q5nq6v24qq0584nf00wuhqrku4anlxaq80aqy4"
	finalReceiver := "osmo1q5nq6v24qq0584nf00wuhqrku4anlxaq05wsj8"

	firstClaim := bridgeStatusClaim(t, "0xduplicate-source-tx-1", claimRecipient, "1000000000000000")
	if _, err := app.SubmitDepositClaim(firstClaim, bridgeStatusAttestation(t, firstClaim)); err != nil {
		t.Fatalf("submit first deposit claim: %v", err)
	}
	if _, err := RegisterDeliveryIntent(app.Config, DeliveryIntent{
		SourceTxHash: firstClaim.Identity.SourceTxHash,
		Sender:       claimRecipient,
		RouteID:      "osmosis-public-wallet",
		AssetID:      "eth",
		Amount:       "1000000000000000",
		Receiver:     finalReceiver,
	}); err != nil {
		t.Fatalf("register first delivery intent: %v", err)
	}
	firstTransfer, err := app.InitiateIBCTransfer("eth", big.NewInt(1_000_000_000_000_000), finalReceiver, 120, "first-transfer")
	if err != nil {
		t.Fatalf("initiate first ibc transfer: %v", err)
	}
	if _, err := app.CompleteIBCTransfer(firstTransfer.TransferID); err != nil {
		t.Fatalf("complete first ibc transfer: %v", err)
	}

	secondClaim := bridgeStatusClaim(t, "0xduplicate-source-tx-2", claimRecipient, "1000000000000000")
	if _, err := app.SubmitDepositClaim(secondClaim, bridgeStatusAttestation(t, secondClaim)); err != nil {
		t.Fatalf("submit second deposit claim: %v", err)
	}
	secondTransfer, err := app.InitiateIBCTransfer("eth", big.NewInt(1_000_000_000_000_000), finalReceiver, 120, "second-transfer")
	if err != nil {
		t.Fatalf("initiate second ibc transfer: %v", err)
	}
	if _, err := RegisterDeliveryIntent(app.Config, DeliveryIntent{
		SourceTxHash: secondClaim.Identity.SourceTxHash,
		Sender:       claimRecipient,
		RouteID:      "osmosis-public-wallet",
		AssetID:      "eth",
		Amount:       "1000000000000000",
		Receiver:     finalReceiver,
		TransferID:   secondTransfer.TransferID,
		ChannelID:    secondTransfer.ChannelID,
	}); err != nil {
		t.Fatalf("register second delivery intent: %v", err)
	}

	view, err := ResolveBridgeSessionView(context.Background(), app, secondClaim.Identity.SourceTxHash, nil)
	if err != nil {
		t.Fatalf("resolve bridge session view: %v", err)
	}
	if view.Status != "osmosis_pending" {
		t.Fatalf("expected osmosis_pending status for second transfer, got %+v", view)
	}
	if view.TransferID != secondTransfer.TransferID {
		t.Fatalf("expected second transfer id %q, got %+v", secondTransfer.TransferID, view)
	}
	if view.TransferID == firstTransfer.TransferID {
		t.Fatalf("expected not to reuse first transfer id %q, got %+v", firstTransfer.TransferID, view)
	}
}

func TestDemoNodeServeHTTPBridgeStatusSetsCORSHeaders(t *testing.T) {
	t.Parallel()

	node := DemoNode{}
	ready := ReadyState{Status: "ready", ChainID: "aegislink-public-testnet-1"}

	req := httptest.NewRequest(http.MethodGet, "/bridge-status?sourceTxHash=0xtest", nil)
	recorder := httptest.NewRecorder()

	node.serveHTTP(recorder, req, ready)

	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("expected wildcard cors origin, got %q", got)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Methods"); got == "" {
		t.Fatalf("expected allow methods header to be set")
	}
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
}

func TestDemoNodeServeHTTPOptionsReturnsNoContent(t *testing.T) {
	t.Parallel()

	node := DemoNode{}
	ready := ReadyState{Status: "ready", ChainID: "aegislink-public-testnet-1"}

	req := httptest.NewRequest(http.MethodOptions, "/bridge-status?sourceTxHash=0xtest", nil)
	recorder := httptest.NewRecorder()

	node.serveHTTP(recorder, req, ready)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", recorder.Code)
	}
}

func TestLCDDestinationTxResolverUsesQuerySearchParameter(t *testing.T) {
	t.Parallel()

	client := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if got := r.URL.Query().Get("query"); got != "fungible_token_packet.memo='bridge:0xsource-tx'" {
				t.Fatalf("expected query search parameter, got %q", got)
			}
			if got := r.URL.Query().Get("pagination.limit"); got != "1" {
				t.Fatalf("expected pagination limit of 1, got %q", got)
			}
			if got := r.URL.Query()["events"]; len(got) != 0 {
				t.Fatalf("expected no legacy events parameters, got %v", got)
			}
			body, err := json.Marshal(map[string]any{
				"tx_responses": []map[string]any{
					{"txhash": "D15359D08DAFC92DE5E45D81F1F0FAC54B6433949E8CBF17265195574E98B84F"},
				},
			})
			if err != nil {
				t.Fatalf("marshal response: %v", err)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(string(body))),
				Header:     make(http.Header),
			}, nil
		}),
	}

	result, found, err := LCDDestinationTxResolver{
		BaseURL: "https://lcd.osmotest5.osmosis.zone",
		Client:  client,
	}.Resolve(context.Background(), DestinationTxLookup{
		SourceTxHash:    "0xsource-tx",
		SourceChannelID: "channel-0",
		PacketSequence:  1,
	})
	if err != nil {
		t.Fatalf("resolve destination tx: %v", err)
	}
	if !found {
		t.Fatal("expected destination tx to be found")
	}
	if result.TxHash != "D15359D08DAFC92DE5E45D81F1F0FAC54B6433949E8CBF17265195574E98B84F" {
		t.Fatalf("unexpected tx hash: %+v", result)
	}
}

func TestLCDDestinationTxResolverFallsBackToPacketQueryWithoutSourceTxHash(t *testing.T) {
	t.Parallel()

	client := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if got := r.URL.Query().Get("query"); got != "recv_packet.packet_src_channel='channel-0' AND recv_packet.packet_sequence='1'" {
				t.Fatalf("expected fallback packet query, got %q", got)
			}
			body, err := json.Marshal(map[string]any{
				"tx_responses": []map[string]any{
					{"txhash": "A5D359D08DAFC92DE5E45D81F1F0FAC54B6433949E8CBF17265195574E98B84F"},
				},
			})
			if err != nil {
				t.Fatalf("marshal response: %v", err)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(string(body))),
				Header:     make(http.Header),
			}, nil
		}),
	}

	result, found, err := LCDDestinationTxResolver{
		BaseURL: "https://lcd.osmotest5.osmosis.zone",
		Client:  client,
	}.Resolve(context.Background(), DestinationTxLookup{
		SourceChannelID: "channel-0",
		PacketSequence:  1,
	})
	if err != nil {
		t.Fatalf("resolve destination tx: %v", err)
	}
	if !found {
		t.Fatal("expected destination tx to be found")
	}
	if result.TxHash != "A5D359D08DAFC92DE5E45D81F1F0FAC54B6433949E8CBF17265195574E98B84F" {
		t.Fatalf("unexpected tx hash: %+v", result)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
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
