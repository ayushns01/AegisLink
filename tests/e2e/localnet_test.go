package e2e

import (
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	bridgetypes "github.com/ayushns01/aegislink/chain/aegislink/x/bridge/types"
)

type fixturePaths struct {
	root             string
	evmStatePath     string
	voteStatePath    string
	cosmosStatePath  string
	cosmosOutboxPath string
	evmOutboxPath    string
	replayStorePath  string
}

type persistedDepositState struct {
	LatestBlock   uint64                  `json:"latest_block"`
	DepositEvents []persistedDepositEvent `json:"deposit_events"`
}

type persistedDepositEvent struct {
	BlockNumber    uint64 `json:"block_number"`
	SourceChainID  string `json:"source_chain_id"`
	SourceContract string `json:"source_contract"`
	TxHash         string `json:"tx_hash"`
	LogIndex       uint64 `json:"log_index"`
	Nonce          uint64 `json:"nonce"`
	DepositID      string `json:"deposit_id"`
	MessageID      string `json:"message_id"`
	AssetAddress   string `json:"asset_address"`
	AssetID        string `json:"asset_id"`
	Amount         string `json:"amount"`
	Recipient      string `json:"recipient"`
	Expiry         uint64 `json:"expiry"`
}

type persistedVoteState struct {
	Votes []persistedVote `json:"votes"`
}

type persistedVote struct {
	MessageID   string `json:"message_id"`
	PayloadHash string `json:"payload_hash"`
	Signer      string `json:"signer"`
	Expiry      uint64 `json:"expiry"`
}

type persistedClaimOutbox struct {
	Submissions []persistedClaimSubmission `json:"submissions"`
}

type persistedClaimSubmission struct {
	Claim       persistedDepositClaim `json:"claim"`
	Attestation persistedAttestation  `json:"attestation"`
}

type persistedDepositClaim struct {
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
}

type persistedAttestation struct {
	MessageID   string   `json:"message_id"`
	PayloadHash string   `json:"payload_hash"`
	Signers     []string `json:"signers"`
	Threshold   uint32   `json:"threshold"`
	Expiry      uint64   `json:"expiry"`
}

type persistedWithdrawalState struct {
	LatestHeight uint64                `json:"latest_height"`
	Withdrawals  []persistedWithdrawal `json:"withdrawals"`
}

type persistedWithdrawal struct {
	BlockHeight    uint64 `json:"block_height"`
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
	Signature      string `json:"signature"`
}

type persistedReleaseOutbox struct {
	Requests []persistedReleaseRequest `json:"requests"`
}

type persistedReleaseRequest struct {
	MessageID    string `json:"message_id"`
	AssetAddress string `json:"asset_address"`
	Amount       string `json:"amount"`
	Recipient    string `json:"recipient"`
	Deadline     uint64 `json:"deadline"`
	Signature    string `json:"signature"`
}

func TestAegisLinkShellStartsWithSafetyModules(t *testing.T) {
	t.Parallel()

	output := runGoCommand(t, repoRoot(t), nil, "run", "./chain/aegislink/cmd/aegislinkd")
	if !strings.Contains(output, "aegislink initialized with modules: registry, limits, pauser") {
		t.Fatalf("expected aegislinkd module list in output, got %q", output)
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("unable to locate current test file")
	}
	return filepath.Dir(filepath.Dir(filepath.Dir(file)))
}

func writeInboundFixtures(t *testing.T) fixturePaths {
	t.Helper()

	root := t.TempDir()
	fixtures := fixturePaths{
		root:             root,
		evmStatePath:     filepath.Join(root, "evm-state.json"),
		voteStatePath:    filepath.Join(root, "attestations.json"),
		cosmosStatePath:  filepath.Join(root, "cosmos-state.json"),
		cosmosOutboxPath: filepath.Join(root, "cosmos-outbox.json"),
		evmOutboxPath:    filepath.Join(root, "evm-outbox.json"),
		replayStorePath:  filepath.Join(root, "replay-store.json"),
	}

	deposit := persistedDepositEvent{
		BlockNumber:    10,
		SourceChainID:  "11155111",
		SourceContract: "0xgateway",
		TxHash:         "0xdeposit-tx",
		LogIndex:       7,
		Nonce:          1,
		DepositID:      "deposit-1",
		MessageID:      "unused-event-message",
		AssetAddress:   "0xasset",
		AssetID:        "eth.usdc",
		Amount:         "100000000",
		Recipient:      "cosmos1recipient",
		Expiry:         100,
	}

	claim := depositClaimFromEvent(t, deposit)
	votes := persistedVoteState{
		Votes: []persistedVote{
			{MessageID: claim.Identity.MessageID, PayloadHash: claim.Digest(), Signer: "relayer-1", Expiry: 140},
			{MessageID: claim.Identity.MessageID, PayloadHash: claim.Digest(), Signer: "relayer-2", Expiry: 150},
			{MessageID: claim.Identity.MessageID, PayloadHash: claim.Digest(), Signer: "relayer-3", Expiry: 120},
		},
	}

	writeJSON(t, fixtures.evmStatePath, persistedDepositState{
		LatestBlock:   12,
		DepositEvents: []persistedDepositEvent{deposit},
	})
	writeJSON(t, fixtures.voteStatePath, votes)
	writeJSON(t, fixtures.cosmosStatePath, map[string]any{
		"latest_height": 0,
		"withdrawals":   []any{},
	})

	return fixtures
}

func runRelayerOnce(t *testing.T, fixtures fixturePaths) {
	t.Helper()

	env := map[string]string{
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
	}

	_ = runGoCommand(t, repoRoot(t), env, "run", "./relayer/cmd/bridge-relayer")
}

func loadCosmosOutbox(t *testing.T, path string) []persistedClaimSubmission {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read cosmos outbox: %v", err)
	}

	var outbox persistedClaimOutbox
	if err := json.Unmarshal(data, &outbox); err != nil {
		t.Fatalf("decode cosmos outbox: %v", err)
	}
	return outbox.Submissions
}

func loadEVMOutbox(t *testing.T, path string) []persistedReleaseRequest {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read evm outbox: %v", err)
	}

	var outbox persistedReleaseOutbox
	if err := json.Unmarshal(data, &outbox); err != nil {
		t.Fatalf("decode evm outbox: %v", err)
	}
	return outbox.Requests
}

func decodeSubmission(t *testing.T, submission persistedClaimSubmission) (bridgetypes.DepositClaim, bridgetypes.Attestation) {
	t.Helper()

	amount, ok := new(big.Int).SetString(submission.Claim.Amount, 10)
	if !ok {
		t.Fatalf("invalid claim amount %q", submission.Claim.Amount)
	}

	claim := bridgetypes.DepositClaim{
		Identity: bridgetypes.ClaimIdentity{
			Kind:           bridgetypes.ClaimKind(submission.Claim.Kind),
			SourceChainID:  submission.Claim.SourceChainID,
			SourceContract: submission.Claim.SourceContract,
			SourceTxHash:   submission.Claim.SourceTxHash,
			SourceLogIndex: submission.Claim.SourceLogIndex,
			Nonce:          submission.Claim.Nonce,
			MessageID:      submission.Claim.MessageID,
		},
		DestinationChainID: submission.Claim.DestinationChainID,
		AssetID:            submission.Claim.AssetID,
		Amount:             amount,
		Recipient:          submission.Claim.Recipient,
		Deadline:           submission.Claim.Deadline,
	}
	attestation := bridgetypes.Attestation{
		MessageID:   submission.Attestation.MessageID,
		PayloadHash: submission.Attestation.PayloadHash,
		Signers:     append([]string(nil), submission.Attestation.Signers...),
		Threshold:   submission.Attestation.Threshold,
		Expiry:      submission.Attestation.Expiry,
	}
	return claim, attestation
}

func depositClaimFromEvent(t *testing.T, event persistedDepositEvent) bridgetypes.DepositClaim {
	t.Helper()

	amount, ok := new(big.Int).SetString(event.Amount, 10)
	if !ok {
		t.Fatalf("invalid deposit fixture amount %q", event.Amount)
	}
	identity := bridgetypes.ClaimIdentity{
		Kind:           bridgetypes.ClaimKindDeposit,
		SourceChainID:  event.SourceChainID,
		SourceContract: event.SourceContract,
		SourceTxHash:   event.TxHash,
		SourceLogIndex: event.LogIndex,
		Nonce:          event.Nonce,
	}
	identity.MessageID = identity.DerivedMessageID()

	return bridgetypes.DepositClaim{
		Identity:           identity,
		DestinationChainID: "aegislink-1",
		AssetID:            event.AssetID,
		Amount:             amount,
		Recipient:          event.Recipient,
		Deadline:           event.Expiry,
	}
}

func writeJSON(t *testing.T, path string, value any) {
	t.Helper()

	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatalf("marshal json for %s: %v", path, err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write json fixture %s: %v", path, err)
	}
}

func runGoCommand(t *testing.T, dir string, extraEnv map[string]string, args ...string) string {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = dir
	cmd.Env = append([]string{}, os.Environ()...)

	cacheRoot := t.TempDir()
	cmd.Env = append(cmd.Env,
		"GOCACHE="+filepath.Join(cacheRoot, "gocache"),
		"GOMODCACHE="+filepath.Join(cacheRoot, "gomodcache"),
	)
	for key, value := range extraEnv {
		cmd.Env = append(cmd.Env, key+"="+value)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			t.Fatalf("command timed out: go %s\n%s", strings.Join(args, " "), output)
		}
		t.Fatalf("command failed: go %s\n%s", strings.Join(args, " "), output)
	}
	return string(output)
}
