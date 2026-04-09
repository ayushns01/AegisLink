package e2e

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestRealABCIChain(t *testing.T) {
	t.Parallel()

	homeDir := filepath.Join(t.TempDir(), "real-abci-home")
	bootstrapRealChain(t, homeDir)
	seedRealChainRuntime(t, homeDir)

	claim := validRuntimeClaim(t)
	submissionPath := filepath.Join(t.TempDir(), "submission.json")
	writeRuntimeSubmissionFile(t, submissionPath, claim)

	queueOutput := runGoCommand(
		t,
		filepath.Join(repoRoot(t), "chain", "aegislink"),
		nil,
		"run",
		"./cmd/aegislinkd",
		"tx",
		"queue-deposit-claim",
		"--home",
		homeDir,
		"--submission-file",
		submissionPath,
	)
	var queued struct {
		Status    string `json:"status"`
		MessageID string `json:"message_id"`
	}
	if err := decodeLastJSONObject(queueOutput, &queued); err != nil {
		t.Fatalf("decode queued output: %v\n%s", err, queueOutput)
	}
	if queued.Status != "queued" {
		t.Fatalf("expected queued status, got %+v", queued)
	}
	if queued.MessageID != claim.Identity.MessageID {
		t.Fatalf("expected queued message id %q, got %q", claim.Identity.MessageID, queued.MessageID)
	}

	startOutput := runGoCommand(
		t,
		filepath.Join(repoRoot(t), "chain", "aegislink"),
		nil,
		"run",
		"./cmd/aegislinkd",
		"start",
		"--home",
		homeDir,
		"--daemon",
		"--tick-interval-ms",
		"1",
		"--max-blocks",
		"2",
	)

	var stopped struct {
		Status               string `json:"status"`
		CurrentHeight        uint64 `json:"current_height"`
		PendingDepositClaims int    `json:"pending_deposit_claims"`
		ProcessedClaims      int    `json:"processed_claims"`
		ProducedBlocks       uint64 `json:"produced_blocks"`
	}
	if err := decodeLastJSONObject(startOutput, &stopped); err != nil {
		t.Fatalf("decode stopped output: %v\n%s", err, startOutput)
	}
	if stopped.Status != "stopped" {
		t.Fatalf("expected stopped daemon status, got %+v", stopped)
	}
	if stopped.CurrentHeight < 52 {
		t.Fatalf("expected daemon to advance height beyond seed height, got %d", stopped.CurrentHeight)
	}
	if stopped.PendingDepositClaims != 0 {
		t.Fatalf("expected queued claims to be drained, got %d", stopped.PendingDepositClaims)
	}
	if stopped.ProcessedClaims != 1 {
		t.Fatalf("expected one processed claim, got %d", stopped.ProcessedClaims)
	}
	if stopped.ProducedBlocks != 2 {
		t.Fatalf("expected two produced blocks, got %d", stopped.ProducedBlocks)
	}
	if !strings.Contains(startOutput, "runtime_stop") {
		t.Fatalf("expected daemon shutdown log in output\n%s", startOutput)
	}

	statusOutput := runGoCommand(
		t,
		filepath.Join(repoRoot(t), "chain", "aegislink"),
		nil,
		"run",
		"./cmd/aegislinkd",
		"query",
		"status",
		"--home",
		homeDir,
	)

	var status struct {
		CurrentHeight        uint64 `json:"current_height"`
		PendingDepositClaims int    `json:"pending_deposit_claims"`
		ProcessedClaims      int    `json:"processed_claims"`
	}
	if err := decodeLastJSONObject(statusOutput, &status); err != nil {
		t.Fatalf("decode status output: %v\n%s", err, statusOutput)
	}
	if status.CurrentHeight < 52 {
		t.Fatalf("expected current height to advance, got %d", status.CurrentHeight)
	}
	if status.PendingDepositClaims != 0 {
		t.Fatalf("expected no pending deposit claims, got %d", status.PendingDepositClaims)
	}
	if status.ProcessedClaims != 1 {
		t.Fatalf("expected one processed claim, got %d", status.ProcessedClaims)
	}
}
