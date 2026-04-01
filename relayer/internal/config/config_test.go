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
