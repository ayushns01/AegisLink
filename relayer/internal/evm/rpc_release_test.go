package evm

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const anvilFirstAccountPrivateKey = "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"

func TestRPCReleaseTargetExecutesLiveGatewayRelease(t *testing.T) {
	t.Parallel()

	port := reservePort(t)
	rpcURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, "anvil", "--silent", "--port", fmt.Sprintf("%d", port), "--chain-id", "11155111")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start anvil: %v", err)
	}
	defer func() {
		cancel()
		_ = cmd.Wait()
	}()

	waitForRPC(t, rpcURL)

	accounts := rpcAccounts(t, rpcURL)
	owner := accounts[0]
	recipient := accounts[2]

	repo := repoRoot(t)
	verifier := deployContract(t, rpcURL, owner, filepath.Join(repo, "contracts/ethereum/out/BridgeVerifier.sol/BridgeVerifier.json"), "constructor(address)", owner)
	gateway := deployContract(t, rpcURL, owner, filepath.Join(repo, "contracts/ethereum/out/BridgeGateway.sol/BridgeGateway.json"), "constructor(address)", verifier)
	sendTx(t, rpcURL, owner, verifier, castCalldata(t, "setGateway(address)", gateway))

	token := deployContract(
		t,
		rpcURL,
		owner,
		filepath.Join(repo, "contracts/ethereum/out/BridgeGateway.t.sol/TestToken.json"),
		"constructor(string,string,uint8)",
		"USDC",
		"USDC",
		"6",
	)
	sendTx(t, rpcURL, owner, token, castCalldata(t, "mint(address,uint256)", gateway, "100000000"))
	sendTx(t, rpcURL, owner, gateway, castCalldata(t, "setSupportedAsset(address,string,bool)", token, "eth.usdc", "true"))

	messageID := castKeccak(t, "release-1")
	amount := big.NewInt(25000000)
	expiry := uint64(10000000000)
	signature := signReleaseAttestation(t, verifier, gateway, token, recipient, amount, messageID, expiry)

	target := NewRPCReleaseTarget(rpcURL, gateway)
	txHash, err := target.ReleaseWithdrawal(context.Background(), ReleaseRequest{
		MessageID:    messageID,
		AssetAddress: token,
		Amount:       amount,
		Recipient:    recipient,
		Deadline:     expiry,
		Signature:    signature,
	})
	if err != nil {
		t.Fatalf("release withdrawal: %v", err)
	}
	if strings.TrimSpace(txHash) == "" {
		t.Fatal("expected release transaction hash")
	}

	if balance := tokenBalanceOf(t, rpcURL, token, recipient); balance.String() != amount.String() {
		t.Fatalf("expected recipient balance %s, got %s", amount.String(), balance.String())
	}
	if !verifierUsedProof(t, rpcURL, verifier, messageID) {
		t.Fatalf("expected verifier proof for %s to be marked used", messageID)
	}
}

func TestRPCReleaseTargetExecutesLiveGatewayReleaseWithPrivateKey(t *testing.T) {
	t.Parallel()

	port := reservePort(t)
	rpcURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, "anvil", "--silent", "--port", fmt.Sprintf("%d", port), "--chain-id", "11155111")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start anvil: %v", err)
	}
	defer func() {
		cancel()
		_ = cmd.Wait()
	}()

	waitForRPC(t, rpcURL)

	accounts := rpcAccounts(t, rpcURL)
	owner := accounts[0]
	recipient := accounts[2]

	repo := repoRoot(t)
	verifier := deployContract(t, rpcURL, owner, filepath.Join(repo, "contracts/ethereum/out/BridgeVerifier.sol/BridgeVerifier.json"), "constructor(address)", owner)
	gateway := deployContract(t, rpcURL, owner, filepath.Join(repo, "contracts/ethereum/out/BridgeGateway.sol/BridgeGateway.json"), "constructor(address)", verifier)
	sendTx(t, rpcURL, owner, verifier, castCalldata(t, "setGateway(address)", gateway))
	sendTxWithValue(t, rpcURL, owner, gateway, castCalldata(t, "depositETH(string,uint64)", "cosmos1gateway-fund", "10000000000"), "25000000")

	messageID := castKeccak(t, "native-release-1")
	amount := big.NewInt(25000000)
	expiry := uint64(10000000000)
	signature := signReleaseAttestation(t, verifier, gateway, "0x0000000000000000000000000000000000000000", recipient, amount, messageID, expiry)

	target := NewRPCReleaseTargetWithSigner(rpcURL, gateway, anvilFirstAccountPrivateKey, "")
	txHash, err := target.ReleaseWithdrawal(context.Background(), ReleaseRequest{
		MessageID:    messageID,
		AssetAddress: "0x0000000000000000000000000000000000000000",
		Amount:       amount,
		Recipient:    recipient,
		Deadline:     expiry,
		Signature:    signature,
	})
	if err != nil {
		t.Fatalf("release withdrawal with private key: %v", err)
	}
	if strings.TrimSpace(txHash) == "" {
		t.Fatal("expected release transaction hash")
	}
}

func signReleaseAttestation(
	t *testing.T,
	verifier,
	gateway,
	asset,
	recipient string,
	amount *big.Int,
	messageID string,
	expiry uint64,
) []byte {
	t.Helper()

	payloadInput := castAbiEncode(
		t,
		"f(uint256,address,address,address,uint256,bytes32,uint64)",
		"11155111",
		gateway,
		asset,
		recipient,
		amount.String(),
		messageID,
		fmt.Sprintf("%d", expiry),
	)
	payloadHash := castKeccak(t, payloadInput)
	digest := bridgeVerifierTypedDigest(t, verifier, messageID, payloadHash, expiry)

	cmd := exec.Command("cast", "wallet", "sign", "--private-key", anvilFirstAccountPrivateKey, "--no-hash", digest)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("cast wallet sign failed: %v\n%s", err, output)
	}
	signatureHex := strings.TrimSpace(string(output))
	signature, err := hex.DecodeString(strings.TrimPrefix(signatureHex, "0x"))
	if err != nil {
		t.Fatalf("decode signature %q: %v", signatureHex, err)
	}
	return signature
}

func bridgeVerifierTypedDigest(t *testing.T, verifier, messageID, payloadHash string, expiry uint64) string {
	t.Helper()

	domainTypeHash := castKeccak(t, "EIP712Domain(string name,string version,uint256 chainId,address verifyingContract)")
	nameHash := castKeccak(t, "AegisLink Bridge Verifier")
	versionHash := castKeccak(t, "1")
	domainInput := castAbiEncode(
		t,
		"f(bytes32,bytes32,bytes32,uint256,address)",
		domainTypeHash,
		nameHash,
		versionHash,
		"11155111",
		verifier,
	)
	domainSeparator := castKeccak(t, domainInput)

	typeHash := castKeccak(t, "BridgeAttestation(bytes32 messageId,bytes32 payloadHash,uint64 expiry)")
	structInput := castAbiEncode(t, "f(bytes32,bytes32,bytes32,uint64)", typeHash, messageID, payloadHash, fmt.Sprintf("%d", expiry))
	structHash := castKeccak(t, structInput)
	return castKeccak(
		t,
		"0x1901"+strings.TrimPrefix(domainSeparator, "0x")+strings.TrimPrefix(structHash, "0x"),
	)
}

func castKeccak(t *testing.T, value string) string {
	t.Helper()

	cmd := exec.Command("cast", "keccak", value)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("cast keccak failed: %v\n%s", err, output)
	}
	return strings.TrimSpace(string(output))
}

func tokenBalanceOf(t *testing.T, rpcURL, token, account string) *big.Int {
	t.Helper()

	raw := ethCall(t, rpcURL, token, castCalldata(t, "balanceOf(address)", account))
	balance, err := parseHexBigIntString(raw)
	if err != nil {
		t.Fatalf("parse token balance %q: %v", raw, err)
	}
	return balance
}

func verifierUsedProof(t *testing.T, rpcURL, verifier, messageID string) bool {
	t.Helper()

	raw := ethCall(t, rpcURL, verifier, castCalldata(t, "usedProofs(bytes32)", messageID))
	trimmed := strings.TrimPrefix(raw, "0x")
	return strings.TrimLeft(trimmed, "0") == "1"
}

func ethCall(t *testing.T, rpcURL, to, data string) string {
	t.Helper()

	return rpcCallResult[string](t, rpcURL, "eth_call", []any{map[string]any{
		"to":   to,
		"data": data,
	}, "latest"})
}
