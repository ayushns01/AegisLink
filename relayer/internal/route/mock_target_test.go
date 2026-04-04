package route

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestMockTargetPersistsReceivedPacketAndSwapIntent(t *testing.T) {
	t.Parallel()

	statePath := filepath.Join(t.TempDir(), "mock-osmosis-state.json")
	handler := NewMockTargetHandler("success", statePath, 0)
	target := newHTTPTargetWithClient("http://mock-osmosis", &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, req)
			return recorder.Result(), nil
		}),
	})
	ack, err := target.SubmitTransfer(context.Background(), Transfer{
		TransferID:         "ibc/eth.usdc/1",
		AssetID:            "eth.usdc",
		Amount:             "25000000",
		Receiver:           "osmo1recipient",
		DestinationChainID: "osmosis-1",
		ChannelID:          "channel-0",
		DestinationDenom:   "ibc/uatom-usdc",
		TimeoutHeight:      140,
		Memo:               "swap:uosmo",
	})
	if err != nil {
		t.Fatalf("submit transfer: %v", err)
	}
	if ack.Status != AckStatusCompleted {
		t.Fatalf("expected completed ack, got %q", ack.Status)
	}

	var state struct {
		Receipts []struct {
			TransferID string `json:"transfer_id"`
			Packet     struct {
				Sequence uint64 `json:"sequence"`
				Data     struct {
					Denom    string `json:"denom"`
					Amount   string `json:"amount"`
					Receiver string `json:"receiver"`
					Memo     string `json:"memo"`
				} `json:"data"`
			} `json:"packet"`
			DenomTrace struct {
				Path      string `json:"path"`
				BaseDenom string `json:"base_denom"`
				IBCDenom  string `json:"ibc_denom"`
			} `json:"denom_trace"`
			Action struct {
				Type        string `json:"type"`
				TargetDenom string `json:"target_denom"`
			} `json:"action"`
		} `json:"receipts"`
		Swaps []struct {
			TransferID  string `json:"transfer_id"`
			InputDenom  string `json:"input_denom"`
			OutputDenom string `json:"output_denom"`
			InputAmount string `json:"input_amount"`
			Recipient   string `json:"recipient"`
			DexChainID  string `json:"dex_chain_id"`
		} `json:"swaps"`
	}
	readJSONFile(t, statePath, &state)

	if len(state.Receipts) != 1 {
		t.Fatalf("expected one receipt, got %d", len(state.Receipts))
	}
	receipt := state.Receipts[0]
	if receipt.TransferID != "ibc/eth.usdc/1" {
		t.Fatalf("expected transfer id ibc/eth.usdc/1, got %q", receipt.TransferID)
	}
	if receipt.Packet.Sequence != 1 {
		t.Fatalf("expected packet sequence 1, got %d", receipt.Packet.Sequence)
	}
	if receipt.Packet.Data.Denom != "eth.usdc" {
		t.Fatalf("expected source denom eth.usdc, got %q", receipt.Packet.Data.Denom)
	}
	if receipt.DenomTrace.Path != "transfer/channel-0" {
		t.Fatalf("expected denom trace path transfer/channel-0, got %q", receipt.DenomTrace.Path)
	}
	if receipt.Action.Type != "swap" || receipt.Action.TargetDenom != "uosmo" {
		t.Fatalf("expected swap action to uosmo, got %+v", receipt.Action)
	}
	if len(state.Swaps) != 1 {
		t.Fatalf("expected one swap record, got %d", len(state.Swaps))
	}
	if state.Swaps[0].InputDenom != "ibc/uatom-usdc" {
		t.Fatalf("expected input denom ibc/uatom-usdc, got %q", state.Swaps[0].InputDenom)
	}
	if state.Swaps[0].OutputDenom != "uosmo" {
		t.Fatalf("expected output denom uosmo, got %q", state.Swaps[0].OutputDenom)
	}
}

func readJSONFile(t *testing.T, path string, out any) {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if err := json.Unmarshal(data, out); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
}
