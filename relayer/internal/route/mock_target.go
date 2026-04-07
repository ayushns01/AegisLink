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

	relayermetrics "github.com/ayushns01/aegislink/relayer/internal/metrics"
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
	Packets    []MockTargetPacket    `json:"packets"`
	Receipts   []MockTargetPacket    `json:"receipts,omitempty"`
	Executions []MockTargetExecution `json:"executions"`
	Swaps      []MockTargetSwap      `json:"swaps"`
	Stakes     []MockTargetStake     `json:"stakes"`
	Pools      []MockTargetPool      `json:"pools"`
	Balances   []MockTargetBalance   `json:"balances"`
}

type MockTargetPacket struct {
	TransferID         string       `json:"transfer_id"`
	DestinationChainID string       `json:"destination_chain_id"`
	PacketState        string       `json:"packet_state"`
	AckState           string       `json:"ack_state"`
	AckReason          string       `json:"ack_reason,omitempty"`
	AckPayload         *Ack         `json:"ack_payload,omitempty"`
	AckRelayed         bool         `json:"ack_relayed"`
	Packet             Packet       `json:"packet"`
	DenomTrace         DenomTrace   `json:"denom_trace"`
	Action             *RouteAction `json:"action,omitempty"`
	ActionError        string       `json:"action_error,omitempty"`
}

type MockTargetExecution struct {
	TransferID     string `json:"transfer_id"`
	PacketSequence uint64 `json:"packet_sequence"`
	Result         string `json:"result"`
	Recipient      string `json:"recipient"`
	InputDenom     string `json:"input_denom"`
	InputAmount    string `json:"input_amount"`
	OutputDenom    string `json:"output_denom,omitempty"`
	OutputAmount   string `json:"output_amount,omitempty"`
	DexChainID     string `json:"dex_chain_id,omitempty"`
	RoutePath      string `json:"route_path,omitempty"`
	Error          string `json:"error,omitempty"`
}

type MockTargetSwap struct {
	TransferID   string `json:"transfer_id"`
	InputDenom   string `json:"input_denom"`
	OutputDenom  string `json:"output_denom"`
	InputAmount  string `json:"input_amount"`
	OutputAmount string `json:"output_amount"`
	Recipient    string `json:"recipient"`
	DexChainID   string `json:"dex_chain_id"`
	RoutePath    string `json:"route_path,omitempty"`
}

type MockTargetStake struct {
	TransferID string `json:"transfer_id"`
	Denom      string `json:"denom"`
	Amount     string `json:"amount"`
	Staker     string `json:"staker"`
	Validator  string `json:"validator"`
}

type MockTargetPool struct {
	InputDenom  string `json:"input_denom"`
	OutputDenom string `json:"output_denom"`
	ReserveIn   string `json:"reserve_in"`
	ReserveOut  string `json:"reserve_out"`
	FeeBPS      uint32 `json:"fee_bps,omitempty"`
}

type MockTargetBalance struct {
	Address string `json:"address"`
	Denom   string `json:"denom"`
	Amount  string `json:"amount"`
}

type MockTargetStatus struct {
	Packets         int `json:"packets"`
	Receipts        int `json:"receipts"`
	Executions      int `json:"executions"`
	Pools           int `json:"pools"`
	Balances        int `json:"balances"`
	Swaps           int `json:"swaps"`
	Stakes          int `json:"stakes"`
	SwapFailures    int `json:"swap_failures"`
	ReceivedPackets int `json:"received_packets"`
	ExecutedPackets int `json:"executed_packets"`
	ReadyAcks       int `json:"ready_acks"`
	CompletedAcks   int `json:"completed_acks"`
	FailedAcks      int `json:"failed_acks"`
	TimedOutAcks    int `json:"timed_out_acks"`
	RelayedAcks     int `json:"relayed_acks"`
	PendingReceipts int `json:"pending_receipts"`
}

type MockTargetConfig struct {
	Mode      string
	Delay     time.Duration
	StatePath string
	Pools     []MockTargetPool
}

func NewMockTargetHandler(mode string, statePath string, delay time.Duration) http.Handler {
	return NewMockTargetHandlerWithConfig(MockTargetConfig{
		Mode:      mode,
		Delay:     delay,
		StatePath: statePath,
	})
}

func NewMockTargetHandlerWithConfig(cfg MockTargetConfig) http.Handler {
	target := &MockTarget{
		mode:      normalizeMockTargetMode(cfg.Mode),
		delay:     cfg.Delay,
		statePath: strings.TrimSpace(cfg.StatePath),
		state: MockTargetState{
			Pools: cloneMockTargetPools(cfg.Pools),
		},
	}
	target.ensurePoolsLocked()

	mux := http.NewServeMux()
	mux.HandleFunc("/transfers", target.handleTransfers)
	mux.HandleFunc("/acks", target.handleAcks)
	mux.HandleFunc("/acks/", target.handleAckControl)
	mux.HandleFunc("/packets", target.handlePackets)
	mux.HandleFunc("/executions", target.handleExecutions)
	mux.HandleFunc("/pools", target.handlePools)
	mux.HandleFunc("/balances", target.handleBalances)
	mux.HandleFunc("/swaps", target.handleSwaps)
	mux.HandleFunc("/stakes", target.handleStakes)
	mux.HandleFunc("/status", target.handleStatus)
	mux.HandleFunc("/metrics", target.handleMetrics)
	return mux
}

func (t *MockTarget) handleTransfers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		t.mu.Lock()
		t.syncLegacyReceiptsLocked()
		defer t.mu.Unlock()
		_ = json.NewEncoder(w).Encode(t.state)
	case http.MethodPost:
		var envelope DeliveryEnvelope
		if err := json.NewDecoder(r.Body).Decode(&envelope); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		t.mu.Lock()
		if _, exists := t.findPacketLocked(envelope.Transfer.TransferID); !exists {
			t.state.Packets = append(t.state.Packets, MockTargetPacket{
				TransferID:         envelope.Transfer.TransferID,
				DestinationChainID: envelope.Transfer.DestinationChainID,
				PacketState:        "received",
				AckState:           "pending",
				Packet:             envelope.Packet,
				DenomTrace:         envelope.DenomTrace,
				Action:             envelope.Action,
				ActionError:        strings.TrimSpace(envelope.ActionError),
			})
		}
		t.syncLegacyReceiptsLocked()
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
	if changed := t.advanceReceivedPacketsLocked(); changed {
		t.syncLegacyReceiptsLocked()
		if err := persistMockTargetState(t.statePath, t.state); err != nil {
			t.mu.Unlock()
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	acks := make([]AckRecord, 0, len(t.state.Packets))
	for _, packet := range t.state.Packets {
		if packet.PacketState != "ack_ready" || packet.AckPayload == nil || packet.AckRelayed {
			continue
		}
		acks = append(acks, AckRecord{
			TransferID: packet.TransferID,
			Status:     packet.AckPayload.Status,
			Reason:     packet.AckPayload.Reason,
		})
	}
	t.mu.Unlock()
	_ = json.NewEncoder(w).Encode(acks)
}

func (t *MockTarget) handlePackets(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	_ = json.NewEncoder(w).Encode(t.state.Packets)
}

func (t *MockTarget) handleExecutions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	_ = json.NewEncoder(w).Encode(t.state.Executions)
}

func (t *MockTarget) handlePools(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	_ = json.NewEncoder(w).Encode(t.state.Pools)
}

func (t *MockTarget) handleBalances(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	_ = json.NewEncoder(w).Encode(t.state.Balances)
}

func (t *MockTarget) handleSwaps(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	_ = json.NewEncoder(w).Encode(t.state.Swaps)
}

func (t *MockTarget) handleStakes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	_ = json.NewEncoder(w).Encode(t.state.Stakes)
}

func (t *MockTarget) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	_ = json.NewEncoder(w).Encode(t.statusLocked())
}

func (t *MockTarget) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	t.mu.Lock()
	status := t.statusLocked()
	t.mu.Unlock()

	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	_, _ = w.Write([]byte(relayermetrics.FormatTargetSnapshot(relayermetrics.TargetSnapshot{
		Packets:      status.Packets,
		Executions:   status.Executions,
		SwapFailures: status.SwapFailures,
		ReadyAcks:    status.ReadyAcks,
	})))
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

	packet, ok := t.findPacketLocked(transferID)
	if !ok {
		http.Error(w, "packet not found", http.StatusNotFound)
		return
	}

	switch {
	case action == "confirm":
		if packet.PacketState != "ack_ready" {
			http.Error(w, "ack not ready", http.StatusConflict)
			return
		}
		packet.AckRelayed = true
		packet.PacketState = "ack_relayed"
	case action == "complete":
		t.ensurePacketExecutedLocked(packet)
		t.markPacketAckReadyLocked(packet, Ack{Status: AckStatusCompleted})
	case action == "fail":
		t.ensurePacketExecutedLocked(packet)
		t.markPacketAckReadyLocked(packet, Ack{Status: AckStatusFailed, Reason: "mock ack failed"})
	case action == "timeout":
		t.markPacketAckReadyLocked(packet, Ack{Status: AckStatusTimedOut, Reason: "mock timeout"})
	default:
		http.NotFound(w, r)
		return
	}

	t.syncLegacyReceiptsLocked()
	if err := persistMockTargetState(t.statePath, t.state); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_ = json.NewEncoder(w).Encode(packet)
}

func (t *MockTarget) statusLocked() MockTargetStatus {
	status := MockTargetStatus{
		Packets:    len(t.state.Packets),
		Receipts:   len(t.state.Packets),
		Executions: len(t.state.Executions),
		Pools:      len(t.state.Pools),
		Balances:   len(t.state.Balances),
		Swaps:      len(t.state.Swaps),
		Stakes:     len(t.state.Stakes),
	}

	for _, packet := range t.state.Packets {
		switch strings.TrimSpace(packet.PacketState) {
		case "received":
			status.ReceivedPackets++
			status.PendingReceipts++
		case "executed":
			status.ExecutedPackets++
			status.PendingReceipts++
		case "ack_relayed":
			status.RelayedAcks++
		}
		switch strings.TrimSpace(packet.AckState) {
		case string(AckStatusCompleted):
			status.CompletedAcks++
			if packet.PacketState == "ack_ready" && !packet.AckRelayed {
				status.ReadyAcks++
			}
		case string(AckStatusFailed):
			status.FailedAcks++
			if packet.PacketState == "ack_ready" && !packet.AckRelayed {
				status.ReadyAcks++
			}
		case string(AckStatusTimedOut):
			status.TimedOutAcks++
			if packet.PacketState == "ack_ready" && !packet.AckRelayed {
				status.ReadyAcks++
			}
		}
	}
	for _, execution := range t.state.Executions {
		if execution.Result == "swap_failed" {
			status.SwapFailures++
		}
	}

	return status
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

func (t *MockTarget) findPacketLocked(transferID string) (*MockTargetPacket, bool) {
	for i := range t.state.Packets {
		if t.state.Packets[i].TransferID == strings.TrimSpace(transferID) {
			return &t.state.Packets[i], true
		}
	}
	return nil, false
}

func (t *MockTarget) syncLegacyReceiptsLocked() {
	t.state.Receipts = append([]MockTargetPacket(nil), t.state.Packets...)
}

func (t *MockTarget) advanceReceivedPacketsLocked() bool {
	changed := false
	for i := range t.state.Packets {
		if t.state.Packets[i].PacketState != "received" {
			continue
		}
		if t.mode == MockTargetModeTimeout {
			t.markPacketAckReadyLocked(&t.state.Packets[i], Ack{Status: AckStatusTimedOut, Reason: "mock timeout"})
			changed = true
			continue
		}
		t.executePacketLocked(&t.state.Packets[i])
		changed = true
	}
	return changed
}

func (t *MockTarget) ensurePacketExecutedLocked(packet *MockTargetPacket) {
	if packet.PacketState == "received" {
		t.executePacketLocked(packet)
	}
}

func (t *MockTarget) executePacketLocked(packet *MockTargetPacket) {
	t.ensurePoolsLocked()

	recipient := packetExecutionRecipient(packet)
	routePath := packetRoutePath(packet)

	if strings.TrimSpace(packet.ActionError) != "" {
		t.state.Executions = append(t.state.Executions, MockTargetExecution{
			TransferID:     packet.TransferID,
			PacketSequence: packet.Packet.Sequence,
			Result:         "invalid_action",
			Recipient:      recipient,
			InputDenom:     packet.DenomTrace.IBCDenom,
			InputAmount:    strings.TrimSpace(packet.Packet.Data.Amount),
			DexChainID:     packet.DestinationChainID,
			RoutePath:      routePath,
			Error:          strings.TrimSpace(packet.ActionError),
		})
		t.markPacketAckReadyLocked(packet, Ack{Status: AckStatusFailed, Reason: strings.TrimSpace(packet.ActionError)})
		return
	}

	if packet.Action != nil && packet.Action.Type == "swap" {
		execution := MockTargetExecution{
			TransferID:     packet.TransferID,
			PacketSequence: packet.Packet.Sequence,
			Result:         "swap_success",
			Recipient:      recipient,
			InputDenom:     packet.DenomTrace.IBCDenom,
			InputAmount:    strings.TrimSpace(packet.Packet.Data.Amount),
			OutputDenom:    packet.Action.TargetDenom,
			DexChainID:     packet.DestinationChainID,
			RoutePath:      routePath,
		}
		outputAmount, err := t.executeSwapLocked(packet)
		if err != nil {
			execution.Result = "swap_failed"
			execution.Error = err.Error()
			t.state.Executions = append(t.state.Executions, execution)
			t.markPacketAckReadyLocked(packet, Ack{Status: AckStatusFailed, Reason: err.Error()})
			return
		}
		execution.OutputAmount = outputAmount
		t.state.Swaps = append(t.state.Swaps, MockTargetSwap{
			TransferID:   packet.TransferID,
			InputDenom:   packet.DenomTrace.IBCDenom,
			OutputDenom:  packet.Action.TargetDenom,
			InputAmount:  strings.TrimSpace(packet.Packet.Data.Amount),
			OutputAmount: outputAmount,
			Recipient:    recipient,
			DexChainID:   packet.DestinationChainID,
			RoutePath:    routePath,
		})
		if err := t.creditBalanceLocked(recipient, packet.Action.TargetDenom, outputAmount); err != nil {
			execution.Result = "swap_failed"
			execution.Error = err.Error()
			t.state.Executions = append(t.state.Executions, execution)
			t.markPacketAckReadyLocked(packet, Ack{Status: AckStatusFailed, Reason: err.Error()})
			return
		}
		t.state.Executions = append(t.state.Executions, execution)
		t.advancePacketAfterExecutionLocked(packet)
		return
	}

	if packet.Action != nil && packet.Action.Type == "stake" {
		execution := MockTargetExecution{
			TransferID:     packet.TransferID,
			PacketSequence: packet.Packet.Sequence,
			Result:         "stake_success",
			Recipient:      recipient,
			InputDenom:     packet.DenomTrace.IBCDenom,
			InputAmount:    strings.TrimSpace(packet.Packet.Data.Amount),
			OutputDenom:    packet.Action.TargetDenom,
			OutputAmount:   strings.TrimSpace(packet.Packet.Data.Amount),
			DexChainID:     packet.DestinationChainID,
			RoutePath:      routePath,
		}
		if strings.TrimSpace(packet.Action.TargetDenom) != strings.TrimSpace(packet.DenomTrace.IBCDenom) {
			execution.Result = "stake_failed"
			execution.Error = fmt.Sprintf("stake action requires received denom %s", packet.DenomTrace.IBCDenom)
			t.state.Executions = append(t.state.Executions, execution)
			t.markPacketAckReadyLocked(packet, Ack{Status: AckStatusFailed, Reason: execution.Error})
			return
		}
		if strings.TrimSpace(routePath) == "" {
			execution.Result = "stake_failed"
			execution.Error = "missing validator path for stake action"
			t.state.Executions = append(t.state.Executions, execution)
			t.markPacketAckReadyLocked(packet, Ack{Status: AckStatusFailed, Reason: execution.Error})
			return
		}
		t.state.Stakes = append(t.state.Stakes, MockTargetStake{
			TransferID: packet.TransferID,
			Denom:      packet.Action.TargetDenom,
			Amount:     strings.TrimSpace(packet.Packet.Data.Amount),
			Staker:     recipient,
			Validator:  routePath,
		})
		t.state.Executions = append(t.state.Executions, execution)
		t.advancePacketAfterExecutionLocked(packet)
		return
	}

	execution := MockTargetExecution{
		TransferID:     packet.TransferID,
		PacketSequence: packet.Packet.Sequence,
		Result:         "credit",
		Recipient:      recipient,
		InputDenom:     packet.DenomTrace.IBCDenom,
		InputAmount:    strings.TrimSpace(packet.Packet.Data.Amount),
		OutputDenom:    packet.DenomTrace.IBCDenom,
		OutputAmount:   strings.TrimSpace(packet.Packet.Data.Amount),
		DexChainID:     packet.DestinationChainID,
		RoutePath:      routePath,
	}
	if err := t.creditBalanceLocked(
		recipient,
		packet.DenomTrace.IBCDenom,
		strings.TrimSpace(packet.Packet.Data.Amount),
	); err != nil {
		execution.Result = "credit_failed"
		execution.Error = err.Error()
		t.state.Executions = append(t.state.Executions, execution)
		t.markPacketAckReadyLocked(packet, Ack{Status: AckStatusFailed, Reason: err.Error()})
		return
	}
	t.state.Executions = append(t.state.Executions, execution)
	t.advancePacketAfterExecutionLocked(packet)
}

func (t *MockTarget) advancePacketAfterExecutionLocked(packet *MockTargetPacket) {
	switch t.mode {
	case MockTargetModeSuccess:
		t.markPacketAckReadyLocked(packet, Ack{Status: AckStatusCompleted})
	case MockTargetModeFail:
		t.markPacketAckReadyLocked(packet, Ack{Status: AckStatusFailed, Reason: "mock ack failed"})
	case MockTargetModeTimeout:
		t.markPacketAckReadyLocked(packet, Ack{Status: AckStatusTimedOut, Reason: "mock timeout"})
	default:
		packet.PacketState = "executed"
		packet.AckState = "pending"
		packet.AckReason = ""
		packet.AckPayload = nil
		packet.AckRelayed = false
	}
}

func packetExecutionRecipient(packet *MockTargetPacket) string {
	if packet.Action != nil {
		if recipient := strings.TrimSpace(packet.Action.Recipient); recipient != "" {
			return recipient
		}
	}
	return strings.TrimSpace(packet.Packet.Data.Receiver)
}

func packetRoutePath(packet *MockTargetPacket) string {
	if packet.Action == nil {
		return ""
	}
	return strings.TrimSpace(packet.Action.Path)
}

func (t *MockTarget) markPacketAckReadyLocked(packet *MockTargetPacket, ack Ack) {
	packet.PacketState = "ack_ready"
	packet.AckState = string(ack.Status)
	packet.AckReason = strings.TrimSpace(ack.Reason)
	packet.AckRelayed = false
	ackCopy := ack
	packet.AckPayload = &ackCopy
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
		{
			InputDenom:  "ibc/uethusdc",
			OutputDenom: "uosmo",
			ReserveIn:   "500000000",
			ReserveOut:  "1000000000",
		},
	}
}

func cloneMockTargetPools(pools []MockTargetPool) []MockTargetPool {
	if len(pools) == 0 {
		return nil
	}
	cloned := make([]MockTargetPool, len(pools))
	copy(cloned, pools)
	return cloned
}

func (t *MockTarget) executeSwapLocked(packet *MockTargetPacket) (string, error) {
	poolIndex := -1
	for i := range t.state.Pools {
		pool := t.state.Pools[i]
		if pool.InputDenom == strings.TrimSpace(packet.DenomTrace.IBCDenom) && pool.OutputDenom == strings.TrimSpace(packet.Action.TargetDenom) {
			poolIndex = i
			break
		}
	}
	if poolIndex < 0 {
		return "", fmt.Errorf("no pool for %s -> %s", packet.DenomTrace.IBCDenom, packet.Action.TargetDenom)
	}

	pool := t.state.Pools[poolIndex]
	inputAmount, err := parsePositiveDecimal(strings.TrimSpace(packet.Packet.Data.Amount), "swap amount")
	if err != nil {
		return "", err
	}
	effectiveInput := new(big.Int).Set(inputAmount)
	if pool.FeeBPS > 0 {
		feeMultiplier := int64(10_000 - pool.FeeBPS)
		effectiveInput = new(big.Int).Mul(effectiveInput, big.NewInt(feeMultiplier))
		effectiveInput = effectiveInput.Div(effectiveInput, big.NewInt(10_000))
		if effectiveInput.Sign() <= 0 {
			return "", fmt.Errorf("effective input is zero after fee")
		}
	}
	reserveIn, err := parsePositiveDecimal(pool.ReserveIn, "pool reserve in")
	if err != nil {
		return "", err
	}
	reserveOut, err := parsePositiveDecimal(pool.ReserveOut, "pool reserve out")
	if err != nil {
		return "", err
	}

	numerator := new(big.Int).Mul(new(big.Int).Set(reserveOut), new(big.Int).Set(effectiveInput))
	denominator := new(big.Int).Add(new(big.Int).Set(reserveIn), new(big.Int).Set(effectiveInput))
	if denominator.Sign() <= 0 {
		return "", fmt.Errorf("invalid pool denominator")
	}
	outputAmount := new(big.Int).Div(numerator, denominator)
	if outputAmount.Sign() <= 0 || outputAmount.Cmp(reserveOut) >= 0 {
		return "", fmt.Errorf("insufficient liquidity for %s -> %s", pool.InputDenom, pool.OutputDenom)
	}
	if minOut := strings.TrimSpace(packet.Action.MinOut); minOut != "" {
		minOutAmount, err := parsePositiveDecimal(minOut, "min_out")
		if err != nil {
			return "", err
		}
		if outputAmount.Cmp(minOutAmount) < 0 {
			return "", fmt.Errorf("min_out not met: expected at least %s, got %s", minOutAmount.String(), outputAmount.String())
		}
	}

	t.state.Pools[poolIndex].ReserveIn = new(big.Int).Add(reserveIn, effectiveInput).String()
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
