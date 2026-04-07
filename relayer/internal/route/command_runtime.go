package route

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

type commandRunner func(context.Context, string, ...string) ([]byte, error)

type RuntimeLocator struct {
	Home        string
	StatePath   string
	RuntimeMode string
}

type CommandTransferSource struct {
	run       commandRunner
	command   string
	baseArgs  []string
	locator   RuntimeLocator
}

type CommandAckSink struct {
	run       commandRunner
	command   string
	baseArgs  []string
	locator   RuntimeLocator
}

func NewCommandTransferSource(command string, baseArgs []string, locator RuntimeLocator) *CommandTransferSource {
	return &CommandTransferSource{
		run:       runCommand,
		command:   command,
		baseArgs:  append([]string(nil), baseArgs...),
		locator:   locator,
	}
}

func NewCommandAckSink(command string, baseArgs []string, locator RuntimeLocator) *CommandAckSink {
	return &CommandAckSink{
		run:       runCommand,
		command:   command,
		baseArgs:  append([]string(nil), baseArgs...),
		locator:   locator,
	}
}

func (s *CommandTransferSource) PendingTransfers(ctx context.Context) ([]Transfer, error) {
	args := append(append([]string(nil), s.baseArgs...),
		"query", "transfers",
	)
	args = appendRuntimeLocatorArgs(args, s.locator)
	output, err := s.run(ctx, s.command, args...)
	if err != nil {
		return nil, err
	}

	var transfers []Transfer
	if err := json.Unmarshal(output, &transfers); err != nil {
		return nil, err
	}

	pending := make([]Transfer, 0, len(transfers))
	for _, transfer := range transfers {
		if strings.TrimSpace(transfer.Status) == "pending" {
			pending = append(pending, transfer)
		}
	}
	return pending, nil
}

func (s *CommandAckSink) CompleteTransfer(ctx context.Context, transferID string) error {
	return s.runTx(ctx, "complete-ibc-transfer", transferID, "")
}

func (s *CommandAckSink) FailTransfer(ctx context.Context, transferID, reason string) error {
	return s.runTx(ctx, "fail-ibc-transfer", transferID, reason)
}

func (s *CommandAckSink) TimeoutTransfer(ctx context.Context, transferID string) error {
	return s.runTx(ctx, "timeout-ibc-transfer", transferID, "")
}

func (s *CommandAckSink) runTx(ctx context.Context, subcommand, transferID, reason string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	args := append(append([]string(nil), s.baseArgs...),
		"tx", subcommand,
		"--transfer-id", strings.TrimSpace(transferID),
	)
	args = appendRuntimeLocatorArgs(args, s.locator)
	if strings.TrimSpace(reason) != "" {
		args = append(args, "--reason", strings.TrimSpace(reason))
	}
	_, err := s.run(ctx, s.command, args...)
	return err
}

func runCommand(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if len(output) == 0 {
			return nil, err
		}
		return nil, fmt.Errorf("%w: %s", err, output)
	}
	return output, nil
}

func newCommandTransferSourceWithRunner(run commandRunner, command string, baseArgs []string, locator RuntimeLocator) *CommandTransferSource {
	return &CommandTransferSource{
		run:       run,
		command:   command,
		baseArgs:  append([]string(nil), baseArgs...),
		locator:   locator,
	}
}

func newCommandAckSinkWithRunner(run commandRunner, command string, baseArgs []string, locator RuntimeLocator) *CommandAckSink {
	return &CommandAckSink{
		run:       run,
		command:   command,
		baseArgs:  append([]string(nil), baseArgs...),
		locator:   locator,
	}
}

var errUnexpectedCommand = errors.New("unexpected command")

func appendRuntimeLocatorArgs(args []string, locator RuntimeLocator) []string {
	if home := strings.TrimSpace(locator.Home); home != "" {
		args = append(args, "--home", home)
	}
	if statePath := strings.TrimSpace(locator.StatePath); statePath != "" {
		args = append(args, "--state-path", statePath)
	}
	if runtimeMode := strings.TrimSpace(locator.RuntimeMode); runtimeMode != "" {
		args = append(args, "--runtime-mode", runtimeMode)
	}
	return args
}
