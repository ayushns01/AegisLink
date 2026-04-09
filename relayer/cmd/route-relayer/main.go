package main

import (
	"context"
	"flag"
	"io"
	"os"
	"strings"

	"github.com/ayushns01/aegislink/relayer/internal/config"
	relayermetrics "github.com/ayushns01/aegislink/relayer/internal/metrics"
	"github.com/ayushns01/aegislink/relayer/internal/opslog"
	"github.com/ayushns01/aegislink/relayer/internal/route"
)

func main() {
	if err := run(context.Background(), os.Args[1:], os.Stdout, os.Stderr); err != nil {
		_ = opslog.Write(os.Stderr, "error", "route-relayer", "run_failed", "route relayer run failed", map[string]any{
			"error": err.Error(),
		})
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	cfg := config.LoadRouteFromEnv()
	loop, err := parseLoopFlag(args, cfg.Loop)
	if err != nil {
		return err
	}

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
		"loop_mode":                loop,
		"poll_interval_ms":         cfg.PollInterval.Milliseconds(),
		"failure_backoff_ms":       cfg.FailureBackoff.Milliseconds(),
		"target_url":               cfg.TargetURL,
		"target_timeout":           cfg.TargetTimeout.String(),
		"aegislink_home":           cfg.AegisLinkHome,
		"aegislink_state":          cfg.AegisLinkStatePath,
		"aegislink_runtime_mode":   cfg.AegisLinkRuntimeMode,
		"destination_home":         cfg.DestinationHome,
		"destination_state":        cfg.DestinationStatePath,
		"destination_runtime_mode": cfg.DestinationRuntimeMode,
		"destination_command":      cfg.DestinationCommand,
	})

	if loop {
		return relayer.RunLoop(ctx, route.LoopConfig{
			PollInterval:       cfg.PollInterval,
			FailureBackoff:     cfg.FailureBackoff,
			MaxConsecutiveRuns: cfg.MaxRuns,
			OnResult: func(event route.LoopEvent) {
				logRouteOutcome(stdout, stderr, event.Summary, event.Err, event.ConsecutiveFailures)
			},
		})
	}

	summary, err := relayer.RunOnceWithSummary(ctx)
	logRouteOutcome(stdout, stderr, summary, err, 0)
	return err
}

func metricsOutputEnabled() bool {
	value := strings.TrimSpace(os.Getenv("AEGISLINK_PRINT_METRICS"))
	return value == "1" || strings.EqualFold(value, "true")
}

func parseLoopFlag(args []string, fallback bool) (bool, error) {
	flags := flag.NewFlagSet("route-relayer", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	loop := flags.Bool("loop", fallback, "run continuously")
	if err := flags.Parse(args); err != nil {
		return false, err
	}
	return *loop, nil
}

func logRouteOutcome(stdout, stderr io.Writer, summary route.RunSummary, err error, consecutiveFailures int) {
	fields := map[string]any{
		"ready_acks":          summary.ReadyAcks,
		"completed_acks":      summary.CompletedAcks,
		"failed_acks":         summary.FailedAcks,
		"timed_out_acks":      summary.TimedOutAcks,
		"transfers_observed":  summary.TransfersObserved,
		"transfers_delivered": summary.TransfersDelivered,
		"received_deliveries": summary.ReceivedDeliveries,
	}
	if consecutiveFailures > 0 {
		fields["consecutive_failures"] = consecutiveFailures
	}
	if err != nil {
		fields["error"] = err.Error()
		_ = opslog.Write(stderr, "warn", "route-relayer", "run_retry", "route relayer run failed temporarily", fields)
		return
	}
	_ = opslog.Write(stderr, "info", "route-relayer", "run_complete", "route relayer run completed", fields)
	if metricsOutputEnabled() {
		_, _ = io.WriteString(stdout, relayermetrics.FormatRouteRunSnapshot(relayermetrics.RouteRunSnapshot{
			PendingTransfers:   summary.TransfersObserved,
			TransfersDelivered: summary.TransfersDelivered,
			TimedOutAcks:       summary.TimedOutAcks,
			FailedAcks:         summary.FailedAcks,
		}))
	}
}
