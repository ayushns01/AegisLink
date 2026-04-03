package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"os"
	"path/filepath"
	"testing"

	aegisapp "github.com/ayushns01/aegislink/chain/aegislink/app"
	bridgetypes "github.com/ayushns01/aegislink/chain/aegislink/x/bridge/types"
	limittypes "github.com/ayushns01/aegislink/chain/aegislink/x/limits/types"
	registrytypes "github.com/ayushns01/aegislink/chain/aegislink/x/registry/types"
)

func TestRunTxSubmitDepositClaimPersistsAcceptedClaim(t *testing.T) {
	t.Parallel()

	statePath := filepath.Join(t.TempDir(), "aegislink-state.json")
	app := aegisapp.NewWithConfig(aegisapp.Config{
		AppName:           aegisapp.AppName,
		Modules:           []string{"bridge", "registry", "limits", "pauser"},
		StatePath:         statePath,
		AllowedSigners:    []string{"relayer-1", "relayer-2", "relayer-3"},
		RequiredThreshold: 2,
	})
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
		MaxAmount:     mustAmount(t, "1000000000000000000"),
	}); err != nil {
		t.Fatalf("set limit: %v", err)
	}
	app.SetCurrentHeight(50)
	if err := app.Save(); err != nil {
		t.Fatalf("save state: %v", err)
	}

	submissionPath := filepath.Join(t.TempDir(), "submission.json")
	claim := validDepositClaim(t)
	writeSubmissionFile(t, submissionPath, claim, validAttestationForClaim(claim))

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := run([]string{
		"tx", "submit-deposit-claim",
		"--state-path", statePath,
		"--submission-file", submissionPath,
	}, &stdout, &stderr); err != nil {
		t.Fatalf("run tx submit-deposit-claim: %v\nstderr=%s", err, stderr.String())
	}

	loaded, err := aegisapp.Load(statePath)
	if err != nil {
		t.Fatalf("reload state: %v", err)
	}
	if supply := loaded.BridgeKeeper.SupplyForDenom("uethusdc"); supply.String() != "100000000" {
		t.Fatalf("expected minted supply 100000000, got %s", supply.String())
	}

	var result struct {
		Status    string `json:"status"`
		MessageID string `json:"message_id"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("decode output: %v\n%s", err, stdout.String())
	}
	if result.Status != "accepted" {
		t.Fatalf("expected accepted status, got %q", result.Status)
	}
}

func TestRunQueryClaimPrintsPersistedAcceptedClaim(t *testing.T) {
	t.Parallel()

	statePath := filepath.Join(t.TempDir(), "aegislink-state.json")
	app := seededRuntimeApp(t, statePath)
	claim := validDepositClaim(t)
	if _, err := app.SubmitDepositClaim(claim, validAttestationForClaim(claim)); err != nil {
		t.Fatalf("submit deposit claim: %v", err)
	}
	if err := app.Save(); err != nil {
		t.Fatalf("save state: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := run([]string{
		"query", "claim",
		"--state-path", statePath,
		"--message-id", claim.Identity.MessageID,
	}, &stdout, &stderr); err != nil {
		t.Fatalf("run query claim: %v\nstderr=%s", err, stderr.String())
	}

	var result struct {
		MessageID string `json:"message_id"`
		AssetID   string `json:"asset_id"`
		Denom     string `json:"denom"`
		Amount    string `json:"amount"`
		Status    string `json:"status"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("decode output: %v\n%s", err, stdout.String())
	}
	if result.MessageID != claim.Identity.MessageID {
		t.Fatalf("expected message id %q, got %q", claim.Identity.MessageID, result.MessageID)
	}
	if result.AssetID != claim.AssetID {
		t.Fatalf("expected asset id %q, got %q", claim.AssetID, result.AssetID)
	}
	if result.Denom != "uethusdc" {
		t.Fatalf("expected denom uethusdc, got %q", result.Denom)
	}
	if result.Amount != claim.Amount.String() {
		t.Fatalf("expected amount %s, got %q", claim.Amount.String(), result.Amount)
	}
	if result.Status != "accepted" {
		t.Fatalf("expected accepted status, got %q", result.Status)
	}
}

func TestRunTxExecuteWithdrawalPersistsWithdrawalAndBurnsSupply(t *testing.T) {
	t.Parallel()

	statePath := filepath.Join(t.TempDir(), "aegislink-state.json")
	app := seededRuntimeApp(t, statePath)
	claim := validDepositClaim(t)
	if _, err := app.SubmitDepositClaim(claim, validAttestationForClaim(claim)); err != nil {
		t.Fatalf("submit deposit claim: %v", err)
	}
	if err := app.Save(); err != nil {
		t.Fatalf("save state: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := run([]string{
		"tx", "execute-withdrawal",
		"--state-path", statePath,
		"--asset-id", claim.AssetID,
		"--amount", "25000000",
		"--recipient", "0xrecipient",
		"--deadline", "140",
		"--signature-base64", base64.StdEncoding.EncodeToString([]byte("threshold-proof")),
		"--height", "60",
	}, &stdout, &stderr); err != nil {
		t.Fatalf("run tx execute-withdrawal: %v\nstderr=%s", err, stderr.String())
	}

	loaded, err := aegisapp.Load(statePath)
	if err != nil {
		t.Fatalf("reload state: %v", err)
	}
	if supply := loaded.BridgeKeeper.SupplyForDenom("uethusdc"); supply.String() != "75000000" {
		t.Fatalf("expected remaining supply 75000000, got %s", supply.String())
	}

	withdrawals := loaded.Withdrawals(60, 60)
	if len(withdrawals) != 1 {
		t.Fatalf("expected one stored withdrawal, got %d", len(withdrawals))
	}
	if withdrawals[0].Recipient != "0xrecipient" {
		t.Fatalf("expected recipient 0xrecipient, got %q", withdrawals[0].Recipient)
	}
	if string(withdrawals[0].Signature) != "threshold-proof" {
		t.Fatalf("expected decoded signature threshold-proof, got %q", withdrawals[0].Signature)
	}

	var result struct {
		MessageID   string `json:"message_id"`
		AssetID     string `json:"asset_id"`
		Amount      string `json:"amount"`
		Recipient   string `json:"recipient"`
		BlockHeight uint64 `json:"block_height"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("decode output: %v\n%s", err, stdout.String())
	}
	if result.AssetID != claim.AssetID {
		t.Fatalf("expected asset id %q, got %q", claim.AssetID, result.AssetID)
	}
	if result.Amount != "25000000" {
		t.Fatalf("expected amount 25000000, got %q", result.Amount)
	}
	if result.Recipient != "0xrecipient" {
		t.Fatalf("expected recipient 0xrecipient, got %q", result.Recipient)
	}
	if result.BlockHeight != 60 {
		t.Fatalf("expected block height 60, got %d", result.BlockHeight)
	}
	if result.MessageID == "" {
		t.Fatal("expected withdrawal message id")
	}
}

func writeSubmissionFile(t *testing.T, path string, claim bridgetypes.DepositClaim, attestation bridgetypes.Attestation) {
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
			MessageID   string   `json:"message_id"`
			PayloadHash string   `json:"payload_hash"`
			Signers     []string `json:"signers"`
			Threshold   uint32   `json:"threshold"`
			Expiry      uint64   `json:"expiry"`
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

	payload.Attestation.MessageID = attestation.MessageID
	payload.Attestation.PayloadHash = attestation.PayloadHash
	payload.Attestation.Signers = append([]string(nil), attestation.Signers...)
	payload.Attestation.Threshold = attestation.Threshold
	payload.Attestation.Expiry = attestation.Expiry

	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal submission: %v", err)
	}
	if err := os.WriteFile(path, encoded, 0o644); err != nil {
		t.Fatalf("write submission: %v", err)
	}
}

func validDepositClaim(t *testing.T) bridgetypes.DepositClaim {
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
		DestinationChainID: "aegislink-1",
		AssetID:            "eth.usdc",
		Amount:             mustAmount(t, "100000000"),
		Recipient:          "cosmos1recipient",
		Deadline:           100,
	}
}

func validAttestationForClaim(claim bridgetypes.DepositClaim) bridgetypes.Attestation {
	return bridgetypes.Attestation{
		MessageID:   claim.Identity.MessageID,
		PayloadHash: claim.Digest(),
		Signers:     []string{"relayer-1", "relayer-2"},
		Threshold:   2,
		Expiry:      120,
	}
}

func mustAmount(t *testing.T, value string) *big.Int {
	t.Helper()

	amount, ok := new(big.Int).SetString(value, 10)
	if !ok {
		t.Fatalf("invalid amount %q", value)
	}
	return amount
}

func seededRuntimeApp(t *testing.T, statePath string) *aegisapp.App {
	t.Helper()

	app := aegisapp.NewWithConfig(aegisapp.Config{
		AppName:           aegisapp.AppName,
		Modules:           []string{"bridge", "registry", "limits", "pauser"},
		StatePath:         statePath,
		AllowedSigners:    []string{"relayer-1", "relayer-2", "relayer-3"},
		RequiredThreshold: 2,
	})
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
		MaxAmount:     mustAmount(t, "1000000000000000000"),
	}); err != nil {
		t.Fatalf("set limit: %v", err)
	}
	app.SetCurrentHeight(50)
	return app
}
