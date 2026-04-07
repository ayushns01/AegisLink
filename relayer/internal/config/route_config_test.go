package config

import "testing"

func TestLoadRouteFromEnvParsesMockTargetPoolsJSON(t *testing.T) {
	t.Setenv("AEGISLINK_MOCK_OSMOSIS_POOLS_JSON", `[{"input_denom":"ibc/uatom-usdc","output_denom":"uion","reserve_in":"800000000","reserve_out":"400000000","fee_bps":25}]`)
	t.Setenv("AEGISLINK_ROUTE_RELAYER_AEGISLINK_HOME", "/tmp/aegis-home")
	t.Setenv("AEGISLINK_ROUTE_RELAYER_AEGISLINK_RUNTIME_MODE", "sdk-store-runtime")
	t.Setenv("AEGISLINK_ROUTE_RELAYER_DESTINATION_CMD", "go")
	t.Setenv("AEGISLINK_ROUTE_RELAYER_DESTINATION_CMD_ARGS", "run ./relayer/cmd/osmo-locald")
	t.Setenv("AEGISLINK_ROUTE_RELAYER_DESTINATION_HOME", "/tmp/osmo-home")
	t.Setenv("AEGISLINK_ROUTE_RELAYER_DESTINATION_RUNTIME_MODE", "osmo-local-runtime")

	cfg := LoadRouteFromEnv()

	if cfg.MockTargetPoolsJSON == "" {
		t.Fatal("expected mock target pools json to parse")
	}
	if cfg.AegisLinkHome != "/tmp/aegis-home" {
		t.Fatalf("expected aegis home, got %q", cfg.AegisLinkHome)
	}
	if cfg.AegisLinkRuntimeMode != "sdk-store-runtime" {
		t.Fatalf("expected aegis runtime mode, got %q", cfg.AegisLinkRuntimeMode)
	}
	if cfg.DestinationCommand != "go" {
		t.Fatalf("expected destination command go, got %q", cfg.DestinationCommand)
	}
	if cfg.DestinationHome != "/tmp/osmo-home" {
		t.Fatalf("expected destination home, got %q", cfg.DestinationHome)
	}
	if cfg.AegisLinkStatePath != "" {
		t.Fatalf("expected empty default aegis state path when home is used, got %q", cfg.AegisLinkStatePath)
	}
	if cfg.DestinationStatePath != "" {
		t.Fatalf("expected empty default destination state path when home is used, got %q", cfg.DestinationStatePath)
	}
}
