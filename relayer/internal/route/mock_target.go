package route

import (
	"encoding/json"
	"fmt"
	"math/big"
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
	MockTargetModeManual  MockTargetMode = "manual"
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
	Pools    []MockTargetPool    `json:"pools"`
	Balances []MockTargetBalance `json:"balances"`
}

type MockTargetReceipt struct {
	TransferID string       `json:"transfer_id"`
	AckState   string       `json:"ack_state"`
	AckReason  string       `json:"ack_reason,omitempty"`
	AckRelayed bool         `json:"ack_relayed"`
	Packet     Packet       `json:"packet"`
	DenomTrace DenomTrace   `json:"denom_trace"`
	Action     *RouteAction `json:"action,omitempty"`
}

type MockTargetSwap struct {
	TransferID   string `json:"transfer_id"`
	InputDenom   string `json:"input_denom"`
	OutputDenom  string `json:"output_denom"`
	InputAmount  string `json:"input_amount"`
	OutputAmount string `json:"output_amount"`
	Recipient    string `json:"recipient"`
	DexChainID   string `json:"dex_chain_id"`
}

type MockTargetPool struct {
	InputDenom  string `json:"input_denom"`
	OutputDenom string `json:"output_denom"`
	ReserveIn   string `json:"reserve_in"`
	ReserveOut  string `json:"reserve_out"`
}

type MockTargetBalance struct {
	Address string `json:"address"`
	Denom   string `json:"denom"`
	Amount  string `json:"amount"`
}

func NewMockTargetHandler(mode string, statePath string, delay time.Duration) http.Handler {
	target := &MockTarget{
		mode:      normalizeMockTargetMode(mode),
		delay:     delay,
		statePath: strings.TrimSpace(statePath),
		state: MockTargetState{
			Pools: defaultMockTargetPools(),
		},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/transfers", target.handleTransfers)
	mux.HandleFunc("/acks", target.handleAcks)
	mux.HandleFunc("/acks/", target.handleAckControl)
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

		t.mu.Lock()
		if _, exists := t.findReceiptLocked(envelope.Transfer.TransferID); !exists {
			receipt := MockTargetReceipt{
				TransferID: envelope.Transfer.TransferID,
				AckState:   ackStateForMode(t.mode),
				AckReason:  ackReasonForMode(t.mode),
				Packet:     envelope.Packet,
				DenomTrace: envelope.DenomTrace,
				Action:     envelope.Action,
			}
			t.applyExecutionLocked(&receipt, envelope)
			t.state.Receipts = append(t.state.Receipts, receipt)
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

		_ = json.NewEncoder(w).Encode(Ack{Status: AckStatusReceived})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (t *MockTarget) handleAcks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	acks := make([]AckRecord, 0, len(t.state.Receipts))
	for _, receipt := range t.state.Receipts {
		if !ackStateReady(receipt.AckState) || receipt.AckRelayed {
			continue
		}
		acks = append(acks, AckRecord{
			TransferID: receipt.TransferID,
			Status:     AckStatus(receipt.AckState),
			Reason:     receipt.AckReason,
		})
	}
	_ = json.NewEncoder(w).Encode(acks)
}

func (t *MockTarget) handleAckControl(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	action := strings.TrimPrefix(r.URL.Path, "/acks/")
	transferID := strings.TrimSpace(r.URL.Query().Get("transfer_id"))
	if transferID == "" {
		http.Error(w, "missing transfer_id", http.StatusBadRequest)
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	receipt, ok := t.findReceiptLocked(transferID)
	if !ok {
		http.Error(w, "receipt not found", http.StatusNotFound)
		return
	}

	switch {
	case action == "confirm":
		receipt.AckRelayed = true
	case action == "complete":
		receipt.AckState = string(AckStatusCompleted)
		receipt.AckReason = ""
	case action == "fail":
		receipt.AckState = string(AckStatusFailed)
		receipt.AckReason = "mock ack failed"
	case action == "timeout":
		receipt.AckState = string(AckStatusTimedOut)
		receipt.AckReason = "mock timeout"
	default:
		http.NotFound(w, r)
		return
	}

	if err := persistMockTargetState(t.statePath, t.state); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_ = json.NewEncoder(w).Encode(receipt)
}

func normalizeMockTargetMode(mode string) MockTargetMode {
	switch MockTargetMode(strings.TrimSpace(mode)) {
	case MockTargetModeFail:
		return MockTargetModeFail
	case MockTargetModeTimeout:
		return MockTargetModeTimeout
	case MockTargetModeManual:
		return MockTargetModeManual
	default:
		return MockTargetModeSuccess
	}
}

func (t *MockTarget) findReceiptLocked(transferID string) (*MockTargetReceipt, bool) {
	for i := range t.state.Receipts {
		if t.state.Receipts[i].TransferID == strings.TrimSpace(transferID) {
			return &t.state.Receipts[i], true
		}
	}
	return nil, false
}

func (t *MockTarget) applyExecutionLocked(receipt *MockTargetReceipt, envelope DeliveryEnvelope) {
	t.ensurePoolsLocked()

	if envelope.Action != nil && envelope.Action.Type == "swap" {
		outputAmount, err := t.executeSwapLocked(envelope)
		if err != nil {
			receipt.AckState = string(AckStatusFailed)
			receipt.AckReason = err.Error()
			return
		}
		t.state.Swaps = append(t.state.Swaps, MockTargetSwap{
			TransferID:   envelope.Transfer.TransferID,
			InputDenom:   envelope.DenomTrace.IBCDenom,
			OutputDenom:  envelope.Action.TargetDenom,
			InputAmount:  strings.TrimSpace(envelope.Packet.Data.Amount),
			OutputAmount: outputAmount,
			Recipient:    envelope.Packet.Data.Receiver,
			DexChainID:   envelope.Transfer.DestinationChainID,
		})
		if err := t.creditBalanceLocked(envelope.Packet.Data.Receiver, envelope.Action.TargetDenom, outputAmount); err != nil {
			receipt.AckState = string(AckStatusFailed)
			receipt.AckReason = err.Error()
		}
		return
	}

	if err := t.creditBalanceLocked(
		envelope.Packet.Data.Receiver,
		envelope.DenomTrace.IBCDenom,
		strings.TrimSpace(envelope.Packet.Data.Amount),
	); err != nil {
		receipt.AckState = string(AckStatusFailed)
		receipt.AckReason = err.Error()
	}
}

func (t *MockTarget) ensurePoolsLocked() {
	if len(t.state.Pools) == 0 {
		t.state.Pools = defaultMockTargetPools()
	}
}

func defaultMockTargetPools() []MockTargetPool {
	return []MockTargetPool{
		{
			InputDenom:  "ibc/uatom-usdc",
			OutputDenom: "uosmo",
			ReserveIn:   "500000000",
			ReserveOut:  "1000000000",
		},
	}
}

func (t *MockTarget) executeSwapLocked(envelope DeliveryEnvelope) (string, error) {
	poolIndex := -1
	for i := range t.state.Pools {
		pool := t.state.Pools[i]
		if pool.InputDenom == strings.TrimSpace(envelope.DenomTrace.IBCDenom) && pool.OutputDenom == strings.TrimSpace(envelope.Action.TargetDenom) {
			poolIndex = i
			break
		}
	}
	if poolIndex < 0 {
		return "", fmt.Errorf("no pool for %s -> %s", envelope.DenomTrace.IBCDenom, envelope.Action.TargetDenom)
	}

	pool := t.state.Pools[poolIndex]
	inputAmount, err := parsePositiveDecimal(strings.TrimSpace(envelope.Packet.Data.Amount), "swap amount")
	if err != nil {
		return "", err
	}
	reserveIn, err := parsePositiveDecimal(pool.ReserveIn, "pool reserve in")
	if err != nil {
		return "", err
	}
	reserveOut, err := parsePositiveDecimal(pool.ReserveOut, "pool reserve out")
	if err != nil {
		return "", err
	}

	numerator := new(big.Int).Mul(new(big.Int).Set(reserveOut), new(big.Int).Set(inputAmount))
	denominator := new(big.Int).Add(new(big.Int).Set(reserveIn), new(big.Int).Set(inputAmount))
	if denominator.Sign() <= 0 {
		return "", fmt.Errorf("invalid pool denominator")
	}
	outputAmount := new(big.Int).Div(numerator, denominator)
	if outputAmount.Sign() <= 0 || outputAmount.Cmp(reserveOut) >= 0 {
		return "", fmt.Errorf("insufficient liquidity for %s -> %s", pool.InputDenom, pool.OutputDenom)
	}
	if minOut := strings.TrimSpace(envelope.Action.MinOut); minOut != "" {
		minOutAmount, err := parsePositiveDecimal(minOut, "min_out")
		if err != nil {
			return "", err
		}
		if outputAmount.Cmp(minOutAmount) < 0 {
			return "", fmt.Errorf("min_out not met: expected at least %s, got %s", minOutAmount.String(), outputAmount.String())
		}
	}

	t.state.Pools[poolIndex].ReserveIn = new(big.Int).Add(reserveIn, inputAmount).String()
	t.state.Pools[poolIndex].ReserveOut = new(big.Int).Sub(reserveOut, outputAmount).String()
	return outputAmount.String(), nil
}

func (t *MockTarget) creditBalanceLocked(address, denom, amount string) error {
	amount = strings.TrimSpace(amount)
	if amount == "" {
		return nil
	}
	for i := range t.state.Balances {
		if t.state.Balances[i].Address == strings.TrimSpace(address) && t.state.Balances[i].Denom == strings.TrimSpace(denom) {
			next, err := addDecimalStrings(t.state.Balances[i].Amount, amount)
			if err != nil {
				return err
			}
			t.state.Balances[i].Amount = next
			return nil
		}
	}
	t.state.Balances = append(t.state.Balances, MockTargetBalance{
		Address: strings.TrimSpace(address),
		Denom:   strings.TrimSpace(denom),
		Amount:  amount,
	})
	return nil
}

func addDecimalStrings(current, delta string) (string, error) {
	currentInt, ok := new(big.Int).SetString(strings.TrimSpace(current), 10)
	if !ok {
		return "", fmt.Errorf("invalid balance amount %q", current)
	}
	deltaInt, ok := new(big.Int).SetString(strings.TrimSpace(delta), 10)
	if !ok {
		return "", fmt.Errorf("invalid credit amount %q", delta)
	}
	return new(big.Int).Add(currentInt, deltaInt).String(), nil
}

func parsePositiveDecimal(value, label string) (*big.Int, error) {
	parsed, ok := new(big.Int).SetString(strings.TrimSpace(value), 10)
	if !ok {
		return nil, fmt.Errorf("invalid %s %q", label, value)
	}
	if parsed.Sign() <= 0 {
		return nil, fmt.Errorf("invalid %s %q", label, value)
	}
	return parsed, nil
}

func ackStateForMode(mode MockTargetMode) string {
	switch mode {
	case MockTargetModeSuccess:
		return string(AckStatusCompleted)
	case MockTargetModeFail:
		return string(AckStatusFailed)
	case MockTargetModeTimeout:
		return string(AckStatusTimedOut)
	default:
		return "pending"
	}
}

func ackReasonForMode(mode MockTargetMode) string {
	switch mode {
	case MockTargetModeFail:
		return "mock ack failed"
	case MockTargetModeTimeout:
		return "mock timeout"
	default:
		return ""
	}
}

func ackStateReady(state string) bool {
	switch strings.TrimSpace(state) {
	case string(AckStatusCompleted), string(AckStatusFailed), string(AckStatusTimedOut):
		return true
	default:
		return false
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
