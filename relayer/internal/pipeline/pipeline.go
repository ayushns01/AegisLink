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
	if err := c.store.Err(); err != nil {
		return fmt.Errorf("replay store: %w", err)
	}
	if err := c.runDeposits(ctx); err != nil {
		return err
	}
	return c.runWithdrawals(ctx)
}

func (c *Coordinator) runDeposits(ctx context.Context) error {
	fromBlock := c.store.Checkpoint(depositCheckpointKey)
	events, nextCursor, err := c.depositWatcher.Observe(ctx, fromBlock)
	if err != nil {
		return err
	}

	for _, event := range events {
		if c.store.IsProcessed(event.ReplayKey()) {
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
		if err := c.submitWithRetry(ctx, claim, attestation); err != nil {
			return err
		}

		if err := c.store.MarkProcessed(event.ReplayKey()); err != nil {
			return fmt.Errorf("mark deposit processed: %w", err)
		}
	}

	if err := c.store.SaveCheckpoint(depositCheckpointKey, nextCursor); err != nil {
		return fmt.Errorf("save deposit checkpoint: %w", err)
	}
	return nil
}

func (c *Coordinator) runWithdrawals(ctx context.Context) error {
	fromHeight := c.store.Checkpoint(withdrawalCheckpointKey)
	withdrawals, nextCursor, err := c.withdrawalWatcher.Observe(ctx, fromHeight)
	if err != nil {
		return err
	}

	for _, withdrawal := range withdrawals {
		if c.store.IsProcessed(withdrawal.ReplayKey()) {
			continue
		}

		if err := withdrawal.Validate(); err != nil {
			return err
		}
		if err := c.releaseWithRetry(ctx, withdrawal.ReleaseRequest()); err != nil {
			return err
		}

		if err := c.store.MarkProcessed(withdrawal.ReplayKey()); err != nil {
			return fmt.Errorf("mark withdrawal processed: %w", err)
		}
	}

	if err := c.store.SaveCheckpoint(withdrawalCheckpointKey, nextCursor); err != nil {
		return fmt.Errorf("save withdrawal checkpoint: %w", err)
	}
	return nil
}

func (c *Coordinator) submitWithRetry(ctx context.Context, claim bridgetypes.DepositClaim, attestation bridgetypes.Attestation) error {
	attempts := c.cfg.SubmissionRetryLimit
	if attempts <= 0 {
		attempts = 1
	}

	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		lastErr = c.submitter.SubmitDepositClaim(ctx, claim, attestation)
		if lastErr == nil {
			return nil
		}
		if !isTemporary(lastErr) || attempt == attempts {
			break
		}
	}

	return fmt.Errorf("submit deposit claim: %w", lastErr)
}

func (c *Coordinator) releaseWithRetry(ctx context.Context, request evm.ReleaseRequest) error {
	attempts := c.cfg.SubmissionRetryLimit
	if attempts <= 0 {
		attempts = 1
	}

	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		_, lastErr = c.evmRelease.ReleaseWithdrawal(ctx, request)
		if lastErr == nil {
			return nil
		}
		if !isTemporary(lastErr) || attempt == attempts {
			break
		}
	}

	return fmt.Errorf("release withdrawal: %w", lastErr)
}

func isTemporary(err error) bool {
	var temporary interface{ Temporary() bool }
	return errors.As(err, &temporary) && temporary.Temporary()
}
