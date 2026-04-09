package config

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	bridgetypes "github.com/ayushns01/aegislink/chain/aegislink/x/bridge/types"
)

type Config struct {
	CosmosChainID               string
	AttestationThreshold        uint32
	AttestationSignerSetVersion uint64
	AttestationSignerKeys       []string
	Loop                        bool
	PollInterval                time.Duration
	FailureBackoff              time.Duration
	MaxRuns                     int
	SubmissionRetryLimit        int
	EVMConfirmations            uint64
	CosmosConfirmations         uint64
	EVMRPCURL                   string
	EVMGatewayAddress           string
	ReplayStorePath             string
	EVMStatePath                string
	AttestationStatePath        string
	CosmosStatePath             string
	CosmosOutboxPath            string
	EVMOutboxPath               string
	AegisLinkCommand            string
	AegisLinkCommandArgs        []string
	AegisLinkStatePath          string
}

func LoadFromEnv() Config {
	return Config{
		CosmosChainID:               getString("AEGISLINK_RELAYER_COSMOS_CHAIN_ID", "aegislink-1"),
		AttestationThreshold:        uint32(getInt("AEGISLINK_RELAYER_ATTESTATION_THRESHOLD", 2)),
		AttestationSignerSetVersion: uint64(getInt("AEGISLINK_RELAYER_ATTESTATION_SIGNER_SET_VERSION", 1)),
		AttestationSignerKeys:       getFieldsWithFallback("AEGISLINK_RELAYER_ATTESTATION_SIGNER_KEYS", bridgetypes.DefaultHarnessSignerPrivateKeys()[:3]),
		Loop:                        getBool("AEGISLINK_RELAYER_LOOP", false),
		PollInterval:                time.Duration(getInt("AEGISLINK_RELAYER_POLL_INTERVAL_MS", 1000)) * time.Millisecond,
		FailureBackoff:              time.Duration(getInt("AEGISLINK_RELAYER_FAILURE_BACKOFF_MS", 5000)) * time.Millisecond,
		MaxRuns:                     getInt("AEGISLINK_RELAYER_MAX_RUNS", 0),
		SubmissionRetryLimit:        getInt("AEGISLINK_RELAYER_SUBMISSION_RETRY_LIMIT", 3),
		EVMConfirmations:            uint64(getInt("AEGISLINK_RELAYER_EVM_CONFIRMATIONS", 2)),
		CosmosConfirmations:         uint64(getInt("AEGISLINK_RELAYER_COSMOS_CONFIRMATIONS", 1)),
		EVMRPCURL:                   getString("AEGISLINK_RELAYER_EVM_RPC_URL", ""),
		EVMGatewayAddress:           getString("AEGISLINK_RELAYER_EVM_GATEWAY_ADDRESS", ""),
		ReplayStorePath:             getString("AEGISLINK_RELAYER_REPLAY_STORE_PATH", ""),
		EVMStatePath:                getString("AEGISLINK_RELAYER_EVM_STATE_PATH", defaultRuntimePath("evm-state.json")),
		AttestationStatePath:        getString("AEGISLINK_RELAYER_ATTESTATION_STATE_PATH", defaultRuntimePath("attestations.json")),
		CosmosStatePath:             getString("AEGISLINK_RELAYER_COSMOS_STATE_PATH", defaultRuntimePath("cosmos-state.json")),
		CosmosOutboxPath:            getString("AEGISLINK_RELAYER_COSMOS_OUTBOX_PATH", defaultRuntimePath("cosmos-outbox.json")),
		EVMOutboxPath:               getString("AEGISLINK_RELAYER_EVM_OUTBOX_PATH", defaultRuntimePath("evm-outbox.json")),
		AegisLinkCommand:            getString("AEGISLINK_RELAYER_AEGISLINK_CMD", ""),
		AegisLinkCommandArgs:        getFields("AEGISLINK_RELAYER_AEGISLINK_CMD_ARGS"),
		AegisLinkStatePath:          getString("AEGISLINK_RELAYER_AEGISLINK_STATE_PATH", defaultRuntimePath("aegislink-state.json")),
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

func getBool(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	switch strings.ToLower(value) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
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

func getFieldsWithFallback(key string, fallback []string) []string {
	fields := getFields(key)
	if len(fields) == 0 {
		return append([]string(nil), fallback...)
	}
	return fields
}
