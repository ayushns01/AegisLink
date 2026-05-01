package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ayushns01/aegislink/chain/aegislink/networked"
	"github.com/ayushns01/aegislink/relayer/internal/attestations"
	"github.com/ayushns01/aegislink/relayer/internal/autodelivery"
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
	AutoDeliveryEnabled         bool
	AutoDeliveryRelayerCommand  string
	AutoDeliveryRelayerHome     string
	AutoDeliveryPathName        string
	AutoDeliveryPathByRoute     map[string]string
	AutoDeliveryTimeoutHeight   uint64
	AttestationThreshold        uint32
	AttestationSignerSetVersion uint64
	AttestationSignerKeys       []string
}

const autoDeliveryTimeoutHeightBuffer uint64 = 1000

var latestLCDHeightFunc = fetchLatestLCDHeight

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
	combined, err := buildCombinedCoordinator(cfg, coord)
	if err != nil {
		return err
	}

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
		daemon := pipeline.NewDaemon(combined, pipeline.DaemonConfig{
			PollInterval:       cfg.PollInterval,
			FailureBackoff:     cfg.FailureBackoff,
			MaxConsecutiveRuns: cfg.MaxRuns,
			OnResult: func(event pipeline.DaemonEvent) {
				logPublicBridgeOutcome(stdout, stderr, event.Summary, event.Err, event.ConsecutiveFailures)
			},
		})
		return daemon.Run(ctx)
	}

	summary, err := combined.RunOnceWithSummary(ctx)
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
		AutoDeliveryEnabled:         autoDeliveryEnabled(),
		AutoDeliveryRelayerCommand:  strings.TrimSpace(os.Getenv("AEGISLINK_RELAYER_RLY_CMD")),
		AutoDeliveryRelayerHome:     strings.TrimSpace(os.Getenv("AEGISLINK_RELAYER_RLY_HOME")),
		AutoDeliveryPathName:        strings.TrimSpace(os.Getenv("AEGISLINK_RELAYER_RLY_PATH_NAME")),
		AutoDeliveryPathByRoute:     parseRlyPathMap(os.Getenv("AEGISLINK_RELAYER_RLY_PATH_MAP")),
		AutoDeliveryTimeoutHeight:   loadAutoDeliveryTimeoutHeight(),
		AttestationThreshold:        cfg.AttestationThreshold,
		AttestationSignerSetVersion: cfg.AttestationSignerSetVersion,
		AttestationSignerKeys:       append([]string(nil), cfg.AttestationSignerKeys...),
	}
	if public.AutoDeliveryRelayerCommand == "" {
		public.AutoDeliveryRelayerCommand = "./bin/relayer"
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
	if cfg.AttestationThreshold == 0 {
		return fmt.Errorf("attestation threshold must be greater than zero")
	}
	if len(cfg.AttestationSignerKeys) < int(cfg.AttestationThreshold) {
		return fmt.Errorf(
			"configured attestation signer keys (%d) do not satisfy threshold %d",
			len(cfg.AttestationSignerKeys),
			cfg.AttestationThreshold,
		)
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

func autoDeliveryEnabled() bool {
	value := strings.TrimSpace(os.Getenv("AEGISLINK_RELAYER_AUTODELIVERY_ENABLED"))
	if value != "" {
		return value == "1" || strings.EqualFold(value, "true")
	}
	return strings.TrimSpace(os.Getenv("AEGISLINK_RELAYER_RLY_HOME")) != ""
}

func loadAutoDeliveryTimeoutHeight() uint64 {
	value := strings.TrimSpace(os.Getenv("AEGISLINK_RELAYER_IBC_TIMEOUT_HEIGHT"))
	configured, hasConfigured := parseConfiguredAutoDeliveryTimeoutHeight(value)
	destinationLCDBaseURL := strings.TrimSpace(os.Getenv("AEGISLINK_RELAYER_DESTINATION_LCD_BASE_URL"))
	if destinationLCDBaseURL != "" {
		latestHeight, err := latestLCDHeightFunc(destinationLCDBaseURL)
		if err == nil {
			minimumSafeHeight := latestHeight + autoDeliveryTimeoutHeightBuffer
			if !hasConfigured || configured < minimumSafeHeight {
				return minimumSafeHeight
			}
		}
	}
	if hasConfigured {
		return configured
	}
	return 120
}

func parseConfiguredAutoDeliveryTimeoutHeight(value string) (uint64, bool) {
	if value == "" {
		return 0, false
	}
	parsed, err := strconv.ParseUint(strings.TrimSpace(value), 10, 64)
	if err != nil || parsed == 0 {
		return 0, false
	}
	return parsed, true
}

// parseRlyPathMap parses "routeID:pathName,routeID:pathName" into a map.
func parseRlyPathMap(raw string) map[string]string {
	result := make(map[string]string)
	for _, entry := range strings.Split(strings.TrimSpace(raw), ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		routeID, pathName, ok := strings.Cut(entry, ":")
		if !ok {
			continue
		}
		routeID = strings.TrimSpace(routeID)
		pathName = strings.TrimSpace(pathName)
		if routeID != "" && pathName != "" {
			result[routeID] = pathName
		}
	}
	return result
}

func fetchLatestLCDHeight(baseURL string) (uint64, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return 0, fmt.Errorf("missing destination lcd base url")
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, baseURL+"/cosmos/base/tendermint/v1beta1/blocks/latest", nil)
	if err != nil {
		return 0, err
	}
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("destination lcd latest block request failed with %s", resp.Status)
	}

	var payload struct {
		Block struct {
			Header struct {
				Height string `json:"height"`
			} `json:"header"`
		} `json:"block"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return 0, err
	}
	height, err := strconv.ParseUint(strings.TrimSpace(payload.Block.Header.Height), 10, 64)
	if err != nil || height == 0 {
		return 0, fmt.Errorf("invalid destination lcd latest height %q", payload.Block.Header.Height)
	}
	return height, nil
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
		"auto_delivery_intents":       summary.AutoDeliveryIntents,
		"auto_delivery_waiting":       summary.AutoDeliveryWaiting,
		"auto_transfers_initiated":    summary.AutoTransfersInitiated,
		"auto_flushes_triggered":      summary.AutoFlushesTriggered,
		"auto_completed_deliveries":   summary.AutoCompletedDeliveries,
		"auto_failed_deliveries":      summary.AutoFailedDeliveries,
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

type combinedCoordinator struct {
	bridge *pipeline.Coordinator
	auto   *autodelivery.Coordinator
}

func buildCombinedCoordinator(cfg publicBridgeConfig, bridge *pipeline.Coordinator) (*combinedCoordinator, error) {
	auto, err := buildAutoDeliveryCoordinator(cfg)
	if err != nil {
		return nil, err
	}
	return &combinedCoordinator{
		bridge: bridge,
		auto:   auto,
	}, nil
}

func (c *combinedCoordinator) RunOnceWithSummary(ctx context.Context) (pipeline.RunSummary, error) {
	summary, err := c.bridge.RunOnceWithSummary(ctx)
	if err != nil || c.auto == nil {
		return summary, err
	}

	autoSummary, err := c.auto.RunOnce(ctx)
	summary.AutoDeliveryIntents = autoSummary.IntentsObserved
	summary.AutoDeliveryWaiting = autoSummary.IntentsWaiting
	summary.AutoTransfersInitiated = autoSummary.TransfersInitiated
	summary.AutoFlushesTriggered = autoSummary.FlushesTriggered
	summary.AutoCompletedDeliveries = autoSummary.CompletedDeliveries
	summary.AutoFailedDeliveries = autoSummary.FailedDeliveries
	return summary, err
}

func buildAutoDeliveryCoordinator(cfg publicBridgeConfig) (*autodelivery.Coordinator, error) {
	if !cfg.AutoDeliveryEnabled {
		return nil, nil
	}
	nodeConfig, err := resolveAutoDeliveryNodeConfig(cfg.AegisLinkCommandArgs)
	if err != nil {
		return nil, err
	}
	return autodelivery.NewCoordinator(
		autodelivery.NetworkedIntentSource{Config: nodeConfig},
		autodelivery.NetworkedStatusSource{Config: nodeConfig},
		autodelivery.NetworkedTransferSubmitter{
			Config:        nodeConfig,
			TimeoutHeight: cfg.AutoDeliveryTimeoutHeight,
		},
		autodelivery.RlyFlusher{
			Command:     cfg.AutoDeliveryRelayerCommand,
			PathByRoute: cfg.AutoDeliveryPathByRoute,
			DefaultPath: cfg.AutoDeliveryPathName,
			Home:        cfg.AutoDeliveryRelayerHome,
		},
	), nil
}

func resolveAutoDeliveryNodeConfig(args []string) (networked.Config, error) {
	var cfg networked.Config
	for index := 0; index < len(args); index++ {
		switch strings.TrimSpace(args[index]) {
		case "--home":
			if index+1 < len(args) {
				cfg.HomeDir = strings.TrimSpace(args[index+1])
				index++
			}
		case "--demo-node-ready-file":
			if index+1 < len(args) {
				cfg.ReadyFile = strings.TrimSpace(args[index+1])
				index++
			}
		}
	}
	if strings.TrimSpace(cfg.HomeDir) == "" && strings.TrimSpace(cfg.ReadyFile) == "" {
		return networked.Config{}, fmt.Errorf("autodelivery requires --home or --demo-node-ready-file in AEGISLINK_RELAYER_AEGISLINK_CMD_ARGS")
	}
	return cfg, nil
}
