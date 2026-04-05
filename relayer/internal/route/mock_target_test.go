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
	handler := NewMockTargetHandler("manual", statePath, 0)
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
	if ack.Status != AckStatusReceived {
		t.Fatalf("expected received ack, got %q", ack.Status)
	}

	var delivered struct {
		Packets []struct {
			TransferID  string `json:"transfer_id"`
			PacketState string `json:"packet_state"`
			AckState    string `json:"ack_state"`
			AckRelayed  bool   `json:"ack_relayed"`
			Packet      struct {
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
		} `json:"packets"`
		Executions []struct{} `json:"executions"`
		Swaps      []struct{} `json:"swaps"`
		Balances   []struct{} `json:"balances"`
	}
	readJSONFile(t, statePath, &delivered)

	if len(delivered.Packets) != 1 {
		t.Fatalf("expected one packet after delivery, got %d", len(delivered.Packets))
	}
	packet := delivered.Packets[0]
	if packet.TransferID != "ibc/eth.usdc/1" {
		t.Fatalf("expected transfer id ibc/eth.usdc/1, got %q", packet.TransferID)
	}
	if packet.PacketState != "received" {
		t.Fatalf("expected received packet state after delivery, got %q", packet.PacketState)
	}
	if packet.AckState != "pending" {
		t.Fatalf("expected pending ack state after delivery, got %q", packet.AckState)
	}
	if packet.AckRelayed {
		t.Fatal("expected packet ack to be unrelayed after delivery")
	}
	if packet.Packet.Sequence != 1 {
		t.Fatalf("expected packet sequence 1, got %d", packet.Packet.Sequence)
	}
	if packet.Packet.Data.Denom != "eth.usdc" {
		t.Fatalf("expected source denom eth.usdc, got %q", packet.Packet.Data.Denom)
	}
	if packet.DenomTrace.Path != "transfer/channel-0" {
		t.Fatalf("expected denom trace path transfer/channel-0, got %q", packet.DenomTrace.Path)
	}
	if packet.Action.Type != "swap" || packet.Action.TargetDenom != "uosmo" {
		t.Fatalf("expected swap action to uosmo, got %+v", packet.Action)
	}
	if len(delivered.Executions) != 0 {
		t.Fatalf("expected no executions before ack poll, got %d", len(delivered.Executions))
	}
	if len(delivered.Swaps) != 0 {
		t.Fatalf("expected no swaps before ack poll, got %d", len(delivered.Swaps))
	}
	if len(delivered.Balances) != 0 {
		t.Fatalf("expected no balances before ack poll, got %d", len(delivered.Balances))
	}

	acks, err := target.ReadyAcks(context.Background())
	if err != nil {
		t.Fatalf("ready acks before resolution: %v", err)
	}
	if len(acks) != 0 {
		t.Fatalf("expected no ready acks before resolution, got %d", len(acks))
	}

	var state struct {
		Packets []struct {
			TransferID  string `json:"transfer_id"`
			PacketState string `json:"packet_state"`
			AckState    string `json:"ack_state"`
			AckRelayed  bool   `json:"ack_relayed"`
			Packet      struct {
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
		} `json:"packets"`
		Executions []struct {
			TransferID   string `json:"transfer_id"`
			Result       string `json:"result"`
			InputDenom   string `json:"input_denom"`
			OutputDenom  string `json:"output_denom"`
			OutputAmount string `json:"output_amount"`
			Recipient    string `json:"recipient"`
			DexChainID   string `json:"dex_chain_id"`
		} `json:"executions"`
		Swaps []struct {
			TransferID   string `json:"transfer_id"`
			InputDenom   string `json:"input_denom"`
			OutputDenom  string `json:"output_denom"`
			InputAmount  string `json:"input_amount"`
			OutputAmount string `json:"output_amount"`
			Recipient    string `json:"recipient"`
			DexChainID   string `json:"dex_chain_id"`
		} `json:"swaps"`
		Pools []struct {
			InputDenom  string `json:"input_denom"`
			OutputDenom string `json:"output_denom"`
			ReserveIn   string `json:"reserve_in"`
			ReserveOut  string `json:"reserve_out"`
		} `json:"pools"`
		Balances []struct {
			Address string `json:"address"`
			Denom   string `json:"denom"`
			Amount  string `json:"amount"`
		} `json:"balances"`
	}
	readJSONFile(t, statePath, &state)

	if len(state.Packets) != 1 {
		t.Fatalf("expected one packet, got %d", len(state.Packets))
	}
	packet = state.Packets[0]
	if packet.TransferID != "ibc/eth.usdc/1" {
		t.Fatalf("expected transfer id ibc/eth.usdc/1, got %q", packet.TransferID)
	}
	if packet.PacketState != "executed" {
		t.Fatalf("expected executed packet state after ack poll, got %q", packet.PacketState)
	}
	if packet.AckState != "pending" {
		t.Fatalf("expected pending ack state, got %q", packet.AckState)
	}
	if packet.AckRelayed {
		t.Fatal("expected ack to be unrelayed")
	}
	if packet.Packet.Sequence != 1 {
		t.Fatalf("expected packet sequence 1, got %d", packet.Packet.Sequence)
	}
	if packet.Packet.Data.Denom != "eth.usdc" {
		t.Fatalf("expected source denom eth.usdc, got %q", packet.Packet.Data.Denom)
	}
	if packet.DenomTrace.Path != "transfer/channel-0" {
		t.Fatalf("expected denom trace path transfer/channel-0, got %q", packet.DenomTrace.Path)
	}
	if packet.Action.Type != "swap" || packet.Action.TargetDenom != "uosmo" {
		t.Fatalf("expected swap action to uosmo, got %+v", packet.Action)
	}
	if len(state.Executions) != 1 {
		t.Fatalf("expected one execution record, got %d", len(state.Executions))
	}
	if state.Executions[0].Result != "swap_success" {
		t.Fatalf("expected swap_success execution, got %q", state.Executions[0].Result)
	}
	if state.Executions[0].InputDenom != "ibc/uatom-usdc" {
		t.Fatalf("expected input denom ibc/uatom-usdc, got %q", state.Executions[0].InputDenom)
	}
	if state.Executions[0].OutputDenom != "uosmo" {
		t.Fatalf("expected output denom uosmo, got %q", state.Executions[0].OutputDenom)
	}
	if state.Executions[0].OutputAmount != "47619047" {
		t.Fatalf("expected execution output amount 47619047, got %q", state.Executions[0].OutputAmount)
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
	if state.Swaps[0].OutputAmount != "47619047" {
		t.Fatalf("expected output amount 47619047, got %q", state.Swaps[0].OutputAmount)
	}
	if len(state.Pools) != 1 {
		t.Fatalf("expected one pool record, got %d", len(state.Pools))
	}
	if state.Pools[0].ReserveIn != "525000000" {
		t.Fatalf("expected input reserve 525000000, got %q", state.Pools[0].ReserveIn)
	}
	if state.Pools[0].ReserveOut != "952380953" {
		t.Fatalf("expected output reserve 952380953, got %q", state.Pools[0].ReserveOut)
	}
	if len(state.Balances) != 1 {
		t.Fatalf("expected one balance record, got %d", len(state.Balances))
	}
	if state.Balances[0].Address != "osmo1recipient" {
		t.Fatalf("expected balance address osmo1recipient, got %q", state.Balances[0].Address)
	}
	if state.Balances[0].Denom != "uosmo" {
		t.Fatalf("expected balance denom uosmo, got %q", state.Balances[0].Denom)
	}
	if state.Balances[0].Amount != "47619047" {
		t.Fatalf("expected balance amount 47619047, got %q", state.Balances[0].Amount)
	}
}

func TestMockTargetExposesPoolsBalancesAndSwapsEndpoints(t *testing.T) {
	t.Parallel()

	statePath := filepath.Join(t.TempDir(), "mock-osmosis-state.json")
	handler := NewMockTargetHandler("manual", statePath, 0)
	target := newHTTPTargetWithClient("http://mock-osmosis", &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, req)
			return recorder.Result(), nil
		}),
	})

	if _, err := target.SubmitTransfer(context.Background(), Transfer{
		TransferID:         "ibc/eth.usdc/8",
		AssetID:            "eth.usdc",
		Amount:             "25000000",
		Receiver:           "osmo1query",
		DestinationChainID: "osmosis-1",
		ChannelID:          "channel-0",
		DestinationDenom:   "ibc/uatom-usdc",
		TimeoutHeight:      140,
		Memo:               "swap:uosmo",
	}); err != nil {
		t.Fatalf("submit transfer: %v", err)
	}
	if _, err := target.ReadyAcks(context.Background()); err != nil {
		t.Fatalf("ready acks: %v", err)
	}

	assertEndpointJSONCount(t, target.client, target.baseURL+"/packets", 1)
	assertEndpointJSONCount(t, target.client, target.baseURL+"/executions", 1)
	assertEndpointJSONCount(t, target.client, target.baseURL+"/pools", 1)
	assertEndpointJSONCount(t, target.client, target.baseURL+"/balances", 1)
	assertEndpointJSONCount(t, target.client, target.baseURL+"/swaps", 1)
}

func TestMockTargetExposesStatusEndpoint(t *testing.T) {
	t.Parallel()

	statePath := filepath.Join(t.TempDir(), "mock-osmosis-state.json")
	handler := NewMockTargetHandler("manual", statePath, 0)
	target := newHTTPTargetWithClient("http://mock-osmosis", &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, req)
			return recorder.Result(), nil
		}),
	})

	if _, err := target.SubmitTransfer(context.Background(), Transfer{
		TransferID:         "ibc/eth.usdc/9",
		AssetID:            "eth.usdc",
		Amount:             "25000000",
		Receiver:           "osmo1status",
		DestinationChainID: "osmosis-1",
		ChannelID:          "channel-0",
		DestinationDenom:   "ibc/uatom-usdc",
		TimeoutHeight:      140,
		Memo:               "swap:uosmo",
	}); err != nil {
		t.Fatalf("submit transfer: %v", err)
	}

	resp, err := target.client.Get(target.baseURL + "/status")
	if err != nil {
		t.Fatalf("get status: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from status, got %d", resp.StatusCode)
	}

	var status struct {
		Packets         int `json:"packets"`
		Receipts        int `json:"receipts"`
		Executions      int `json:"executions"`
		Pools           int `json:"pools"`
		Balances        int `json:"balances"`
		Swaps           int `json:"swaps"`
		ReceivedPackets int `json:"received_packets"`
		ExecutedPackets int `json:"executed_packets"`
		ReadyAcks       int `json:"ready_acks"`
		CompletedAcks   int `json:"completed_acks"`
		FailedAcks      int `json:"failed_acks"`
		TimedOutAcks    int `json:"timed_out_acks"`
		RelayedAcks     int `json:"relayed_acks"`
		PendingReceipts int `json:"pending_receipts"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		t.Fatalf("decode status: %v", err)
	}
	if status.Packets != 1 || status.Receipts != 1 || status.Executions != 0 || status.Pools != 1 || status.Balances != 0 || status.Swaps != 0 {
		t.Fatalf("unexpected status counts: %+v", status)
	}
	if status.ReceivedPackets != 1 || status.ExecutedPackets != 0 {
		t.Fatalf("unexpected packet lifecycle counts: %+v", status)
	}
	if status.PendingReceipts != 1 {
		t.Fatalf("expected 1 pending receipt, got %d", status.PendingReceipts)
	}
	if status.ReadyAcks != 0 || status.CompletedAcks != 0 || status.FailedAcks != 0 || status.TimedOutAcks != 0 || status.RelayedAcks != 0 {
		t.Fatalf("unexpected ack counters before resolution: %+v", status)
	}
}

func TestMockTargetCreditsRecipientBalanceForPlainIBCReceive(t *testing.T) {
	t.Parallel()

	statePath := filepath.Join(t.TempDir(), "mock-osmosis-state.json")
	handler := NewMockTargetHandler("manual", statePath, 0)
	target := newHTTPTargetWithClient("http://mock-osmosis", &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, req)
			return recorder.Result(), nil
		}),
	})

	ack, err := target.SubmitTransfer(context.Background(), Transfer{
		TransferID:         "ibc/eth.usdc/2",
		AssetID:            "eth.usdc",
		Amount:             "17000000",
		Receiver:           "osmo1plain",
		DestinationChainID: "osmosis-1",
		ChannelID:          "channel-0",
		DestinationDenom:   "ibc/uatom-usdc",
		TimeoutHeight:      140,
	})
	if err != nil {
		t.Fatalf("submit transfer: %v", err)
	}
	if ack.Status != AckStatusReceived {
		t.Fatalf("expected received ack, got %q", ack.Status)
	}
	if _, err := target.ReadyAcks(context.Background()); err != nil {
		t.Fatalf("ready acks: %v", err)
	}

	var state struct {
		Packets []struct {
			TransferID  string `json:"transfer_id"`
			PacketState string `json:"packet_state"`
		} `json:"packets"`
		Executions []struct {
			Result string `json:"result"`
		} `json:"executions"`
		Swaps []struct {
			TransferID string `json:"transfer_id"`
		} `json:"swaps"`
		Balances []struct {
			Address string `json:"address"`
			Denom   string `json:"denom"`
			Amount  string `json:"amount"`
		} `json:"balances"`
	}
	readJSONFile(t, statePath, &state)

	if len(state.Packets) != 1 {
		t.Fatalf("expected one packet, got %d", len(state.Packets))
	}
	if state.Packets[0].PacketState != "executed" {
		t.Fatalf("expected executed packet state, got %q", state.Packets[0].PacketState)
	}
	if len(state.Executions) != 1 || state.Executions[0].Result != "credit" {
		t.Fatalf("expected one credit execution, got %+v", state.Executions)
	}
	if len(state.Swaps) != 0 {
		t.Fatalf("expected no swaps for plain receive, got %d", len(state.Swaps))
	}
	if len(state.Balances) != 1 {
		t.Fatalf("expected one balance record, got %d", len(state.Balances))
	}
	if state.Balances[0].Address != "osmo1plain" {
		t.Fatalf("expected balance address osmo1plain, got %q", state.Balances[0].Address)
	}
	if state.Balances[0].Denom != "ibc/uatom-usdc" {
		t.Fatalf("expected ibc denom balance, got %q", state.Balances[0].Denom)
	}
	if state.Balances[0].Amount != "17000000" {
		t.Fatalf("expected balance amount 17000000, got %q", state.Balances[0].Amount)
	}
}

func TestMockTargetRejectsSwapWhenMinOutIsNotMet(t *testing.T) {
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
		TransferID:         "ibc/eth.usdc/3",
		AssetID:            "eth.usdc",
		Amount:             "25000000",
		Receiver:           "osmo1minout",
		DestinationChainID: "osmosis-1",
		ChannelID:          "channel-0",
		DestinationDenom:   "ibc/uatom-usdc",
		TimeoutHeight:      140,
		Memo:               "swap:uosmo:min_out=50000000",
	})
	if err != nil {
		t.Fatalf("submit transfer: %v", err)
	}
	if ack.Status != AckStatusReceived {
		t.Fatalf("expected received ack, got %q", ack.Status)
	}

	acks, err := target.ReadyAcks(context.Background())
	if err != nil {
		t.Fatalf("ready acks: %v", err)
	}
	if len(acks) != 1 {
		t.Fatalf("expected one ready ack, got %d", len(acks))
	}
	if acks[0].Status != AckStatusFailed {
		t.Fatalf("expected failed ack, got %q", acks[0].Status)
	}

	var state struct {
		Receipts []struct {
			AckState  string `json:"ack_state"`
			AckReason string `json:"ack_reason"`
		} `json:"receipts"`
		Swaps    []struct{} `json:"swaps"`
		Balances []struct{} `json:"balances"`
	}
	readJSONFile(t, statePath, &state)
	if len(state.Receipts) != 1 {
		t.Fatalf("expected one receipt, got %d", len(state.Receipts))
	}
	if state.Receipts[0].AckState != "ack_failed" {
		t.Fatalf("expected ack_failed state, got %q", state.Receipts[0].AckState)
	}
	if state.Receipts[0].AckReason == "" {
		t.Fatal("expected failure reason to be recorded")
	}
	if len(state.Swaps) != 0 {
		t.Fatalf("expected no swaps, got %d", len(state.Swaps))
	}
	if len(state.Balances) != 0 {
		t.Fatalf("expected no balances, got %d", len(state.Balances))
	}
}

func TestMockTargetRejectsSwapWhenTargetPoolIsMissing(t *testing.T) {
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
		TransferID:         "ibc/eth.usdc/4",
		AssetID:            "eth.usdc",
		Amount:             "25000000",
		Receiver:           "osmo1missingpool",
		DestinationChainID: "osmosis-1",
		ChannelID:          "channel-0",
		DestinationDenom:   "ibc/uatom-usdc",
		TimeoutHeight:      140,
		Memo:               "swap:uion",
	})
	if err != nil {
		t.Fatalf("submit transfer: %v", err)
	}
	if ack.Status != AckStatusReceived {
		t.Fatalf("expected received ack, got %q", ack.Status)
	}

	acks, err := target.ReadyAcks(context.Background())
	if err != nil {
		t.Fatalf("ready acks: %v", err)
	}
	if len(acks) != 1 {
		t.Fatalf("expected one ready ack, got %d", len(acks))
	}
	if acks[0].Status != AckStatusFailed {
		t.Fatalf("expected failed ack, got %q", acks[0].Status)
	}
	if acks[0].Reason == "" {
		t.Fatal("expected missing pool reason")
	}
}

func TestMockTargetRejectsSwapWhenPoolHasNoOutputLiquidity(t *testing.T) {
	t.Parallel()

	target := &MockTarget{
		state: MockTargetState{
			Pools: []MockTargetPool{
				{
					InputDenom:  "ibc/uatom-usdc",
					OutputDenom: "uosmo",
					ReserveIn:   "500000000",
					ReserveOut:  "0",
				},
			},
		},
	}

	packet := &MockTargetPacket{
		TransferID:         "ibc/eth.usdc/5",
		DestinationChainID: "osmosis-1",
		PacketState:        "received",
		AckState:           "pending",
		Packet: Packet{
			Sequence: 5,
			Data: PacketData{
				Amount:   "25000000",
				Receiver: "osmo1noliquidity",
			},
		},
		DenomTrace: DenomTrace{IBCDenom: "ibc/uatom-usdc"},
		Action: &RouteAction{
			Type:        "swap",
			TargetDenom: "uosmo",
		},
	}
	target.executePacketLocked(packet)

	if packet.PacketState != "ack_ready" {
		t.Fatalf("expected ack_ready packet state, got %q", packet.PacketState)
	}
	if packet.AckState != "ack_failed" {
		t.Fatalf("expected ack_failed state, got %q", packet.AckState)
	}
	if packet.AckReason == "" {
		t.Fatal("expected insufficient liquidity reason")
	}
	if len(target.state.Executions) != 1 || target.state.Executions[0].Result != "swap_failed" {
		t.Fatalf("expected one swap_failed execution, got %+v", target.state.Executions)
	}
	if len(target.state.Swaps) != 0 {
		t.Fatalf("expected no swap records, got %d", len(target.state.Swaps))
	}
	if len(target.state.Balances) != 0 {
		t.Fatalf("expected no balances, got %d", len(target.state.Balances))
	}
}

func TestMockTargetUsesConfiguredFeeAwarePoolForSwap(t *testing.T) {
	t.Parallel()

	statePath := filepath.Join(t.TempDir(), "mock-osmosis-state.json")
	handler := NewMockTargetHandlerWithConfig(MockTargetConfig{
		Mode:      "manual",
		StatePath: statePath,
		Pools: []MockTargetPool{
			{
				InputDenom:  "ibc/uatom-usdc",
				OutputDenom: "uosmo",
				ReserveIn:   "500000000",
				ReserveOut:  "1000000000",
				FeeBPS:      100,
			},
		},
	})
	target := newHTTPTargetWithClient("http://mock-osmosis", &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, req)
			return recorder.Result(), nil
		}),
	})

	ack, err := target.SubmitTransfer(context.Background(), Transfer{
		TransferID:         "ibc/eth.usdc/6",
		AssetID:            "eth.usdc",
		Amount:             "25000000",
		Receiver:           "osmo1feeaware",
		DestinationChainID: "osmosis-1",
		ChannelID:          "channel-0",
		DestinationDenom:   "ibc/uatom-usdc",
		TimeoutHeight:      140,
		Memo:               "swap:uosmo",
	})
	if err != nil {
		t.Fatalf("submit transfer: %v", err)
	}
	if ack.Status != AckStatusReceived {
		t.Fatalf("expected received ack, got %q", ack.Status)
	}
	if _, err := target.ReadyAcks(context.Background()); err != nil {
		t.Fatalf("ready acks: %v", err)
	}

	var state struct {
		Swaps []struct {
			OutputAmount string `json:"output_amount"`
		} `json:"swaps"`
		Pools []struct {
			ReserveIn  string `json:"reserve_in"`
			ReserveOut string `json:"reserve_out"`
			FeeBPS     uint32 `json:"fee_bps"`
		} `json:"pools"`
		Balances []struct {
			Denom  string `json:"denom"`
			Amount string `json:"amount"`
		} `json:"balances"`
	}
	readJSONFile(t, statePath, &state)

	if len(state.Swaps) != 1 {
		t.Fatalf("expected one swap record, got %d", len(state.Swaps))
	}
	if state.Swaps[0].OutputAmount != "47165316" {
		t.Fatalf("expected output amount 47165316, got %q", state.Swaps[0].OutputAmount)
	}
	if len(state.Pools) != 1 {
		t.Fatalf("expected one pool, got %d", len(state.Pools))
	}
	if state.Pools[0].FeeBPS != 100 {
		t.Fatalf("expected fee bps 100, got %d", state.Pools[0].FeeBPS)
	}
	if state.Pools[0].ReserveIn != "524750000" {
		t.Fatalf("expected reserve in 524750000, got %q", state.Pools[0].ReserveIn)
	}
	if state.Pools[0].ReserveOut != "952834684" {
		t.Fatalf("expected reserve out 952834684, got %q", state.Pools[0].ReserveOut)
	}
	if len(state.Balances) != 1 {
		t.Fatalf("expected one balance, got %d", len(state.Balances))
	}
	if state.Balances[0].Denom != "uosmo" {
		t.Fatalf("expected uosmo balance, got %q", state.Balances[0].Denom)
	}
	if state.Balances[0].Amount != "47165316" {
		t.Fatalf("expected credited amount 47165316, got %q", state.Balances[0].Amount)
	}
}

func TestMockTargetUsesConfiguredAlternatePoolForDifferentOutputDenom(t *testing.T) {
	t.Parallel()

	statePath := filepath.Join(t.TempDir(), "mock-osmosis-state.json")
	handler := NewMockTargetHandlerWithConfig(MockTargetConfig{
		Mode:      "manual",
		StatePath: statePath,
		Pools: []MockTargetPool{
			{
				InputDenom:  "ibc/uatom-usdc",
				OutputDenom: "uion",
				ReserveIn:   "800000000",
				ReserveOut:  "400000000",
				FeeBPS:      0,
			},
		},
	})
	target := newHTTPTargetWithClient("http://mock-osmosis", &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, req)
			return recorder.Result(), nil
		}),
	})

	ack, err := target.SubmitTransfer(context.Background(), Transfer{
		TransferID:         "ibc/eth.usdc/7",
		AssetID:            "eth.usdc",
		Amount:             "25000000",
		Receiver:           "osmo1altpool",
		DestinationChainID: "osmosis-1",
		ChannelID:          "channel-0",
		DestinationDenom:   "ibc/uatom-usdc",
		TimeoutHeight:      140,
		Memo:               "swap:uion",
	})
	if err != nil {
		t.Fatalf("submit transfer: %v", err)
	}
	if ack.Status != AckStatusReceived {
		t.Fatalf("expected received ack, got %q", ack.Status)
	}
	if _, err := target.ReadyAcks(context.Background()); err != nil {
		t.Fatalf("ready acks: %v", err)
	}

	var state struct {
		Swaps []struct {
			OutputDenom  string `json:"output_denom"`
			OutputAmount string `json:"output_amount"`
		} `json:"swaps"`
		Balances []struct {
			Address string `json:"address"`
			Denom   string `json:"denom"`
			Amount  string `json:"amount"`
		} `json:"balances"`
	}
	readJSONFile(t, statePath, &state)

	if len(state.Swaps) != 1 {
		t.Fatalf("expected one swap record, got %d", len(state.Swaps))
	}
	if state.Swaps[0].OutputDenom != "uion" {
		t.Fatalf("expected output denom uion, got %q", state.Swaps[0].OutputDenom)
	}
	if state.Swaps[0].OutputAmount != "12121212" {
		t.Fatalf("expected output amount 12121212, got %q", state.Swaps[0].OutputAmount)
	}
	if len(state.Balances) != 1 {
		t.Fatalf("expected one balance, got %d", len(state.Balances))
	}
	if state.Balances[0].Address != "osmo1altpool" {
		t.Fatalf("expected altpool address, got %q", state.Balances[0].Address)
	}
	if state.Balances[0].Denom != "uion" {
		t.Fatalf("expected uion balance, got %q", state.Balances[0].Denom)
	}
	if state.Balances[0].Amount != "12121212" {
		t.Fatalf("expected credited amount 12121212, got %q", state.Balances[0].Amount)
	}
}

func TestMockTargetSupportsRecipientOverrideAndRoutePath(t *testing.T) {
	t.Parallel()

	statePath := filepath.Join(t.TempDir(), "mock-osmosis-state.json")
	handler := NewMockTargetHandler("manual", statePath, 0)
	target := newHTTPTargetWithClient("http://mock-osmosis", &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, req)
			return recorder.Result(), nil
		}),
	})

	ack, err := target.SubmitTransfer(context.Background(), Transfer{
		TransferID:         "ibc/eth.usdc/7",
		AssetID:            "eth.usdc",
		Amount:             "25000000",
		Receiver:           "osmo1base",
		DestinationChainID: "osmosis-1",
		ChannelID:          "channel-0",
		DestinationDenom:   "ibc/uatom-usdc",
		TimeoutHeight:      140,
		Memo:               "swap:uosmo:recipient=osmo1override:path=pool-7",
	})
	if err != nil {
		t.Fatalf("submit transfer: %v", err)
	}
	if ack.Status != AckStatusReceived {
		t.Fatalf("expected received ack, got %q", ack.Status)
	}
	if _, err := target.ReadyAcks(context.Background()); err != nil {
		t.Fatalf("ready acks: %v", err)
	}

	var state struct {
		Executions []struct {
			Result    string `json:"result"`
			Recipient string `json:"recipient"`
			RoutePath string `json:"route_path"`
		} `json:"executions"`
		Swaps []struct {
			Recipient string `json:"recipient"`
			RoutePath string `json:"route_path"`
		} `json:"swaps"`
		Balances []struct {
			Address string `json:"address"`
		} `json:"balances"`
	}
	readJSONFile(t, statePath, &state)

	if len(state.Executions) != 1 {
		t.Fatalf("expected one execution, got %d", len(state.Executions))
	}
	if state.Executions[0].Result != "swap_success" {
		t.Fatalf("expected swap_success execution, got %q", state.Executions[0].Result)
	}
	if state.Executions[0].Recipient != "osmo1override" {
		t.Fatalf("expected override recipient on execution, got %q", state.Executions[0].Recipient)
	}
	if state.Executions[0].RoutePath != "pool-7" {
		t.Fatalf("expected route path pool-7 on execution, got %q", state.Executions[0].RoutePath)
	}
	if len(state.Swaps) != 1 {
		t.Fatalf("expected one swap record, got %d", len(state.Swaps))
	}
	if state.Swaps[0].Recipient != "osmo1override" {
		t.Fatalf("expected override recipient on swap, got %q", state.Swaps[0].Recipient)
	}
	if state.Swaps[0].RoutePath != "pool-7" {
		t.Fatalf("expected route path pool-7 on swap, got %q", state.Swaps[0].RoutePath)
	}
	if len(state.Balances) != 1 {
		t.Fatalf("expected one balance, got %d", len(state.Balances))
	}
	if state.Balances[0].Address != "osmo1override" {
		t.Fatalf("expected balance credited to override recipient, got %q", state.Balances[0].Address)
	}
}

func TestMockTargetRejectsMalformedRouteActionAsExecutionFailure(t *testing.T) {
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
		TransferID:         "ibc/eth.usdc/8",
		AssetID:            "eth.usdc",
		Amount:             "25000000",
		Receiver:           "osmo1badmemo",
		DestinationChainID: "osmosis-1",
		ChannelID:          "channel-0",
		DestinationDenom:   "ibc/uatom-usdc",
		TimeoutHeight:      140,
		Memo:               "stake:uosmo",
	})
	if err != nil {
		t.Fatalf("submit transfer: %v", err)
	}
	if ack.Status != AckStatusReceived {
		t.Fatalf("expected received ack, got %q", ack.Status)
	}

	acks, err := target.ReadyAcks(context.Background())
	if err != nil {
		t.Fatalf("ready acks: %v", err)
	}
	if len(acks) != 1 {
		t.Fatalf("expected one ready ack, got %d", len(acks))
	}
	if acks[0].Status != AckStatusFailed {
		t.Fatalf("expected failed ack, got %q", acks[0].Status)
	}
	if acks[0].Reason == "" {
		t.Fatal("expected invalid action reason")
	}

	var state struct {
		Packets []struct {
			AckState    string `json:"ack_state"`
			ActionError string `json:"action_error"`
		} `json:"packets"`
		Executions []struct {
			Result string `json:"result"`
			Error  string `json:"error"`
		} `json:"executions"`
		Swaps    []struct{} `json:"swaps"`
		Balances []struct{} `json:"balances"`
	}
	readJSONFile(t, statePath, &state)

	if len(state.Packets) != 1 {
		t.Fatalf("expected one packet, got %d", len(state.Packets))
	}
	if state.Packets[0].AckState != "ack_failed" {
		t.Fatalf("expected ack_failed packet state, got %q", state.Packets[0].AckState)
	}
	if state.Packets[0].ActionError == "" {
		t.Fatal("expected action error to be persisted")
	}
	if len(state.Executions) != 1 {
		t.Fatalf("expected one execution receipt, got %d", len(state.Executions))
	}
	if state.Executions[0].Result != "invalid_action" {
		t.Fatalf("expected invalid_action execution result, got %q", state.Executions[0].Result)
	}
	if state.Executions[0].Error == "" {
		t.Fatal("expected execution error to be recorded")
	}
	if len(state.Swaps) != 0 || len(state.Balances) != 0 {
		t.Fatalf("expected no swaps or balances after invalid action, got swaps=%d balances=%d", len(state.Swaps), len(state.Balances))
	}
}

func TestMockTargetTimeoutModeSkipsDestinationExecution(t *testing.T) {
	t.Parallel()

	statePath := filepath.Join(t.TempDir(), "mock-osmosis-state.json")
	handler := NewMockTargetHandler("timeout", statePath, 0)
	target := newHTTPTargetWithClient("http://mock-osmosis", &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, req)
			return recorder.Result(), nil
		}),
	})

	ack, err := target.SubmitTransfer(context.Background(), Transfer{
		TransferID:         "ibc/eth.usdc/9",
		AssetID:            "eth.usdc",
		Amount:             "25000000",
		Receiver:           "osmo1timeout",
		DestinationChainID: "osmosis-1",
		ChannelID:          "channel-0",
		DestinationDenom:   "ibc/uatom-usdc",
		TimeoutHeight:      140,
		Memo:               "swap:uosmo",
	})
	if err != nil {
		t.Fatalf("submit transfer: %v", err)
	}
	if ack.Status != AckStatusReceived {
		t.Fatalf("expected received ack, got %q", ack.Status)
	}

	acks, err := target.ReadyAcks(context.Background())
	if err != nil {
		t.Fatalf("ready acks: %v", err)
	}
	if len(acks) != 1 {
		t.Fatalf("expected one ready ack, got %d", len(acks))
	}
	if acks[0].Status != AckStatusTimedOut {
		t.Fatalf("expected timed out ack, got %q", acks[0].Status)
	}

	var state struct {
		Packets []struct {
			PacketState string `json:"packet_state"`
			AckState    string `json:"ack_state"`
		} `json:"packets"`
		Executions []struct{} `json:"executions"`
		Swaps      []struct{} `json:"swaps"`
		Balances   []struct{} `json:"balances"`
	}
	readJSONFile(t, statePath, &state)

	if len(state.Packets) != 1 {
		t.Fatalf("expected one packet, got %d", len(state.Packets))
	}
	if state.Packets[0].PacketState != "ack_ready" {
		t.Fatalf("expected ack_ready packet state for timeout, got %q", state.Packets[0].PacketState)
	}
	if state.Packets[0].AckState != "timed_out" {
		t.Fatalf("expected timed_out ack state, got %q", state.Packets[0].AckState)
	}
	if len(state.Executions) != 0 || len(state.Swaps) != 0 || len(state.Balances) != 0 {
		t.Fatalf("expected no destination execution for timeout, got executions=%d swaps=%d balances=%d", len(state.Executions), len(state.Swaps), len(state.Balances))
	}
}

func TestMockTargetCanResolveReadyAckAndMarkItConfirmed(t *testing.T) {
	t.Parallel()

	statePath := filepath.Join(t.TempDir(), "mock-osmosis-state.json")
	handler := NewMockTargetHandler("manual", statePath, 0)
	target := newHTTPTargetWithClient("http://mock-osmosis", &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, req)
			return recorder.Result(), nil
		}),
	})

	if _, err := target.SubmitTransfer(context.Background(), Transfer{
		TransferID:         "ibc/eth.usdc/1",
		AssetID:            "eth.usdc",
		Amount:             "25000000",
		Receiver:           "osmo1recipient",
		DestinationChainID: "osmosis-1",
		ChannelID:          "channel-0",
		DestinationDenom:   "ibc/uatom-usdc",
		TimeoutHeight:      140,
		Memo:               "swap:uosmo",
	}); err != nil {
		t.Fatalf("submit transfer: %v", err)
	}

	resolveRequest := httptest.NewRequest(http.MethodPost, "/acks/complete?transfer_id=ibc%2Feth.usdc%2F1", nil)
	resolveRecorder := httptest.NewRecorder()
	handler.ServeHTTP(resolveRecorder, resolveRequest)
	if resolveRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 from resolve, got %d", resolveRecorder.Code)
	}

	acks, err := target.ReadyAcks(context.Background())
	if err != nil {
		t.Fatalf("ready acks after resolution: %v", err)
	}
	if len(acks) != 1 {
		t.Fatalf("expected 1 ready ack, got %d", len(acks))
	}
	if acks[0].TransferID != "ibc/eth.usdc/1" || acks[0].Status != AckStatusCompleted {
		t.Fatalf("unexpected ack: %+v", acks[0])
	}

	if err := target.ConfirmAck(context.Background(), "ibc/eth.usdc/1"); err != nil {
		t.Fatalf("confirm ack: %v", err)
	}

	var state struct {
		Receipts []struct {
			TransferID string `json:"transfer_id"`
			AckState   string `json:"ack_state"`
			AckRelayed bool   `json:"ack_relayed"`
		} `json:"receipts"`
	}
	readJSONFile(t, statePath, &state)
	if len(state.Receipts) != 1 {
		t.Fatalf("expected one receipt, got %d", len(state.Receipts))
	}
	if !state.Receipts[0].AckRelayed {
		t.Fatal("expected ack to be marked relayed after confirmation")
	}
	if state.Receipts[0].AckState != "completed" {
		t.Fatalf("expected completed ack state, got %q", state.Receipts[0].AckState)
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

func assertEndpointJSONCount(t *testing.T, client *http.Client, endpoint string, expected int) {
	t.Helper()

	resp, err := client.Get(endpoint)
	if err != nil {
		t.Fatalf("get %s: %v", endpoint, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from %s, got %d", endpoint, resp.StatusCode)
	}

	var items []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		t.Fatalf("decode %s: %v", endpoint, err)
	}
	if len(items) != expected {
		t.Fatalf("expected %d items from %s, got %d", expected, endpoint, len(items))
	}
}
