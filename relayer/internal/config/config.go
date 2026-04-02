package config

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	CosmosChainID        string
	AttestationThreshold uint32
	SubmissionRetryLimit int
	EVMConfirmations     uint64
	CosmosConfirmations  uint64
	ReplayStorePath      string
	EVMStatePath         string
	AttestationStatePath string
	CosmosStatePath      string
	CosmosOutboxPath     string
	EVMOutboxPath        string
	AegisLinkCommand     string
	AegisLinkCommandArgs []string
	AegisLinkStatePath   string
}

func LoadFromEnv() Config {
	return Config{
		CosmosChainID:        getString("AEGISLINK_RELAYER_COSMOS_CHAIN_ID", "aegislink-1"),
		AttestationThreshold: uint32(getInt("AEGISLINK_RELAYER_ATTESTATION_THRESHOLD", 2)),
		SubmissionRetryLimit: getInt("AEGISLINK_RELAYER_SUBMISSION_RETRY_LIMIT", 3),
		EVMConfirmations:     uint64(getInt("AEGISLINK_RELAYER_EVM_CONFIRMATIONS", 2)),
		CosmosConfirmations:  uint64(getInt("AEGISLINK_RELAYER_COSMOS_CONFIRMATIONS", 1)),
		ReplayStorePath:      getString("AEGISLINK_RELAYER_REPLAY_STORE_PATH", ""),
		EVMStatePath:         getString("AEGISLINK_RELAYER_EVM_STATE_PATH", defaultRuntimePath("evm-state.json")),
		AttestationStatePath: getString("AEGISLINK_RELAYER_ATTESTATION_STATE_PATH", defaultRuntimePath("attestations.json")),
		CosmosStatePath:      getString("AEGISLINK_RELAYER_COSMOS_STATE_PATH", defaultRuntimePath("cosmos-state.json")),
		CosmosOutboxPath:     getString("AEGISLINK_RELAYER_COSMOS_OUTBOX_PATH", defaultRuntimePath("cosmos-outbox.json")),
		EVMOutboxPath:        getString("AEGISLINK_RELAYER_EVM_OUTBOX_PATH", defaultRuntimePath("evm-outbox.json")),
		AegisLinkCommand:     getString("AEGISLINK_RELAYER_AEGISLINK_CMD", ""),
		AegisLinkCommandArgs: getFields("AEGISLINK_RELAYER_AEGISLINK_CMD_ARGS"),
		AegisLinkStatePath:   getString("AEGISLINK_RELAYER_AEGISLINK_STATE_PATH", defaultRuntimePath("aegislink-state.json")),
	}
}

func getString(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	if parsed < 0 {
		return fallback
	}
	return parsed
}

func defaultRuntimePath(name string) string {
	return filepath.Join(os.TempDir(), "aegislink-relayer", name)
}

func getFields(key string) []string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return nil
	}
	return strings.Fields(value)
}
