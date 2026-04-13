package autodelivery

import (
	"context"
	"testing"

	"github.com/ayushns01/aegislink/chain/aegislink/networked"
)

func TestNetworkedTransferSubmitterInitiateTransferRecordsTransferOnIntent(t *testing.T) {
	t.Parallel()

	var (
		submittedPayload networked.InitiateIBCTransferPayload
		recordedIntent   networked.DeliveryIntent
	)

	submitter := NetworkedTransferSubmitter{
		Config:        networked.Config{HomeDir: "/tmp/unused"},
		TimeoutHeight: 55000000,
		submit: func(_ context.Context, _ networked.Config, payload networked.InitiateIBCTransferPayload) (networked.TransferView, error) {
			submittedPayload = payload
			return networked.TransferView{
				TransferID: "ibc/eth/9",
				ChannelID:  "channel-7",
				Status:     "pending",
			}, nil
		},
		registerIntent: func(_ context.Context, _ networked.Config, intent networked.DeliveryIntent) (networked.DeliveryIntent, error) {
			recordedIntent = intent
			return intent, nil
		},
	}

	transfer, err := submitter.InitiateTransfer(context.Background(), Intent{
		SourceTxHash: "0xnew-source-tx",
		Sender:       "cosmos1sender",
		RouteID:      "osmosis-public-wallet",
		AssetID:      "eth",
		Amount:       "1000000000000000",
		Receiver:     "osmo1receiver",
	})
	if err != nil {
		t.Fatalf("initiate transfer: %v", err)
	}

	if transfer.TransferID != "ibc/eth/9" || transfer.ChannelID != "channel-7" {
		t.Fatalf("unexpected submitted transfer: %+v", transfer)
	}
	if submittedPayload.Receiver != "osmo1receiver" || submittedPayload.RouteID != "osmosis-public-wallet" {
		t.Fatalf("unexpected submitted payload: %+v", submittedPayload)
	}
	if submittedPayload.Memo != "bridge:0xnew-source-tx" {
		t.Fatalf("expected bridge memo, got %+v", submittedPayload)
	}
	if recordedIntent.SourceTxHash != "0xnew-source-tx" {
		t.Fatalf("expected source tx hash to be recorded, got %+v", recordedIntent)
	}
	if recordedIntent.TransferID != "ibc/eth/9" || recordedIntent.ChannelID != "channel-7" {
		t.Fatalf("expected transfer linkage to be recorded, got %+v", recordedIntent)
	}
}
