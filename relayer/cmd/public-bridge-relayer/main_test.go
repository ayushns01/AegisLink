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
		AttestationSignerKeys:       []string{"key-1", "key-2"},
	}
	if err := validatePublicBridgeConfig(cfg); err != nil {
		t.Fatalf("expected complete config to pass, got %v", err)
	}
}

func TestValidatePublicBridgeConfigRejectsInsufficientAttestationSignerKeys(t *testing.T) {
	cfg := publicBridgeConfig{
		EVMRPCURL:                   "http://127.0.0.1:8545",
		EVMVerifierAddress:          "0x1111111111111111111111111111111111111111",
		EVMGatewayAddress:           "0x2222222222222222222222222222222222222222",
		AegisLinkCommand:            "go",
		AegisLinkCommandArgs:        []string{"run", "./chain/aegislink/cmd/aegislinkd"},
		AttestationThreshold:        2,
		AttestationSignerSetVersion: 1,
		AttestationSignerKeys:       []string{"key-1"},
	}
	if err := validatePublicBridgeConfig(cfg); err == nil {
		t.Fatal("expected insufficient attestation signer keys to be rejected")
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

func TestLoadAutoDeliveryTimeoutHeightUsesDestinationHeightBufferWhenConfiguredTimeoutIsStale(t *testing.T) {
	t.Setenv("AEGISLINK_RELAYER_IBC_TIMEOUT_HEIGHT", "55000000")
	t.Setenv("AEGISLINK_RELAYER_DESTINATION_LCD_BASE_URL", "https://lcd.osmotest5.osmosis.zone")
	restore := stubLatestLCDHeightFunc(func(string) (uint64, error) { return 55226038, nil })
	defer restore()

	got := loadAutoDeliveryTimeoutHeight()
	want := uint64(55226038 + autoDeliveryTimeoutHeightBuffer)
	if got != want {
		t.Fatalf("expected stale timeout to be raised to %d, got %d", want, got)
	}
}

func TestLoadAutoDeliveryTimeoutHeightPreservesFutureConfiguredTimeout(t *testing.T) {
	t.Setenv("AEGISLINK_RELAYER_IBC_TIMEOUT_HEIGHT", "56000000")
	t.Setenv("AEGISLINK_RELAYER_DESTINATION_LCD_BASE_URL", "https://lcd.osmotest5.osmosis.zone")
	restore := stubLatestLCDHeightFunc(func(string) (uint64, error) { return 55226038, nil })
	defer restore()

	got := loadAutoDeliveryTimeoutHeight()
	if got != 56000000 {
		t.Fatalf("expected explicit future timeout to be preserved, got %d", got)
	}
}

func TestLoadAutoDeliveryTimeoutHeightUsesDestinationHeightBufferWhenConfigMissing(t *testing.T) {
	t.Setenv("AEGISLINK_RELAYER_DESTINATION_LCD_BASE_URL", "https://lcd.osmotest5.osmosis.zone")
	restore := stubLatestLCDHeightFunc(func(string) (uint64, error) { return 55226038, nil })
	defer restore()

	got := loadAutoDeliveryTimeoutHeight()
	want := uint64(55226038 + autoDeliveryTimeoutHeightBuffer)
	if got != want {
		t.Fatalf("expected missing timeout to resolve to %d, got %d", want, got)
	}
}

func stubLatestLCDHeightFunc(stub func(string) (uint64, error)) func() {
	previous := latestLCDHeightFunc
	latestLCDHeightFunc = stub
	return func() {
		latestLCDHeightFunc = previous
	}
}
