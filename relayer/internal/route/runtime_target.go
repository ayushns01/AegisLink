package route

import (
	"encoding/json"
	"errors"
	"os"
	"strings"
	"time"
)

func LoadMockTargetRuntime(cfg MockTargetConfig) (*MockTarget, error) {
	target := &MockTarget{
		mode:      normalizeMockTargetMode(cfg.Mode),
		delay:     cfg.Delay,
		statePath: strings.TrimSpace(cfg.StatePath),
		state: MockTargetState{
			Pools: cloneMockTargetPools(cfg.Pools),
		},
	}

	if strings.TrimSpace(cfg.StatePath) != "" {
		state, err := loadMockTargetState(cfg.StatePath)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		if err == nil {
			target.state = state
			if len(cfg.Pools) > 0 {
				target.state.Pools = cloneMockTargetPools(cfg.Pools)
			}
		}
	}

	target.ensurePoolsLocked()
	target.ensureBalancesLocked()
	target.syncLegacyReceiptsLocked()
	if err := persistMockTargetState(target.statePath, target.state); err != nil {
		return nil, err
	}
	return target, nil
}

func (t *MockTarget) ReceiveTransfer(transfer Transfer) (Ack, error) {
	envelope, err := buildDeliveryEnvelope(transfer)
	if err != nil {
		return Ack{}, err
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
	err = persistMockTargetState(t.statePath, t.state)
	t.mu.Unlock()
	if err != nil {
		return Ack{}, err
	}

	if t.delay > 0 {
		time.Sleep(t.delay)
	}
	return Ack{Status: AckStatusReceived}, nil
}

func (t *MockTarget) ReadyAckRecords() ([]AckRecord, error) {
	t.mu.Lock()
	if changed := t.advanceReceivedPacketsLocked(); changed {
		t.syncLegacyReceiptsLocked()
		if err := persistMockTargetState(t.statePath, t.state); err != nil {
			t.mu.Unlock()
			return nil, err
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
	return acks, nil
}

func (t *MockTarget) ConfirmReadyAck(transferID string) error {
	return t.setAckState(transferID, "confirm", "")
}

func (t *MockTarget) CompleteAck(transferID string) error {
	return t.setAckState(transferID, "complete", "")
}

func (t *MockTarget) FailAck(transferID, reason string) error {
	if strings.TrimSpace(reason) == "" {
		reason = "mock ack failed"
	}
	return t.setAckState(transferID, "fail", reason)
}

func (t *MockTarget) TimeoutAck(transferID, reason string) error {
	if strings.TrimSpace(reason) == "" {
		reason = "mock timeout"
	}
	return t.setAckState(transferID, "timeout", reason)
}

func (t *MockTarget) setAckState(transferID, action, reason string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	packet, ok := t.findPacketLocked(transferID)
	if !ok {
		return os.ErrNotExist
	}

	switch action {
	case "confirm":
		if packet.PacketState != "ack_ready" {
			return errors.New("ack not ready")
		}
		packet.AckRelayed = true
		packet.PacketState = "ack_relayed"
	case "complete":
		t.ensurePacketExecutedLocked(packet)
		t.markPacketAckReadyLocked(packet, Ack{Status: AckStatusCompleted})
	case "fail":
		t.ensurePacketExecutedLocked(packet)
		t.markPacketAckReadyLocked(packet, Ack{Status: AckStatusFailed, Reason: strings.TrimSpace(reason)})
	case "timeout":
		t.markPacketAckReadyLocked(packet, Ack{Status: AckStatusTimedOut, Reason: strings.TrimSpace(reason)})
	default:
		return errors.New("unknown ack action")
	}

	t.syncLegacyReceiptsLocked()
	return persistMockTargetState(t.statePath, t.state)
}

func (t *MockTarget) PacketsSnapshot() []MockTargetPacket {
	t.mu.Lock()
	defer t.mu.Unlock()
	cloned := make([]MockTargetPacket, len(t.state.Packets))
	copy(cloned, t.state.Packets)
	return cloned
}

func (t *MockTarget) ExecutionsSnapshot() []MockTargetExecution {
	t.mu.Lock()
	defer t.mu.Unlock()
	cloned := make([]MockTargetExecution, len(t.state.Executions))
	copy(cloned, t.state.Executions)
	return cloned
}

func (t *MockTarget) PoolsSnapshot() []MockTargetPool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return cloneMockTargetPools(t.state.Pools)
}

func (t *MockTarget) BalancesSnapshot() []MockTargetBalance {
	t.mu.Lock()
	defer t.mu.Unlock()
	cloned := make([]MockTargetBalance, len(t.state.Balances))
	copy(cloned, t.state.Balances)
	return cloned
}

func (t *MockTarget) SwapsSnapshot() []MockTargetSwap {
	t.mu.Lock()
	defer t.mu.Unlock()
	cloned := make([]MockTargetSwap, len(t.state.Swaps))
	copy(cloned, t.state.Swaps)
	return cloned
}

func (t *MockTarget) StatusSnapshot() MockTargetStatus {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.statusLocked()
}

func (t *MockTarget) ensureBalancesLocked() {
	if len(t.state.Balances) == 0 {
		t.state.Balances = defaultMockTargetBalances()
	}
}

func defaultMockTargetBalances() []MockTargetBalance {
	return []MockTargetBalance{
		{
			Address: "osmo1faucet",
			Denom:   "uosmo",
			Amount:  "1000000000",
		},
	}
}

func loadMockTargetState(path string) (MockTargetState, error) {
	data, err := os.ReadFile(strings.TrimSpace(path))
	if err != nil {
		return MockTargetState{}, err
	}
	var state MockTargetState
	if err := json.Unmarshal(data, &state); err != nil {
		return MockTargetState{}, err
	}
	return state, nil
}
