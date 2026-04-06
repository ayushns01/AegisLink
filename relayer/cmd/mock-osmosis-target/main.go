package main

import (
	"encoding/json"
	"os"
	"net/http"

	"github.com/ayushns01/aegislink/relayer/internal/config"
	"github.com/ayushns01/aegislink/relayer/internal/opslog"
	"github.com/ayushns01/aegislink/relayer/internal/route"
)

func main() {
	cfg := config.LoadRouteFromEnv()
	pools := make([]route.MockTargetPool, 0)
	if cfg.MockTargetPoolsJSON != "" {
		if err := json.Unmarshal([]byte(cfg.MockTargetPoolsJSON), &pools); err != nil {
			_ = opslog.Write(os.Stderr, "error", "mock-osmosis-target", "startup_failed", "decode mock osmosis pools", map[string]any{
				"error": err.Error(),
			})
			os.Exit(1)
		}
	}
	handler := route.NewMockTargetHandlerWithConfig(route.MockTargetConfig{
		Mode:      cfg.MockTargetMode,
		Delay:     cfg.MockTargetDelay,
		StatePath: cfg.MockTargetStatePath,
		Pools:     pools,
	})

	server := &http.Server{
		Addr:    cfg.MockTargetAddress,
		Handler: handler,
	}

	_ = opslog.Write(os.Stderr, "info", "mock-osmosis-target", "server_start", "mock Osmosis target starting", map[string]any{
		"address":    cfg.MockTargetAddress,
		"mode":       cfg.MockTargetMode,
		"state_path": cfg.MockTargetStatePath,
		"pool_count": len(pools),
	})

	if err := server.ListenAndServe(); err != nil {
		_ = opslog.Write(os.Stderr, "error", "mock-osmosis-target", "server_stopped", "mock Osmosis target stopped", map[string]any{
			"error": err.Error(),
		})
		os.Exit(1)
	}
}
