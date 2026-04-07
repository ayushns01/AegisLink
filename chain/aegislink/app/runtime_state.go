package app

import (
	"encoding/json"
	"os"
	"path/filepath"

	bridgekeeper "github.com/ayushns01/aegislink/chain/aegislink/x/bridge/keeper"
	governancekeeper "github.com/ayushns01/aegislink/chain/aegislink/x/governance/keeper"
	ibcrouterkeeper "github.com/ayushns01/aegislink/chain/aegislink/x/ibcrouter/keeper"
	limittypes "github.com/ayushns01/aegislink/chain/aegislink/x/limits/types"
	registrytypes "github.com/ayushns01/aegislink/chain/aegislink/x/registry/types"
)

type runtimeState struct {
	Assets      []registrytypes.Asset         `json:"assets"`
	Limits      []limittypes.RateLimit        `json:"limits"`
	PausedFlows []string                      `json:"paused_flows"`
	Bridge      bridgekeeper.StateSnapshot    `json:"bridge"`
	IBCRouter   ibcrouterkeeper.StateSnapshot `json:"ibc_router"`
	Governance  governancekeeper.StateSnapshot `json:"governance"`
}

func loadRuntimeState(path string) (runtimeState, error) {
	if path == "" {
		return runtimeState{}, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return runtimeState{}, nil
		}
		return runtimeState{}, err
	}

	var state runtimeState
	if err := json.Unmarshal(data, &state); err != nil {
		return runtimeState{}, err
	}
	return state, nil
}

func persistRuntimeState(path string, state runtimeState) error {
	if path == "" {
		return nil
	}

	encoded, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "aegislink-state-*.json")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.Write(encoded); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
