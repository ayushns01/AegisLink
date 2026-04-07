package config

import "testing"

func TestLoadFromEnvFallsBackOnNegativeNumericValues(t *testing.T) {
	t.Setenv("AEGISLINK_RELAYER_ATTESTATION_THRESHOLD", "-1")
	t.Setenv("AEGISLINK_RELAYER_SUBMISSION_RETRY_LIMIT", "-2")
	t.Setenv("AEGISLINK_RELAYER_EVM_CONFIRMATIONS", "-3")
	t.Setenv("AEGISLINK_RELAYER_COSMOS_CONFIRMATIONS", "-4")

	cfg := LoadFromEnv()

	if cfg.AttestationThreshold != 2 {
		t.Fatalf("expected attestation threshold fallback 2, got %d", cfg.AttestationThreshold)
	}
	if cfg.AttestationSignerSetVersion != 1 {
		t.Fatalf("expected signer set version fallback 1, got %d", cfg.AttestationSignerSetVersion)
	}
	if cfg.SubmissionRetryLimit != 3 {
		t.Fatalf("expected retry limit fallback 3, got %d", cfg.SubmissionRetryLimit)
	}
	if cfg.EVMConfirmations != 2 {
		t.Fatalf("expected evm confirmations fallback 2, got %d", cfg.EVMConfirmations)
	}
	if cfg.CosmosConfirmations != 1 {
		t.Fatalf("expected cosmos confirmations fallback 1, got %d", cfg.CosmosConfirmations)
	}
}

func TestLoadFromEnvParsesAegisLinkCommandArgs(t *testing.T) {
	t.Setenv("AEGISLINK_RELAYER_AEGISLINK_CMD", "go")
	t.Setenv("AEGISLINK_RELAYER_AEGISLINK_CMD_ARGS", "run ./chain/aegislink/cmd/aegislinkd")

	cfg := LoadFromEnv()

	if cfg.AegisLinkCommand != "go" {
		t.Fatalf("expected command go, got %q", cfg.AegisLinkCommand)
	}
	if len(cfg.AegisLinkCommandArgs) != 2 {
		t.Fatalf("expected 2 command args, got %d: %v", len(cfg.AegisLinkCommandArgs), cfg.AegisLinkCommandArgs)
	}
	if cfg.AegisLinkCommandArgs[0] != "run" || cfg.AegisLinkCommandArgs[1] != "./chain/aegislink/cmd/aegislinkd" {
		t.Fatalf("unexpected command args: %v", cfg.AegisLinkCommandArgs)
	}
}

func TestLoadFromEnvParsesEthereumRPCSourceConfig(t *testing.T) {
	t.Setenv("AEGISLINK_RELAYER_EVM_RPC_URL", "http://127.0.0.1:8545")
	t.Setenv("AEGISLINK_RELAYER_EVM_GATEWAY_ADDRESS", "0x1234567890abcdef1234567890abcdef12345678")

	cfg := LoadFromEnv()

	if cfg.EVMRPCURL != "http://127.0.0.1:8545" {
		t.Fatalf("expected rpc url to parse, got %q", cfg.EVMRPCURL)
	}
	if cfg.EVMGatewayAddress != "0x1234567890abcdef1234567890abcdef12345678" {
		t.Fatalf("expected gateway address to parse, got %q", cfg.EVMGatewayAddress)
	}
}
