package main

import (
	"context"
	"log"

	"github.com/ayushns01/aegislink/relayer/internal/config"
	"github.com/ayushns01/aegislink/relayer/internal/route"
)

func main() {
	cfg := config.LoadRouteFromEnv()

	relayer := route.NewRelayer(
		route.NewCommandTransferSource(cfg.AegisLinkCommand, cfg.AegisLinkCommandArgs, cfg.AegisLinkStatePath),
		route.NewCommandAckSink(cfg.AegisLinkCommand, cfg.AegisLinkCommandArgs, cfg.AegisLinkStatePath),
		route.NewHTTPTarget(cfg.TargetURL, cfg.TargetTimeout),
	)

	if err := relayer.RunOnce(context.Background()); err != nil {
		log.Fatal(err)
	}
}
