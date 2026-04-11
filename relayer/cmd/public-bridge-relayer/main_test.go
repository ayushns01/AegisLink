package main

import (
	"testing"
	"time"

	"github.com/ayushns01/aegislink/relayer/internal/config"
)

func TestValidatePublicBridgeConfigRejectsMissingRequiredEnv(t *testing.T) {
	cfg := publicBridgeConfig{}
	if err := validatePublicBridgeConfig(cfg); err == nil {
		t.Fatal("expected missing env to be rejected")
	}
}

func TestValidatePublicBridgeConfigAcceptsCompleteConfig(t *testing.T) {
	cfg := publicBridgeConfig{
		EVMRPCURL:                   "http://127.0.0.1:8545",
		EVMVerifierAddress:          "0x1111111111111111111111111111111111111111",
		EVMGatewayAddress:           "0x2222222222222222222222222222222222222222",
		AegisLinkCommand:            "go",
		AegisLinkCommandArgs:        []string{"run", "./chain/aegislink/cmd/aegislinkd"},
		AttestationThreshold:        2,
		AttestationSignerSetVersion: 1,
	}
	if err := validatePublicBridgeConfig(cfg); err != nil {
		t.Fatalf("expected complete config to pass, got %v", err)
	}
}

func TestBuildPublicBridgeConfigPrefersHomeOverDefaultStatePath(t *testing.T) {
	cfg, err := buildPublicBridgeConfig(config.Config{
		Loop:                        true,
		PollInterval:                time.Second,
		FailureBackoff:              2 * time.Second,
		CosmosChainID:               "aegislink-public-1",
		EVMRPCURL:                   "http://127.0.0.1:8545",
		EVMVerifierAddress:          "0x1111111111111111111111111111111111111111",
		EVMGatewayAddress:           "0x2222222222222222222222222222222222222222",
		AegisLinkCommand:            "go",
		AegisLinkCommandArgs:        []string{"run", "./chain/aegislink/cmd/aegislinkd", "--home", "/tmp/public-home"},
		AegisLinkStatePath:          "/tmp/should-be-cleared.json",
		AttestationThreshold:        2,
		AttestationSignerSetVersion: 1,
		AttestationSignerKeys:       []string{"key-1", "key-2"},
	})
	if err != nil {
		t.Fatalf("build public config: %v", err)
	}
	if cfg.AegisLinkStatePath != "" {
		t.Fatalf("expected state path to be cleared when --home is present, got %q", cfg.AegisLinkStatePath)
	}
}
