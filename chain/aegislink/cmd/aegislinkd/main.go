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
	"strings"

	"github.com/ayushns01/aegislink/chain/aegislink/app"
	bridgetypes "github.com/ayushns01/aegislink/chain/aegislink/x/bridge/types"
)

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
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
	case "query":
		return runQuery(args[1:], stdout)
	case "tx":
		return runTx(args[1:], stdout)
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func runQuery(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("missing query subcommand")
	}

	switch args[0] {
	case "summary":
		return querySummary(args[1:], stdout)
	case "withdrawals":
		return queryWithdrawals(args[1:], stdout)
	default:
		return fmt.Errorf("unknown query subcommand %q", args[0])
	}
}

func runTx(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("missing tx subcommand")
	}

	switch args[0] {
	case "submit-deposit-claim":
		return txSubmitDepositClaim(args[1:], stdout)
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

func writeJSON(stdout io.Writer, value any) error {
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}
