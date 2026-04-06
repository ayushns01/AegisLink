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

	relayer := route.NewRelayer(
		route.NewCommandTransferSource(cfg.AegisLinkCommand, cfg.AegisLinkCommandArgs, cfg.AegisLinkStatePath),
		route.NewCommandAckSink(cfg.AegisLinkCommand, cfg.AegisLinkCommandArgs, cfg.AegisLinkStatePath),
		route.NewHTTPTarget(cfg.TargetURL, cfg.TargetTimeout),
	)

	_ = opslog.Write(stderr, "info", "route-relayer", "run_start", "route relayer run started", map[string]any{
		"target_url":      cfg.TargetURL,
		"target_timeout":  cfg.TargetTimeout.String(),
		"aegislink_state": cfg.AegisLinkStatePath,
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
