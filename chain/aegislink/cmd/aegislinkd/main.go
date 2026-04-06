package main

import (
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

	"github.com/ayushns01/aegislink/chain/aegislink/app"
	"github.com/ayushns01/aegislink/chain/aegislink/internal/opslog"
	bridgetypes "github.com/ayushns01/aegislink/chain/aegislink/x/bridge/types"
	ibcrouterkeeper "github.com/ayushns01/aegislink/chain/aegislink/x/ibcrouter/keeper"
)

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		_ = opslog.Write(os.Stderr, "error", "aegislinkd", "command_failed", "aegislinkd command failed", map[string]any{
			"error": err.Error(),
		})
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
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
		return runStart(args[1:], stdout, stderr)
	case "query":
		return runQuery(args[1:], stdout, stderr)
	case "tx":
		return runTx(args[1:], stdout)
	default:
		return fmt.Errorf("unknown command %q", args[0])
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
	case "routes":
		return queryRoutes(args[1:], stdout)
	case "transfers":
		return queryTransfers(args[1:], stdout)
	case "withdrawals":
		return queryWithdrawals(args[1:], stdout)
	default:
		return fmt.Errorf("unknown query subcommand %q", args[0])
	}
}

func runInit(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("init", flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	home := flags.String("home", "", "runtime home directory")
	chainID := flags.String("chain-id", "", "runtime chain id")
	force := flags.Bool("force", false, "overwrite existing runtime artifacts")
	if err := flags.Parse(args); err != nil {
		return err
	}

	cfg, err := app.InitHome(app.Config{
		HomeDir: *home,
		ChainID: *chainID,
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

func runStart(args []string, stdout, stderr io.Writer) error {
	cfg, err := resolveRuntimeConfigFromArgs("start", args)
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
	status := a.Status()
	_ = opslog.Write(stderr, "info", "aegislinkd", "runtime_start", "runtime started", map[string]any{
		"chain_id":           status.ChainID,
		"home_dir":           status.HomeDir,
		"module_count":       status.Modules,
		"configured_signers": len(status.AllowedSigners),
		"enabled_route_ids":  status.EnabledRouteIDs,
		"current_height":     status.CurrentHeight,
	})
	return writeJSON(stdout, statusEnvelope("started", status))
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
		"chain_id":          status.ChainID,
		"home_dir":          status.HomeDir,
		"module_count":      status.Modules,
		"enabled_route_ids": status.EnabledRouteIDs,
		"transfers":         status.Transfers,
		"processed_claims":  status.ProcessedClaims,
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
	default:
		return fmt.Errorf("unknown tx subcommand %q", args[0])
	}
}

func querySummary(args []string, stdout io.Writer) error {
	flags := flag.NewFlagSet("summary", flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	statePath := flags.String("state-path", "", "path to persisted app state")
	if err := flags.Parse(args); err != nil {
		return err
	}

	a, err := app.Load(*statePath)
	if err != nil {
		return err
	}
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

	statePath := flags.String("state-path", "", "path to persisted app state")
	messageID := flags.String("message-id", "", "message id for the processed claim")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*messageID) == "" {
		return fmt.Errorf("missing message id")
	}

	a, err := app.Load(*statePath)
	if err != nil {
		return err
	}

	for _, claim := range a.BridgeKeeper.ExportState().ProcessedClaims {
		if claim.MessageID != *messageID {
			continue
		}
		return writeJSON(stdout, struct {
			ClaimKey  string `json:"claim_key"`
			MessageID string `json:"message_id"`
			Denom     string `json:"denom"`
			AssetID   string `json:"asset_id"`
			Amount    string `json:"amount"`
			Status    string `json:"status"`
		}{
			ClaimKey:  claim.ClaimKey,
			MessageID: claim.MessageID,
			Denom:     claim.Denom,
			AssetID:   claim.AssetID,
			Amount:    claim.Amount,
			Status:    string(claim.Status),
		})
	}

	return fmt.Errorf("claim %q not found", *messageID)
}

func queryWithdrawals(args []string, stdout io.Writer) error {
	flags := flag.NewFlagSet("withdrawals", flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	statePath := flags.String("state-path", "", "path to persisted app state")
	fromHeight := flags.Uint64("from-height", 0, "inclusive start height")
	toHeight := flags.Uint64("to-height", math.MaxUint64, "inclusive end height")
	if err := flags.Parse(args); err != nil {
		return err
	}

	a, err := app.Load(*statePath)
	if err != nil {
		return err
	}
	withdrawals := a.Withdrawals(*fromHeight, *toHeight)
	response := make([]struct {
		Kind           string `json:"kind"`
		SourceChainID  string `json:"source_chain_id"`
		SourceContract string `json:"source_contract"`
		SourceTxHash   string `json:"source_tx_hash"`
		SourceLogIndex uint64 `json:"source_log_index"`
		Nonce          uint64 `json:"nonce"`
		MessageID      string `json:"message_id"`
		AssetID        string `json:"asset_id"`
		AssetAddress   string `json:"asset_address"`
		Amount         string `json:"amount"`
		Recipient      string `json:"recipient"`
		Deadline       uint64 `json:"deadline"`
		BlockHeight    uint64 `json:"block_height"`
		Signature      string `json:"signature"`
	}, 0, len(withdrawals))
	for _, withdrawal := range withdrawals {
		response = append(response, struct {
			Kind           string `json:"kind"`
			SourceChainID  string `json:"source_chain_id"`
			SourceContract string `json:"source_contract"`
			SourceTxHash   string `json:"source_tx_hash"`
			SourceLogIndex uint64 `json:"source_log_index"`
			Nonce          uint64 `json:"nonce"`
			MessageID      string `json:"message_id"`
			AssetID        string `json:"asset_id"`
			AssetAddress   string `json:"asset_address"`
			Amount         string `json:"amount"`
			Recipient      string `json:"recipient"`
			Deadline       uint64 `json:"deadline"`
			BlockHeight    uint64 `json:"block_height"`
			Signature      string `json:"signature"`
		}{
			Kind:           string(withdrawal.Identity.Kind),
			SourceChainID:  withdrawal.Identity.SourceChainID,
			SourceContract: withdrawal.Identity.SourceContract,
			SourceTxHash:   withdrawal.Identity.SourceTxHash,
			SourceLogIndex: withdrawal.Identity.SourceLogIndex,
			Nonce:          withdrawal.Identity.Nonce,
			MessageID:      withdrawal.Identity.MessageID,
			AssetID:        withdrawal.AssetID,
			AssetAddress:   withdrawal.AssetAddress,
			Amount:         withdrawal.Amount.String(),
			Recipient:      withdrawal.Recipient,
			Deadline:       withdrawal.Deadline,
			BlockHeight:    withdrawal.BlockHeight,
			Signature:      base64.StdEncoding.EncodeToString(withdrawal.Signature),
		})
	}
	return writeJSON(stdout, response)
}

func queryRoutes(args []string, stdout io.Writer) error {
	flags := flag.NewFlagSet("routes", flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	statePath := flags.String("state-path", "", "path to persisted app state")
	if err := flags.Parse(args); err != nil {
		return err
	}

	a, err := app.Load(*statePath)
	if err != nil {
		return err
	}
	routes := a.Routes()
	sort.Slice(routes, func(i, j int) bool {
		if routes[i].AssetID != routes[j].AssetID {
			return routes[i].AssetID < routes[j].AssetID
		}
		if routes[i].DestinationChainID != routes[j].DestinationChainID {
			return routes[i].DestinationChainID < routes[j].DestinationChainID
		}
		return routes[i].ChannelID < routes[j].ChannelID
	})
	return writeJSON(stdout, routes)
}

func queryTransfers(args []string, stdout io.Writer) error {
	flags := flag.NewFlagSet("transfers", flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	statePath := flags.String("state-path", "", "path to persisted app state")
	if err := flags.Parse(args); err != nil {
		return err
	}

	a, err := app.Load(*statePath)
	if err != nil {
		return err
	}
	transfers := a.Transfers()
	sort.Slice(transfers, func(i, j int) bool {
		return transfers[i].TransferID < transfers[j].TransferID
	})

	response := make([]struct {
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
	}, 0, len(transfers))
	for _, transfer := range transfers {
		response = append(response, struct {
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
		})
	}
	return writeJSON(stdout, response)
}

func txSubmitDepositClaim(args []string, stdout io.Writer) error {
	flags := flag.NewFlagSet("submit-deposit-claim", flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	statePath := flags.String("state-path", "", "path to persisted app state")
	submissionFile := flags.String("submission-file", "", "path to claim+attestation json")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*submissionFile) == "" {
		return fmt.Errorf("missing submission file")
	}

	a, err := app.Load(*statePath)
	if err != nil {
		return err
	}
	claim, attestation, err := loadSubmission(*submissionFile)
	if err != nil {
		return err
	}
	result, err := a.SubmitDepositClaim(claim, attestation)
	if err != nil {
		return err
	}
	if err := a.Save(); err != nil {
		return err
	}
	return writeJSON(stdout, result)
}

func txExecuteWithdrawal(args []string, stdout io.Writer) error {
	flags := flag.NewFlagSet("execute-withdrawal", flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	statePath := flags.String("state-path", "", "path to persisted app state")
	assetID := flags.String("asset-id", "", "asset identifier to withdraw")
	amountRaw := flags.String("amount", "", "withdrawal amount")
	recipient := flags.String("recipient", "", "ethereum recipient address")
	deadline := flags.Uint64("deadline", 0, "withdrawal expiry")
	signatureBase64 := flags.String("signature-base64", "", "base64-encoded withdrawal attestation")
	height := flags.Uint64("height", 0, "optional runtime block height override")
	if err := flags.Parse(args); err != nil {
		return err
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

	a, err := app.Load(*statePath)
	if err != nil {
		return err
	}
	if *height > 0 {
		a.SetCurrentHeight(*height)
	}

	withdrawal, err := a.ExecuteWithdrawal(*assetID, amount, *recipient, *deadline, signature)
	if err != nil {
		return err
	}
	if err := a.Save(); err != nil {
		return err
	}

	return writeJSON(stdout, struct {
		Kind           string `json:"kind"`
		SourceChainID  string `json:"source_chain_id"`
		SourceContract string `json:"source_contract"`
		SourceTxHash   string `json:"source_tx_hash"`
		SourceLogIndex uint64 `json:"source_log_index"`
		Nonce          uint64 `json:"nonce"`
		MessageID      string `json:"message_id"`
		AssetID        string `json:"asset_id"`
		AssetAddress   string `json:"asset_address"`
		Amount         string `json:"amount"`
		Recipient      string `json:"recipient"`
		Deadline       uint64 `json:"deadline"`
		BlockHeight    uint64 `json:"block_height"`
		Signature      string `json:"signature"`
	}{
		Kind:           string(withdrawal.Identity.Kind),
		SourceChainID:  withdrawal.Identity.SourceChainID,
		SourceContract: withdrawal.Identity.SourceContract,
		SourceTxHash:   withdrawal.Identity.SourceTxHash,
		SourceLogIndex: withdrawal.Identity.SourceLogIndex,
		Nonce:          withdrawal.Identity.Nonce,
		MessageID:      withdrawal.Identity.MessageID,
		AssetID:        withdrawal.AssetID,
		AssetAddress:   withdrawal.AssetAddress,
		Amount:         withdrawal.Amount.String(),
		Recipient:      withdrawal.Recipient,
		Deadline:       withdrawal.Deadline,
		BlockHeight:    withdrawal.BlockHeight,
		Signature:      base64.StdEncoding.EncodeToString(withdrawal.Signature),
	})
}

func txInitiateIBCTransfer(args []string, stdout io.Writer) error {
	flags := flag.NewFlagSet("initiate-ibc-transfer", flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	statePath := flags.String("state-path", "", "path to persisted app state")
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
	a, err := app.Load(*statePath)
	if err != nil {
		return err
	}
	transfer, err := a.InitiateIBCTransfer(*assetID, amount, *receiver, *timeoutHeight, *memo)
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

	statePath := flags.String("state-path", "", "path to persisted app state")
	transferID := flags.String("transfer-id", "", "transfer identifier")
	reason := flags.String("reason", "", "ack failure reason")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*transferID) == "" {
		return fmt.Errorf("missing transfer id")
	}

	a, err := app.Load(*statePath)
	if err != nil {
		return err
	}
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

	statePath := flags.String("state-path", "", "path to persisted app state")
	transferID := flags.String("transfer-id", "", "transfer identifier")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*transferID) == "" {
		return fmt.Errorf("missing transfer id")
	}

	a, err := app.Load(*statePath)
	if err != nil {
		return err
	}
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

	statePath := flags.String("state-path", "", "path to persisted app state")
	transferID := flags.String("transfer-id", "", "transfer identifier")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*transferID) == "" {
		return fmt.Errorf("missing transfer id")
	}

	a, err := app.Load(*statePath)
	if err != nil {
		return err
	}
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

	statePath := flags.String("state-path", "", "path to persisted app state")
	transferID := flags.String("transfer-id", "", "transfer identifier")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*transferID) == "" {
		return fmt.Errorf("missing transfer id")
	}

	a, err := app.Load(*statePath)
	if err != nil {
		return err
	}
	transfer, err := a.RefundIBCTransfer(*transferID)
	if err != nil {
		return err
	}
	if err := a.Save(); err != nil {
		return err
	}
	return writeJSON(stdout, transferJSONResponse(transfer))
}

type submissionFilePayload struct {
	Claim struct {
		Kind               string `json:"kind"`
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
		MessageID   string   `json:"message_id"`
		PayloadHash string   `json:"payload_hash"`
		Signers     []string `json:"signers"`
		Threshold   uint32   `json:"threshold"`
		Expiry      uint64   `json:"expiry"`
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
			Kind:           bridgetypes.ClaimKind(payload.Claim.Kind),
			SourceChainID:  payload.Claim.SourceChainID,
			SourceContract: payload.Claim.SourceContract,
			SourceTxHash:   payload.Claim.SourceTxHash,
			SourceLogIndex: payload.Claim.SourceLogIndex,
			Nonce:          payload.Claim.Nonce,
			MessageID:      payload.Claim.MessageID,
		},
		DestinationChainID: payload.Claim.DestinationChainID,
		AssetID:            payload.Claim.AssetID,
		Amount:             amount,
		Recipient:          payload.Claim.Recipient,
		Deadline:           payload.Claim.Deadline,
	}
	attestation := bridgetypes.Attestation{
		MessageID:   payload.Attestation.MessageID,
		PayloadHash: payload.Attestation.PayloadHash,
		Signers:     append([]string(nil), payload.Attestation.Signers...),
		Threshold:   payload.Attestation.Threshold,
		Expiry:      payload.Attestation.Expiry,
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
	if err := flags.Parse(args); err != nil {
		return app.Config{}, err
	}

	return app.ResolveConfig(app.Config{
		HomeDir:     *home,
		ConfigPath:  *configPath,
		StatePath:   *statePath,
		GenesisPath: *genesisPath,
	})
}

func statusEnvelope(kind string, status app.Status) map[string]any {
	return map[string]any{
		"status":              kind,
		"app_name":            status.AppName,
		"chain_id":            status.ChainID,
		"runtime_mode":        status.RuntimeMode,
		"home_dir":            status.HomeDir,
		"config_path":         status.ConfigPath,
		"genesis_path":        status.GenesisPath,
		"state_path":          status.StatePath,
		"initialized":         status.Initialized,
		"modules":             status.Modules,
		"module_names":        status.ModuleNames,
		"allowed_signers":     status.AllowedSigners,
		"required_threshold":  status.RequiredThreshold,
		"current_height":      status.CurrentHeight,
		"assets":              status.Assets,
		"limits":              status.Limits,
		"paused_flows":        status.PausedFlows,
		"processed_claims":    status.ProcessedClaims,
		"withdrawals":         status.Withdrawals,
		"routes":              status.Routes,
		"transfers":           status.Transfers,
		"pending_transfers":   status.PendingTransfers,
		"completed_transfers": status.CompletedTransfers,
		"failed_transfers":    status.FailedTransfers,
		"timed_out_transfers": status.TimedOutTransfers,
		"refunded_transfers":  status.RefundedTransfers,
		"supply_by_denom":     status.SupplyByDenom,
	}
}
