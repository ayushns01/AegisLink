package e2e

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	aegisapp "github.com/ayushns01/aegislink/chain/aegislink/app"
	bridgekeeper "github.com/ayushns01/aegislink/chain/aegislink/x/bridge/keeper"
	bridgetypes "github.com/ayushns01/aegislink/chain/aegislink/x/bridge/types"
	ibcrouterkeeper "github.com/ayushns01/aegislink/chain/aegislink/x/ibcrouter/keeper"
	limitskeeper "github.com/ayushns01/aegislink/chain/aegislink/x/limits/keeper"
	limittypes "github.com/ayushns01/aegislink/chain/aegislink/x/limits/types"
)

func TestRecoveryDrillRelayerRestartUsesReplayPersistence(t *testing.T) {
	t.Parallel()

	fixtures := writeInboundFixtures(t)

	firstRun := runGoCommand(t, repoRoot(t), map[string]string{
		"AEGISLINK_RELAYER_COSMOS_CHAIN_ID":        "aegislink-1",
		"AEGISLINK_RELAYER_ATTESTATION_THRESHOLD":  "2",
		"AEGISLINK_RELAYER_SUBMISSION_RETRY_LIMIT": "2",
		"AEGISLINK_RELAYER_EVM_CONFIRMATIONS":      "2",
		"AEGISLINK_RELAYER_COSMOS_CONFIRMATIONS":   "1",
		"AEGISLINK_RELAYER_EVM_STATE_PATH":         fixtures.evmStatePath,
		"AEGISLINK_RELAYER_ATTESTATION_STATE_PATH": fixtures.voteStatePath,
		"AEGISLINK_RELAYER_COSMOS_STATE_PATH":      fixtures.cosmosStatePath,
		"AEGISLINK_RELAYER_COSMOS_OUTBOX_PATH":     fixtures.cosmosOutboxPath,
		"AEGISLINK_RELAYER_EVM_OUTBOX_PATH":        fixtures.evmOutboxPath,
		"AEGISLINK_RELAYER_REPLAY_STORE_PATH":      fixtures.replayStorePath,
	}, "run", "./relayer/cmd/bridge-relayer")

	if !strings.Contains(firstRun, `"deposits_submitted":1`) {
		t.Fatalf("expected first run to submit one deposit, got:\n%s", firstRun)
	}
	if submissions := loadCosmosOutbox(t, fixtures.cosmosOutboxPath); len(submissions) != 1 {
		t.Fatalf("expected one outbox submission after first run, got %d", len(submissions))
	}

	secondRun := runGoCommand(t, repoRoot(t), map[string]string{
		"AEGISLINK_RELAYER_COSMOS_CHAIN_ID":        "aegislink-1",
		"AEGISLINK_RELAYER_ATTESTATION_THRESHOLD":  "2",
		"AEGISLINK_RELAYER_SUBMISSION_RETRY_LIMIT": "2",
		"AEGISLINK_RELAYER_EVM_CONFIRMATIONS":      "2",
		"AEGISLINK_RELAYER_COSMOS_CONFIRMATIONS":   "1",
		"AEGISLINK_RELAYER_EVM_STATE_PATH":         fixtures.evmStatePath,
		"AEGISLINK_RELAYER_ATTESTATION_STATE_PATH": fixtures.voteStatePath,
		"AEGISLINK_RELAYER_COSMOS_STATE_PATH":      fixtures.cosmosStatePath,
		"AEGISLINK_RELAYER_COSMOS_OUTBOX_PATH":     fixtures.cosmosOutboxPath,
		"AEGISLINK_RELAYER_EVM_OUTBOX_PATH":        fixtures.evmOutboxPath,
		"AEGISLINK_RELAYER_REPLAY_STORE_PATH":      fixtures.replayStorePath,
	}, "run", "./relayer/cmd/bridge-relayer")

	if !strings.Contains(secondRun, `"deposits_observed":0`) {
		t.Fatalf("expected restart run to observe no new deposits after checkpoint recovery, got:\n%s", secondRun)
	}
	if submissions := loadCosmosOutbox(t, fixtures.cosmosOutboxPath); len(submissions) != 1 {
		t.Fatalf("expected replay persistence to suppress duplicate outbox writes, got %d submissions", len(submissions))
	}

	var replayState struct {
		Checkpoints map[string]uint64 `json:"checkpoints"`
		Processed   []string          `json:"processed"`
	}
	data, err := os.ReadFile(fixtures.replayStorePath)
	if err != nil {
		t.Fatalf("read replay store: %v", err)
	}
	if err := json.Unmarshal(data, &replayState); err != nil {
		t.Fatalf("decode replay store: %v", err)
	}
	if replayState.Checkpoints["evm-deposits"] == 0 {
		t.Fatalf("expected deposit checkpoint to persist, got %+v", replayState.Checkpoints)
	}
	if len(replayState.Processed) != 1 {
		t.Fatalf("expected one processed replay key, got %+v", replayState.Processed)
	}
}

func TestRecoveryDrillTimesOutRouteAndAllowsRefundRecovery(t *testing.T) {
	t.Parallel()

	statePath := writeRuntimeChainBootstrapWithOsmosisRoute(t)
	app, err := aegisapp.Load(statePath)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	transfer, err := app.IBCRouterKeeper.InitiateTransfer("eth.usdc", mustBigAmount(t, "25000000"), "osmo1timeout", 140, "swap:uosmo")
	if err != nil {
		t.Fatalf("initiate transfer: %v", err)
	}
	if err := app.Save(); err != nil {
		t.Fatalf("save state: %v", err)
	}

	target := startMockOsmosisTarget(t, filepath.Join(t.TempDir(), "mock-osmosis-timeout.json"), "timeout")
	defer target.cancel()

	runRouteRelayerOnce(t, statePath, target.url)
	runRouteRelayerOnce(t, statePath, target.url)

	reloaded, err := aegisapp.Load(statePath)
	if err != nil {
		t.Fatalf("reload state after timeout: %v", err)
	}
	transfers := reloaded.IBCRouterKeeper.ExportTransfers()
	if len(transfers) != 1 {
		t.Fatalf("expected one transfer, got %d", len(transfers))
	}
	if transfers[0].Status != ibcrouterkeeper.TransferStatusTimedOut {
		t.Fatalf("expected timed_out status, got %q", transfers[0].Status)
	}

	runGoCommand(
		t,
		repoRoot(t),
		nil,
		"run",
		"./chain/aegislink/cmd/aegislinkd",
		"tx",
		"refund-ibc-transfer",
		"--state-path",
		statePath,
		"--transfer-id",
		transfer.TransferID,
	)

	reloaded, err = aegisapp.Load(statePath)
	if err != nil {
		t.Fatalf("reload state after refund: %v", err)
	}
	transfers = reloaded.IBCRouterKeeper.ExportTransfers()
	if transfers[0].Status != ibcrouterkeeper.TransferStatusRefunded {
		t.Fatalf("expected refunded status, got %q", transfers[0].Status)
	}
}

func TestRecoveryDrillPausedAssetCanRecoverAfterUnpause(t *testing.T) {
	t.Parallel()

	fixtures := writeInboundFixtures(t)
	runRelayerOnce(t, fixtures)
	submissions := loadCosmosOutbox(t, fixtures.cosmosOutboxPath)
	claim, attestation := decodeSubmission(t, submissions[0])

	statePath := writeRuntimeChainBootstrap(t)
	app, err := aegisapp.Load(statePath)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if err := app.Pause("eth.usdc"); err != nil {
		t.Fatalf("pause asset: %v", err)
	}
	if err := app.Save(); err != nil {
		t.Fatalf("save paused state: %v", err)
	}

	if _, err := app.SubmitDepositClaim(claim, attestation); !errors.Is(err, bridgekeeper.ErrAssetPaused) {
		t.Fatalf("expected paused asset rejection, got %v", err)
	}

	if err := app.Unpause("eth.usdc"); err != nil {
		t.Fatalf("unpause asset: %v", err)
	}
	if err := app.Save(); err != nil {
		t.Fatalf("save unpaused state: %v", err)
	}

	reloaded, err := aegisapp.Load(statePath)
	if err != nil {
		t.Fatalf("reload state: %v", err)
	}
	result, err := reloaded.SubmitDepositClaim(claim, attestation)
	if err != nil {
		t.Fatalf("submit claim after unpause: %v", err)
	}
	if result.Status != bridgekeeper.ClaimStatusAccepted {
		t.Fatalf("expected accepted claim after unpause, got %q", result.Status)
	}
}

func TestRecoveryDrillRejectsSignerSetMismatchUntilAttestationIsUpdated(t *testing.T) {
	t.Parallel()

	fixtures := writeInboundFixtures(t)
	runRelayerOnce(t, fixtures)
	submissions := loadCosmosOutbox(t, fixtures.cosmosOutboxPath)
	claim, attestation := decodeSubmission(t, submissions[0])

	statePath := writeRuntimeChainBootstrap(t)
	app, err := aegisapp.Load(statePath)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	app.SetCurrentHeight(90)
	if err := app.BridgeKeeper.UpsertSignerSet(bridgekeeper.SignerSet{
		Version:     2,
		Signers:     bridgetypes.DefaultHarnessSignerAddresses()[:3],
		Threshold:   2,
		ActivatedAt: 80,
	}); err != nil {
		t.Fatalf("upsert signer set: %v", err)
	}

	if _, err := app.SubmitDepositClaim(claim, attestation); !errors.Is(err, bridgekeeper.ErrSignerSetVersionMismatch) {
		t.Fatalf("expected signer set mismatch rejection, got %v", err)
	}

	attestation.SignerSetVersion = 2
	attestation.Proofs = signAttestationWithSignerIndexes(t, attestation, 0, 1)
	result, err := app.SubmitDepositClaim(claim, attestation)
	if err != nil {
		t.Fatalf("submit claim after signer-set fix: %v", err)
	}
	if result.Status != bridgekeeper.ClaimStatusAccepted {
		t.Fatalf("expected accepted claim after signer-set fix, got %q", result.Status)
	}
}

func TestRecoveryDrillRateLimitWindowRecoversAfterWindowExpires(t *testing.T) {
	t.Parallel()

	statePath := writeRuntimeChainBootstrap(t)
	app, err := aegisapp.Load(statePath)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if err := app.SetLimit(limittypes.RateLimit{
		AssetID:       "eth.usdc",
		WindowSeconds: 10,
		MaxAmount:     mustBigAmount(t, "100000000"),
	}); err != nil {
		t.Fatalf("set narrow limit: %v", err)
	}

	firstClaim := sampleAttestationDepositClaim(t, 1)
	firstAttestation := signedAttestationForClaim(t, firstClaim, 0, 1)
	app.SetCurrentHeight(50)
	if _, err := app.SubmitDepositClaim(firstClaim, firstAttestation); err != nil {
		t.Fatalf("submit first claim: %v", err)
	}

	secondClaim := sampleAttestationDepositClaim(t, 2)
	secondAttestation := signedAttestationForClaim(t, secondClaim, 0, 1)
	app.SetCurrentHeight(55)
	if _, err := app.SubmitDepositClaim(secondClaim, secondAttestation); !errors.Is(err, limitskeeper.ErrRateLimitExceeded) {
		t.Fatalf("expected rolling-window limit rejection, got %v", err)
	}

	thirdClaim := sampleAttestationDepositClaim(t, 3)
	thirdAttestation := signedAttestationForClaim(t, thirdClaim, 0, 1)
	app.SetCurrentHeight(61)
	result, err := app.SubmitDepositClaim(thirdClaim, thirdAttestation)
	if err != nil {
		t.Fatalf("submit claim after window expiry: %v", err)
	}
	if result.Status != bridgekeeper.ClaimStatusAccepted {
		t.Fatalf("expected accepted claim after window expiry, got %q", result.Status)
	}
}

func TestRecoveryDrillCircuitBreakerTripsOnCorruptedSupply(t *testing.T) {
	t.Parallel()

	statePath := writeRuntimeChainBootstrap(t)
	app, err := aegisapp.Load(statePath)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}

	firstClaim := sampleAttestationDepositClaim(t, 1)
	firstAttestation := signedAttestationForClaim(t, firstClaim, 0, 1)
	app.SetCurrentHeight(50)
	if _, err := app.SubmitDepositClaim(firstClaim, firstAttestation); err != nil {
		t.Fatalf("submit first claim: %v", err)
	}

	state := app.BridgeKeeper.ExportState()
	state.SupplyByDenom["uethusdc"] = "999999999"
	if err := app.BridgeKeeper.ImportState(state); err != nil {
		t.Fatalf("import tampered bridge state: %v", err)
	}
	if err := app.BridgeKeeper.CheckAccountingInvariant(); !errors.Is(err, bridgekeeper.ErrAccountingInvariantBroken) {
		t.Fatalf("expected accounting invariant failure after tamper, got %v", err)
	}
	if err := app.Save(); err != nil {
		t.Fatalf("save tripped circuit state: %v", err)
	}

	reloaded, err := aegisapp.Load(statePath)
	if err != nil {
		t.Fatalf("reload tripped circuit state: %v", err)
	}
	if !reloaded.BridgeKeeper.CircuitBreakerTripped() {
		t.Fatal("expected circuit breaker to persist across reload")
	}
	if status := reloaded.Status(); !status.BridgeCircuitOpen {
		t.Fatalf("expected runtime status to report bridge circuit open, got %+v", status)
	}

	secondClaim := sampleAttestationDepositClaim(t, 2)
	secondAttestation := signedAttestationForClaim(t, secondClaim, 0, 1)
	reloaded.SetCurrentHeight(60)
	if _, err := reloaded.SubmitDepositClaim(secondClaim, secondAttestation); !errors.Is(err, bridgekeeper.ErrBridgeCircuitOpen) {
		t.Fatalf("expected bridge circuit breaker rejection after tamper, got %v", err)
	}
}

func TestRecoveryDrillRunbookDocumentsCoreScenarios(t *testing.T) {
	t.Parallel()

	runbookPath := filepath.Join(repoRoot(t), "docs", "runbooks", "incident-drills.md")
	data, err := os.ReadFile(runbookPath)
	if err != nil {
		t.Fatalf("read incident drills runbook: %v", err)
	}

	content := string(data)
	for _, expected := range []string{
		"relayer restart with replay persistence",
		"timed-out route refund",
		"paused asset recovery",
		"signer-set mismatch rejection",
		"rate-limit window recovery",
		"bridge circuit breaker",
	} {
		if !strings.Contains(content, expected) {
			t.Fatalf("expected incident drills runbook to mention %q\n%s", expected, content)
		}
	}
}
