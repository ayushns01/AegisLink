package pipeline

import (
	"context"
	"errors"
	"fmt"

	bridgetypes "github.com/ayushns01/aegislink/chain/aegislink/x/bridge/types"
	"github.com/ayushns01/aegislink/relayer/internal/config"
	"github.com/ayushns01/aegislink/relayer/internal/cosmos"
	"github.com/ayushns01/aegislink/relayer/internal/evm"
	"github.com/ayushns01/aegislink/relayer/internal/replay"
)

const (
	depositCheckpointKey    = "evm-deposits"
	withdrawalCheckpointKey = "cosmos-withdrawals"
)

type DepositWatcher interface {
	Observe(context.Context, uint64) ([]evm.DepositEvent, uint64, error)
}

type WithdrawalWatcher interface {
	Observe(context.Context, uint64) ([]cosmos.Withdrawal, uint64, error)
}

type AttestationCollector interface {
	Collect(context.Context, string, string) (bridgetypes.Attestation, error)
}

type CosmosSubmitter interface {
	SubmitDepositClaim(context.Context, bridgetypes.DepositClaim, bridgetypes.Attestation) error
}

type EVMReleaser interface {
	ReleaseWithdrawal(context.Context, evm.ReleaseRequest) (string, error)
}

type Coordinator struct {
	cfg               config.Config
	store             *replay.Store
	depositWatcher    DepositWatcher
	collector         AttestationCollector
	submitter         CosmosSubmitter
	withdrawalWatcher WithdrawalWatcher
	evmRelease        EVMReleaser
}

type RunSummary struct {
	DepositFromBlock          uint64 `json:"deposit_from_block"`
	DepositNextCursor         uint64 `json:"deposit_next_cursor"`
	DepositsObserved          int    `json:"deposits_observed"`
	DuplicateDeposits         int    `json:"duplicate_deposits"`
	DepositsSubmitted         int    `json:"deposits_submitted"`
	DepositSubmitAttempts     int    `json:"deposit_submit_attempts"`
	WithdrawalFromHeight      uint64 `json:"withdrawal_from_height"`
	WithdrawalNextCursor      uint64 `json:"withdrawal_next_cursor"`
	WithdrawalsObserved       int    `json:"withdrawals_observed"`
	DuplicateWithdrawals      int    `json:"duplicate_withdrawals"`
	WithdrawalsReleased       int    `json:"withdrawals_released"`
	WithdrawalReleaseAttempts int    `json:"withdrawal_release_attempts"`
	AutoDeliveryIntents       int    `json:"auto_delivery_intents"`
	AutoDeliveryWaiting       int    `json:"auto_delivery_waiting"`
	AutoTransfersInitiated    int    `json:"auto_transfers_initiated"`
	AutoFlushesTriggered      int    `json:"auto_flushes_triggered"`
	AutoCompletedDeliveries   int    `json:"auto_completed_deliveries"`
	AutoFailedDeliveries      int    `json:"auto_failed_deliveries"`
}

func New(cfg config.Config, store *replay.Store, depositWatcher DepositWatcher, collector AttestationCollector, submitter CosmosSubmitter, withdrawalWatcher WithdrawalWatcher, evmRelease EVMReleaser) *Coordinator {
	return &Coordinator{
		cfg:               cfg,
		store:             store,
		depositWatcher:    depositWatcher,
		collector:         collector,
		submitter:         submitter,
		withdrawalWatcher: withdrawalWatcher,
		evmRelease:        evmRelease,
	}
}

func (c *Coordinator) RunOnce(ctx context.Context) error {
	_, err := c.RunOnceWithSummary(ctx)
	return err
}

func (c *Coordinator) RunOnceWithSummary(ctx context.Context) (RunSummary, error) {
	var summary RunSummary
	if err := c.store.Err(); err != nil {
		return summary, fmt.Errorf("replay store: %w", err)
	}
	if err := c.runDeposits(ctx, &summary); err != nil {
		return summary, err
	}
	if err := c.runWithdrawals(ctx, &summary); err != nil {
		return summary, err
	}
	return summary, nil
}

func (c *Coordinator) runDeposits(ctx context.Context, summary *RunSummary) error {
	fromBlock := c.store.Checkpoint(depositCheckpointKey)
	summary.DepositFromBlock = fromBlock
	events, nextCursor, err := c.depositWatcher.Observe(ctx, fromBlock)
	if err != nil {
		return err
	}
	summary.DepositsObserved = len(events)

	for _, event := range events {
		if c.store.IsProcessed(event.ReplayKey()) {
			summary.DuplicateDeposits++
			continue
		}

		claim := event.Claim(c.cfg.CosmosChainID)
		if err := claim.ValidateBasic(); err != nil {
			return err
		}

		attestation, err := c.collector.Collect(ctx, claim.Identity.MessageID, claim.Digest())
		if err != nil {
			return err
		}
		attempts, err := c.submitWithRetry(ctx, claim, attestation)
		summary.DepositSubmitAttempts += attempts
		if err != nil {
			return err
		}
		summary.DepositsSubmitted++

		if err := c.store.MarkProcessed(event.ReplayKey()); err != nil {
			return fmt.Errorf("mark deposit processed: %w", err)
		}
	}

	if err := c.store.SaveCheckpoint(depositCheckpointKey, nextCursor); err != nil {
		return fmt.Errorf("save deposit checkpoint: %w", err)
	}
	summary.DepositNextCursor = nextCursor
	return nil
}

func (c *Coordinator) runWithdrawals(ctx context.Context, summary *RunSummary) error {
	fromHeight := c.store.Checkpoint(withdrawalCheckpointKey)
	summary.WithdrawalFromHeight = fromHeight
	withdrawals, nextCursor, err := c.withdrawalWatcher.Observe(ctx, fromHeight)
	if err != nil {
		return err
	}
	summary.WithdrawalsObserved = len(withdrawals)

	for _, withdrawal := range withdrawals {
		if c.store.IsProcessed(withdrawal.ReplayKey()) {
			summary.DuplicateWithdrawals++
			continue
		}

		if err := withdrawal.Validate(); err != nil {
			return err
		}
		attempts, err := c.releaseWithRetry(ctx, withdrawal.ReleaseRequest())
		summary.WithdrawalReleaseAttempts += attempts
		if err != nil {
			return err
		}
		summary.WithdrawalsReleased++

		if err := c.store.MarkProcessed(withdrawal.ReplayKey()); err != nil {
			return fmt.Errorf("mark withdrawal processed: %w", err)
		}
	}

	if err := c.store.SaveCheckpoint(withdrawalCheckpointKey, nextCursor); err != nil {
		return fmt.Errorf("save withdrawal checkpoint: %w", err)
	}
	summary.WithdrawalNextCursor = nextCursor
	return nil
}

func (c *Coordinator) submitWithRetry(ctx context.Context, claim bridgetypes.DepositClaim, attestation bridgetypes.Attestation) (int, error) {
	attempts := c.cfg.SubmissionRetryLimit
	if attempts <= 0 {
		attempts = 1
	}

	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		lastErr = c.submitter.SubmitDepositClaim(ctx, claim, attestation)
		if lastErr == nil {
			return attempt, nil
		}
		if !isTemporary(lastErr) || attempt == attempts {
			break
		}
	}

	return attempts, fmt.Errorf("submit deposit claim: %w", lastErr)
}

func (c *Coordinator) releaseWithRetry(ctx context.Context, request evm.ReleaseRequest) (int, error) {
	attempts := c.cfg.SubmissionRetryLimit
	if attempts <= 0 {
		attempts = 1
	}

	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		_, lastErr = c.evmRelease.ReleaseWithdrawal(ctx, request)
		if lastErr == nil {
			return attempt, nil
		}
		if !isTemporary(lastErr) || attempt == attempts {
			break
		}
	}

	return attempts, fmt.Errorf("release withdrawal: %w", lastErr)
}

func isTemporary(err error) bool {
	var temporary interface{ Temporary() bool }
	return errors.As(err, &temporary) && temporary.Temporary()
}
