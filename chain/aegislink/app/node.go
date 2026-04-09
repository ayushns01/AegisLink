package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	bridgetypes "github.com/ayushns01/aegislink/chain/aegislink/x/bridge/types"
)

type QueuedDepositClaim struct {
	Claim            bridgetypes.DepositClaim `json:"claim"`
	Attestation      bridgetypes.Attestation  `json:"attestation"`
	EnqueuedAtHeight uint64                   `json:"enqueued_at_height"`
}

type BlockProgress struct {
	Height                uint64 `json:"height"`
	AppliedQueuedClaims   int    `json:"applied_queued_claims"`
	PendingQueuedClaims   int    `json:"pending_queued_claims"`
	BridgeCurrentHeight   uint64 `json:"bridge_current_height"`
	LastSubmissionMessage string `json:"last_submission_message,omitempty"`
}

type runtimeNodeState struct {
	PendingClaims []QueuedDepositClaim `json:"pending_claims"`
}

func runtimeNodeStatePath(cfg Config) string {
	if home := strings.TrimSpace(cfg.HomeDir); home != "" {
		return filepath.Join(home, "data", "node.json")
	}
	if statePath := strings.TrimSpace(cfg.StatePath); statePath != "" {
		return filepath.Join(filepath.Dir(statePath), "node.json")
	}
	return filepath.Join(defaultHomeDir(), "data", "node.json")
}

func loadRuntimeNodeState(cfg Config) (runtimeNodeState, error) {
	path := runtimeNodeStatePath(cfg)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return runtimeNodeState{}, nil
	}
	if err != nil {
		return runtimeNodeState{}, err
	}

	var state runtimeNodeState
	if err := json.Unmarshal(data, &state); err != nil {
		return runtimeNodeState{}, err
	}
	return state, nil
}

func persistRuntimeNodeState(cfg Config, claims []QueuedDepositClaim) error {
	path := runtimeNodeStatePath(cfg)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	state := runtimeNodeState{
		PendingClaims: append([]QueuedDepositClaim(nil), claims...),
	}
	encoded, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, encoded, 0o644)
}
