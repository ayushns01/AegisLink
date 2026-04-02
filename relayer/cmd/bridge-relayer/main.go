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
		evm.NewWatcher(evm.NewClient(evm.NewFileLogSource(cfg.EVMStatePath)), cfg.EVMConfirmations),
		attestations.NewCollector(attestations.NewFileVoteSource(cfg.AttestationStatePath), cfg.AttestationThreshold),
		cosmos.NewSubmitter(claimSink),
		cosmos.NewWatcher(cosmos.NewClient(withdrawalSource), cfg.CosmosConfirmations),
		evm.NewReleaser(evm.NewFileReleaseTarget(cfg.EVMOutboxPath)),
	)

	if err := coord.RunOnce(context.Background()); err != nil {
		log.Fatal(err)
	}
}
