package main

import (
	"log"
	"net/http"

	"github.com/ayushns01/aegislink/relayer/internal/config"
	"github.com/ayushns01/aegislink/relayer/internal/route"
)

func main() {
	cfg := config.LoadRouteFromEnv()
	handler := route.NewMockTargetHandler(cfg.MockTargetMode, cfg.MockTargetStatePath, cfg.MockTargetDelay)

	server := &http.Server{
		Addr:    cfg.MockTargetAddress,
		Handler: handler,
	}

	log.Fatal(server.ListenAndServe())
}
