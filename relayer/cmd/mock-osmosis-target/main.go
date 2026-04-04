package main

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/ayushns01/aegislink/relayer/internal/config"
	"github.com/ayushns01/aegislink/relayer/internal/route"
)

func main() {
	cfg := config.LoadRouteFromEnv()
	pools := make([]route.MockTargetPool, 0)
	if cfg.MockTargetPoolsJSON != "" {
		if err := json.Unmarshal([]byte(cfg.MockTargetPoolsJSON), &pools); err != nil {
			log.Fatalf("decode mock osmosis pools: %v", err)
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

	log.Fatal(server.ListenAndServe())
}
