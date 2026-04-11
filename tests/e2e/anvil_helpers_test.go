package e2e

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	bridgetypes "github.com/ayushns01/aegislink/chain/aegislink/x/bridge/types"
)

type anvilRuntime struct {
	rpcURL string
	cancel context.CancelFunc
	cmd    *exec.Cmd
}

type chainContracts struct {
	Verifier string
	Gateway  string
	Token    string
	User     string
}

type txReceipt struct {
	ContractAddress string `json:"contractAddress"`
	TransactionHash string `json:"transactionHash"`
	Logs            []struct {
		LogIndex string `json:"logIndex"`
	} `json:"logs"`
}

const anvilFirstAccountPrivateKey = "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"

func startAnvilRuntime(t *testing.T) *anvilRuntime {
	t.Helper()

	port := reservePort(t)
	rpcURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	ctx, cancel := context.WithCancel(context.Background())

	cmd := exec.CommandContext(ctx, "anvil", "--silent", "--port", fmt.Sprintf("%d", port), "--chain-id", "11155111")
	if err := cmd.Start(); err != nil {
		cancel()
		t.Fatalf("start anvil: %v", err)
	}

	runtime := &anvilRuntime{
		rpcURL: rpcURL,
		cancel: cancel,
		cmd:    cmd,
	}
	t.Cleanup(func() {
		cancel()
		_ = cmd.Wait()
	})

	waitForRPC(t, rpcURL)
	return runtime
}

func deployBridgeContractsToAnvil(t *testing.T, rpcURL string) chainContracts {
	t.Helper()

	accounts := rpcAccounts(t, rpcURL)
	owner := accounts[0]
	user := accounts[1]
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
	sendTx(t, rpcURL, owner, token, castCalldata(t, "mint(address,uint256)", user, "100000000"))
	sendTx(t, rpcURL, owner, gateway, castCalldata(t, "setSupportedAsset(address,string,bool)", token, "eth.usdc", "true"))

	return chainContracts{
		Verifier: verifier,
		Gateway:  gateway,
		Token:    token,
		User:     user,
	}
}

func createAnvilDeposit(t *testing.T, rpcURL string, contracts chainContracts, amount, recipient, expiry string) txReceipt {
	t.Helper()
	sendTx(t, rpcURL, contracts.User, contracts.Token, castCalldata(t, "approve(address,uint256)", contracts.Gateway, amount))
	return sendTx(t, rpcURL, contracts.User, contracts.Gateway, castCalldata(t, "deposit(address,uint256,string,uint64)", contracts.Token, amount, recipient, expiry))
}

func createAnvilNativeDeposit(t *testing.T, rpcURL string, contracts chainContracts, amount, recipient, expiry string) txReceipt {
	t.Helper()
	return sendTxWithValue(t, rpcURL, contracts.User, contracts.Gateway, castCalldata(t, "depositETH(string,uint64)", recipient, expiry), amount)
}

func reservePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve port: %v", err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

func waitForRPC(t *testing.T, rpcURL string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		_, err := rpcCallRaw(rpcURL, "eth_chainId", []any{})
		if err == nil {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("rpc %s did not become ready", rpcURL)
}

func rpcAccounts(t *testing.T, rpcURL string) []string {
	t.Helper()
	return rpcCallResult[[]string](t, rpcURL, "eth_accounts", []any{})
}

func deployContract(t *testing.T, rpcURL, from, artifactPath, constructorSig string, args ...string) string {
	t.Helper()

	artifact := readArtifactBytecode(t, artifactPath)
	ctor := ""
	if constructorSig != "" {
		ctor = strings.TrimPrefix(castAbiEncode(t, constructorSig, args...), "0x")
	}
	receipt := sendTx(t, rpcURL, from, "", artifact+ctor)
	if receipt.ContractAddress == "" {
		t.Fatalf("expected contract deployment address for %s", artifactPath)
	}
	return receipt.ContractAddress
}

func sendTx(t *testing.T, rpcURL, from, to, data string) txReceipt {
	t.Helper()

	tx := map[string]any{
		"from": from,
		"data": data,
	}
	if to != "" {
		tx["to"] = to
	}

	hash := rpcCallResult[string](t, rpcURL, "eth_sendTransaction", []any{tx})
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		receipt := rpcCallResult[*txReceipt](t, rpcURL, "eth_getTransactionReceipt", []any{hash})
		if receipt != nil {
			return *receipt
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("transaction %s was not mined", hash)
	return txReceipt{}
}

func sendTxWithValue(t *testing.T, rpcURL, from, to, data, value string) txReceipt {
	t.Helper()

	tx := map[string]any{
		"from":  from,
		"to":    to,
		"data":  data,
		"value": value,
	}

	hash := rpcCallResult[string](t, rpcURL, "eth_sendTransaction", []any{tx})
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		receipt := rpcCallResult[*txReceipt](t, rpcURL, "eth_getTransactionReceipt", []any{hash})
		if receipt != nil {
			return *receipt
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("transaction %s was not mined", hash)
	return txReceipt{}
}

func castCalldata(t *testing.T, signature string, args ...string) string {
	t.Helper()
	cmd := exec.Command("cast", append([]string{"calldata", signature}, args...)...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("cast calldata %s failed: %v\n%s", signature, err, output)
	}
	return strings.TrimSpace(string(output))
}

func castAbiEncode(t *testing.T, signature string, args ...string) string {
	t.Helper()
	cmd := exec.Command("cast", append([]string{"abi-encode", signature}, args...)...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("cast abi-encode %s failed: %v\n%s", signature, err, output)
	}
	return strings.TrimSpace(string(output))
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

func castWalletSignHash(t *testing.T, privateKey, digest string) []byte {
	t.Helper()
	cmd := exec.Command("cast", "wallet", "sign", "--private-key", privateKey, "--no-hash", digest)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("cast wallet sign failed: %v\n%s", err, output)
	}
	signature, err := hex.DecodeString(strings.TrimPrefix(strings.TrimSpace(string(output)), "0x"))
	if err != nil {
		t.Fatalf("decode cast signature: %v", err)
	}
	return signature
}

func readArtifactBytecode(t *testing.T, path string) string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read artifact %s: %v", path, err)
	}
	var payload struct {
		Bytecode struct {
			Object string `json:"object"`
		} `json:"bytecode"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("decode artifact %s: %v", path, err)
	}
	return payload.Bytecode.Object
}

func rpcCallResult[T any](t *testing.T, rpcURL, method string, params []any) T {
	t.Helper()
	raw, err := rpcCallRaw(rpcURL, method, params)
	if err != nil {
		t.Fatalf("rpc %s failed: %v", method, err)
	}
	var result T
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("decode rpc result for %s: %v\n%s", method, err, raw)
	}
	return result
}

func rpcCallRaw(rpcURL, method string, params []any) ([]byte, error) {
	payload, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  method,
		"params":  params,
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, rpcURL, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var envelope struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, err
	}
	if envelope.Error != nil {
		return nil, fmt.Errorf(envelope.Error.Message)
	}
	return envelope.Result, nil
}

func tokenBalanceOf(t *testing.T, rpcURL, token, account string) *big.Int {
	t.Helper()
	raw := ethCall(t, rpcURL, token, castCalldata(t, "balanceOf(address)", account))
	return parseHexBigInt(t, raw)
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

func parseHexBigInt(t *testing.T, value string) *big.Int {
	t.Helper()
	trimmed := strings.TrimPrefix(value, "0x")
	if trimmed == "" {
		return big.NewInt(0)
	}
	parsed, ok := new(big.Int).SetString(trimmed, 16)
	if !ok {
		t.Fatalf("invalid hex big int %q", value)
	}
	return parsed
}

func mustParseHexUint64(t *testing.T, value string) uint64 {
	t.Helper()

	trimmed := strings.TrimPrefix(value, "0x")
	if trimmed == "" {
		return 0
	}
	parsed, ok := new(big.Int).SetString(trimmed, 16)
	if !ok || !parsed.IsUint64() {
		t.Fatalf("invalid hex uint64 %q", value)
	}
	return parsed.Uint64()
}

func signWithdrawalReleaseAttestation(
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
	return castWalletSignHash(t, anvilFirstAccountPrivateKey, digest)
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

func predictWithdrawalMessageID(height, nonce uint64, assetID, recipient string, amount *big.Int) string {
	txHash := fmt.Sprintf("0x%x", sha256.Sum256([]byte(
		fmt.Sprintf("%d:%d:%s:%s:%s", height, nonce, assetID, strings.TrimSpace(recipient), amount.String()),
	)))
	identity := bridgetypes.ClaimIdentity{
		Kind:           bridgetypes.ClaimKindWithdrawal,
		SourceChainID:  "aegislink-1",
		SourceContract: "aegislink.bridge",
		SourceTxHash:   txHash,
		SourceLogIndex: 0,
		Nonce:          nonce,
	}
	identity.MessageID = identity.DerivedMessageID()
	return identity.MessageID
}
