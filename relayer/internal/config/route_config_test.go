package config

import "testing"

func TestLoadRouteFromEnvParsesMockTargetPoolsJSON(t *testing.T) {
	t.Setenv("AEGISLINK_MOCK_OSMOSIS_POOLS_JSON", `[{"input_denom":"ibc/uatom-usdc","output_denom":"uion","reserve_in":"800000000","reserve_out":"400000000","fee_bps":25}]`)

	cfg := LoadRouteFromEnv()

	if cfg.MockTargetPoolsJSON == "" {
		t.Fatal("expected mock target pools json to parse")
	}
}
