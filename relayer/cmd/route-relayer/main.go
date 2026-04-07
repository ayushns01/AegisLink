package main

import (
	"context"
	"io"
	"os"

	"github.com/ayushns01/aegislink/relayer/internal/config"
	"github.com/ayushns01/aegislink/relayer/internal/opslog"
	"github.com/ayushns01/aegislink/relayer/internal/route"
)

func main() {
	if err := run(context.Background(), os.Stderr); err != nil {
		_ = opslog.Write(os.Stderr, "error", "route-relayer", "run_failed", "route relayer run failed", map[string]any{
			"error": err.Error(),
		})
		os.Exit(1)
	}
}

func run(ctx context.Context, stderr io.Writer) error {
	cfg := config.LoadRouteFromEnv()

	sourceLocator := route.RuntimeLocator{
		Home:        cfg.AegisLinkHome,
		StatePath:   cfg.AegisLinkStatePath,
		RuntimeMode: cfg.AegisLinkRuntimeMode,
	}
	targetLocator := route.RuntimeLocator{
		Home:        cfg.DestinationHome,
		StatePath:   cfg.DestinationStatePath,
		RuntimeMode: cfg.DestinationRuntimeMode,
	}

	var target route.Target
	if cfg.DestinationCommand != "" {
		target = route.NewCommandIBCTarget(cfg.DestinationCommand, cfg.DestinationCommandArgs, targetLocator)
	} else {
		target = route.NewHTTPTarget(cfg.TargetURL, cfg.TargetTimeout)
	}

	relayer := route.NewRelayer(
		route.NewCommandTransferSource(cfg.AegisLinkCommand, cfg.AegisLinkCommandArgs, sourceLocator),
		route.NewCommandAckSink(cfg.AegisLinkCommand, cfg.AegisLinkCommandArgs, sourceLocator),
		target,
	)

	_ = opslog.Write(stderr, "info", "route-relayer", "run_start", "route relayer run started", map[string]any{
		"target_url":             cfg.TargetURL,
		"target_timeout":         cfg.TargetTimeout.String(),
		"aegislink_home":         cfg.AegisLinkHome,
		"aegislink_state":        cfg.AegisLinkStatePath,
		"aegislink_runtime_mode": cfg.AegisLinkRuntimeMode,
		"destination_home":       cfg.DestinationHome,
		"destination_state":      cfg.DestinationStatePath,
		"destination_runtime_mode": cfg.DestinationRuntimeMode,
		"destination_command":    cfg.DestinationCommand,
	})

	summary, err := relayer.RunOnceWithSummary(ctx)
	if err != nil {
		return err
	}
	return opslog.Write(stderr, "info", "route-relayer", "run_complete", "route relayer run completed", map[string]any{
		"ready_acks":          summary.ReadyAcks,
		"completed_acks":      summary.CompletedAcks,
		"failed_acks":         summary.FailedAcks,
		"timed_out_acks":      summary.TimedOutAcks,
		"transfers_observed":  summary.TransfersObserved,
		"transfers_delivered": summary.TransfersDelivered,
		"received_deliveries": summary.ReceivedDeliveries,
	})
}
