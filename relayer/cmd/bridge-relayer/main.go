package main

import (
	"context"
	"flag"
	"io"
	"os"
	"strings"

	"github.com/ayushns01/aegislink/relayer/internal/attestations"
	"github.com/ayushns01/aegislink/relayer/internal/config"
	"github.com/ayushns01/aegislink/relayer/internal/cosmos"
	"github.com/ayushns01/aegislink/relayer/internal/evm"
	relayermetrics "github.com/ayushns01/aegislink/relayer/internal/metrics"
	"github.com/ayushns01/aegislink/relayer/internal/opslog"
	"github.com/ayushns01/aegislink/relayer/internal/pipeline"
	"github.com/ayushns01/aegislink/relayer/internal/replay"
)

func main() {
	if err := run(context.Background(), os.Args[1:], os.Stdout, os.Stderr); err != nil {
		_ = opslog.Write(os.Stderr, "error", "bridge-relayer", "run_failed", "bridge relayer run failed", map[string]any{
			"error": err.Error(),
		})
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	cfg := config.LoadFromEnv()
	loop, err := parseLoopFlag(args, cfg.Loop)
	if err != nil {
		return err
	}

	evmSource := evm.NewFileLogSource(cfg.EVMStatePath)
	if cfg.EVMRPCURL != "" && cfg.EVMGatewayAddress != "" {
		evmSource = nil
	}

	var logSource evm.LogSource
	if cfg.EVMRPCURL != "" && cfg.EVMGatewayAddress != "" {
		logSource = evm.NewRPCLogSource(cfg.EVMRPCURL, cfg.EVMGatewayAddress)
	} else {
		logSource = evmSource
	}

	var releaseTarget evm.ReleaseTarget
	if cfg.EVMRPCURL != "" && cfg.EVMGatewayAddress != "" {
		releaseTarget = evm.NewRPCReleaseTarget(cfg.EVMRPCURL, cfg.EVMGatewayAddress)
	} else {
		releaseTarget = evm.NewFileReleaseTarget(cfg.EVMOutboxPath)
	}

	cosmosSource := cosmos.NewFileWithdrawalSource(cfg.CosmosStatePath)
	cosmosSink := cosmos.NewFileClaimSink(cfg.CosmosOutboxPath)
	if cfg.AegisLinkCommand != "" {
		cosmosSource = nil
		cosmosSink = nil
	}

	var withdrawalSource cosmos.WithdrawalSource
	var claimSink cosmos.ClaimSink
	if cfg.AegisLinkCommand != "" {
		withdrawalSource = cosmos.NewCommandWithdrawalSource(cfg.AegisLinkCommand, cfg.AegisLinkCommandArgs, cfg.AegisLinkStatePath)
		claimSink = cosmos.NewCommandClaimSink(cfg.AegisLinkCommand, cfg.AegisLinkCommandArgs, cfg.AegisLinkStatePath)
	} else {
		withdrawalSource = cosmosSource
		claimSink = cosmosSink
	}

	coord := pipeline.New(
		cfg,
		replay.NewStoreAt(cfg.ReplayStorePath),
		evm.NewWatcher(evm.NewClient(logSource), cfg.EVMConfirmations),
		attestations.NewCollector(
			attestations.NewFileVoteSource(cfg.AttestationStatePath),
			cfg.AttestationThreshold,
			cfg.AttestationSignerSetVersion,
			cfg.AttestationSignerKeys,
		),
		cosmos.NewSubmitter(claimSink),
		cosmos.NewWatcher(cosmos.NewClient(withdrawalSource), cfg.CosmosConfirmations),
		evm.NewReleaser(releaseTarget),
	)

	_ = opslog.Write(stderr, "info", "bridge-relayer", "run_start", "bridge relayer run started", map[string]any{
		"cosmos_chain_id":        cfg.CosmosChainID,
		"attestation_threshold":  cfg.AttestationThreshold,
		"signer_set_version":     cfg.AttestationSignerSetVersion,
		"submission_retry_limit": cfg.SubmissionRetryLimit,
		"loop_mode":              loop,
		"poll_interval_ms":       cfg.PollInterval.Milliseconds(),
		"failure_backoff_ms":     cfg.FailureBackoff.Milliseconds(),
		"evm_source_mode":        evmSourceMode(cfg),
		"cosmos_runtime_mode":    cosmosRuntimeMode(cfg),
	})

	if loop {
		daemon := pipeline.NewDaemon(coord, pipeline.DaemonConfig{
			PollInterval:       cfg.PollInterval,
			FailureBackoff:     cfg.FailureBackoff,
			MaxConsecutiveRuns: cfg.MaxRuns,
			OnResult: func(event pipeline.DaemonEvent) {
				logBridgeOutcome(stdout, stderr, event.Summary, event.Err, event.ConsecutiveFailures)
			},
		})
		return daemon.Run(ctx)
	}

	summary, err := coord.RunOnceWithSummary(ctx)
	logBridgeOutcome(stdout, stderr, summary, err, 0)
	return err
}

func evmSourceMode(cfg config.Config) string {
	if cfg.EVMRPCURL != "" && cfg.EVMGatewayAddress != "" {
		return "rpc"
	}
	return "file"
}

func cosmosRuntimeMode(cfg config.Config) string {
	if cfg.AegisLinkCommand != "" {
		return "command"
	}
	return "file"
}

func metricsOutputEnabled() bool {
	value := strings.TrimSpace(os.Getenv("AEGISLINK_PRINT_METRICS"))
	return value == "1" || strings.EqualFold(value, "true")
}

func parseLoopFlag(args []string, fallback bool) (bool, error) {
	flags := flag.NewFlagSet("bridge-relayer", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	loop := flags.Bool("loop", fallback, "run continuously")
	if err := flags.Parse(args); err != nil {
		return false, err
	}
	return *loop, nil
}

func logBridgeOutcome(stdout, stderr io.Writer, summary pipeline.RunSummary, err error, consecutiveFailures int) {
	fields := map[string]any{
		"deposits_observed":           summary.DepositsObserved,
		"duplicate_deposits":          summary.DuplicateDeposits,
		"deposits_submitted":          summary.DepositsSubmitted,
		"deposit_submit_attempts":     summary.DepositSubmitAttempts,
		"withdrawals_observed":        summary.WithdrawalsObserved,
		"duplicate_withdrawals":       summary.DuplicateWithdrawals,
		"withdrawals_released":        summary.WithdrawalsReleased,
		"withdrawal_release_attempts": summary.WithdrawalReleaseAttempts,
		"deposit_next_cursor":         summary.DepositNextCursor,
		"withdrawal_next_cursor":      summary.WithdrawalNextCursor,
	}
	if consecutiveFailures > 0 {
		fields["consecutive_failures"] = consecutiveFailures
	}
	if err != nil {
		fields["error"] = err.Error()
		_ = opslog.Write(stderr, "warn", "bridge-relayer", "run_retry", "bridge relayer run failed temporarily", fields)
		return
	}
	_ = opslog.Write(stderr, "info", "bridge-relayer", "run_complete", "bridge relayer run completed", fields)
	if metricsOutputEnabled() {
		_, _ = io.WriteString(stdout, relayermetrics.FormatBridgeRunSnapshot(relayermetrics.BridgeRunSnapshot{
			DepositsObserved:      summary.DepositsObserved,
			DepositsSubmitted:     summary.DepositsSubmitted,
			DepositSubmitAttempts: summary.DepositSubmitAttempts,
			WithdrawalsObserved:   summary.WithdrawalsObserved,
			WithdrawalsReleased:   summary.WithdrawalsReleased,
		}))
	}
}
