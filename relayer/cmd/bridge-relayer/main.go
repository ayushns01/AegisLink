package main

import (
	"context"
	"io"
	"os"

	"github.com/ayushns01/aegislink/relayer/internal/attestations"
	"github.com/ayushns01/aegislink/relayer/internal/config"
	"github.com/ayushns01/aegislink/relayer/internal/cosmos"
	"github.com/ayushns01/aegislink/relayer/internal/evm"
	"github.com/ayushns01/aegislink/relayer/internal/opslog"
	"github.com/ayushns01/aegislink/relayer/internal/pipeline"
	"github.com/ayushns01/aegislink/relayer/internal/replay"
)

func main() {
	if err := run(context.Background(), os.Stderr); err != nil {
		_ = opslog.Write(os.Stderr, "error", "bridge-relayer", "run_failed", "bridge relayer run failed", map[string]any{
			"error": err.Error(),
		})
		os.Exit(1)
	}
}

func run(ctx context.Context, stderr io.Writer) error {
	cfg := config.LoadFromEnv()

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
		attestations.NewCollector(attestations.NewFileVoteSource(cfg.AttestationStatePath), cfg.AttestationThreshold),
		cosmos.NewSubmitter(claimSink),
		cosmos.NewWatcher(cosmos.NewClient(withdrawalSource), cfg.CosmosConfirmations),
		evm.NewReleaser(releaseTarget),
	)

	_ = opslog.Write(stderr, "info", "bridge-relayer", "run_start", "bridge relayer run started", map[string]any{
		"cosmos_chain_id":        cfg.CosmosChainID,
		"attestation_threshold":  cfg.AttestationThreshold,
		"submission_retry_limit": cfg.SubmissionRetryLimit,
		"evm_source_mode":        evmSourceMode(cfg),
		"cosmos_runtime_mode":    cosmosRuntimeMode(cfg),
	})

	summary, err := coord.RunOnceWithSummary(ctx)
	if err != nil {
		return err
	}
	return opslog.Write(stderr, "info", "bridge-relayer", "run_complete", "bridge relayer run completed", map[string]any{
		"deposits_observed":            summary.DepositsObserved,
		"duplicate_deposits":           summary.DuplicateDeposits,
		"deposits_submitted":           summary.DepositsSubmitted,
		"deposit_submit_attempts":      summary.DepositSubmitAttempts,
		"withdrawals_observed":         summary.WithdrawalsObserved,
		"duplicate_withdrawals":        summary.DuplicateWithdrawals,
		"withdrawals_released":         summary.WithdrawalsReleased,
		"withdrawal_release_attempts":  summary.WithdrawalReleaseAttempts,
		"deposit_next_cursor":          summary.DepositNextCursor,
		"withdrawal_next_cursor":       summary.WithdrawalNextCursor,
	})
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
