package cosmos

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	bridgetypes "github.com/ayushns01/aegislink/chain/aegislink/x/bridge/types"
)

type commandRunner func(context.Context, string, ...string) ([]byte, error)

type CommandWithdrawalSource struct {
	run       commandRunner
	command   string
	baseArgs  []string
	statePath string
}

type CommandClaimSink struct {
	run       commandRunner
	command   string
	baseArgs  []string
	statePath string
}

func NewCommandWithdrawalSource(command string, baseArgs []string, statePath string) *CommandWithdrawalSource {
	return newCommandWithdrawalSourceWithRunner(runCommand, command, baseArgs, statePath)
}

func NewCommandClaimSink(command string, baseArgs []string, statePath string) *CommandClaimSink {
	return newCommandClaimSinkWithRunner(runCommand, command, baseArgs, statePath)
}

func newCommandWithdrawalSourceWithRunner(run commandRunner, command string, baseArgs []string, statePath string) *CommandWithdrawalSource {
	return &CommandWithdrawalSource{
		run:       run,
		command:   command,
		baseArgs:  append([]string(nil), baseArgs...),
		statePath: statePath,
	}
}

func newCommandClaimSinkWithRunner(run commandRunner, command string, baseArgs []string, statePath string) *CommandClaimSink {
	return &CommandClaimSink{
		run:       run,
		command:   command,
		baseArgs:  append([]string(nil), baseArgs...),
		statePath: statePath,
	}
}

func (s *CommandWithdrawalSource) LatestHeight(ctx context.Context) (uint64, error) {
	payload, err := s.runQuery(ctx, "summary")
	if err != nil {
		return 0, err
	}

	var summary struct {
		CurrentHeight uint64 `json:"current_height"`
	}
	if err := json.Unmarshal(payload, &summary); err != nil {
		return 0, err
	}
	return summary.CurrentHeight, nil
}

func (s *CommandWithdrawalSource) Withdrawals(ctx context.Context, fromHeight, toHeight uint64) ([]Withdrawal, error) {
	commandArgs, runtimeArgs := splitRuntimeArgs(s.baseArgs)
	args := []string{
		"query", "withdrawals",
		"--from-height", strconv.FormatUint(fromHeight, 10),
		"--to-height", strconv.FormatUint(toHeight, 10),
	}
	args = appendRuntimeArgs(args, runtimeArgs, s.statePath)
	payload, err := s.run(ctx, s.command, append(commandArgs, args...)...)
	if err != nil {
		return nil, err
	}

	var encoded []persistedWithdrawal
	if err := json.Unmarshal(payload, &encoded); err != nil {
		return nil, err
	}

	withdrawals := make([]Withdrawal, 0, len(encoded))
	for _, withdrawal := range encoded {
		decoded, err := decodePersistedWithdrawal(withdrawal)
		if err != nil {
			return nil, err
		}
		withdrawals = append(withdrawals, decoded)
	}
	return withdrawals, nil
}

func (s *CommandWithdrawalSource) runQuery(ctx context.Context, subcommand string) ([]byte, error) {
	commandArgs, runtimeArgs := splitRuntimeArgs(s.baseArgs)
	args := []string{
		"query", subcommand,
	}
	args = appendRuntimeArgs(args, runtimeArgs, s.statePath)
	return s.run(ctx, s.command, append(commandArgs, args...)...)
}

func (s *CommandClaimSink) SubmitDepositClaim(ctx context.Context, claim bridgetypes.DepositClaim, attestation bridgetypes.Attestation) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	tmpDir, err := os.MkdirTemp("", "aegislink-command-submission-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	submissionPath := filepath.Join(tmpDir, "submission.json")
	payload := persistedClaimSubmission{
		Claim: persistedDepositClaim{
			Kind:               string(claim.Identity.Kind),
			SourceAssetKind:    claim.Identity.SourceAssetKind,
			SourceChainID:      claim.Identity.SourceChainID,
			SourceContract:     claim.Identity.SourceContract,
			SourceTxHash:       claim.Identity.SourceTxHash,
			SourceLogIndex:     claim.Identity.SourceLogIndex,
			Nonce:              claim.Identity.Nonce,
			MessageID:          claim.Identity.MessageID,
			DestinationChainID: claim.DestinationChainID,
			AssetID:            claim.AssetID,
			Amount:             claim.Amount.String(),
			Recipient:          claim.Recipient,
			Deadline:           claim.Deadline,
		},
		Attestation: persistedAttestation{
			MessageID:        attestation.MessageID,
			PayloadHash:      attestation.PayloadHash,
			Signers:          append([]string(nil), attestation.Signers...),
			Proofs:           append([]bridgetypes.AttestationProof(nil), attestation.Proofs...),
			Threshold:        attestation.Threshold,
			Expiry:           attestation.Expiry,
			SignerSetVersion: attestation.SignerSetVersion,
		},
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if err := os.WriteFile(submissionPath, encoded, 0o644); err != nil {
		return err
	}

	commandArgs, runtimeArgs := splitRuntimeArgs(s.baseArgs)
	args := []string{
		"tx", "submit-deposit-claim",
		"--submission-file", submissionPath,
	}
	args = appendRuntimeArgs(args, runtimeArgs, s.statePath)
	_, err = s.run(ctx, s.command, append(commandArgs, args...)...)
	return err
}

func splitRuntimeArgs(baseArgs []string) ([]string, []string) {
	commandArgs := make([]string, 0, len(baseArgs))
	runtimeArgs := make([]string, 0, 6)
	for i := 0; i < len(baseArgs); i++ {
		arg := strings.TrimSpace(baseArgs[i])
		if runtimeFlagArity(arg) == 1 && i+1 < len(baseArgs) {
			runtimeArgs = append(runtimeArgs, baseArgs[i], baseArgs[i+1])
			i++
			continue
		}
		commandArgs = append(commandArgs, baseArgs[i])
	}
	return commandArgs, runtimeArgs
}

func appendRuntimeArgs(args, runtimeArgs []string, statePath string) []string {
	args = append(args, runtimeArgs...)
	if strings.TrimSpace(statePath) == "" || commandArgsContainFlag(runtimeArgs, "--home") || commandArgsContainFlag(runtimeArgs, "--state-path") {
		return args
	}
	return append(args, "--state-path", strings.TrimSpace(statePath))
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

func runtimeFlagArity(arg string) int {
	switch strings.TrimSpace(arg) {
	case "--home", "--config-path", "--state-path", "--genesis-path", "--runtime-mode":
		return 1
	default:
		return 0
	}
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

func decodePersistedWithdrawal(withdrawal persistedWithdrawal) (Withdrawal, error) {
	amount, ok := new(big.Int).SetString(withdrawal.Amount, 10)
	if !ok {
		return Withdrawal{}, fmt.Errorf("invalid withdrawal amount %q", withdrawal.Amount)
	}
	signatureBase64 := withdrawal.Signature
	if signatureBase64 == "" {
		signatureBase64 = withdrawal.SignatureBase64
	}
	signature, err := base64.StdEncoding.DecodeString(signatureBase64)
	if err != nil {
		return Withdrawal{}, fmt.Errorf("decode withdrawal signature: %w", err)
	}
	return Withdrawal{
		BlockHeight: withdrawal.BlockHeight,
		Identity: bridgetypes.ClaimIdentity{
			Kind:           bridgetypes.ClaimKind(withdrawal.Kind),
			SourceChainID:  withdrawal.SourceChainID,
			SourceContract: withdrawal.SourceContract,
			SourceTxHash:   withdrawal.SourceTxHash,
			SourceLogIndex: withdrawal.SourceLogIndex,
			Nonce:          withdrawal.Nonce,
			MessageID:      withdrawal.MessageID,
		},
		AssetID:      withdrawal.AssetID,
		AssetAddress: withdrawal.AssetAddress,
		Amount:       amount,
		Recipient:    withdrawal.Recipient,
		Deadline:     withdrawal.Deadline,
		Signature:    signature,
	}, nil
}
