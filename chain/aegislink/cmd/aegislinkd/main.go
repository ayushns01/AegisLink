package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"math/big"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ayushns01/aegislink/chain/aegislink/app"
	appmetrics "github.com/ayushns01/aegislink/chain/aegislink/internal/metrics"
	"github.com/ayushns01/aegislink/chain/aegislink/internal/opslog"
	"github.com/ayushns01/aegislink/chain/aegislink/networked"
	bridgecli "github.com/ayushns01/aegislink/chain/aegislink/x/bridge/client/cli"
	bridgetypes "github.com/ayushns01/aegislink/chain/aegislink/x/bridge/types"
	governancekeeper "github.com/ayushns01/aegislink/chain/aegislink/x/governance/keeper"
	ibcroutercli "github.com/ayushns01/aegislink/chain/aegislink/x/ibcrouter/client/cli"
	ibcrouterkeeper "github.com/ayushns01/aegislink/chain/aegislink/x/ibcrouter/keeper"
	ibcroutertypes "github.com/ayushns01/aegislink/chain/aegislink/x/ibcrouter/types"
	limittypes "github.com/ayushns01/aegislink/chain/aegislink/x/limits/types"
)

func main() {
	if err := runWithContext(context.Background(), os.Args[1:], os.Stdout, os.Stderr); err != nil {
		_ = opslog.Write(os.Stderr, "error", "aegislinkd", "command_failed", "aegislinkd command failed", map[string]any{
			"error": err.Error(),
		})
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	return runWithContext(context.Background(), args, stdout, stderr)
}

func runWithContext(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		a := app.New()
		_, err := fmt.Fprintf(
			stdout,
			"%s initialized with modules: %s\n",
			a.Config.AppName,
			strings.Join(a.ModuleNames(), ", "),
		)
		return err
	}

	switch args[0] {
	case "init":
		return runInit(args[1:], stdout, stderr)
	case "start":
		return runStart(ctx, args[1:], stdout, stderr)
	case "demo-node":
		return runDemoNode(ctx, args[1:], stdout, stderr)
	case "query":
		return runQuery(args[1:], stdout, stderr)
	case "tx":
		return runTx(args[1:], stdout)
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func runDemoNode(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("missing demo-node subcommand")
	}

	switch args[0] {
	case "start":
		return runDemoNodeStart(ctx, args[1:], stdout, stderr)
	case "status":
		return runDemoNodeStatus(args[1:], stdout, stderr)
	case "balances":
		return runDemoNodeBalances(args[1:], stdout, stderr)
	case "transfers":
		return runDemoNodeTransfers(args[1:], stdout, stderr)
	default:
		return fmt.Errorf("unknown demo-node subcommand %q", args[0])
	}
}

func runQuery(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("missing query subcommand")
	}

	switch args[0] {
	case "status":
		return queryStatus(args[1:], stdout, stderr)
	case "summary":
		return querySummary(args[1:], stdout)
	case "claim":
		return queryClaim(args[1:], stdout)
	case "metrics":
		return queryMetrics(args[1:], stdout)
	case "signer-set":
		return querySignerSet(args[1:], stdout)
	case "signer-sets":
		return querySignerSets(args[1:], stdout)
	case "routes":
		return queryRoutes(args[1:], stdout)
	case "route-profiles":
		return queryRouteProfiles(args[1:], stdout)
	case "transfers":
		return queryTransfers(args[1:], stdout)
	case "withdrawals":
		return queryWithdrawals(args[1:], stdout)
	case "balances":
		return queryBalances(args[1:], stdout)
	default:
		return fmt.Errorf("unknown query subcommand %q", args[0])
	}
}

func runInit(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("init", flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	home := flags.String("home", "", "runtime home directory")
	chainID := flags.String("chain-id", "", "runtime chain id")
	runtimeMode := flags.String("runtime-mode", "", "runtime mode")
	force := flags.Bool("force", false, "overwrite existing runtime artifacts")
	if err := flags.Parse(args); err != nil {
		return err
	}

	cfg, err := app.InitHome(app.Config{
		HomeDir:     *home,
		ChainID:     *chainID,
		RuntimeMode: *runtimeMode,
	}, *force)
	if err != nil {
		return err
	}

	_ = opslog.Write(stderr, "info", "aegislinkd", "runtime_init", "runtime home initialized", map[string]any{
		"chain_id":           cfg.ChainID,
		"home_dir":           cfg.HomeDir,
		"runtime_mode":       cfg.RuntimeMode,
		"module_count":       len(cfg.Modules),
		"configured_signers": len(cfg.AllowedSigners),
		"required_threshold": cfg.RequiredThreshold,
		"config_path":        cfg.ConfigPath,
		"genesis_path":       cfg.GenesisPath,
		"state_path":         cfg.StatePath,
	})

	return writeJSON(stdout, map[string]any{
		"status":       "initialized",
		"app_name":     cfg.AppName,
		"chain_id":     cfg.ChainID,
		"runtime_mode": cfg.RuntimeMode,
		"home_dir":     cfg.HomeDir,
		"config_path":  cfg.ConfigPath,
		"genesis_path": cfg.GenesisPath,
		"state_path":   cfg.StatePath,
	})
}

func runStart(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("start", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	home := flags.String("home", "", "runtime home directory")
	configPath := flags.String("config-path", "", "runtime config path")
	statePath := flags.String("state-path", "", "runtime state path")
	genesisPath := flags.String("genesis-path", "", "runtime genesis path")
	runtimeMode := flags.String("runtime-mode", "", "runtime mode")
	daemon := flags.Bool("daemon", false, "run a block loop instead of a one-shot status summary")
	tickIntervalMS := flags.Uint("tick-interval-ms", 50, "daemon block tick interval in milliseconds")
	maxBlocks := flags.Uint("max-blocks", 0, "maximum blocks to produce in daemon mode")
	if err := flags.Parse(args); err != nil {
		return err
	}
	cfg, err := app.ResolveConfig(app.Config{
		HomeDir:     *home,
		ConfigPath:  *configPath,
		StatePath:   *statePath,
		GenesisPath: *genesisPath,
		RuntimeMode: *runtimeMode,
	})
	if err != nil {
		return err
	}
	if _, err := app.LoadGenesis(cfg.GenesisPath); err != nil {
		return err
	}
	a, err := app.LoadWithConfig(cfg)
	if err != nil {
		return err
	}
	defer closeApp(a)
	status := a.Status()
	if !*daemon {
		_ = opslog.Write(stderr, "info", "aegislinkd", "runtime_start", "runtime started", map[string]any{
			"chain_id":                  status.ChainID,
			"home_dir":                  status.HomeDir,
			"module_count":              status.Modules,
			"configured_signers":        len(status.AllowedSigners),
			"active_signer_set_version": status.ActiveSignerSetVersion,
			"signer_set_count":          status.SignerSetCount,
			"enabled_route_ids":         status.EnabledRouteIDs,
			"current_height":            status.CurrentHeight,
			"daemon":                    false,
		})
		return writeJSON(stdout, statusEnvelope("started", status))
	}

	tickInterval := time.Duration(*tickIntervalMS) * time.Millisecond
	if tickInterval <= 0 {
		tickInterval = 50 * time.Millisecond
	}
	blockCount := uint64(0)
	_ = opslog.Write(stderr, "info", "aegislinkd", "runtime_start", "daemon node loop starting", map[string]any{
		"chain_id":           status.ChainID,
		"home_dir":           status.HomeDir,
		"module_count":       status.Modules,
		"configured_signers": len(status.AllowedSigners),
		"daemon":             true,
		"tick_interval_ms":   tickInterval.Milliseconds(),
		"max_blocks":         *maxBlocks,
	})
	ticker := time.NewTicker(tickInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			finalStatus := a.Status()
			_ = opslog.Write(stderr, "info", "aegislinkd", "runtime_stop", "daemon node loop stopped", map[string]any{
				"chain_id":        finalStatus.ChainID,
				"home_dir":        finalStatus.HomeDir,
				"current_height":  finalStatus.CurrentHeight,
				"produced_blocks": blockCount,
				"pending_claims":  finalStatus.PendingDepositClaims,
				"reason":          "context_cancelled",
			})
			return writeJSON(stdout, withProducedBlocks(statusEnvelope("stopped", finalStatus), blockCount))
		case <-ticker.C:
			progress := a.AdvanceBlock()
			blockCount++
			if err := a.Save(); err != nil {
				return err
			}
			_ = opslog.Write(stderr, "info", "aegislinkd", "block_advanced", "daemon block advanced", map[string]any{
				"height":                  progress.Height,
				"applied_queued_claims":   progress.AppliedQueuedClaims,
				"pending_queued_claims":   progress.PendingQueuedClaims,
				"last_submission_message": progress.LastSubmissionMessage,
			})
			if *maxBlocks > 0 && blockCount >= uint64(*maxBlocks) {
				finalStatus := a.Status()
				_ = opslog.Write(stderr, "info", "aegislinkd", "runtime_stop", "daemon node loop stopped", map[string]any{
					"chain_id":        finalStatus.ChainID,
					"home_dir":        finalStatus.HomeDir,
					"current_height":  finalStatus.CurrentHeight,
					"produced_blocks": blockCount,
					"pending_claims":  finalStatus.PendingDepositClaims,
					"reason":          "max_blocks",
				})
				return writeJSON(stdout, withProducedBlocks(statusEnvelope("stopped", finalStatus), blockCount))
			}
		}
	}
}

func runDemoNodeStart(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("demo-node start", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	home := flags.String("home", "", "runtime home directory")
	rpcAddress := flags.String("rpc-address", "", "demo node RPC address")
	cometRPCAddress := flags.String("comet-rpc-address", "", "demo node Comet RPC address")
	grpcAddress := flags.String("grpc-address", "", "demo node gRPC address")
	abciAddress := flags.String("abci-address", "", "demo node ABCI socket address")
	readyFile := flags.String("ready-file", "", "path to a ready-state file written after startup")
	tickIntervalMS := flags.Uint("tick-interval-ms", 0, "optional demo-node block tick interval in milliseconds")
	if err := flags.Parse(args); err != nil {
		return err
	}

	state, err := networked.Start(ctx, networked.Config{
		HomeDir:         *home,
		RPCAddress:      *rpcAddress,
		CometRPCAddress: *cometRPCAddress,
		GRPCAddress:     *grpcAddress,
		ABCIAddress:     *abciAddress,
		ReadyFile:       *readyFile,
		TickInterval:    time.Duration(*tickIntervalMS) * time.Millisecond,
	})
	if err != nil {
		return err
	}

	_ = opslog.Write(stderr, "info", "aegislinkd", "demo_node_start", "demo node started", map[string]any{
		"chain_id":          state.ChainID,
		"home_dir":          state.HomeDir,
		"rpc_address":       state.RPCAddress,
		"comet_rpc_address": state.CometRPCAddress,
		"grpc_address":      state.GRPCAddress,
		"abci_address":      state.ABCIAddress,
		"ready_file":        strings.TrimSpace(*readyFile),
	})

	<-ctx.Done()

	_ = opslog.Write(stderr, "info", "aegislinkd", "demo_node_stop", "demo node stopped", map[string]any{
		"chain_id":          state.ChainID,
		"home_dir":          state.HomeDir,
		"rpc_address":       state.RPCAddress,
		"comet_rpc_address": state.CometRPCAddress,
		"grpc_address":      state.GRPCAddress,
		"reason":            "context_cancelled",
	})

	return writeJSON(stdout, map[string]any{
		"status":            "stopped",
		"chain_id":          state.ChainID,
		"home_dir":          state.HomeDir,
		"rpc_address":       state.RPCAddress,
		"comet_rpc_address": state.CometRPCAddress,
		"grpc_address":      state.GRPCAddress,
		"abci_address":      state.ABCIAddress,
	})
}

func runDemoNodeStatus(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("demo-node status", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	home := flags.String("home", "", "runtime home directory")
	readyFile := flags.String("ready-file", "", "path to the demo node ready-state file")
	if err := flags.Parse(args); err != nil {
		return err
	}

	status, err := networked.ReadStatus(context.Background(), networked.Config{
		HomeDir:   *home,
		ReadyFile: *readyFile,
	})
	if err != nil {
		return err
	}

	_ = opslog.Write(stderr, "info", "aegislinkd", "demo_node_status", "demo node status queried", map[string]any{
		"chain_id":          status.ChainID,
		"home_dir":          status.HomeDir,
		"rpc_address":       status.RPCAddress,
		"comet_rpc_address": status.CometRPCAddress,
		"grpc_address":      status.GRPCAddress,
		"healthy":           status.Healthy,
	})

	return writeJSON(stdout, status)
}

func runDemoNodeBalances(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("demo-node balances", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	home := flags.String("home", "", "runtime home directory")
	readyFile := flags.String("ready-file", "", "path to the demo node ready-state file")
	address := flags.String("address", "", "bech32 wallet address")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*address) == "" {
		return fmt.Errorf("missing address")
	}

	balances, err := networked.QueryBalances(context.Background(), networked.Config{
		HomeDir:   *home,
		ReadyFile: *readyFile,
	}, *address)
	if err != nil {
		return err
	}

	_ = opslog.Write(stderr, "info", "aegislinkd", "demo_node_balances", "demo node balances queried", map[string]any{
		"home_dir": strings.TrimSpace(*home),
		"address":  strings.TrimSpace(*address),
		"records":  len(balances),
	})
	return writeJSON(stdout, balances)
}

func runDemoNodeTransfers(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("demo-node transfers", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	home := flags.String("home", "", "runtime home directory")
	readyFile := flags.String("ready-file", "", "path to the demo node ready-state file")
	if err := flags.Parse(args); err != nil {
		return err
	}

	transfers, err := networked.QueryTransfers(context.Background(), networked.Config{
		HomeDir:   *home,
		ReadyFile: *readyFile,
	})
	if err != nil {
		return err
	}

	_ = opslog.Write(stderr, "info", "aegislinkd", "demo_node_transfers", "demo node transfers queried", map[string]any{
		"home_dir": strings.TrimSpace(*home),
		"records":  len(transfers),
	})
	return writeJSON(stdout, transfers)
}

func queryStatus(args []string, stdout, stderr io.Writer) error {
	cfg, err := resolveRuntimeConfigFromArgs("status", args)
	if err != nil {
		return err
	}
	a, err := app.LoadWithConfig(cfg)
	if err != nil {
		return err
	}
	status := a.Status()
	_ = opslog.Write(stderr, "info", "aegislinkd", "runtime_status", "runtime status queried", map[string]any{
		"chain_id":                  status.ChainID,
		"home_dir":                  status.HomeDir,
		"module_count":              status.Modules,
		"active_signer_set_version": status.ActiveSignerSetVersion,
		"signer_set_count":          status.SignerSetCount,
		"enabled_route_ids":         status.EnabledRouteIDs,
		"transfers":                 status.Transfers,
		"processed_claims":          status.ProcessedClaims,
		"pending_deposit_claims":    status.PendingDepositClaims,
	})
	return writeJSON(stdout, status)
}

func runTx(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("missing tx subcommand")
	}

	switch args[0] {
	case "submit-deposit-claim":
		return txSubmitDepositClaim(args[1:], stdout)
	case "queue-deposit-claim":
		return txQueueDepositClaim(args[1:], stdout)
	case "execute-withdrawal":
		return txExecuteWithdrawal(args[1:], stdout)
	case "initiate-ibc-transfer":
		return txInitiateIBCTransfer(args[1:], stdout)
	case "fail-ibc-transfer":
		return txFailIBCTransfer(args[1:], stdout)
	case "timeout-ibc-transfer":
		return txTimeoutIBCTransfer(args[1:], stdout)
	case "complete-ibc-transfer":
		return txCompleteIBCTransfer(args[1:], stdout)
	case "refund-ibc-transfer":
		return txRefundIBCTransfer(args[1:], stdout)
	case "apply-asset-status-proposal":
		return txApplyAssetStatusProposal(args[1:], stdout)
	case "apply-limit-update-proposal":
		return txApplyLimitUpdateProposal(args[1:], stdout)
	case "apply-route-policy-update-proposal":
		return txApplyRoutePolicyUpdateProposal(args[1:], stdout)
	default:
		return fmt.Errorf("unknown tx subcommand %q", args[0])
	}
}

func querySummary(args []string, stdout io.Writer) error {
	flags := flag.NewFlagSet("summary", flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	runtimeFlags := addRuntimeFlags(flags)
	if err := flags.Parse(args); err != nil {
		return err
	}

	a, err := loadRuntimeApp(runtimeFlags)
	if err != nil {
		return err
	}
	defer closeApp(a)
	bridgeState := a.BridgeKeeper.ExportState()
	summary := struct {
		AppName       string            `json:"app_name"`
		Modules       []string          `json:"modules"`
		Assets        int               `json:"assets"`
		Limits        int               `json:"limits"`
		PausedFlows   int               `json:"paused_flows"`
		CurrentHeight uint64            `json:"current_height"`
		Withdrawals   int               `json:"withdrawals"`
		SupplyByDenom map[string]string `json:"supply_by_denom"`
	}{
		AppName:       a.Config.AppName,
		Modules:       a.ModuleNames(),
		Assets:        len(a.RegistryKeeper.ExportAssets()),
		Limits:        len(a.LimitsKeeper.ExportLimits()),
		PausedFlows:   len(a.PauserKeeper.ExportPausedFlows()),
		CurrentHeight: bridgeState.CurrentHeight,
		Withdrawals:   len(bridgeState.Withdrawals),
		SupplyByDenom: bridgeState.SupplyByDenom,
	}
	return writeJSON(stdout, summary)
}

func queryClaim(args []string, stdout io.Writer) error {
	flags := flag.NewFlagSet("claim", flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	runtimeFlags := addRuntimeFlags(flags)
	messageID := flags.String("message-id", "", "message id for the processed claim")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*messageID) == "" {
		return fmt.Errorf("missing message id")
	}

	a, err := loadRuntimeApp(runtimeFlags)
	if err != nil {
		return err
	}
	defer closeApp(a)
	service := app.NewBridgeQueryService(a)

	if claim, ok := service.GetClaim(*messageID); ok {
		return writeJSON(stdout, bridgecli.ClaimResponse(claim))
	}

	return fmt.Errorf("claim %q not found", *messageID)
}

func queryMetrics(args []string, stdout io.Writer) error {
	flags := flag.NewFlagSet("metrics", flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	runtimeFlags := addRuntimeFlags(flags)
	if err := flags.Parse(args); err != nil {
		return err
	}

	a, err := loadRuntimeApp(runtimeFlags)
	if err != nil {
		return err
	}
	defer closeApp(a)

	status := a.Status()
	_, err = io.WriteString(stdout, appmetrics.FormatRuntimeSnapshot(appmetrics.RuntimeSnapshot{
		AppName:           status.AppName,
		ChainID:           status.ChainID,
		ProcessedClaims:   uint64(status.ProcessedClaims),
		FailedClaims:      status.FailedClaims,
		PendingTransfers:  status.PendingTransfers,
		TimedOutTransfers: status.TimedOutTransfers,
	}))
	return err
}

func querySignerSet(args []string, stdout io.Writer) error {
	flags := flag.NewFlagSet("signer-set", flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	runtimeFlags := addRuntimeFlags(flags)
	version := flags.Uint64("version", 0, "signer set version, defaults to active")
	if err := flags.Parse(args); err != nil {
		return err
	}

	a, err := loadRuntimeApp(runtimeFlags)
	if err != nil {
		return err
	}
	defer closeApp(a)
	service := app.NewBridgeQueryService(a)

	if *version == 0 {
		signerSet, err := service.ActiveSignerSet()
		if err != nil {
			return err
		}
		return writeJSON(stdout, signerSet)
	}

	signerSet, ok := service.GetSignerSet(*version)
	if !ok {
		return fmt.Errorf("signer set version %d not found", *version)
	}
	return writeJSON(stdout, signerSet)
}

func querySignerSets(args []string, stdout io.Writer) error {
	flags := flag.NewFlagSet("signer-sets", flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	runtimeFlags := addRuntimeFlags(flags)
	if err := flags.Parse(args); err != nil {
		return err
	}

	a, err := loadRuntimeApp(runtimeFlags)
	if err != nil {
		return err
	}
	defer closeApp(a)
	service := app.NewBridgeQueryService(a)
	return writeJSON(stdout, service.ListSignerSets())
}

func queryWithdrawals(args []string, stdout io.Writer) error {
	flags := flag.NewFlagSet("withdrawals", flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	runtimeFlags := addRuntimeFlags(flags)
	fromHeight := flags.Uint64("from-height", 0, "inclusive start height")
	toHeight := flags.Uint64("to-height", math.MaxUint64, "inclusive end height")
	if err := flags.Parse(args); err != nil {
		return err
	}

	a, err := loadRuntimeApp(runtimeFlags)
	if err != nil {
		return err
	}
	defer closeApp(a)
	service := app.NewBridgeQueryService(a)
	withdrawals := service.ListWithdrawals(*fromHeight, *toHeight)
	return writeJSON(stdout, bridgecli.WithdrawalsResponse(withdrawals).Withdrawals)
}

func queryBalances(args []string, stdout io.Writer) error {
	flags := flag.NewFlagSet("balances", flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	runtimeFlags := addRuntimeFlags(flags)
	address := flags.String("address", "", "bech32 wallet address")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*address) == "" {
		return fmt.Errorf("missing address")
	}

	a, err := loadRuntimeApp(runtimeFlags)
	if err != nil {
		return err
	}
	defer closeApp(a)

	service := app.NewBankQueryService(a)
	balances, err := service.ListBalances(*address)
	if err != nil {
		return err
	}
	return writeJSON(stdout, balances)
}

func queryRoutes(args []string, stdout io.Writer) error {
	flags := flag.NewFlagSet("routes", flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	runtimeFlags := addRuntimeFlags(flags)
	if err := flags.Parse(args); err != nil {
		return err
	}

	a, err := loadRuntimeApp(runtimeFlags)
	if err != nil {
		return err
	}
	defer closeApp(a)
	service := app.NewIBCRouterQueryService(a)
	routes := service.ListRoutes()
	sort.Slice(routes, func(i, j int) bool {
		if routes[i].AssetID != routes[j].AssetID {
			return routes[i].AssetID < routes[j].AssetID
		}
		if routes[i].DestinationChainID != routes[j].DestinationChainID {
			return routes[i].DestinationChainID < routes[j].DestinationChainID
		}
		return routes[i].ChannelID < routes[j].ChannelID
	})
	return writeJSON(stdout, ibcroutercli.RoutesResponse(routes).Routes)
}

func queryRouteProfiles(args []string, stdout io.Writer) error {
	flags := flag.NewFlagSet("route-profiles", flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	runtimeFlags := addRuntimeFlags(flags)
	if err := flags.Parse(args); err != nil {
		return err
	}

	a, err := loadRuntimeApp(runtimeFlags)
	if err != nil {
		return err
	}
	defer closeApp(a)

	return writeJSON(stdout, a.RouteProfiles())
}

func queryTransfers(args []string, stdout io.Writer) error {
	flags := flag.NewFlagSet("transfers", flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	runtimeFlags := addRuntimeFlags(flags)
	if err := flags.Parse(args); err != nil {
		return err
	}

	a, err := loadRuntimeApp(runtimeFlags)
	if err != nil {
		return err
	}
	defer closeApp(a)
	service := app.NewIBCRouterQueryService(a)
	transfers := service.ListTransfers()
	sort.Slice(transfers, func(i, j int) bool {
		return transfers[i].TransferID < transfers[j].TransferID
	})

	return writeJSON(stdout, ibcroutercli.TransfersResponse(transfers).Transfers)
}

func txSubmitDepositClaim(args []string, stdout io.Writer) error {
	flags := flag.NewFlagSet("submit-deposit-claim", flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	runtimeFlags := addRuntimeFlags(flags)
	submissionFile := flags.String("submission-file", "", "path to claim+attestation json")
	demoNodeReadyFile := flags.String("demo-node-ready-file", "", "submit to a running demo node via its ready-state file")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*submissionFile) == "" {
		return fmt.Errorf("missing submission file")
	}

	claim, attestation, err := loadSubmission(*submissionFile)
	if err != nil {
		return err
	}
	if strings.TrimSpace(*demoNodeReadyFile) != "" {
		result, err := networked.SubmitDepositClaim(context.Background(), networked.Config{
			HomeDir:   *runtimeFlags.home,
			ReadyFile: *demoNodeReadyFile,
		}, claim, attestation)
		if err != nil {
			return err
		}
		return writeJSON(stdout, bridgecli.SubmitDepositClaimResponse(result))
	}

	a, err := loadRuntimeApp(runtimeFlags)
	if err != nil {
		return err
	}
	defer closeApp(a)
	service := app.NewBridgeTxService(a)
	result, err := service.SubmitDepositClaim(claim, attestation)
	if err != nil {
		return err
	}
	if err := a.Save(); err != nil {
		return err
	}
	return writeJSON(stdout, bridgecli.SubmitDepositClaimResponse(result))
}

func txQueueDepositClaim(args []string, stdout io.Writer) error {
	flags := flag.NewFlagSet("queue-deposit-claim", flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	runtimeFlags := addRuntimeFlags(flags)
	submissionFile := flags.String("submission-file", "", "path to claim+attestation json")
	demoNodeReadyFile := flags.String("demo-node-ready-file", "", "submit to a running demo node via its ready-state file")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*submissionFile) == "" {
		return fmt.Errorf("missing submission file")
	}

	claim, attestation, err := loadSubmission(*submissionFile)
	if err != nil {
		return err
	}
	if strings.TrimSpace(*demoNodeReadyFile) != "" {
		result, err := networked.SubmitQueueDepositClaim(context.Background(), networked.Config{
			HomeDir:   *runtimeFlags.home,
			ReadyFile: *demoNodeReadyFile,
		}, claim, attestation)
		if err != nil {
			return err
		}
		return writeJSON(stdout, result)
	}

	a, err := loadRuntimeApp(runtimeFlags)
	if err != nil {
		return err
	}
	defer closeApp(a)
	service := app.NewBridgeTxService(a)
	if err := service.QueueDepositClaim(claim, attestation); err != nil {
		return err
	}
	if err := a.Save(); err != nil {
		return err
	}
	return writeJSON(stdout, map[string]any{
		"status":     "queued",
		"message_id": claim.Identity.MessageID,
		"asset_id":   claim.AssetID,
		"amount":     claim.Amount.String(),
	})
}

func txExecuteWithdrawal(args []string, stdout io.Writer) error {
	flags := flag.NewFlagSet("execute-withdrawal", flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	runtimeFlags := addRuntimeFlags(flags)
	ownerAddress := flags.String("owner-address", "", "wallet owner bech32 address")
	assetID := flags.String("asset-id", "", "asset identifier to withdraw")
	amountRaw := flags.String("amount", "", "withdrawal amount")
	recipient := flags.String("recipient", "", "ethereum recipient address")
	deadline := flags.Uint64("deadline", 0, "withdrawal expiry")
	signatureBase64 := flags.String("signature-base64", "", "base64-encoded withdrawal attestation")
	height := flags.Uint64("height", 0, "optional runtime block height override")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*ownerAddress) == "" {
		return fmt.Errorf("missing owner address")
	}
	if strings.TrimSpace(*assetID) == "" {
		return fmt.Errorf("missing asset id")
	}
	if strings.TrimSpace(*amountRaw) == "" {
		return fmt.Errorf("missing amount")
	}
	if strings.TrimSpace(*recipient) == "" {
		return fmt.Errorf("missing recipient")
	}
	if *deadline == 0 {
		return fmt.Errorf("missing deadline")
	}
	if strings.TrimSpace(*signatureBase64) == "" {
		return fmt.Errorf("missing signature")
	}

	amount, err := parseBase10Amount(*amountRaw)
	if err != nil {
		return err
	}
	signature, err := base64.StdEncoding.DecodeString(*signatureBase64)
	if err != nil {
		return fmt.Errorf("decode signature: %w", err)
	}

	a, err := loadRuntimeApp(runtimeFlags)
	if err != nil {
		return err
	}
	defer closeApp(a)
	if *height > 0 {
		a.SetCurrentHeight(*height)
	}

	service := app.NewBridgeTxService(a)
	withdrawal, err := service.ExecuteWithdrawal(*ownerAddress, *assetID, amount, *recipient, *deadline, signature)
	if err != nil {
		return err
	}
	if err := a.Save(); err != nil {
		return err
	}

	return writeJSON(stdout, bridgecli.ExecuteWithdrawalResponse(withdrawal))
}

func txInitiateIBCTransfer(args []string, stdout io.Writer) error {
	flags := flag.NewFlagSet("initiate-ibc-transfer", flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	runtimeFlags := addRuntimeFlags(flags)
	demoNodeReadyFile := flags.String("demo-node-ready-file", "", "submit to a running demo node via its ready-state file")
	routeID := flags.String("route-id", "", "route profile identifier")
	assetID := flags.String("asset-id", "", "asset identifier to route")
	amountRaw := flags.String("amount", "", "transfer amount")
	receiver := flags.String("receiver", "", "destination receiver")
	timeoutHeight := flags.Uint64("timeout-height", 0, "ibc timeout height")
	memo := flags.String("memo", "", "optional ibc memo")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*assetID) == "" {
		return fmt.Errorf("missing asset id")
	}
	if strings.TrimSpace(*amountRaw) == "" {
		return fmt.Errorf("missing amount")
	}
	if strings.TrimSpace(*receiver) == "" {
		return fmt.Errorf("missing receiver")
	}
	if *timeoutHeight == 0 {
		return fmt.Errorf("missing timeout height")
	}

	amount, err := parseBase10Amount(*amountRaw)
	if err != nil {
		return err
	}
	if strings.TrimSpace(*demoNodeReadyFile) != "" {
		transfer, err := networked.SubmitInitiateIBCTransfer(context.Background(), networked.Config{
			HomeDir:   *runtimeFlags.home,
			ReadyFile: *demoNodeReadyFile,
		}, networked.InitiateIBCTransferPayload{
			RouteID:       *routeID,
			AssetID:       *assetID,
			Amount:        amount.String(),
			Receiver:      *receiver,
			TimeoutHeight: *timeoutHeight,
			Memo:          *memo,
		})
		if err != nil {
			return err
		}
		return writeJSON(stdout, transfer)
	}
	a, err := loadRuntimeApp(runtimeFlags)
	if err != nil {
		return err
	}
	defer closeApp(a)
	var transfer ibcrouterkeeper.TransferRecord
	if strings.TrimSpace(*routeID) != "" {
		transfer, err = a.InitiateIBCTransferWithProfile(*routeID, *assetID, amount, *receiver, *timeoutHeight, *memo)
	} else {
		transfer, err = a.InitiateIBCTransfer(*assetID, amount, *receiver, *timeoutHeight, *memo)
	}
	if err != nil {
		return err
	}
	if err := a.Save(); err != nil {
		return err
	}
	return writeJSON(stdout, transferJSONResponse(transfer))
}

func txFailIBCTransfer(args []string, stdout io.Writer) error {
	flags := flag.NewFlagSet("fail-ibc-transfer", flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	runtimeFlags := addRuntimeFlags(flags)
	transferID := flags.String("transfer-id", "", "transfer identifier")
	reason := flags.String("reason", "", "ack failure reason")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*transferID) == "" {
		return fmt.Errorf("missing transfer id")
	}

	a, err := loadRuntimeApp(runtimeFlags)
	if err != nil {
		return err
	}
	defer closeApp(a)
	transfer, err := a.FailIBCTransfer(*transferID, *reason)
	if err != nil {
		return err
	}
	if err := a.Save(); err != nil {
		return err
	}
	return writeJSON(stdout, transferJSONResponse(transfer))
}

func txTimeoutIBCTransfer(args []string, stdout io.Writer) error {
	flags := flag.NewFlagSet("timeout-ibc-transfer", flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	runtimeFlags := addRuntimeFlags(flags)
	transferID := flags.String("transfer-id", "", "transfer identifier")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*transferID) == "" {
		return fmt.Errorf("missing transfer id")
	}

	a, err := loadRuntimeApp(runtimeFlags)
	if err != nil {
		return err
	}
	defer closeApp(a)
	transfer, err := a.TimeoutIBCTransfer(*transferID)
	if err != nil {
		return err
	}
	if err := a.Save(); err != nil {
		return err
	}
	return writeJSON(stdout, transferJSONResponse(transfer))
}

func txCompleteIBCTransfer(args []string, stdout io.Writer) error {
	flags := flag.NewFlagSet("complete-ibc-transfer", flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	runtimeFlags := addRuntimeFlags(flags)
	transferID := flags.String("transfer-id", "", "transfer identifier")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*transferID) == "" {
		return fmt.Errorf("missing transfer id")
	}

	a, err := loadRuntimeApp(runtimeFlags)
	if err != nil {
		return err
	}
	defer closeApp(a)
	transfer, err := a.CompleteIBCTransfer(*transferID)
	if err != nil {
		return err
	}
	if err := a.Save(); err != nil {
		return err
	}
	return writeJSON(stdout, transferJSONResponse(transfer))
}

func txRefundIBCTransfer(args []string, stdout io.Writer) error {
	flags := flag.NewFlagSet("refund-ibc-transfer", flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	runtimeFlags := addRuntimeFlags(flags)
	transferID := flags.String("transfer-id", "", "transfer identifier")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*transferID) == "" {
		return fmt.Errorf("missing transfer id")
	}

	a, err := loadRuntimeApp(runtimeFlags)
	if err != nil {
		return err
	}
	defer closeApp(a)
	transfer, err := a.RefundIBCTransfer(*transferID)
	if err != nil {
		return err
	}
	if err := a.Save(); err != nil {
		return err
	}
	return writeJSON(stdout, transferJSONResponse(transfer))
}

func txApplyAssetStatusProposal(args []string, stdout io.Writer) error {
	flags := flag.NewFlagSet("apply-asset-status-proposal", flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	runtimeFlags := addRuntimeFlags(flags)
	proposalID := flags.String("proposal-id", "", "proposal identifier")
	assetID := flags.String("asset-id", "", "asset identifier")
	enabled := flags.Bool("enabled", true, "desired asset enabled status")
	authority := flags.String("authority", "", "governance authority identifier")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*proposalID) == "" {
		return fmt.Errorf("missing proposal id")
	}
	if strings.TrimSpace(*assetID) == "" {
		return fmt.Errorf("missing asset id")
	}
	if strings.TrimSpace(*authority) == "" {
		return fmt.Errorf("missing authority")
	}

	a, err := loadRuntimeApp(runtimeFlags)
	if err != nil {
		return err
	}
	defer closeApp(a)

	service := app.NewGovernanceTxService(a)
	proposal := governancekeeper.AssetStatusProposal{
		ProposalID: *proposalID,
		AssetID:    *assetID,
		Enabled:    *enabled,
	}
	if err := service.ApplyAssetStatusProposal(*authority, proposal); err != nil {
		return err
	}
	if err := a.Save(); err != nil {
		return err
	}
	return writeJSON(stdout, map[string]any{
		"proposal_id": proposal.ProposalID,
		"kind":        governancekeeper.ProposalKindAssetStatus,
		"target_id":   proposal.AssetID,
		"enabled":     proposal.Enabled,
		"applied_by":  strings.TrimSpace(*authority),
	})
}

func txApplyLimitUpdateProposal(args []string, stdout io.Writer) error {
	flags := flag.NewFlagSet("apply-limit-update-proposal", flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	runtimeFlags := addRuntimeFlags(flags)
	proposalID := flags.String("proposal-id", "", "proposal identifier")
	assetID := flags.String("asset-id", "", "asset identifier")
	windowSeconds := flags.Uint64("window-seconds", 0, "rate limit window seconds")
	maxAmountRaw := flags.String("max-amount", "", "rate limit max amount")
	authority := flags.String("authority", "", "governance authority identifier")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*proposalID) == "" {
		return fmt.Errorf("missing proposal id")
	}
	if strings.TrimSpace(*assetID) == "" {
		return fmt.Errorf("missing asset id")
	}
	if *windowSeconds == 0 {
		return fmt.Errorf("missing window seconds")
	}
	if strings.TrimSpace(*maxAmountRaw) == "" {
		return fmt.Errorf("missing max amount")
	}
	if strings.TrimSpace(*authority) == "" {
		return fmt.Errorf("missing authority")
	}

	maxAmount, err := parseBase10Amount(*maxAmountRaw)
	if err != nil {
		return err
	}
	a, err := loadRuntimeApp(runtimeFlags)
	if err != nil {
		return err
	}
	defer closeApp(a)

	service := app.NewGovernanceTxService(a)
	proposal := governancekeeper.LimitUpdateProposal{
		ProposalID: *proposalID,
		Limit: limittypes.RateLimit{
			AssetID:       *assetID,
			WindowSeconds: *windowSeconds,
			MaxAmount:     maxAmount,
		},
	}
	if err := service.ApplyLimitUpdateProposal(*authority, proposal); err != nil {
		return err
	}
	if err := a.Save(); err != nil {
		return err
	}
	return writeJSON(stdout, map[string]any{
		"proposal_id":    proposal.ProposalID,
		"kind":           governancekeeper.ProposalKindLimitUpdate,
		"target_id":      proposal.Limit.AssetID,
		"window_seconds": proposal.Limit.WindowSeconds,
		"max_amount":     proposal.Limit.MaxAmount.String(),
		"applied_by":     strings.TrimSpace(*authority),
	})
}

func txApplyRoutePolicyUpdateProposal(args []string, stdout io.Writer) error {
	flags := flag.NewFlagSet("apply-route-policy-update-proposal", flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	runtimeFlags := addRuntimeFlags(flags)
	proposalID := flags.String("proposal-id", "", "proposal identifier")
	routeID := flags.String("route-id", "", "route profile identifier")
	memoPrefixes := flags.String("memo-prefixes", "", "comma-separated allowed memo prefixes")
	actionTypes := flags.String("action-types", "", "comma-separated allowed action types")
	authority := flags.String("authority", "", "governance authority identifier")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*proposalID) == "" {
		return fmt.Errorf("missing proposal id")
	}
	if strings.TrimSpace(*routeID) == "" {
		return fmt.Errorf("missing route id")
	}
	if strings.TrimSpace(*authority) == "" {
		return fmt.Errorf("missing authority")
	}

	a, err := loadRuntimeApp(runtimeFlags)
	if err != nil {
		return err
	}
	defer closeApp(a)

	service := app.NewGovernanceTxService(a)
	proposal := governancekeeper.RoutePolicyUpdateProposal{
		ProposalID: *proposalID,
		RouteID:    *routeID,
		Policy: ibcroutertypes.RoutePolicy{
			AllowedMemoPrefixes: splitCommaSeparated(*memoPrefixes),
			AllowedActionTypes:  splitCommaSeparated(*actionTypes),
		},
	}
	if err := service.ApplyRoutePolicyUpdateProposal(*authority, proposal); err != nil {
		return err
	}
	if err := a.Save(); err != nil {
		return err
	}
	return writeJSON(stdout, map[string]any{
		"proposal_id":           proposal.ProposalID,
		"kind":                  governancekeeper.ProposalKindRoutePolicy,
		"target_id":             proposal.RouteID,
		"allowed_memo_prefixes": proposal.Policy.AllowedMemoPrefixes,
		"allowed_action_types":  proposal.Policy.AllowedActionTypes,
		"applied_by":            strings.TrimSpace(*authority),
	})
}

type submissionFilePayload struct {
	Claim struct {
		Kind               string `json:"kind"`
		SourceAssetKind    string `json:"source_asset_kind,omitempty"`
		SourceChainID      string `json:"source_chain_id"`
		SourceContract     string `json:"source_contract"`
		SourceTxHash       string `json:"source_tx_hash"`
		SourceLogIndex     uint64 `json:"source_log_index"`
		Nonce              uint64 `json:"nonce"`
		MessageID          string `json:"message_id"`
		DestinationChainID string `json:"destination_chain_id"`
		AssetID            string `json:"asset_id"`
		Amount             string `json:"amount"`
		Recipient          string `json:"recipient"`
		Deadline           uint64 `json:"deadline"`
	} `json:"claim"`
	Attestation struct {
		MessageID        string                         `json:"message_id"`
		PayloadHash      string                         `json:"payload_hash"`
		Signers          []string                       `json:"signers"`
		Proofs           []bridgetypes.AttestationProof `json:"proofs"`
		Threshold        uint32                         `json:"threshold"`
		Expiry           uint64                         `json:"expiry"`
		SignerSetVersion uint64                         `json:"signer_set_version"`
	} `json:"attestation"`
}

func loadSubmission(path string) (bridgetypes.DepositClaim, bridgetypes.Attestation, error) {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return bridgetypes.DepositClaim{}, bridgetypes.Attestation{}, err
	}

	var payload submissionFilePayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return bridgetypes.DepositClaim{}, bridgetypes.Attestation{}, err
	}
	amount, ok := new(big.Int).SetString(payload.Claim.Amount, 10)
	if !ok {
		return bridgetypes.DepositClaim{}, bridgetypes.Attestation{}, fmt.Errorf("invalid claim amount %q", payload.Claim.Amount)
	}

	claim := bridgetypes.DepositClaim{
		Identity: bridgetypes.ClaimIdentity{
			Kind:            bridgetypes.ClaimKind(payload.Claim.Kind),
			SourceAssetKind: payload.Claim.SourceAssetKind,
			SourceChainID:   payload.Claim.SourceChainID,
			SourceContract:  payload.Claim.SourceContract,
			SourceTxHash:    payload.Claim.SourceTxHash,
			SourceLogIndex:  payload.Claim.SourceLogIndex,
			Nonce:           payload.Claim.Nonce,
			MessageID:       payload.Claim.MessageID,
		},
		DestinationChainID: payload.Claim.DestinationChainID,
		AssetID:            payload.Claim.AssetID,
		Amount:             amount,
		Recipient:          payload.Claim.Recipient,
		Deadline:           payload.Claim.Deadline,
	}
	attestation := bridgetypes.Attestation{
		MessageID:        payload.Attestation.MessageID,
		PayloadHash:      payload.Attestation.PayloadHash,
		Signers:          append([]string(nil), payload.Attestation.Signers...),
		Proofs:           append([]bridgetypes.AttestationProof(nil), payload.Attestation.Proofs...),
		Threshold:        payload.Attestation.Threshold,
		Expiry:           payload.Attestation.Expiry,
		SignerSetVersion: payload.Attestation.SignerSetVersion,
	}
	return claim, attestation, nil
}

func parseBase10Amount(raw string) (*big.Int, error) {
	amount, ok := new(big.Int).SetString(strings.TrimSpace(raw), 10)
	if !ok {
		return nil, fmt.Errorf("invalid amount %q", raw)
	}
	return amount, nil
}

func transferJSONResponse(transfer ibcrouterkeeper.TransferRecord) struct {
	TransferID         string `json:"transfer_id"`
	AssetID            string `json:"asset_id"`
	Amount             string `json:"amount"`
	Receiver           string `json:"receiver"`
	DestinationChainID string `json:"destination_chain_id"`
	ChannelID          string `json:"channel_id"`
	DestinationDenom   string `json:"destination_denom"`
	TimeoutHeight      uint64 `json:"timeout_height"`
	Memo               string `json:"memo"`
	Status             string `json:"status"`
	FailureReason      string `json:"failure_reason"`
} {
	return struct {
		TransferID         string `json:"transfer_id"`
		AssetID            string `json:"asset_id"`
		Amount             string `json:"amount"`
		Receiver           string `json:"receiver"`
		DestinationChainID string `json:"destination_chain_id"`
		ChannelID          string `json:"channel_id"`
		DestinationDenom   string `json:"destination_denom"`
		TimeoutHeight      uint64 `json:"timeout_height"`
		Memo               string `json:"memo"`
		Status             string `json:"status"`
		FailureReason      string `json:"failure_reason"`
	}{
		TransferID:         transfer.TransferID,
		AssetID:            transfer.AssetID,
		Amount:             transfer.Amount.String(),
		Receiver:           transfer.Receiver,
		DestinationChainID: transfer.DestinationChainID,
		ChannelID:          transfer.ChannelID,
		DestinationDenom:   transfer.DestinationDenom,
		TimeoutHeight:      transfer.TimeoutHeight,
		Memo:               transfer.Memo,
		Status:             string(transfer.Status),
		FailureReason:      transfer.FailureReason,
	}
}

func writeJSON(stdout io.Writer, value any) error {
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func resolveRuntimeConfigFromArgs(name string, args []string) (app.Config, error) {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	home := flags.String("home", "", "runtime home directory")
	configPath := flags.String("config-path", "", "runtime config path")
	statePath := flags.String("state-path", "", "runtime state path")
	genesisPath := flags.String("genesis-path", "", "runtime genesis path")
	runtimeMode := flags.String("runtime-mode", "", "runtime mode")
	if err := flags.Parse(args); err != nil {
		return app.Config{}, err
	}

	return app.ResolveConfig(app.Config{
		HomeDir:     *home,
		ConfigPath:  *configPath,
		StatePath:   *statePath,
		GenesisPath: *genesisPath,
		RuntimeMode: *runtimeMode,
	})
}

type runtimeFlagSet struct {
	home        *string
	configPath  *string
	statePath   *string
	genesisPath *string
	runtimeMode *string
}

func addRuntimeFlags(flags *flag.FlagSet) runtimeFlagSet {
	return runtimeFlagSet{
		home:        flags.String("home", "", "runtime home directory"),
		configPath:  flags.String("config-path", "", "runtime config path"),
		statePath:   flags.String("state-path", "", "runtime state path"),
		genesisPath: flags.String("genesis-path", "", "runtime genesis path"),
		runtimeMode: flags.String("runtime-mode", "", "runtime mode"),
	}
}

func loadRuntimeApp(flags runtimeFlagSet) (*app.App, error) {
	cfg, err := app.ResolveConfig(app.Config{
		HomeDir:     *flags.home,
		ConfigPath:  *flags.configPath,
		StatePath:   *flags.statePath,
		GenesisPath: *flags.genesisPath,
		RuntimeMode: *flags.runtimeMode,
	})
	if err != nil {
		return nil, err
	}
	return app.LoadWithConfig(cfg)
}

func closeApp(a *app.App) {
	if a == nil {
		return
	}
	_ = a.Close()
}

func statusEnvelope(kind string, status app.Status) map[string]any {
	return map[string]any{
		"status":                    kind,
		"app_name":                  status.AppName,
		"chain_id":                  status.ChainID,
		"runtime_mode":              status.RuntimeMode,
		"home_dir":                  status.HomeDir,
		"config_path":               status.ConfigPath,
		"genesis_path":              status.GenesisPath,
		"state_path":                status.StatePath,
		"initialized":               status.Initialized,
		"modules":                   status.Modules,
		"module_names":              status.ModuleNames,
		"allowed_signers":           status.AllowedSigners,
		"governance_authorities":    status.GovernanceAuthorities,
		"active_signer_set_version": status.ActiveSignerSetVersion,
		"active_signer_threshold":   status.ActiveSignerThreshold,
		"signer_set_count":          status.SignerSetCount,
		"signer_set_versions":       status.SignerSetVersions,
		"required_threshold":        status.RequiredThreshold,
		"current_height":            status.CurrentHeight,
		"assets":                    status.Assets,
		"limits":                    status.Limits,
		"paused_flows":              status.PausedFlows,
		"processed_claims":          status.ProcessedClaims,
		"pending_deposit_claims":    status.PendingDepositClaims,
		"failed_claims":             status.FailedClaims,
		"withdrawals":               status.Withdrawals,
		"routes":                    status.Routes,
		"transfers":                 status.Transfers,
		"pending_transfers":         status.PendingTransfers,
		"completed_transfers":       status.CompletedTransfers,
		"failed_transfers":          status.FailedTransfers,
		"timed_out_transfers":       status.TimedOutTransfers,
		"refunded_transfers":        status.RefundedTransfers,
		"supply_by_denom":           status.SupplyByDenom,
	}
}

func withProducedBlocks(status map[string]any, produced uint64) map[string]any {
	status["produced_blocks"] = produced
	return status
}

func splitCommaSeparated(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		values = append(values, part)
	}
	if len(values) == 0 {
		return nil
	}
	return values
}
