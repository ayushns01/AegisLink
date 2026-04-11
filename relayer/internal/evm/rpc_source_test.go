package evm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestRPCLogSourceChunksLargeLogRanges(t *testing.T) {
	t.Parallel()

	var (
		mu     sync.Mutex
		ranges [][2]string
	)

	source := NewRPCLogSource("http://rpc.test", "0x37ecd127529B14253C8a858976e22c4671c6Bd1E")
	source.client = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			defer r.Body.Close()

			var request struct {
				Method string            `json:"method"`
				Params []json.RawMessage `json:"params"`
			}
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Fatalf("decode rpc request: %v", err)
			}

			var body string
			switch request.Method {
			case "eth_chainId":
				body = `{"jsonrpc":"2.0","id":1,"result":"0xaa36a7"}`
			case "eth_getLogs":
				var filter map[string]any
				if err := json.Unmarshal(request.Params[0], &filter); err != nil {
					t.Fatalf("decode getLogs params: %v", err)
				}
				mu.Lock()
				ranges = append(ranges, [2]string{
					filter["fromBlock"].(string),
					filter["toBlock"].(string),
				})
				mu.Unlock()
				body = `{"jsonrpc":"2.0","id":1,"result":[]}`
			default:
				t.Fatalf("unexpected rpc method %q", request.Method)
			}

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	}
	events, err := source.DepositEvents(context.Background(), 100, 125)
	if err != nil {
		t.Fatalf("deposit events: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("expected no events, got %d", len(events))
	}

	mu.Lock()
	got := append([][2]string(nil), ranges...)
	mu.Unlock()

	want := [][2]string{
		{"0x64", "0x6d"},
		{"0x6e", "0x77"},
		{"0x78", "0x7d"},
	}
	if len(got) != len(want) {
		t.Fatalf("expected %d eth_getLogs calls, got %d (%+v)", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected range %v at call %d, got %v", want[i], i, got[i])
		}
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestRPCLogSourceObservesLiveAnvilDeposit(t *testing.T) {
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
	sendTx(t, rpcURL, user, token, castCalldata(t, "approve(address,uint256)", gateway, "100000000"))
	receipt := sendTx(t, rpcURL, user, gateway, castCalldata(t, "deposit(address,uint256,string,uint64)", token, "25000000", "cosmos1recipient", "10000000000"))

	source := NewRPCLogSource(rpcURL, gateway)

	latest, err := source.LatestBlock(context.Background())
	if err != nil {
		t.Fatalf("latest block: %v", err)
	}
	events, err := source.DepositEvents(context.Background(), 0, latest)
	if err != nil {
		t.Fatalf("deposit events: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected one observed deposit event, got %d", len(events))
	}

	event := events[0]
	if event.SourceChainID != "11155111" {
		t.Fatalf("expected source chain id 11155111, got %q", event.SourceChainID)
	}
	if !strings.EqualFold(event.SourceContract, gateway) {
		t.Fatalf("expected source contract %q, got %q", gateway, event.SourceContract)
	}
	if !strings.EqualFold(event.AssetAddress, token) {
		t.Fatalf("expected asset address %q, got %q", token, event.AssetAddress)
	}
	if event.SourceAssetKind != "erc20" {
		t.Fatalf("expected source asset kind erc20, got %q", event.SourceAssetKind)
	}
	if event.AssetID != "eth.usdc" {
		t.Fatalf("expected asset id eth.usdc, got %q", event.AssetID)
	}
	if event.Amount.String() != "25000000" {
		t.Fatalf("expected amount 25000000, got %s", event.Amount.String())
	}
	if event.Recipient != "cosmos1recipient" {
		t.Fatalf("expected recipient cosmos1recipient, got %q", event.Recipient)
	}
	if event.Expiry != 10000000000 {
		t.Fatalf("expected expiry 10000000000, got %d", event.Expiry)
	}
	if !strings.EqualFold(event.TxHash, receipt.TransactionHash) {
		t.Fatalf("expected tx hash %q, got %q", receipt.TransactionHash, event.TxHash)
	}
	if event.Nonce != 1 {
		t.Fatalf("expected nonce 1, got %d", event.Nonce)
	}
	if event.DepositID == "" || event.MessageID == "" {
		t.Fatalf("expected deposit and message ids to be populated")
	}
}

func TestRPCLogSourceObservesLiveAnvilNativeETHDeposit(t *testing.T) {
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
	user := accounts[1]

	repo := repoRoot(t)
	verifier := deployContract(t, rpcURL, owner, filepath.Join(repo, "contracts/ethereum/out/BridgeVerifier.sol/BridgeVerifier.json"), "constructor(address)", owner)
	gateway := deployContract(t, rpcURL, owner, filepath.Join(repo, "contracts/ethereum/out/BridgeGateway.sol/BridgeGateway.json"), "constructor(address)", verifier)
	sendTx(t, rpcURL, owner, verifier, castCalldata(t, "setGateway(address)", gateway))
	receipt := sendTxWithValue(t, rpcURL, user, gateway, castCalldata(t, "depositETH(string,uint64)", "cosmos1recipient", "10000000000"), "1000000000000000000")

	source := NewRPCLogSource(rpcURL, gateway)
	latest, err := source.LatestBlock(context.Background())
	if err != nil {
		t.Fatalf("latest block: %v", err)
	}
	events, err := source.DepositEvents(context.Background(), 0, latest)
	if err != nil {
		t.Fatalf("deposit events: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected one observed native deposit event, got %d", len(events))
	}

	event := events[0]
	if event.SourceAssetKind != "native_eth" {
		t.Fatalf("expected source asset kind native_eth, got %q", event.SourceAssetKind)
	}
	if !isZeroHexAddress(event.AssetAddress) {
		t.Fatalf("expected zero-address native asset, got %q", event.AssetAddress)
	}
	if event.AssetID != "eth" {
		t.Fatalf("expected asset id eth, got %q", event.AssetID)
	}
	if event.Amount.String() != "1000000000000000000" {
		t.Fatalf("expected amount 1000000000000000000, got %s", event.Amount.String())
	}
	if !strings.EqualFold(event.TxHash, receipt.TransactionHash) {
		t.Fatalf("expected tx hash %q, got %q", receipt.TransactionHash, event.TxHash)
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("unable to resolve test file")
	}
	return filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(file))))
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
	raw := rpcCallResult[[]string](t, rpcURL, "eth_accounts", []any{})
	if len(raw) < 2 {
		t.Fatalf("expected at least 2 accounts, got %d", len(raw))
	}
	return raw
}

type txReceipt struct {
	ContractAddress string `json:"contractAddress"`
	TransactionHash string `json:"transactionHash"`
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
