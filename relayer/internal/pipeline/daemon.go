package pipeline

import (
	"context"
	"time"
)

type DaemonConfig struct {
	PollInterval       time.Duration
	FailureBackoff     time.Duration
	MaxConsecutiveRuns int
	OnResult           func(DaemonEvent)
}

type DaemonEvent struct {
	Iteration           int
	Summary             RunSummary
	Err                 error
	Temporary           bool
	ConsecutiveFailures int
}

type Daemon struct {
	coordinator interface {
		RunOnceWithSummary(context.Context) (RunSummary, error)
	}
	cfg         DaemonConfig
}

func NewDaemon(coordinator interface {
	RunOnceWithSummary(context.Context) (RunSummary, error)
}, cfg DaemonConfig) *Daemon {
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = time.Second
	}
	if cfg.FailureBackoff <= 0 {
		cfg.FailureBackoff = 5 * time.Second
	}
	return &Daemon{
		coordinator: coordinator,
		cfg:         cfg,
	}
}

func (d *Daemon) Run(ctx context.Context) error {
	consecutiveFailures := 0

	for iteration := 1; ; iteration++ {
		if err := ctx.Err(); err != nil {
			return nil
		}

		summary, err := d.coordinator.RunOnceWithSummary(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			temporary := isTemporary(err)
			if temporary {
				consecutiveFailures++
			}
			d.emit(DaemonEvent{
				Iteration:           iteration,
				Summary:             summary,
				Err:                 err,
				Temporary:           temporary,
				ConsecutiveFailures: consecutiveFailures,
			})
			if !temporary {
				return err
			}
			if d.cfg.MaxConsecutiveRuns > 0 && iteration >= d.cfg.MaxConsecutiveRuns {
				return nil
			}
			if waitForNextRun(ctx, d.cfg.FailureBackoff) {
				return nil
			}
			continue
		}

		consecutiveFailures = 0
		d.emit(DaemonEvent{
			Iteration: iteration,
			Summary:   summary,
		})
		if d.cfg.MaxConsecutiveRuns > 0 && iteration >= d.cfg.MaxConsecutiveRuns {
			return nil
		}
		if waitForNextRun(ctx, d.cfg.PollInterval) {
			return nil
		}
	}
}

func (d *Daemon) emit(event DaemonEvent) {
	if d.cfg.OnResult != nil {
		d.cfg.OnResult(event)
	}
}

func waitForNextRun(ctx context.Context, delay time.Duration) bool {
	if delay <= 0 {
		select {
		case <-ctx.Done():
			return true
		default:
			return false
		}
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return true
	case <-timer.C:
		return false
	}
}
