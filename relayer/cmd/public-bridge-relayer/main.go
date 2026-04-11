package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/ayushns01/aegislink/relayer/internal/attestations"
	"github.com/ayushns01/aegislink/relayer/internal/config"
	"github.com/ayushns01/aegislink/relayer/internal/cosmos"
	"github.com/ayushns01/aegislink/relayer/internal/evm"
	relayermetrics "github.com/ayushns01/aegislink/relayer/internal/metrics"
	"github.com/ayushns01/aegislink/relayer/internal/opslog"
	"github.com/ayushns01/aegislink/relayer/internal/pipeline"
	"github.com/ayushns01/aegislink/relayer/internal/replay"
)

type publicBridgeConfig struct {
	Loop                        bool
	PollInterval                time.Duration
	FailureBackoff              time.Duration
	MaxRuns                     int
	SubmissionRetryLimit        int
	EVMConfirmations            uint64
	CosmosConfirmations         uint64
	CosmosChainID               string
	EVMRPCURL                   string
	EVMVerifierAddress          string
	EVMGatewayAddress           string
	EVMReleaseSignerPrivateKey  string
	EVMReleaseSignerAddress     string
	ReplayStorePath             string
	EVMStatePath                string
	AttestationStatePath        string
	CosmosStatePath             string
	CosmosOutboxPath            string
	EVMOutboxPath               string
	AegisLinkCommand            string
	AegisLinkCommandArgs        []string
	AegisLinkStatePath          string
	AttestationThreshold        uint32
	AttestationSignerSetVersion uint64
	AttestationSignerKeys       []string
}

func main() {
	if err := run(context.Background(), os.Args[1:], os.Stdout, os.Stderr); err != nil {
		_ = opslog.Write(os.Stderr, "error", "public-bridge-relayer", "run_failed", "public bridge relayer run failed", map[string]any{
			"error": err.Error(),
		})
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	cfg, err := buildPublicBridgeConfig(config.LoadFromEnv())
	if err != nil {
		return err
	}
	loop, err := parseLoopFlag(args, cfg.Loop)
	if err != nil {
		return err
	}

	evmSource := evm.NewRPCLogSource(cfg.EVMRPCURL, cfg.EVMGatewayAddress)
	releaseTarget := evm.NewRPCReleaseTargetWithSigner(cfg.EVMRPCURL, cfg.EVMGatewayAddress, cfg.EVMReleaseSignerPrivateKey, cfg.EVMReleaseSignerAddress)
	withdrawalSource := cosmos.NewCommandWithdrawalSource(cfg.AegisLinkCommand, cfg.AegisLinkCommandArgs, cfg.AegisLinkStatePath)
	claimSink := cosmos.NewCommandClaimSink(cfg.AegisLinkCommand, cfg.AegisLinkCommandArgs, cfg.AegisLinkStatePath)

	coord := pipeline.New(
		config.Config{
			CosmosChainID:               cfg.CosmosChainID,
			AttestationThreshold:        cfg.AttestationThreshold,
			AttestationSignerSetVersion: cfg.AttestationSignerSetVersion,
			AttestationSignerKeys:       cfg.AttestationSignerKeys,
			Loop:                        loop,
			PollInterval:                cfg.PollInterval,
			FailureBackoff:              cfg.FailureBackoff,
			MaxRuns:                     cfg.MaxRuns,
			SubmissionRetryLimit:        cfg.SubmissionRetryLimit,
			EVMConfirmations:            cfg.EVMConfirmations,
			CosmosConfirmations:         cfg.CosmosConfirmations,
			EVMRPCURL:                   cfg.EVMRPCURL,
			EVMVerifierAddress:          cfg.EVMVerifierAddress,
			EVMGatewayAddress:           cfg.EVMGatewayAddress,
			EVMReleaseSignerPrivateKey:  cfg.EVMReleaseSignerPrivateKey,
			EVMReleaseSignerAddress:     cfg.EVMReleaseSignerAddress,
			ReplayStorePath:             cfg.ReplayStorePath,
			EVMStatePath:                cfg.EVMStatePath,
			AttestationStatePath:        cfg.AttestationStatePath,
			CosmosStatePath:             cfg.CosmosStatePath,
			CosmosOutboxPath:            cfg.CosmosOutboxPath,
			EVMOutboxPath:               cfg.EVMOutboxPath,
			AegisLinkCommand:            cfg.AegisLinkCommand,
			AegisLinkCommandArgs:        cfg.AegisLinkCommandArgs,
			AegisLinkStatePath:          cfg.AegisLinkStatePath,
		},
		replay.NewStoreAt(cfg.ReplayStorePath),
		evm.NewWatcher(evm.NewClient(evmSource), cfg.EVMConfirmations),
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

	_ = opslog.Write(stderr, "info", "public-bridge-relayer", "run_start", "public bridge relayer run started", map[string]any{
		"cosmos_chain_id":        cfg.CosmosChainID,
		"evm_rpc_url":            cfg.EVMRPCURL,
		"evm_verifier_address":   cfg.EVMVerifierAddress,
		"evm_gateway_address":    cfg.EVMGatewayAddress,
		"aegislink_command":      cfg.AegisLinkCommand,
		"submission_retry_limit": cfg.SubmissionRetryLimit,
		"loop_mode":              loop,
		"poll_interval_ms":       cfg.PollInterval.Milliseconds(),
		"failure_backoff_ms":     cfg.FailureBackoff.Milliseconds(),
	})

	if loop {
		daemon := pipeline.NewDaemon(coord, pipeline.DaemonConfig{
			PollInterval:       cfg.PollInterval,
			FailureBackoff:     cfg.FailureBackoff,
			MaxConsecutiveRuns: cfg.MaxRuns,
			OnResult: func(event pipeline.DaemonEvent) {
				logPublicBridgeOutcome(stdout, stderr, event.Summary, event.Err, event.ConsecutiveFailures)
			},
		})
		return daemon.Run(ctx)
	}

	summary, err := coord.RunOnceWithSummary(ctx)
	logPublicBridgeOutcome(stdout, stderr, summary, err, 0)
	return err
}

func buildPublicBridgeConfig(cfg config.Config) (publicBridgeConfig, error) {
	public := publicBridgeConfig{
		Loop:                        cfg.Loop,
		PollInterval:                cfg.PollInterval,
		FailureBackoff:              cfg.FailureBackoff,
		MaxRuns:                     cfg.MaxRuns,
		SubmissionRetryLimit:        cfg.SubmissionRetryLimit,
		EVMConfirmations:            cfg.EVMConfirmations,
		CosmosConfirmations:         cfg.CosmosConfirmations,
		CosmosChainID:               cfg.CosmosChainID,
		EVMRPCURL:                   strings.TrimSpace(cfg.EVMRPCURL),
		EVMVerifierAddress:          strings.TrimSpace(cfg.EVMVerifierAddress),
		EVMGatewayAddress:           strings.TrimSpace(cfg.EVMGatewayAddress),
		EVMReleaseSignerPrivateKey:  strings.TrimSpace(cfg.EVMReleaseSignerPrivateKey),
		EVMReleaseSignerAddress:     strings.TrimSpace(cfg.EVMReleaseSignerAddress),
		ReplayStorePath:             strings.TrimSpace(cfg.ReplayStorePath),
		EVMStatePath:                strings.TrimSpace(cfg.EVMStatePath),
		AttestationStatePath:        strings.TrimSpace(cfg.AttestationStatePath),
		CosmosStatePath:             strings.TrimSpace(cfg.CosmosStatePath),
		CosmosOutboxPath:            strings.TrimSpace(cfg.CosmosOutboxPath),
		EVMOutboxPath:               strings.TrimSpace(cfg.EVMOutboxPath),
		AegisLinkCommand:            strings.TrimSpace(cfg.AegisLinkCommand),
		AegisLinkCommandArgs:        append([]string(nil), cfg.AegisLinkCommandArgs...),
		AegisLinkStatePath:          strings.TrimSpace(cfg.AegisLinkStatePath),
		AttestationThreshold:        cfg.AttestationThreshold,
		AttestationSignerSetVersion: cfg.AttestationSignerSetVersion,
		AttestationSignerKeys:       append([]string(nil), cfg.AttestationSignerKeys...),
	}
	if commandArgsContainFlag(public.AegisLinkCommandArgs, "--home") {
		public.AegisLinkStatePath = ""
	}
	if err := validatePublicBridgeConfig(public); err != nil {
		return publicBridgeConfig{}, err
	}
	return public, nil
}

func validatePublicBridgeConfig(cfg publicBridgeConfig) error {
	if strings.TrimSpace(cfg.EVMRPCURL) == "" {
		return fmt.Errorf("missing required env: AEGISLINK_RELAYER_EVM_RPC_URL")
	}
	if strings.TrimSpace(cfg.EVMVerifierAddress) == "" {
		return fmt.Errorf("missing required env: AEGISLINK_RELAYER_EVM_VERIFIER_ADDRESS")
	}
	if strings.TrimSpace(cfg.EVMGatewayAddress) == "" {
		return fmt.Errorf("missing required env: AEGISLINK_RELAYER_EVM_GATEWAY_ADDRESS")
	}
	if strings.TrimSpace(cfg.AegisLinkCommand) == "" {
		return fmt.Errorf("missing required env: AEGISLINK_RELAYER_AEGISLINK_CMD")
	}
	if len(cfg.AegisLinkCommandArgs) == 0 {
		return fmt.Errorf("missing required env: AEGISLINK_RELAYER_AEGISLINK_CMD_ARGS")
	}
	return nil
}

func commandArgsContainFlag(args []string, flagName string) bool {
	flagName = strings.TrimSpace(flagName)
	if flagName == "" {
		return false
	}
	for _, arg := range args {
		if strings.TrimSpace(arg) == flagName {
			return true
		}
	}
	return false
}

func metricsOutputEnabled() bool {
	value := strings.TrimSpace(os.Getenv("AEGISLINK_PRINT_METRICS"))
	return value == "1" || strings.EqualFold(value, "true")
}

func parseLoopFlag(args []string, fallback bool) (bool, error) {
	flags := flag.NewFlagSet("public-bridge-relayer", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	loop := flags.Bool("loop", fallback, "run continuously")
	if err := flags.Parse(args); err != nil {
		return false, err
	}
	return *loop, nil
}

func logPublicBridgeOutcome(stdout, stderr io.Writer, summary pipeline.RunSummary, err error, consecutiveFailures int) {
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
		_ = opslog.Write(stderr, "warn", "public-bridge-relayer", "run_retry", "public bridge relayer run failed temporarily", fields)
		return
	}
	_ = opslog.Write(stderr, "info", "public-bridge-relayer", "run_complete", "public bridge relayer run completed", fields)
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
