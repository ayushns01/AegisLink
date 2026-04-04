package config

import "testing"

func TestLoadRouteFromEnvParsesRouteRelayerAndMockTargetConfig(t *testing.T) {
	t.Setenv("AEGISLINK_ROUTE_RELAYER_AEGISLINK_CMD", "go")
	t.Setenv("AEGISLINK_ROUTE_RELAYER_AEGISLINK_CMD_ARGS", "run ./chain/aegislink/cmd/aegislinkd")
	t.Setenv("AEGISLINK_ROUTE_RELAYER_TARGET_URL", "http://127.0.0.1:9191")
	t.Setenv("AEGISLINK_ROUTE_RELAYER_TARGET_TIMEOUT_MS", "2500")
	t.Setenv("AEGISLINK_MOCK_OSMOSIS_ADDR", ":9292")
	t.Setenv("AEGISLINK_MOCK_OSMOSIS_MODE", "fail")
	t.Setenv("AEGISLINK_MOCK_OSMOSIS_DELAY_MS", "50")
	t.Setenv("AEGISLINK_MOCK_OSMOSIS_STATE_PATH", "/tmp/mock-osmosis.json")

	cfg := LoadRouteFromEnv()

	if cfg.AegisLinkCommand != "go" {
		t.Fatalf("expected command go, got %q", cfg.AegisLinkCommand)
	}
	if len(cfg.AegisLinkCommandArgs) != 2 {
		t.Fatalf("expected 2 command args, got %d", len(cfg.AegisLinkCommandArgs))
	}
	if cfg.TargetURL != "http://127.0.0.1:9191" {
		t.Fatalf("expected target url to parse, got %q", cfg.TargetURL)
	}
	if cfg.TargetTimeout.Milliseconds() != 2500 {
		t.Fatalf("expected target timeout 2500ms, got %d", cfg.TargetTimeout.Milliseconds())
	}
	if cfg.MockTargetAddress != ":9292" {
		t.Fatalf("expected mock target addr :9292, got %q", cfg.MockTargetAddress)
	}
	if cfg.MockTargetMode != "fail" {
		t.Fatalf("expected mock target mode fail, got %q", cfg.MockTargetMode)
	}
	if cfg.MockTargetDelay.Milliseconds() != 50 {
		t.Fatalf("expected mock target delay 50ms, got %d", cfg.MockTargetDelay.Milliseconds())
	}
	if cfg.MockTargetStatePath != "/tmp/mock-osmosis.json" {
		t.Fatalf("expected mock target state path /tmp/mock-osmosis.json, got %q", cfg.MockTargetStatePath)
	}
}
