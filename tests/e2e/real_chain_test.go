package e2e

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	aegisapp "github.com/ayushns01/aegislink/chain/aegislink/app"
	bridgetypes "github.com/ayushns01/aegislink/chain/aegislink/x/bridge/types"
	limittypes "github.com/ayushns01/aegislink/chain/aegislink/x/limits/types"
	registrytypes "github.com/ayushns01/aegislink/chain/aegislink/x/registry/types"
)

func TestRealAegisLinkChain(t *testing.T) {
	t.Parallel()

	homeDir := filepath.Join(t.TempDir(), "real-chain-home")
	bootstrapRealChain(t, homeDir)
	seedRealChainRuntime(t, homeDir)

	startOutput := runGoCommand(
		t,
		filepath.Join(repoRoot(t), "chain", "aegislink"),
		nil,
		"run",
		"./cmd/aegislinkd",
		"start",
		"--home",
		homeDir,
	)

	var started struct {
		Status      string `json:"status"`
		RuntimeMode string `json:"runtime_mode"`
		Initialized bool   `json:"initialized"`
	}
	if err := decodeLastJSONObject(startOutput, &started); err != nil {
		t.Fatalf("decode start output: %v\n%s", err, startOutput)
	}
	if started.Status != "started" {
		t.Fatalf("expected started status, got %+v", started)
	}
	if started.RuntimeMode != aegisapp.RuntimeModeSDKStore {
		t.Fatalf("expected sdk runtime mode, got %+v", started)
	}
	if !started.Initialized {
		t.Fatalf("expected initialized runtime, got %+v", started)
	}

	claim := validRuntimeClaim(t)
	submissionPath := filepath.Join(t.TempDir(), "submission.json")
	writeRuntimeSubmissionFile(t, submissionPath, claim)

	txOutput := runGoCommand(
		t,
		filepath.Join(repoRoot(t), "chain", "aegislink"),
		nil,
		"run",
		"./cmd/aegislinkd",
		"tx",
		"submit-deposit-claim",
		"--home",
		homeDir,
		"--submission-file",
		submissionPath,
	)

	var txResult struct {
		Status    string `json:"status"`
		MessageID string `json:"message_id"`
	}
	if err := decodeLastJSONObject(txOutput, &txResult); err != nil {
		t.Fatalf("decode tx output: %v\n%s", err, txOutput)
	}
	if txResult.Status != "accepted" {
		t.Fatalf("expected accepted tx result, got %+v", txResult)
	}

	queryOutput := runGoCommand(
		t,
		filepath.Join(repoRoot(t), "chain", "aegislink"),
		nil,
		"run",
		"./cmd/aegislinkd",
		"query",
		"claim",
		"--home",
		homeDir,
		"--message-id",
		claim.Identity.MessageID,
	)

	var stored struct {
		MessageID string `json:"message_id"`
		AssetID   string `json:"asset_id"`
		Amount    string `json:"amount"`
		Status    string `json:"status"`
	}
	if err := decodeLastJSONObject(queryOutput, &stored); err != nil {
		t.Fatalf("decode query output: %v\n%s", err, queryOutput)
	}
	if stored.MessageID != claim.Identity.MessageID {
		t.Fatalf("expected message id %q, got %q", claim.Identity.MessageID, stored.MessageID)
	}
	if stored.AssetID != claim.AssetID {
		t.Fatalf("expected asset id %q, got %q", claim.AssetID, stored.AssetID)
	}
	if stored.Amount != claim.Amount.String() {
		t.Fatalf("expected amount %s, got %q", claim.Amount.String(), stored.Amount)
	}
	if stored.Status != "accepted" {
		t.Fatalf("expected accepted stored claim, got %q", stored.Status)
	}
}

func bootstrapRealChain(t *testing.T, homeDir string) {
	t.Helper()

	cmd := exec.Command("bash", "scripts/localnet/bootstrap_real_chain.sh", homeDir)
	cmd.Dir = repoRoot(t)
	cmd.Env = append([]string{}, os.Environ()...)
	cacheRoot := filepath.Join(os.TempDir(), "aegislink-e2e-go-cache")
	if err := os.MkdirAll(cacheRoot, 0o755); err != nil {
		t.Fatalf("create e2e go cache root: %v", err)
	}
	cmd.Env = append(cmd.Env,
		"GOCACHE="+filepath.Join(cacheRoot, "gocache"),
		"GOMODCACHE="+filepath.Join(cacheRoot, "gomodcache"),
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bootstrap real chain: %v\n%s", err, output)
	}
}

func seedRealChainRuntime(t *testing.T, homeDir string) {
	t.Helper()

	cfg, err := aegisapp.ResolveConfig(aegisapp.Config{
		HomeDir:     homeDir,
		RuntimeMode: aegisapp.RuntimeModeSDKStore,
	})
	if err != nil {
		t.Fatalf("resolve runtime config: %v", err)
	}

	app, err := aegisapp.LoadWithConfig(cfg)
	if err != nil {
		t.Fatalf("load runtime app: %v", err)
	}
	defer func() {
		if err := app.Close(); err != nil {
			t.Fatalf("close runtime app: %v", err)
		}
	}()

	if err := app.RegisterAsset(registrytypes.Asset{
		AssetID:        "eth.usdc",
		SourceChainID:  "11155111",
		SourceContract: "0xasset",
		Denom:          "uethusdc",
		Decimals:       6,
		DisplayName:    "USDC",
		Enabled:        true,
	}); err != nil {
		t.Fatalf("register asset: %v", err)
	}
	if err := app.SetLimit(limittypes.RateLimit{
		AssetID:       "eth.usdc",
		WindowSeconds: 600,
		MaxAmount:     mustBigAmount(t, "1000000000000000000"),
	}); err != nil {
		t.Fatalf("set limit: %v", err)
	}
	app.SetCurrentHeight(50)
	if err := app.Save(); err != nil {
		t.Fatalf("save runtime app: %v", err)
	}
}

func validRuntimeClaim(t *testing.T) bridgetypes.DepositClaim {
	t.Helper()

	identity := bridgetypes.ClaimIdentity{
		Kind:           bridgetypes.ClaimKindDeposit,
		SourceChainID:  "11155111",
		SourceContract: "0xgateway",
		SourceTxHash:   "0xdeposit-tx",
		SourceLogIndex: 7,
		Nonce:          1,
	}
	identity.MessageID = identity.DerivedMessageID()

	return bridgetypes.DepositClaim{
		Identity:           identity,
		DestinationChainID: "aegislink-sdk-1",
		AssetID:            "eth.usdc",
		Amount:             mustBigAmount(t, "100000000"),
		Recipient:          "cosmos1recipient",
		Deadline:           100,
	}
}

func writeRuntimeSubmissionFile(t *testing.T, path string, claim bridgetypes.DepositClaim) {
	t.Helper()

	payload := struct {
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
			MessageID        string                       `json:"message_id"`
			PayloadHash      string                       `json:"payload_hash"`
			Signers          []string                     `json:"signers"`
			Proofs           []bridgetypes.AttestationProof `json:"proofs"`
			Threshold        uint32                       `json:"threshold"`
			Expiry           uint64                       `json:"expiry"`
			SignerSetVersion uint64                       `json:"signer_set_version"`
		} `json:"attestation"`
	}{}

	payload.Claim.Kind = string(claim.Identity.Kind)
	payload.Claim.SourceChainID = claim.Identity.SourceChainID
	payload.Claim.SourceContract = claim.Identity.SourceContract
	payload.Claim.SourceTxHash = claim.Identity.SourceTxHash
	payload.Claim.SourceLogIndex = claim.Identity.SourceLogIndex
	payload.Claim.Nonce = claim.Identity.Nonce
	payload.Claim.MessageID = claim.Identity.MessageID
	payload.Claim.DestinationChainID = claim.DestinationChainID
	payload.Claim.AssetID = claim.AssetID
	payload.Claim.Amount = claim.Amount.String()
	payload.Claim.Recipient = claim.Recipient
	payload.Claim.Deadline = claim.Deadline

	payload.Attestation.MessageID = claim.Identity.MessageID
	payload.Attestation.PayloadHash = claim.Digest()
	payload.Attestation.Signers = bridgetypes.DefaultHarnessSignerAddresses()[:2]
	payload.Attestation.Threshold = 2
	payload.Attestation.Expiry = 120
	payload.Attestation.SignerSetVersion = 1
	for _, key := range bridgetypes.DefaultHarnessSignerPrivateKeys()[:2] {
		proof, err := bridgetypes.SignAttestationWithPrivateKeyHex(bridgetypes.Attestation{
			MessageID:        payload.Attestation.MessageID,
			PayloadHash:      payload.Attestation.PayloadHash,
			Signers:          payload.Attestation.Signers,
			Threshold:        payload.Attestation.Threshold,
			Expiry:           payload.Attestation.Expiry,
			SignerSetVersion: payload.Attestation.SignerSetVersion,
		}, key)
		if err != nil {
			t.Fatalf("sign submission proof: %v", err)
		}
		payload.Attestation.Proofs = append(payload.Attestation.Proofs, proof)
	}

	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal submission: %v", err)
	}
	if err := os.WriteFile(path, encoded, 0o644); err != nil {
		t.Fatalf("write submission: %v", err)
	}
}

func decodeLastJSONObject(raw string, target any) error {
	raw = strings.TrimSpace(raw)

	var lastErr error
	for i := len(raw) - 1; i >= 0; i-- {
		if raw[i] != '{' && raw[i] != '[' {
			continue
		}
		if err := json.Unmarshal([]byte(raw[i:]), target); err == nil {
			return nil
		} else {
			lastErr = err
		}
	}

	if lastErr != nil {
		return lastErr
	}
	return json.Unmarshal([]byte(raw), target)
}
