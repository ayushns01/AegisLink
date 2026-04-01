package main

import (
	"context"
	"log"

	"github.com/ayushns01/aegislink/relayer/internal/attestations"
	"github.com/ayushns01/aegislink/relayer/internal/config"
	"github.com/ayushns01/aegislink/relayer/internal/cosmos"
	"github.com/ayushns01/aegislink/relayer/internal/evm"
	"github.com/ayushns01/aegislink/relayer/internal/pipeline"
	"github.com/ayushns01/aegislink/relayer/internal/replay"
)

func main() {
	cfg := config.LoadFromEnv()

	coord := pipeline.New(
		cfg,
		replay.NewStoreAt(cfg.ReplayStorePath),
		evm.NewWatcher(evm.NewClient(evm.NewFileLogSource(cfg.EVMStatePath)), cfg.EVMConfirmations),
		attestations.NewCollector(attestations.NewFileVoteSource(cfg.AttestationStatePath), cfg.AttestationThreshold),
		cosmos.NewSubmitter(cosmos.NewFileClaimSink(cfg.CosmosOutboxPath)),
		cosmos.NewWatcher(cosmos.NewClient(cosmos.NewFileWithdrawalSource(cfg.CosmosStatePath)), cfg.CosmosConfirmations),
		evm.NewReleaser(evm.NewFileReleaseTarget(cfg.EVMOutboxPath)),
	)

	if err := coord.RunOnce(context.Background()); err != nil {
		log.Fatal(err)
	}
}
