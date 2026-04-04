package config

import "time"

type RouteConfig struct {
	AegisLinkCommand     string
	AegisLinkCommandArgs []string
	AegisLinkStatePath   string
	TargetURL            string
	TargetTimeout        time.Duration
	MockTargetAddress    string
	MockTargetMode       string
	MockTargetDelay      time.Duration
	MockTargetStatePath  string
	MockTargetPoolsJSON  string
}

func LoadRouteFromEnv() RouteConfig {
	return RouteConfig{
		AegisLinkCommand:     getString("AEGISLINK_ROUTE_RELAYER_AEGISLINK_CMD", ""),
		AegisLinkCommandArgs: getFields("AEGISLINK_ROUTE_RELAYER_AEGISLINK_CMD_ARGS"),
		AegisLinkStatePath:   getString("AEGISLINK_ROUTE_RELAYER_AEGISLINK_STATE_PATH", defaultRuntimePath("aegislink-state.json")),
		TargetURL:            getString("AEGISLINK_ROUTE_RELAYER_TARGET_URL", ""),
		TargetTimeout:        time.Duration(getInt("AEGISLINK_ROUTE_RELAYER_TARGET_TIMEOUT_MS", 1000)) * time.Millisecond,
		MockTargetAddress:    getString("AEGISLINK_MOCK_OSMOSIS_ADDR", ":9191"),
		MockTargetMode:       getString("AEGISLINK_MOCK_OSMOSIS_MODE", "success"),
		MockTargetDelay:      time.Duration(getInt("AEGISLINK_MOCK_OSMOSIS_DELAY_MS", 0)) * time.Millisecond,
		MockTargetStatePath:  getString("AEGISLINK_MOCK_OSMOSIS_STATE_PATH", ""),
		MockTargetPoolsJSON:  getString("AEGISLINK_MOCK_OSMOSIS_POOLS_JSON", ""),
	}
}
