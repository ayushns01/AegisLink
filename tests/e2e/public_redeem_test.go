package e2e

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	aegisapp "github.com/ayushns01/aegislink/chain/aegislink/app"
	bridgetypes "github.com/ayushns01/aegislink/chain/aegislink/x/bridge/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

type publicRedeemSpec struct {
	name              string
	assetID           string
	denom             string
	depositAmount     string
	sourceAssetKind   string
	makeDeposit       func(t *testing.T, rpcURL string, contracts chainContracts, amount, recipient, deadline string) txReceipt
	releaseAsset      func(contracts chainContracts) string
	releasedBalanceOf func(t *testing.T, rpcURL string, contracts chainContracts, recipient string) *big.Int
}

type publicRedeemRun struct {
	repo         string
	cfg          aegisapp.Config
	homeDir      string
	replayStore  string
	recipient    string
	evmRecipient string
	contracts    chainContracts
	anvil        *anvilRuntime
	spec         publicRedeemSpec
}

type publicReplayState struct {
	Checkpoints map[string]uint64 `json:"checkpoints"`
	Processed   []string          `json:"processed"`
}

func TestPublicRedeemBackToEthereumViaPublicBridgeRelayer(t *testing.T) {
	t.Parallel()

	result := runSuccessfulPublicRedeem(t, publicRedeemSpec{
		name:            "native eth",
		assetID:         "eth",
		denom:           "ueth",
		depositAmount:   "1000000000000000000",
		sourceAssetKind: bridgetypes.SourceAssetKindNativeETH,
		makeDeposit:     createAnvilNativeDeposit,
		releaseAsset: func(_ chainContracts) string {
			return "0x0000000000000000000000000000000000000000"
		},
		releasedBalanceOf: func(t *testing.T, rpcURL string, _ chainContracts, recipient string) *big.Int {
			return nativeBalanceOf(t, rpcURL, recipient)
		},
	})

	assertWalletBurned(t, result)
}

func TestPublicRedeemBackToEthereumViaPublicBridgeRelayerAgainstDemoNode(t *testing.T) {
	t.Parallel()

	repo := repoRoot(t)
	homeDir := filepath.Join(t.TempDir(), "public-redeem-demo-node-home")
	cfg := initSDKDemoNodeHome(t, homeDir)

	anvil := startAnvilRuntime(t)
	contracts := deployBridgeContractsToAnvil(t, anvil.rpcURL)

	app, err := aegisapp.LoadWithConfig(cfg)
	if err != nil {
		t.Fatalf("load sdk demo runtime: %v", err)
	}
	if err := registerPublicBridgeAssetsWithERC20Address(t, app, contracts.Token); err != nil {
		t.Fatalf("register public assets: %v", err)
	}
	if err := app.Save(); err != nil {
		t.Fatalf("save sdk demo runtime: %v", err)
	}
	closeApp(t, app)

	readyPath := filepath.Join(t.TempDir(), "demo-node-ready.json")
	cmd, logs := startIBCDemoNodeProcess(t, homeDir, readyPath, map[string]string{
		"AEGISLINK_DEMO_NODE_TICK_INTERVAL_MS": "10",
	})
	defer stopIBCDemoNodeProcess(t, cmd, logs)

	recipient := sdk.AccAddress([]byte("wallet-demo-eth")).String()
	evmRecipient := rpcAccounts(t, anvil.rpcURL)[2]
	deadline := "10000000000"
	depositAmount := "1000000000000000000"
	receipt := createAnvilNativeDeposit(t, anvil.rpcURL, contracts, depositAmount, recipient, deadline)

	claim := relayedDepositClaim(
		t,
		bridgetypes.SourceAssetKindNativeETH,
		contracts.Gateway,
		"eth",
		receipt.TransactionHash,
		parseHexUint64(t, receipt.Logs[0].LogIndex),
		1,
		recipient,
		depositAmount,
		10000000000,
	)
	claim.DestinationChainID = cfg.ChainID

	attestationPath := filepath.Join(t.TempDir(), "demo-node-attestations.json")
	writeAttestationVotes(t, attestationPath, claim)

	replayStore := filepath.Join(t.TempDir(), "public-redeem-demo-node-replay.json")
	depositOutput := runGoCommandWithLocalCacheAndEnv(t, repo, map[string]string{
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
	if !strings.Contains(depositOutput, "run_complete") {
		t.Fatalf("expected deposit relayer success log against demo node, got:\n%s", depositOutput)
	}

	waitForWalletDenomsOnDemoNode(t, homeDir, readyPath, recipient, map[string]string{
		"ueth": depositAmount,
	})

	amount := mustWalletAmount(t, depositAmount)
	expectedMessageID := predictWithdrawalMessageID(60, 1, "eth", evmRecipient, amount)
	signature := signWithdrawalReleaseAttestation(
		t,
		contracts.Verifier,
		contracts.Gateway,
		"0x0000000000000000000000000000000000000000",
		evmRecipient,
		amount,
		expectedMessageID,
		10000000000,
	)
	withdrawOutput := runGoCommandWithLocalCacheAndEnv(t, repo, nil,
		"run", "./chain/aegislink/cmd/aegislinkd",
		"tx", "execute-withdrawal",
		"--home", homeDir,
		"--demo-node-ready-file", readyPath,
		"--owner-address", recipient,
		"--asset-id", "eth",
		"--amount", depositAmount,
		"--recipient", evmRecipient,
		"--deadline", deadline,
		"--signature-base64", base64.StdEncoding.EncodeToString(signature),
		"--height", "60",
	)
	if !strings.Contains(withdrawOutput, expectedMessageID) {
		t.Fatalf("expected withdrawal message id %s in demo-node output, got:\n%s", expectedMessageID, withdrawOutput)
	}

	beforeBalance := nativeBalanceOf(t, anvil.rpcURL, evmRecipient)
	redeemOutput := runGoCommandWithLocalCacheAndEnv(t, repo, map[string]string{
		"AEGISLINK_RELAYER_COSMOS_CHAIN_ID":                cfg.ChainID,
		"AEGISLINK_RELAYER_EVM_RPC_URL":                    anvil.rpcURL,
		"AEGISLINK_RELAYER_EVM_VERIFIER_ADDRESS":           contracts.Verifier,
		"AEGISLINK_RELAYER_EVM_GATEWAY_ADDRESS":            contracts.Gateway,
		"AEGISLINK_RELAYER_EVM_RELEASE_SIGNER_PRIVATE_KEY": anvilFirstAccountPrivateKey,
		"AEGISLINK_RELAYER_EVM_CONFIRMATIONS":              "0",
		"AEGISLINK_RELAYER_COSMOS_CONFIRMATIONS":           "0",
		"AEGISLINK_RELAYER_SUBMISSION_RETRY_LIMIT":         "1",
		"AEGISLINK_RELAYER_REPLAY_STORE_PATH":              replayStore,
		"AEGISLINK_RELAYER_ATTESTATION_STATE_PATH":         attestationPath,
		"AEGISLINK_RELAYER_AEGISLINK_CMD":                  "go",
		"AEGISLINK_RELAYER_AEGISLINK_CMD_ARGS":             "run ./chain/aegislink/cmd/aegislinkd --home " + homeDir + " --demo-node-ready-file " + readyPath,
		"AEGISLINK_RELAYER_AEGISLINK_STATE_PATH":           "",
	}, "run", "./relayer/cmd/public-bridge-relayer")
	if !strings.Contains(redeemOutput, "run_complete") {
		t.Fatalf("expected redeem relayer success log against demo node, got:\n%s", redeemOutput)
	}

	afterBalance := nativeBalanceOf(t, anvil.rpcURL, evmRecipient)
	delta := new(big.Int).Sub(afterBalance, beforeBalance)
	if delta.String() != depositAmount {
		t.Fatalf("expected released balance delta %s, got %s", depositAmount, delta.String())
	}

	burned := waitForWalletDenomsOnDemoNode(t, homeDir, readyPath, recipient, map[string]string{})
	if len(burned) != 0 {
		t.Fatalf("expected demo-node wallet balance to burn to zero, got %+v", burned)
	}
}

func TestPublicRedeemERC20BackToEthereumViaPublicBridgeRelayer(t *testing.T) {
	t.Parallel()

	result := runSuccessfulPublicRedeem(t, publicRedeemSpec{
		name:            "erc20",
		assetID:         "eth.usdc",
		denom:           "uethusdc",
		depositAmount:   "25000000",
		sourceAssetKind: bridgetypes.SourceAssetKindERC20,
		makeDeposit:     createAnvilDeposit,
		releaseAsset: func(contracts chainContracts) string {
			return contracts.Token
		},
		releasedBalanceOf: func(t *testing.T, rpcURL string, contracts chainContracts, recipient string) *big.Int {
			return tokenBalanceOf(t, rpcURL, contracts.Token, recipient)
		},
	})

	assertWalletBurned(t, result)
}

func TestPublicRedeemReplayIsRejectedByGateway(t *testing.T) {
	t.Parallel()

	result := runSuccessfulPublicRedeem(t, publicRedeemSpec{
		name:            "native replay",
		assetID:         "eth",
		denom:           "ueth",
		depositAmount:   "1000000000000000000",
		sourceAssetKind: bridgetypes.SourceAssetKindNativeETH,
		makeDeposit:     createAnvilNativeDeposit,
		releaseAsset: func(_ chainContracts) string {
			return "0x0000000000000000000000000000000000000000"
		},
		releasedBalanceOf: func(t *testing.T, rpcURL string, _ chainContracts, recipient string) *big.Int {
			return nativeBalanceOf(t, rpcURL, recipient)
		},
	})

	beforeReplayBalance := result.spec.releasedBalanceOf(t, result.anvil.rpcURL, result.contracts, result.evmRecipient)
	originalReplay := loadPublicReplayState(t, result.replayStore)
	replayRetryStore := filepath.Join(t.TempDir(), "public-redeem-retry-replay.json")
	writePublicReplayState(t, replayRetryStore, publicReplayState{
		Checkpoints: map[string]uint64{
			"evm-deposits":       originalReplay.Checkpoints["evm-deposits"],
			"cosmos-withdrawals": 0,
		},
	})

	output, err := runGoCommandWithLocalCacheAndEnvAllowError(t, result.repo, map[string]string{
		"AEGISLINK_RELAYER_COSMOS_CHAIN_ID":                "aegislink-public-1",
		"AEGISLINK_RELAYER_EVM_RPC_URL":                    result.anvil.rpcURL,
		"AEGISLINK_RELAYER_EVM_VERIFIER_ADDRESS":           result.contracts.Verifier,
		"AEGISLINK_RELAYER_EVM_GATEWAY_ADDRESS":            result.contracts.Gateway,
		"AEGISLINK_RELAYER_EVM_RELEASE_SIGNER_PRIVATE_KEY": anvilFirstAccountPrivateKey,
		"AEGISLINK_RELAYER_EVM_CONFIRMATIONS":              "0",
		"AEGISLINK_RELAYER_COSMOS_CONFIRMATIONS":           "0",
		"AEGISLINK_RELAYER_SUBMISSION_RETRY_LIMIT":         "1",
		"AEGISLINK_RELAYER_REPLAY_STORE_PATH":              replayRetryStore,
		"AEGISLINK_RELAYER_AEGISLINK_CMD":                  "go",
		"AEGISLINK_RELAYER_AEGISLINK_CMD_ARGS":             "run ./chain/aegislink/cmd/aegislinkd --home " + result.homeDir,
	}, "run", "./relayer/cmd/public-bridge-relayer")
	if err == nil {
		t.Fatalf("expected replayed redeem release to fail, got success:\n%s", output)
	}
	if !strings.Contains(output, "run_failed") && !strings.Contains(output, "run_retry") {
		t.Fatalf("expected relayer failure output, got:\n%s", output)
	}
	if !strings.Contains(output, "release withdrawal") {
		t.Fatalf("expected withdrawal release failure in output, got:\n%s", output)
	}

	afterReplayBalance := result.spec.releasedBalanceOf(t, result.anvil.rpcURL, result.contracts, result.evmRecipient)
	if afterReplayBalance.Cmp(beforeReplayBalance) != 0 {
		t.Fatalf("expected replayed redeem to leave recipient balance unchanged, got before=%s after=%s", beforeReplayBalance.String(), afterReplayBalance.String())
	}
}

func runSuccessfulPublicRedeem(t *testing.T, spec publicRedeemSpec) publicRedeemRun {
	t.Helper()

	repo := repoRoot(t)
	homeDir := filepath.Join(t.TempDir(), "public-redeem-home")
	cfg, err := aegisapp.InitHome(aegisapp.Config{
		HomeDir:     homeDir,
		ChainID:     "aegislink-public-1",
		RuntimeMode: aegisapp.RuntimeModeSDKStore,
	}, false)
	if err != nil {
		t.Fatalf("init home: %v", err)
	}

	anvil := startAnvilRuntime(t)
	contracts := deployBridgeContractsToAnvil(t, anvil.rpcURL)

	app, err := aegisapp.LoadWithConfig(cfg)
	if err != nil {
		t.Fatalf("load runtime: %v", err)
	}
	if err := registerPublicBridgeAssetsWithERC20Address(t, app, contracts.Token); err != nil {
		t.Fatalf("register public assets: %v", err)
	}
	if err := app.Save(); err != nil {
		t.Fatalf("save runtime: %v", err)
	}
	closeApp(t, app)

	recipient := sdk.AccAddress([]byte("wallet-" + spec.assetID)).String()
	evmRecipient := rpcAccounts(t, anvil.rpcURL)[2]
	deadline := "10000000000"
	receipt := spec.makeDeposit(t, anvil.rpcURL, contracts, spec.depositAmount, recipient, deadline)

	attestationPath := filepath.Join(t.TempDir(), "public-attestations.json")
	claim := relayedDepositClaim(
		t,
		spec.sourceAssetKind,
		contracts.Gateway,
		spec.assetID,
		receipt.TransactionHash,
		parseHexUint64(t, receipt.Logs[0].LogIndex),
		1,
		recipient,
		spec.depositAmount,
		10000000000,
	)
	writeAttestationVotes(t, attestationPath, claim)

	replayStore := filepath.Join(t.TempDir(), "public-redeem-replay.json")
	depositOutput := runGoCommandWithLocalCacheAndEnv(t, repo, map[string]string{
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
	}, "run", "./relayer/cmd/public-bridge-relayer")
	if !strings.Contains(depositOutput, "run_complete") {
		t.Fatalf("expected deposit relayer success log, got:\n%s", depositOutput)
	}

	loaded, err := aegisapp.LoadWithConfig(cfg)
	if err != nil {
		t.Fatalf("reload runtime after deposit: %v", err)
	}
	if balance := loaded.BankKeeper.BalanceOf(recipient, spec.denom); balance.String() != spec.depositAmount {
		t.Fatalf("expected bridged wallet balance %s, got %s", spec.depositAmount, balance.String())
	}
	closeApp(t, loaded)

	amount := mustWalletAmount(t, spec.depositAmount)
	expectedMessageID := predictWithdrawalMessageID(60, 1, spec.assetID, evmRecipient, amount)
	signature := signWithdrawalReleaseAttestation(
		t,
		contracts.Verifier,
		contracts.Gateway,
		spec.releaseAsset(contracts),
		evmRecipient,
		amount,
		expectedMessageID,
		10000000000,
	)
	withdrawOutput := runGoCommandWithLocalCacheAndEnv(t, repo, nil,
		"run", "./chain/aegislink/cmd/aegislinkd",
		"tx", "execute-withdrawal",
		"--home", homeDir,
		"--owner-address", recipient,
		"--asset-id", spec.assetID,
		"--amount", spec.depositAmount,
		"--recipient", evmRecipient,
		"--deadline", deadline,
		"--signature-base64", base64.StdEncoding.EncodeToString(signature),
		"--height", "60",
	)
	if !strings.Contains(withdrawOutput, expectedMessageID) {
		t.Fatalf("expected withdrawal message id %s in output, got:\n%s", expectedMessageID, withdrawOutput)
	}

	beforeBalance := spec.releasedBalanceOf(t, anvil.rpcURL, contracts, evmRecipient)
	redeemOutput := runGoCommandWithLocalCacheAndEnv(t, repo, map[string]string{
		"AEGISLINK_RELAYER_COSMOS_CHAIN_ID":                "aegislink-public-1",
		"AEGISLINK_RELAYER_EVM_RPC_URL":                    anvil.rpcURL,
		"AEGISLINK_RELAYER_EVM_VERIFIER_ADDRESS":           contracts.Verifier,
		"AEGISLINK_RELAYER_EVM_GATEWAY_ADDRESS":            contracts.Gateway,
		"AEGISLINK_RELAYER_EVM_RELEASE_SIGNER_PRIVATE_KEY": anvilFirstAccountPrivateKey,
		"AEGISLINK_RELAYER_EVM_CONFIRMATIONS":              "0",
		"AEGISLINK_RELAYER_COSMOS_CONFIRMATIONS":           "0",
		"AEGISLINK_RELAYER_SUBMISSION_RETRY_LIMIT":         "1",
		"AEGISLINK_RELAYER_REPLAY_STORE_PATH":              replayStore,
		"AEGISLINK_RELAYER_ATTESTATION_STATE_PATH":         attestationPath,
		"AEGISLINK_RELAYER_AEGISLINK_CMD":                  "go",
		"AEGISLINK_RELAYER_AEGISLINK_CMD_ARGS":             "run ./chain/aegislink/cmd/aegislinkd --home " + homeDir,
	}, "run", "./relayer/cmd/public-bridge-relayer")
	if !strings.Contains(redeemOutput, "run_complete") {
		t.Fatalf("expected redeem relayer success log, got:\n%s", redeemOutput)
	}

	afterBalance := spec.releasedBalanceOf(t, anvil.rpcURL, contracts, evmRecipient)
	delta := new(big.Int).Sub(afterBalance, beforeBalance)
	if delta.String() != spec.depositAmount {
		t.Fatalf("expected released balance delta %s, got %s", spec.depositAmount, delta.String())
	}

	return publicRedeemRun{
		repo:         repo,
		cfg:          cfg,
		homeDir:      homeDir,
		replayStore:  replayStore,
		recipient:    recipient,
		evmRecipient: evmRecipient,
		contracts:    contracts,
		anvil:        anvil,
		spec:         spec,
	}
}

func assertWalletBurned(t *testing.T, result publicRedeemRun) {
	t.Helper()

	reloaded, err := aegisapp.LoadWithConfig(result.cfg)
	if err != nil {
		t.Fatalf("reload runtime after redeem: %v", err)
	}
	if balance := reloaded.BankKeeper.BalanceOf(result.recipient, result.spec.denom); balance.Sign() != 0 {
		t.Fatalf("expected wallet balance to burn to zero, got %s", balance.String())
	}
	if supply := reloaded.BridgeKeeper.SupplyForDenom(result.spec.denom); supply.Sign() != 0 {
		t.Fatalf("expected bridged supply to burn to zero, got %s", supply.String())
	}
	closeApp(t, reloaded)
}

func loadPublicReplayState(t *testing.T, path string) publicReplayState {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read replay state: %v", err)
	}

	var state publicReplayState
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatalf("decode replay state: %v", err)
	}
	return state
}

func writePublicReplayState(t *testing.T, path string, state publicReplayState) {
	t.Helper()

	if state.Checkpoints == nil {
		state.Checkpoints = map[string]uint64{}
	}
	encoded, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		t.Fatalf("marshal replay state: %v", err)
	}
	if err := os.WriteFile(path, encoded, 0o644); err != nil {
		t.Fatalf("write replay state: %v", err)
	}
}

func runGoCommandWithLocalCacheAndEnvAllowError(t *testing.T, dir string, extraEnv map[string]string, args ...string) (string, error) {
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
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		t.Fatalf("command timed out: go %s\n%s", strings.Join(args, " "), output)
	}
	return string(output), err
}
