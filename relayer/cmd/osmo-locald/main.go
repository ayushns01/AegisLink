package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ayushns01/aegislink/relayer/internal/route"
)

const (
	defaultChainID     = "osmo-local-1"
	defaultRuntimeMode = "osmo-local-runtime"
)

type runtimeConfig struct {
	ChainID     string                 `json:"chain_id"`
	RuntimeMode string                 `json:"runtime_mode"`
	HomeDir     string                 `json:"home_dir"`
	ConfigPath  string                 `json:"config_path"`
	StatePath   string                 `json:"state_path"`
	Mode        string                 `json:"mode"`
	Pools       []route.MockTargetPool `json:"pools,omitempty"`
}

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("missing command")
	}

	switch args[0] {
	case "init":
		return runInit(args[1:], stdout)
	case "start":
		return runStart(args[1:], stdout)
	case "query":
		return runQuery(args[1:], stdout)
	case "relay":
		return runRelay(args[1:], stdout)
	case "tx":
		return runTx(args[1:], stdout)
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func runInit(args []string, stdout io.Writer) error {
	flags := flag.NewFlagSet("init", flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	cfg := addRuntimeFlags(flags)
	mode := flags.String("mode", "success", "runtime ack mode")
	poolsJSON := flags.String("pools-json", "", "optional JSON-encoded pool configuration")
	force := flags.Bool("force", false, "overwrite runtime config")
	if err := flags.Parse(args); err != nil {
		return err
	}

	pools, err := parsePoolsJSON(*poolsJSON)
	if err != nil {
		return err
	}

	runtimeCfg := normalizeConfig(runtimeConfig{
		ChainID:     *cfg.chainID,
		RuntimeMode: *cfg.runtimeMode,
		HomeDir:     *cfg.homeDir,
		ConfigPath:  *cfg.configPath,
		StatePath:   *cfg.statePath,
		Mode:        *mode,
		Pools:       pools,
	})

	if err := initHome(runtimeCfg, *force); err != nil {
		return err
	}
	return writeJSON(stdout, map[string]any{
		"status":       "initialized",
		"chain_id":     runtimeCfg.ChainID,
		"runtime_mode": runtimeCfg.RuntimeMode,
		"home_dir":     runtimeCfg.HomeDir,
		"config_path":  runtimeCfg.ConfigPath,
		"state_path":   runtimeCfg.StatePath,
	})
}

func runStart(args []string, stdout io.Writer) error {
	cfg, err := resolveConfigFromArgs("start", args)
	if err != nil {
		return err
	}
	target, err := route.LoadMockTargetRuntime(route.MockTargetConfig{
		Mode:      cfg.Mode,
		StatePath: cfg.StatePath,
		Pools:     cfg.Pools,
	})
	if err != nil {
		return err
	}
	return writeJSON(stdout, statusEnvelope("started", cfg, target.StatusSnapshot()))
}

func runQuery(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("missing query subcommand")
	}

	switch args[0] {
	case "status":
		return queryStatus(args[1:], stdout)
	case "balances":
		return queryBalances(args[1:], stdout)
	case "pools":
		return queryPools(args[1:], stdout)
	case "swaps":
		return querySwaps(args[1:], stdout)
	case "packets":
		return queryPackets(args[1:], stdout)
	case "executions":
		return queryExecutions(args[1:], stdout)
	case "ready-acks":
		return queryReadyAcks(args[1:], stdout)
	case "packet-acks":
		return queryReadyAcks(args[1:], stdout)
	default:
		return fmt.Errorf("unknown query subcommand %q", args[0])
	}
}

func runRelay(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("missing relay subcommand")
	}

	switch args[0] {
	case "recv-packet":
		return relayReceivePacket(args[1:], stdout)
	case "acknowledge-packet":
		return relayAcknowledgePacket(args[1:], stdout)
	default:
		return fmt.Errorf("unknown relay subcommand %q", args[0])
	}
}

func runTx(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("missing tx subcommand")
	}

	switch args[0] {
	case "receive-transfer":
		return txReceiveTransfer(args[1:], stdout)
	case "confirm-ack":
		return txConfirmAck(args[1:], stdout)
	case "complete-ack":
		return txAckAction(args[1:], stdout, "complete")
	case "fail-ack":
		return txAckAction(args[1:], stdout, "fail")
	case "timeout-ack":
		return txAckAction(args[1:], stdout, "timeout")
	default:
		return fmt.Errorf("unknown tx subcommand %q", args[0])
	}
}

type runtimeFlagValues struct {
	chainID     *string
	runtimeMode *string
	homeDir     *string
	configPath  *string
	statePath   *string
}

func addRuntimeFlags(flags *flag.FlagSet) runtimeFlagValues {
	return runtimeFlagValues{
		chainID:     flags.String("chain-id", defaultChainID, "destination chain id"),
		runtimeMode: flags.String("runtime-mode", defaultRuntimeMode, "destination runtime mode"),
		homeDir:     flags.String("home", "", "destination runtime home"),
		configPath:  flags.String("config-path", "", "destination runtime config path"),
		statePath:   flags.String("state-path", "", "destination runtime state path"),
	}
}

func resolveConfigFromArgs(name string, args []string) (runtimeConfig, error) {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	cfg := addRuntimeFlags(flags)
	if err := flags.Parse(args); err != nil {
		return runtimeConfig{}, err
	}
	if strings.TrimSpace(*cfg.homeDir) == "" && strings.TrimSpace(*cfg.configPath) == "" && strings.TrimSpace(*cfg.statePath) == "" {
		*cfg.homeDir = defaultHomeDir()
	}

	resolved := normalizeConfig(runtimeConfig{
		ChainID:     *cfg.chainID,
		RuntimeMode: *cfg.runtimeMode,
		HomeDir:     *cfg.homeDir,
		ConfigPath:  *cfg.configPath,
		StatePath:   *cfg.statePath,
	})
	stored, err := loadConfig(resolved.ConfigPath)
	if err == nil {
		if stored.ChainID != "" {
			resolved = stored
		}
	} else if !os.IsNotExist(err) {
		return runtimeConfig{}, err
	}
	return normalizeConfig(resolved), nil
}

func queryStatus(args []string, stdout io.Writer) error {
	cfg, err := resolveConfigFromArgs("status", args)
	if err != nil {
		return err
	}
	target, err := route.LoadMockTargetRuntime(route.MockTargetConfig{
		Mode:      cfg.Mode,
		StatePath: cfg.StatePath,
		Pools:     cfg.Pools,
	})
	if err != nil {
		return err
	}
	return writeJSON(stdout, statusEnvelope("ok", cfg, target.StatusSnapshot()))
}

func queryBalances(args []string, stdout io.Writer) error {
	target, err := loadRuntimeTarget("balances", args)
	if err != nil {
		return err
	}
	return writeJSON(stdout, target.BalancesSnapshot())
}

func queryPools(args []string, stdout io.Writer) error {
	target, err := loadRuntimeTarget("pools", args)
	if err != nil {
		return err
	}
	return writeJSON(stdout, target.PoolsSnapshot())
}

func querySwaps(args []string, stdout io.Writer) error {
	target, err := loadRuntimeTarget("swaps", args)
	if err != nil {
		return err
	}
	return writeJSON(stdout, target.SwapsSnapshot())
}

func queryPackets(args []string, stdout io.Writer) error {
	target, err := loadRuntimeTarget("packets", args)
	if err != nil {
		return err
	}
	return writeJSON(stdout, target.PacketsSnapshot())
}

func queryExecutions(args []string, stdout io.Writer) error {
	target, err := loadRuntimeTarget("executions", args)
	if err != nil {
		return err
	}
	return writeJSON(stdout, target.ExecutionsSnapshot())
}

func queryReadyAcks(args []string, stdout io.Writer) error {
	target, err := loadRuntimeTarget("ready-acks", args)
	if err != nil {
		return err
	}
	acks, err := target.ReadyAckRecords()
	if err != nil {
		return err
	}
	return writeJSON(stdout, acks)
}

func txReceiveTransfer(args []string, stdout io.Writer) error {
	return relayReceivePacket(args, stdout)
}

func relayReceivePacket(args []string, stdout io.Writer) error {
	flags := flag.NewFlagSet("recv-packet", flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	cfgFlags := addRuntimeFlags(flags)
	transferID := flags.String("transfer-id", "", "transfer identifier")
	_ = flags.Uint64("sequence", 0, "packet sequence")
	_ = flags.String("source-port", "", "source port")
	_ = flags.String("source-channel", "", "source channel")
	_ = flags.String("destination-port", "", "destination port")
	assetID := flags.String("asset-id", "", "asset identifier")
	amount := flags.String("amount", "", "transfer amount")
	receiver := flags.String("receiver", "", "destination receiver")
	destinationChainID := flags.String("destination-chain-id", "", "destination chain id")
	channelID := flags.String("channel-id", "", "channel id")
	destinationDenom := flags.String("destination-denom", "", "destination denom")
	timeoutHeight := flags.Uint64("timeout-height", 0, "timeout height")
	memo := flags.String("memo", "", "memo")
	if err := flags.Parse(args); err != nil {
		return err
	}

	target, err := loadTargetFromFlags(cfgFlags)
	if err != nil {
		return err
	}
	ack, err := target.ReceiveTransfer(route.Transfer{
		TransferID:         strings.TrimSpace(*transferID),
		AssetID:            strings.TrimSpace(*assetID),
		Amount:             strings.TrimSpace(*amount),
		Receiver:           strings.TrimSpace(*receiver),
		DestinationChainID: strings.TrimSpace(*destinationChainID),
		ChannelID:          strings.TrimSpace(*channelID),
		DestinationDenom:   strings.TrimSpace(*destinationDenom),
		TimeoutHeight:      *timeoutHeight,
		Memo:               strings.TrimSpace(*memo),
		Status:             "pending",
	})
	if err != nil {
		return err
	}
	return writeJSON(stdout, ack)
}

func txConfirmAck(args []string, stdout io.Writer) error {
	return relayAcknowledgePacket(args, stdout)
}

func relayAcknowledgePacket(args []string, stdout io.Writer) error {
	flags := flag.NewFlagSet("acknowledge-packet", flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	cfgFlags := addRuntimeFlags(flags)
	transferID := flags.String("transfer-id", "", "transfer identifier")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*transferID) == "" {
		return fmt.Errorf("missing transfer id")
	}

	target, err := loadTargetFromFlags(cfgFlags)
	if err != nil {
		return err
	}
	if err := target.ConfirmReadyAck(*transferID); err != nil {
		return err
	}
	return writeJSON(stdout, map[string]any{"status": "confirmed", "transfer_id": *transferID})
}

func txAckAction(args []string, stdout io.Writer, action string) error {
	flags := flag.NewFlagSet(action+"-ack", flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	cfgFlags := addRuntimeFlags(flags)
	transferID := flags.String("transfer-id", "", "transfer identifier")
	reason := flags.String("reason", "", "ack reason")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*transferID) == "" {
		return fmt.Errorf("missing transfer id")
	}

	target, err := loadTargetFromFlags(cfgFlags)
	if err != nil {
		return err
	}
	switch action {
	case "complete":
		err = target.CompleteAck(*transferID)
	case "fail":
		err = target.FailAck(*transferID, *reason)
	case "timeout":
		err = target.TimeoutAck(*transferID, *reason)
	}
	if err != nil {
		return err
	}
	return writeJSON(stdout, map[string]any{"status": action, "transfer_id": *transferID})
}

func loadRuntimeTarget(name string, args []string) (*route.MockTarget, error) {
	cfg, err := resolveConfigFromArgs(name, args)
	if err != nil {
		return nil, err
	}
	return route.LoadMockTargetRuntime(route.MockTargetConfig{
		Mode:      cfg.Mode,
		StatePath: cfg.StatePath,
		Pools:     cfg.Pools,
	})
}

func loadTargetFromFlags(flags runtimeFlagValues) (*route.MockTarget, error) {
	cfg := normalizeConfig(runtimeConfig{
		ChainID:     *flags.chainID,
		RuntimeMode: *flags.runtimeMode,
		HomeDir:     *flags.homeDir,
		ConfigPath:  *flags.configPath,
		StatePath:   *flags.statePath,
	})
	if stored, err := loadConfig(cfg.ConfigPath); err == nil {
		cfg = normalizeConfig(stored)
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	return route.LoadMockTargetRuntime(route.MockTargetConfig{
		Mode:      cfg.Mode,
		StatePath: cfg.StatePath,
		Pools:     cfg.Pools,
	})
}

func defaultHomeDir() string {
	return filepath.Join(os.TempDir(), "osmo-locald")
}

func defaultConfigPath(home string) string {
	return filepath.Join(home, "config", "runtime.json")
}

func defaultStatePath(home string) string {
	return filepath.Join(home, "data", "state.json")
}

func normalizeConfig(cfg runtimeConfig) runtimeConfig {
	if strings.TrimSpace(cfg.ChainID) == "" {
		cfg.ChainID = defaultChainID
	}
	if strings.TrimSpace(cfg.RuntimeMode) == "" {
		cfg.RuntimeMode = defaultRuntimeMode
	}
	if strings.TrimSpace(cfg.HomeDir) == "" {
		cfg.HomeDir = defaultHomeDir()
	}
	if strings.TrimSpace(cfg.ConfigPath) == "" {
		cfg.ConfigPath = defaultConfigPath(cfg.HomeDir)
	}
	if strings.TrimSpace(cfg.StatePath) == "" {
		cfg.StatePath = defaultStatePath(cfg.HomeDir)
	}
	if strings.TrimSpace(cfg.Mode) == "" {
		cfg.Mode = "success"
	}
	return cfg
}

func initHome(cfg runtimeConfig, force bool) error {
	if !force {
		if _, err := os.Stat(cfg.ConfigPath); err == nil {
			return fmt.Errorf("runtime already initialized")
		}
	}
	if err := os.MkdirAll(filepath.Dir(cfg.ConfigPath), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(cfg.StatePath), 0o755); err != nil {
		return err
	}
	if err := writeConfig(cfg.ConfigPath, cfg); err != nil {
		return err
	}
	_, err := route.LoadMockTargetRuntime(route.MockTargetConfig{
		Mode:      cfg.Mode,
		StatePath: cfg.StatePath,
		Pools:     cfg.Pools,
	})
	return err
}

func loadConfig(path string) (runtimeConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return runtimeConfig{}, err
	}
	var cfg runtimeConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return runtimeConfig{}, err
	}
	return normalizeConfig(cfg), nil
}

func parsePoolsJSON(raw string) ([]route.MockTargetPool, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}

	var pools []route.MockTargetPool
	if err := json.Unmarshal([]byte(raw), &pools); err != nil {
		return nil, fmt.Errorf("decode pools json: %w", err)
	}
	return pools, nil
}

func writeConfig(path string, cfg runtimeConfig) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func statusEnvelope(kind string, cfg runtimeConfig, status route.MockTargetStatus) map[string]any {
	return map[string]any{
		"status":           kind,
		"chain_id":         cfg.ChainID,
		"runtime_mode":     cfg.RuntimeMode,
		"home_dir":         cfg.HomeDir,
		"config_path":      cfg.ConfigPath,
		"state_path":       cfg.StatePath,
		"initialized":      true,
		"packets":          status.Packets,
		"receipts":         status.Receipts,
		"executions":       status.Executions,
		"pools":            status.Pools,
		"balances":         status.Balances,
		"swaps":            status.Swaps,
		"swap_failures":    status.SwapFailures,
		"received_packets": status.ReceivedPackets,
		"executed_packets": status.ExecutedPackets,
		"ready_acks":       status.ReadyAcks,
		"completed_acks":   status.CompletedAcks,
		"failed_acks":      status.FailedAcks,
		"timed_out_acks":   status.TimedOutAcks,
		"relayed_acks":     status.RelayedAcks,
		"pending_receipts": status.PendingReceipts,
	}
}

func writeJSON(w io.Writer, value any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(value)
}
