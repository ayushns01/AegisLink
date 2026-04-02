package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"strings"

	"github.com/ayushns01/aegislink/chain/aegislink/app"
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
		MessageID    string `json:"message_id"`
		AssetID      string `json:"asset_id"`
		AssetAddress string `json:"asset_address"`
		Amount       string `json:"amount"`
		Recipient    string `json:"recipient"`
		Deadline     uint64 `json:"deadline"`
		BlockHeight  uint64 `json:"block_height"`
	}, 0, len(withdrawals))
	for _, withdrawal := range withdrawals {
		response = append(response, struct {
			MessageID    string `json:"message_id"`
			AssetID      string `json:"asset_id"`
			AssetAddress string `json:"asset_address"`
			Amount       string `json:"amount"`
			Recipient    string `json:"recipient"`
			Deadline     uint64 `json:"deadline"`
			BlockHeight  uint64 `json:"block_height"`
		}{
			MessageID:    withdrawal.Identity.MessageID,
			AssetID:      withdrawal.AssetID,
			AssetAddress: withdrawal.AssetAddress,
			Amount:       withdrawal.Amount.String(),
			Recipient:    withdrawal.Recipient,
			Deadline:     withdrawal.Deadline,
			BlockHeight:  withdrawal.BlockHeight,
		})
	}
	return writeJSON(stdout, response)
}

func writeJSON(stdout io.Writer, value any) error {
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}
