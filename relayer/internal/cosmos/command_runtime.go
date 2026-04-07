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
	args := []string{
		"query", "withdrawals",
		"--state-path", s.statePath,
		"--from-height", strconv.FormatUint(fromHeight, 10),
		"--to-height", strconv.FormatUint(toHeight, 10),
	}
	payload, err := s.run(ctx, s.command, append(append([]string(nil), s.baseArgs...), args...)...)
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
	args := append(append([]string(nil), s.baseArgs...),
		"query", subcommand,
		"--state-path", s.statePath,
	)
	return s.run(ctx, s.command, args...)
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

	args := append(append([]string(nil), s.baseArgs...),
		"tx", "submit-deposit-claim",
		"--state-path", s.statePath,
		"--submission-file", submissionPath,
	)
	_, err = s.run(ctx, s.command, args...)
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
