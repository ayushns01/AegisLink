package e2e

import (
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	aegisapp "github.com/ayushns01/aegislink/chain/aegislink/app"
	"github.com/ayushns01/aegislink/chain/aegislink/networked"
	bridgetypes "github.com/ayushns01/aegislink/chain/aegislink/x/bridge/types"
	bridgetestutil "github.com/ayushns01/aegislink/chain/aegislink/x/bridge/types/testutil"
	limittypes "github.com/ayushns01/aegislink/chain/aegislink/x/limits/types"
	registrytypes "github.com/ayushns01/aegislink/chain/aegislink/x/registry/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func TestPublicWalletDelivery(t *testing.T) {
	t.Parallel()

	homeDir := filepath.Join(t.TempDir(), "public-wallet-home")
	cfg, err := aegisapp.InitHome(aegisapp.Config{
		HomeDir:     homeDir,
		ChainID:     "aegislink-public-1",
		RuntimeMode: aegisapp.RuntimeModeSDKStore,
	}, false)
	if err != nil {
		t.Fatalf("init home: %v", err)
	}

	app, err := aegisapp.LoadWithConfig(cfg)
	if err != nil {
		t.Fatalf("load runtime: %v", err)
	}

	recipient := sdk.AccAddress([]byte("wallet-bridge-h1")).String()
	if err := seedPublicWalletAssets(t, app, recipient); err != nil {
		t.Fatalf("seed assets: %v", err)
	}
	if err := app.Save(); err != nil {
		t.Fatalf("save seeded state: %v", err)
	}
	if err := app.Close(); err != nil {
		t.Fatalf("close seeded runtime: %v", err)
	}

	reloaded, err := aegisapp.LoadWithConfig(cfg)
	if err != nil {
		t.Fatalf("reload runtime: %v", err)
	}
	persistedBalances, err := reloaded.WalletBalances(recipient)
	if err != nil {
		t.Fatalf("load wallet balances: %v", err)
	}
	if len(persistedBalances) != 2 {
		t.Fatalf("expected two wallet balances after reload, got %d (%+v)", len(persistedBalances), persistedBalances)
	}
	gotByDenom := make(map[string]string, len(persistedBalances))
	for _, balance := range persistedBalances {
		gotByDenom[balance.Denom] = balance.Amount
	}
	if gotByDenom["ueth"] != "1000000000000000000" {
		t.Fatalf("expected bridged ETH balance to persist, got %+v", gotByDenom)
	}
	if gotByDenom["uethusdc"] != "25000000" {
		t.Fatalf("expected bridged ERC-20 balance to persist, got %+v", gotByDenom)
	}
	if err := reloaded.Close(); err != nil {
		t.Fatalf("close reloaded runtime: %v", err)
	}

	output := runGoCommandWithLocalCache(t, repoRoot(t), "run", "./chain/aegislink/cmd/aegislinkd", "query", "balances", "--home", homeDir, "--address", recipient)

	var balances []struct {
		Address string `json:"address"`
		Denom   string `json:"denom"`
		Amount  string `json:"amount"`
	}
	if err := decodeLastJSONObject(output, &balances); err != nil {
		t.Fatalf("decode balance query output: %v\n%s", err, output)
	}
	if len(balances) != 2 {
		t.Fatalf("expected two wallet balances, got %d (%+v)", len(balances), balances)
	}
}

func seedPublicWalletAssets(t *testing.T, app *aegisapp.App, recipient string) error {
	t.Helper()

	if err := registerPublicBridgeAssets(t, app); err != nil {
		return err
	}

	nativeClaim := depositClaim(t, bridgetypes.SourceAssetKindNativeETH, "", "eth", "0xnative-deposit", 1, 1, recipient, "1000000000000000000")
	if err := submitClaim(t, app, nativeClaim); err != nil {
		return err
	}

	erc20Claim := depositClaim(t, bridgetypes.SourceAssetKindERC20, "0xusdc", "eth.usdc", "0xerc20-deposit", 2, 2, recipient, "25000000")
	if err := submitClaim(t, app, erc20Claim); err != nil {
		return err
	}

	return nil
}

func registerPublicBridgeAssets(t *testing.T, app *aegisapp.App) error {
	t.Helper()
	return registerPublicBridgeAssetsWithERC20Address(t, app, "0xusdc")
}

func registerPublicBridgeAssetsWithERC20Address(t *testing.T, app *aegisapp.App, erc20Address string) error {
	t.Helper()

	nativeETH := registrytypes.Asset{
		AssetID:         "eth",
		SourceChainID:   "11155111",
		SourceAssetKind: registrytypes.SourceAssetKindNativeETH,
		Denom:           "ueth",
		Decimals:        18,
		DisplayName:     "Ether",
		DisplaySymbol:   "ETH",
		Enabled:         true,
	}
	if err := app.RegisterAsset(nativeETH); err != nil {
		return err
	}

	erc20 := registrytypes.Asset{
		AssetID:            "eth.usdc",
		SourceChainID:      "11155111",
		SourceAssetKind:    registrytypes.SourceAssetKindERC20,
		SourceAssetAddress: erc20Address,
		Denom:              "uethusdc",
		Decimals:           6,
		DisplayName:        "USD Coin",
		DisplaySymbol:      "USDC",
		Enabled:            true,
	}
	if err := app.RegisterAsset(erc20); err != nil {
		return err
	}

	if err := app.SetLimit(limittypes.RateLimit{
		AssetID:       nativeETH.AssetID,
		WindowBlocks: 600,
		MaxAmount:     mustWalletAmount(t, "2000000000000000000"),
	}); err != nil {
		return err
	}
	if err := app.SetLimit(limittypes.RateLimit{
		AssetID:       erc20.AssetID,
		WindowBlocks: 600,
		MaxAmount:     mustWalletAmount(t, "100000000"),
	}); err != nil {
		return err
	}
	return nil
}

func TestPublicWalletDeliveryViaPublicBridgeRelayer(t *testing.T) {
	t.Parallel()

	repo := repoRoot(t)
	homeDir := filepath.Join(t.TempDir(), "public-wallet-relayer-home")
	cfg, err := aegisapp.InitHome(aegisapp.Config{
		HomeDir:     homeDir,
		ChainID:     "aegislink-public-1",
		RuntimeMode: aegisapp.RuntimeModeSDKStore,
	}, false)
	if err != nil {
		t.Fatalf("init home: %v", err)
	}

	app, err := aegisapp.LoadWithConfig(cfg)
	if err != nil {
		t.Fatalf("load runtime: %v", err)
	}
	if err := registerPublicBridgeAssets(t, app); err != nil {
		t.Fatalf("register public assets: %v", err)
	}
	if err := app.Save(); err != nil {
		t.Fatalf("save runtime: %v", err)
	}
	if err := app.Close(); err != nil {
		t.Fatalf("close runtime: %v", err)
	}

	anvil := startAnvilRuntime(t)
	contracts := deployBridgeContractsToAnvil(t, anvil.rpcURL)
	recipient := sdk.AccAddress([]byte("wallet-bridge-i2")).String()
	nativeReceipt := createAnvilNativeDeposit(t, anvil.rpcURL, contracts, "1000000000000000000", recipient, "10000000000")
	erc20Receipt := createAnvilDeposit(t, anvil.rpcURL, contracts, "25000000", recipient, "10000000000")

	attestationPath := filepath.Join(t.TempDir(), "public-attestations.json")
	writeAttestationVotes(t, attestationPath,
		relayedDepositClaim(t, bridgetypes.SourceAssetKindNativeETH, contracts.Gateway, "eth", nativeReceipt.TransactionHash, parseHexUint64(t, nativeReceipt.Logs[0].LogIndex), 1, recipient, "1000000000000000000", 10000000000),
		relayedDepositClaim(t, bridgetypes.SourceAssetKindERC20, contracts.Gateway, "eth.usdc", erc20Receipt.TransactionHash, parseHexUint64(t, erc20Receipt.Logs[0].LogIndex), 2, recipient, "25000000", 10000000000),
	)

	replayStore := filepath.Join(t.TempDir(), "public-relayer-replay.json")
	output := runGoCommandWithLocalCacheAndEnv(t, repo, map[string]string{
		"AEGISLINK_RELAYER_COSMOS_CHAIN_ID":        "aegislink-public-1",
		"AEGISLINK_RELAYER_EVM_RPC_URL":            anvil.rpcURL,
		"AEGISLINK_RELAYER_EVM_VERIFIER_ADDRESS":   contracts.Verifier,
		"AEGISLINK_RELAYER_EVM_GATEWAY_ADDRESS":    contracts.Gateway,
		"AEGISLINK_RELAYER_EVM_CONFIRMATIONS":      "0",
		"AEGISLINK_RELAYER_COSMOS_CONFIRMATIONS":   "0",
		"AEGISLINK_RELAYER_SUBMISSION_RETRY_LIMIT": "1",
		"AEGISLINK_RELAYER_REPLAY_STORE_PATH":      replayStore,
		"AEGISLINK_RELAYER_ATTESTATION_STATE_PATH": attestationPath,
		"AEGISLINK_RELAYER_AEGISLINK_CMD":          "go",
		"AEGISLINK_RELAYER_AEGISLINK_CMD_ARGS":     "run ./chain/aegislink/cmd/aegislinkd --home " + homeDir,
		"AEGISLINK_RELAYER_AEGISLINK_STATE_PATH":   "",
	}, "run", "./relayer/cmd/public-bridge-relayer")
	if !strings.Contains(output, "run_complete") {
		t.Fatalf("expected relayer success log, got:\n%s", output)
	}

	reloaded, err := aegisapp.LoadWithConfig(cfg)
	if err != nil {
		t.Fatalf("reload runtime: %v", err)
	}

	balances, err := reloaded.WalletBalances(recipient)
	if err != nil {
		t.Fatalf("wallet balances: %v", err)
	}
	if len(balances) != 2 {
		t.Fatalf("expected two wallet balances after relayer run, got %d (%+v)", len(balances), balances)
	}
	gotByDenom := make(map[string]string, len(balances))
	for _, balance := range balances {
		gotByDenom[balance.Denom] = balance.Amount
	}
	if gotByDenom["ueth"] != "1000000000000000000" {
		t.Fatalf("expected bridged ETH balance, got %+v", gotByDenom)
	}
	if gotByDenom["uethusdc"] != "25000000" {
		t.Fatalf("expected bridged ERC-20 balance, got %+v", gotByDenom)
	}
	closeApp(t, reloaded)

	secondOutput := runGoCommandWithLocalCacheAndEnv(t, repo, map[string]string{
		"AEGISLINK_RELAYER_COSMOS_CHAIN_ID":        "aegislink-public-1",
		"AEGISLINK_RELAYER_EVM_RPC_URL":            anvil.rpcURL,
		"AEGISLINK_RELAYER_EVM_VERIFIER_ADDRESS":   contracts.Verifier,
		"AEGISLINK_RELAYER_EVM_GATEWAY_ADDRESS":    contracts.Gateway,
		"AEGISLINK_RELAYER_EVM_CONFIRMATIONS":      "0",
		"AEGISLINK_RELAYER_COSMOS_CONFIRMATIONS":   "0",
		"AEGISLINK_RELAYER_SUBMISSION_RETRY_LIMIT": "1",
		"AEGISLINK_RELAYER_REPLAY_STORE_PATH":      replayStore,
		"AEGISLINK_RELAYER_ATTESTATION_STATE_PATH": attestationPath,
		"AEGISLINK_RELAYER_AEGISLINK_CMD":          "go",
		"AEGISLINK_RELAYER_AEGISLINK_CMD_ARGS":     "run ./chain/aegislink/cmd/aegislinkd --home " + homeDir,
		"AEGISLINK_RELAYER_AEGISLINK_STATE_PATH":   "",
	}, "run", "./relayer/cmd/public-bridge-relayer")
	if !strings.Contains(secondOutput, "run_complete") {
		t.Fatalf("expected second relayer success log, got:\n%s", secondOutput)
	}

	reloaded, err = aegisapp.LoadWithConfig(cfg)
	if err != nil {
		t.Fatalf("reload runtime after replay pass: %v", err)
	}
	defer closeApp(t, reloaded)

	again, err := reloaded.WalletBalances(recipient)
	if err != nil {
		t.Fatalf("wallet balances after replay pass: %v", err)
	}
	if len(again) != 2 {
		t.Fatalf("expected two wallet balances after replay pass, got %d (%+v)", len(again), again)
	}
	againByDenom := make(map[string]string, len(again))
	for _, balance := range again {
		againByDenom[balance.Denom] = balance.Amount
	}
	if againByDenom["ueth"] != gotByDenom["ueth"] || againByDenom["uethusdc"] != gotByDenom["uethusdc"] {
		t.Fatalf("expected replay-safe wallet balances, got %+v after %+v", againByDenom, gotByDenom)
	}
}

func TestPublicWalletDeliveryViaPublicBridgeRelayerAgainstDemoNode(t *testing.T) {
	t.Parallel()

	repo := repoRoot(t)
	homeDir := filepath.Join(t.TempDir(), "public-wallet-demo-node-home")
	cfg := initSDKDemoNodeHome(t, homeDir)

	app, err := aegisapp.LoadWithConfig(cfg)
	if err != nil {
		t.Fatalf("load sdk demo runtime: %v", err)
	}
	if err := registerPublicBridgeAssets(t, app); err != nil {
		t.Fatalf("register public assets: %v", err)
	}
	if err := app.Save(); err != nil {
		t.Fatalf("save sdk demo runtime: %v", err)
	}
	if err := app.Close(); err != nil {
		t.Fatalf("close sdk demo runtime: %v", err)
	}

	readyPath := filepath.Join(t.TempDir(), "demo-node-ready.json")
	cmd, logs := startIBCDemoNodeProcess(t, homeDir, readyPath, map[string]string{
		"AEGISLINK_DEMO_NODE_TICK_INTERVAL_MS": "10",
	})
	defer stopIBCDemoNodeProcess(t, cmd, logs)

	anvil := startAnvilRuntime(t)
	contracts := deployBridgeContractsToAnvil(t, anvil.rpcURL)
	recipient := testCosmosWalletAddress()
	nativeReceipt := createAnvilNativeDeposit(t, anvil.rpcURL, contracts, "1000000000000000000", recipient, "10000000000")
	erc20Receipt := createAnvilDeposit(t, anvil.rpcURL, contracts, "25000000", recipient, "10000000000")

	nativeClaim := relayedDepositClaim(t, bridgetypes.SourceAssetKindNativeETH, contracts.Gateway, "eth", nativeReceipt.TransactionHash, parseHexUint64(t, nativeReceipt.Logs[0].LogIndex), 1, recipient, "1000000000000000000", 10000000000)
	nativeClaim.DestinationChainID = cfg.ChainID
	erc20Claim := relayedDepositClaim(t, bridgetypes.SourceAssetKindERC20, contracts.Gateway, "eth.usdc", erc20Receipt.TransactionHash, parseHexUint64(t, erc20Receipt.Logs[0].LogIndex), 2, recipient, "25000000", 10000000000)
	erc20Claim.DestinationChainID = cfg.ChainID

	attestationPath := filepath.Join(t.TempDir(), "demo-node-attestations.json")
	writeAttestationVotes(t, attestationPath, nativeClaim, erc20Claim)

	replayStore := filepath.Join(t.TempDir(), "demo-node-relayer-replay.json")
	output := runGoCommandWithLocalCacheAndEnv(t, repo, map[string]string{
		"AEGISLINK_RELAYER_COSMOS_CHAIN_ID":        cfg.ChainID,
		"AEGISLINK_RELAYER_EVM_RPC_URL":            anvil.rpcURL,
		"AEGISLINK_RELAYER_EVM_VERIFIER_ADDRESS":   contracts.Verifier,
		"AEGISLINK_RELAYER_EVM_GATEWAY_ADDRESS":    contracts.Gateway,
		"AEGISLINK_RELAYER_EVM_CONFIRMATIONS":      "0",
		"AEGISLINK_RELAYER_COSMOS_CONFIRMATIONS":   "0",
		"AEGISLINK_RELAYER_SUBMISSION_RETRY_LIMIT": "1",
		"AEGISLINK_RELAYER_REPLAY_STORE_PATH":      replayStore,
		"AEGISLINK_RELAYER_ATTESTATION_STATE_PATH": attestationPath,
		"AEGISLINK_RELAYER_AEGISLINK_CMD":          "go",
		"AEGISLINK_RELAYER_AEGISLINK_CMD_ARGS":     "run ./chain/aegislink/cmd/aegislinkd --home " + homeDir + " --demo-node-ready-file " + readyPath,
		"AEGISLINK_RELAYER_AEGISLINK_STATE_PATH":   "",
	}, "run", "./relayer/cmd/public-bridge-relayer")
	if !strings.Contains(output, "run_complete") {
		t.Fatalf("expected relayer success log against demo node, got:\n%s", output)
	}

	balances := waitForWalletDenomsOnDemoNode(t, homeDir, readyPath, recipient, map[string]string{
		"ueth":     "1000000000000000000",
		"uethusdc": "25000000",
	})
	if len(balances) != 2 {
		t.Fatalf("expected two balances on demo node, got %+v", balances)
	}

	secondOutput := runGoCommandWithLocalCacheAndEnv(t, repo, map[string]string{
		"AEGISLINK_RELAYER_COSMOS_CHAIN_ID":        cfg.ChainID,
		"AEGISLINK_RELAYER_EVM_RPC_URL":            anvil.rpcURL,
		"AEGISLINK_RELAYER_EVM_VERIFIER_ADDRESS":   contracts.Verifier,
		"AEGISLINK_RELAYER_EVM_GATEWAY_ADDRESS":    contracts.Gateway,
		"AEGISLINK_RELAYER_EVM_CONFIRMATIONS":      "0",
		"AEGISLINK_RELAYER_COSMOS_CONFIRMATIONS":   "0",
		"AEGISLINK_RELAYER_SUBMISSION_RETRY_LIMIT": "1",
		"AEGISLINK_RELAYER_REPLAY_STORE_PATH":      replayStore,
		"AEGISLINK_RELAYER_ATTESTATION_STATE_PATH": attestationPath,
		"AEGISLINK_RELAYER_AEGISLINK_CMD":          "go",
		"AEGISLINK_RELAYER_AEGISLINK_CMD_ARGS":     "run ./chain/aegislink/cmd/aegislinkd --home " + homeDir + " --demo-node-ready-file " + readyPath,
		"AEGISLINK_RELAYER_AEGISLINK_STATE_PATH":   "",
	}, "run", "./relayer/cmd/public-bridge-relayer")
	if !strings.Contains(secondOutput, "run_complete") {
		t.Fatalf("expected second relayer success log against demo node, got:\n%s", secondOutput)
	}

	again := waitForWalletDenomsOnDemoNode(t, homeDir, readyPath, recipient, map[string]string{
		"ueth":     "1000000000000000000",
		"uethusdc": "25000000",
	})
	if len(again) != len(balances) {
		t.Fatalf("expected replay-safe balance count, got %+v after %+v", again, balances)
	}
	for denom, amount := range balances {
		if again[denom] != amount {
			t.Fatalf("expected replay-safe balance for %s to stay %s, got %+v after %+v", denom, amount, again, balances)
		}
	}
}

func submitClaim(t *testing.T, app *aegisapp.App, claim bridgetypes.DepositClaim) error {
	t.Helper()

	attestation := testAttestationForClaim(t, claim)
	_, err := app.SubmitDepositClaim(claim, attestation)
	return err
}

func depositClaim(t *testing.T, sourceAssetKind, sourceContract, assetID, txHash string, logIndex, nonce uint64, recipient, amount string) bridgetypes.DepositClaim {
	t.Helper()
	return relayedDepositClaim(t, sourceAssetKind, sourceContract, assetID, txHash, logIndex, nonce, recipient, amount, 120)
}

func relayedDepositClaim(t *testing.T, sourceAssetKind, sourceContract, assetID, txHash string, logIndex, nonce uint64, recipient, amount string, deadline uint64) bridgetypes.DepositClaim {
	t.Helper()

	identity := bridgetypes.ClaimIdentity{
		Kind:            bridgetypes.ClaimKindDeposit,
		SourceAssetKind: sourceAssetKind,
		SourceChainID:   "11155111",
		SourceContract:  sourceContract,
		SourceTxHash:    txHash,
		SourceLogIndex:  logIndex,
		Nonce:           nonce,
	}
	identity.MessageID = identity.DerivedMessageID()

	return bridgetypes.DepositClaim{
		Identity:           identity,
		DestinationChainID: "aegislink-public-1",
		AssetID:            assetID,
		Amount:             mustWalletAmount(t, amount),
		Recipient:          recipient,
		Deadline:           deadline,
	}
}

func testAttestationForClaim(t *testing.T, claim bridgetypes.DepositClaim) bridgetypes.Attestation {
	t.Helper()

	attestation := bridgetypes.Attestation{
		MessageID:        claim.Identity.MessageID,
		PayloadHash:      claim.Digest(),
		Signers:          bridgetestutil.DefaultHarnessSignerAddresses()[:2],
		Threshold:        2,
		Expiry:           200,
		SignerSetVersion: 1,
	}
	for _, key := range bridgetestutil.DefaultHarnessSignerPrivateKeys()[:2] {
		proof, err := bridgetypes.SignAttestationWithPrivateKeyHex(attestation, key)
		if err != nil {
			t.Fatalf("sign attestation: %v", err)
		}
		attestation.Proofs = append(attestation.Proofs, proof)
	}
	return attestation
}

func mustWalletAmount(t *testing.T, value string) *big.Int {
	t.Helper()

	amount, ok := new(big.Int).SetString(value, 10)
	if !ok {
		t.Fatalf("invalid amount %q", value)
	}
	return amount
}

func decodeWalletBalances(t *testing.T, raw string) []struct {
	Address string `json:"address"`
	Denom   string `json:"denom"`
	Amount  string `json:"amount"`
} {
	t.Helper()

	var balances []struct {
		Address string `json:"address"`
		Denom   string `json:"denom"`
		Amount  string `json:"amount"`
	}
	if err := json.Unmarshal([]byte(raw), &balances); err != nil {
		t.Fatalf("decode balances fixture: %v", err)
	}
	return balances
}

func testCosmosWalletAddress() string {
	return sdk.AccAddress([]byte("e2e-wallet-recipient")).String()
}

func runGoCommandWithLocalCache(t *testing.T, dir string, args ...string) string {
	t.Helper()
	return runGoCommandWithLocalCacheAndEnv(t, dir, nil, args...)
}

func runGoCommandWithLocalCacheAndEnv(t *testing.T, dir string, extraEnv map[string]string, args ...string) string {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = dir
	cmd.Env = append([]string{}, os.Environ()...)
	cmd.Env = append(cmd.Env,
		"GOCACHE=/tmp/aegislink-gocache",
		"GOMODCACHE=/Users/ayushns01/go/pkg/mod",
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

type goCommandResult struct {
	Stdout string
	Err    error
}

func runGoCommandWithLocalCacheAllowError(t *testing.T, dir string, args ...string) goCommandResult {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = dir
	cmd.Env = append([]string{}, os.Environ()...)
	cmd.Env = append(cmd.Env,
		"GOCACHE=/tmp/aegislink-gocache",
		"GOMODCACHE=/Users/ayushns01/go/pkg/mod",
	)

	output, err := cmd.CombinedOutput()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		t.Fatalf("command timed out: go %s\n%s", strings.Join(args, " "), output)
	}
	return goCommandResult{
		Stdout: string(output),
		Err:    err,
	}
}

func closeApp(t *testing.T, app *aegisapp.App) {
	t.Helper()
	if err := app.Close(); err != nil {
		t.Fatalf("close app: %v", err)
	}
}

func waitForWalletDenomsOnDemoNode(t *testing.T, homeDir, readyPath, address string, want map[string]string) map[string]string {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		balances, err := networked.QueryBalances(context.Background(), networked.Config{
			HomeDir:   homeDir,
			ReadyFile: readyPath,
		}, address)
		if err == nil {
			got := make(map[string]string, len(balances))
			for _, balance := range balances {
				got[balance.Denom] = balance.Amount
			}
			if len(got) == len(want) {
				allMatched := true
				for denom, amount := range want {
					if got[denom] != amount {
						allMatched = false
						break
					}
				}
				if allMatched {
					return got
				}
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for demo-node balances at %s to reach %+v", address, want)
	return nil
}

func parseHexUint64(t *testing.T, value string) uint64 {
	t.Helper()

	trimmed := strings.TrimPrefix(strings.TrimSpace(value), "0x")
	if trimmed == "" {
		trimmed = "0"
	}
	parsed, err := strconv.ParseUint(trimmed, 16, 64)
	if err != nil {
		t.Fatalf("parse hex uint64 %q: %v", value, err)
	}
	return parsed
}

func writeAttestationVotes(t *testing.T, path string, claims ...bridgetypes.DepositClaim) {
	t.Helper()

	type persistedVote struct {
		MessageID   string `json:"message_id"`
		PayloadHash string `json:"payload_hash"`
		Signer      string `json:"signer"`
		Expiry      uint64 `json:"expiry"`
	}
	type persistedVotes struct {
		Votes []persistedVote `json:"votes"`
	}

	signers := bridgetestutil.DefaultHarnessSignerAddresses()[:2]
	state := persistedVotes{}
	for _, claim := range claims {
		for _, signer := range signers {
			state.Votes = append(state.Votes, persistedVote{
				MessageID:   claim.Identity.MessageID,
				PayloadHash: claim.Digest(),
				Signer:      signer,
				Expiry:      claim.Deadline,
			})
		}
	}

	encoded, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("marshal attestation votes: %v", err)
	}
	if err := os.WriteFile(path, encoded, 0o644); err != nil {
		t.Fatalf("write attestation votes: %v", err)
	}
}
