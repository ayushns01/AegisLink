package route

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type MockTargetMode string

const (
	MockTargetModeSuccess MockTargetMode = "success"
	MockTargetModeFail    MockTargetMode = "fail"
	MockTargetModeTimeout MockTargetMode = "timeout"
)

type MockTarget struct {
	mode      MockTargetMode
	delay     time.Duration
	statePath string

	mu    sync.Mutex
	state MockTargetState
}

type MockTargetState struct {
	Receipts []MockTargetReceipt `json:"receipts"`
	Swaps    []MockTargetSwap    `json:"swaps"`
}

type MockTargetReceipt struct {
	TransferID string       `json:"transfer_id"`
	Packet     Packet       `json:"packet"`
	DenomTrace DenomTrace   `json:"denom_trace"`
	Action     *RouteAction `json:"action,omitempty"`
}

type MockTargetSwap struct {
	TransferID  string `json:"transfer_id"`
	InputDenom  string `json:"input_denom"`
	OutputDenom string `json:"output_denom"`
	InputAmount string `json:"input_amount"`
	Recipient   string `json:"recipient"`
	DexChainID  string `json:"dex_chain_id"`
}

func NewMockTargetHandler(mode string, statePath string, delay time.Duration) http.Handler {
	target := &MockTarget{
		mode:      normalizeMockTargetMode(mode),
		delay:     delay,
		statePath: strings.TrimSpace(statePath),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/transfers", target.handleTransfers)
	return mux
}

func (t *MockTarget) handleTransfers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		t.mu.Lock()
		defer t.mu.Unlock()
		_ = json.NewEncoder(w).Encode(t.state)
	case http.MethodPost:
		var envelope DeliveryEnvelope
		if err := json.NewDecoder(r.Body).Decode(&envelope); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		receipt := MockTargetReceipt{
			TransferID: envelope.Transfer.TransferID,
			Packet:     envelope.Packet,
			DenomTrace: envelope.DenomTrace,
			Action:     envelope.Action,
		}

		t.mu.Lock()
		t.state.Receipts = append(t.state.Receipts, receipt)
		if envelope.Action != nil && envelope.Action.Type == "swap" {
			t.state.Swaps = append(t.state.Swaps, MockTargetSwap{
				TransferID:  envelope.Transfer.TransferID,
				InputDenom:  envelope.DenomTrace.IBCDenom,
				OutputDenom: envelope.Action.TargetDenom,
				InputAmount: envelope.Packet.Data.Amount,
				Recipient:   envelope.Packet.Data.Receiver,
				DexChainID:  envelope.Transfer.DestinationChainID,
			})
		}
		if err := persistMockTargetState(t.statePath, t.state); err != nil {
			t.mu.Unlock()
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		t.mu.Unlock()

		if t.delay > 0 {
			select {
			case <-r.Context().Done():
				return
			case <-time.After(t.delay):
			}
		}

		switch t.mode {
		case MockTargetModeFail:
			_ = json.NewEncoder(w).Encode(Ack{
				Status: AckStatusFailed,
				Reason: "mock ack failed",
			})
		case MockTargetModeTimeout:
			select {
			case <-r.Context().Done():
			case <-time.After(5 * time.Second):
			}
		default:
			_ = json.NewEncoder(w).Encode(Ack{Status: AckStatusCompleted})
		}
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func normalizeMockTargetMode(mode string) MockTargetMode {
	switch MockTargetMode(strings.TrimSpace(mode)) {
	case MockTargetModeFail:
		return MockTargetModeFail
	case MockTargetModeTimeout:
		return MockTargetModeTimeout
	default:
		return MockTargetModeSuccess
	}
}

func persistMockTargetState(path string, state MockTargetState) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
