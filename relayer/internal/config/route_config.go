package config

import "time"

type RouteConfig struct {
	Loop                  bool
	PollInterval          time.Duration
	FailureBackoff        time.Duration
	MaxRuns               int
	AegisLinkCommand     string
	AegisLinkCommandArgs []string
	AegisLinkHome        string
	AegisLinkStatePath   string
	AegisLinkRuntimeMode string
	DestinationCommand   string
	DestinationCommandArgs []string
	DestinationHome      string
	DestinationStatePath string
	DestinationRuntimeMode string
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
		Loop:                  getBool("AEGISLINK_ROUTE_RELAYER_LOOP", false),
		PollInterval:          time.Duration(getInt("AEGISLINK_ROUTE_RELAYER_POLL_INTERVAL_MS", 1000)) * time.Millisecond,
		FailureBackoff:        time.Duration(getInt("AEGISLINK_ROUTE_RELAYER_FAILURE_BACKOFF_MS", 5000)) * time.Millisecond,
		MaxRuns:               getInt("AEGISLINK_ROUTE_RELAYER_MAX_RUNS", 0),
		AegisLinkCommand:       getString("AEGISLINK_ROUTE_RELAYER_AEGISLINK_CMD", ""),
		AegisLinkCommandArgs:   getFields("AEGISLINK_ROUTE_RELAYER_AEGISLINK_CMD_ARGS"),
		AegisLinkHome:          getString("AEGISLINK_ROUTE_RELAYER_AEGISLINK_HOME", ""),
		AegisLinkStatePath:     getString("AEGISLINK_ROUTE_RELAYER_AEGISLINK_STATE_PATH", ""),
		AegisLinkRuntimeMode:   getString("AEGISLINK_ROUTE_RELAYER_AEGISLINK_RUNTIME_MODE", ""),
		DestinationCommand:     getString("AEGISLINK_ROUTE_RELAYER_DESTINATION_CMD", ""),
		DestinationCommandArgs: getFields("AEGISLINK_ROUTE_RELAYER_DESTINATION_CMD_ARGS"),
		DestinationHome:        getString("AEGISLINK_ROUTE_RELAYER_DESTINATION_HOME", ""),
		DestinationStatePath:   getString("AEGISLINK_ROUTE_RELAYER_DESTINATION_STATE_PATH", ""),
		DestinationRuntimeMode: getString("AEGISLINK_ROUTE_RELAYER_DESTINATION_RUNTIME_MODE", ""),
		TargetURL:              getString("AEGISLINK_ROUTE_RELAYER_TARGET_URL", ""),
		TargetTimeout:          time.Duration(getInt("AEGISLINK_ROUTE_RELAYER_TARGET_TIMEOUT_MS", 1000)) * time.Millisecond,
		MockTargetAddress:      getString("AEGISLINK_MOCK_OSMOSIS_ADDR", ":9191"),
		MockTargetMode:         getString("AEGISLINK_MOCK_OSMOSIS_MODE", "success"),
		MockTargetDelay:        time.Duration(getInt("AEGISLINK_MOCK_OSMOSIS_DELAY_MS", 0)) * time.Millisecond,
		MockTargetStatePath:    getString("AEGISLINK_MOCK_OSMOSIS_STATE_PATH", ""),
		MockTargetPoolsJSON:    getString("AEGISLINK_MOCK_OSMOSIS_POOLS_JSON", ""),
	}
}
