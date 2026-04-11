package evm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestRPCReleaseTargetUsesSignerBackedRawTxsWhenConfigured(t *testing.T) {
	t.Parallel()

	port := reservePort(t)
	upstreamRPCURL := fmt.Sprintf("http://127.0.0.1:%d", port)
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

	waitForRPC(t, upstreamRPCURL)

	accounts := rpcAccounts(t, upstreamRPCURL)
	owner := accounts[0]
	recipient := accounts[2]

	repo := repoRoot(t)
	verifier := deployContract(t, upstreamRPCURL, owner, filepath.Join(repo, "contracts/ethereum/out/BridgeVerifier.sol/BridgeVerifier.json"), "constructor(address)", owner)
	gateway := deployContract(t, upstreamRPCURL, owner, filepath.Join(repo, "contracts/ethereum/out/BridgeGateway.sol/BridgeGateway.json"), "constructor(address)", verifier)
	sendTx(t, upstreamRPCURL, owner, verifier, castCalldata(t, "setGateway(address)", gateway))

	token := deployContract(
		t,
		upstreamRPCURL,
		owner,
		filepath.Join(repo, "contracts/ethereum/out/BridgeGateway.t.sol/TestToken.json"),
		"constructor(string,string,uint8)",
		"USDC",
		"USDC",
		"6",
	)
	sendTx(t, upstreamRPCURL, owner, token, castCalldata(t, "mint(address,uint256)", gateway, "100000000"))
	sendTx(t, upstreamRPCURL, owner, gateway, castCalldata(t, "setSupportedAsset(address,string,bool)", token, "eth.usdc", "true"))

	proxy := newRPCMethodProxy(t, upstreamRPCURL)
	target := NewRPCReleaseTargetWithSigner(proxy.URL, gateway, anvilFirstAccountPrivateKey, "")

	messageID := castKeccak(t, "release-signer-1")
	amount := big.NewInt(25000000)
	expiry := uint64(10000000000)
	signature := signReleaseAttestation(t, verifier, gateway, token, recipient, amount, messageID, expiry)

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

	if balance := tokenBalanceOf(t, upstreamRPCURL, token, recipient); balance.String() != amount.String() {
		t.Fatalf("expected recipient balance %s, got %s", amount.String(), balance.String())
	}
	if !verifierUsedProof(t, upstreamRPCURL, verifier, messageID) {
		t.Fatalf("expected verifier proof for %s to be marked used", messageID)
	}

	proxy.assertNoMethod(t, "eth_accounts")
	proxy.assertNoMethod(t, "eth_sendTransaction")
	proxy.assertSawMethod(t, "eth_sendRawTransaction")
	proxy.assertSawMethod(t, "eth_getTransactionReceipt")
}

type rpcMethodProxy struct {
	URL      string
	server   *httptest.Server
	methods  []string
	mu       sync.Mutex
	upstream string
}

func newRPCMethodProxy(t *testing.T, upstream string) *rpcMethodProxy {
	t.Helper()

	proxy := &rpcMethodProxy{upstream: upstream}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read proxy body: %v", err)
		}
		defer r.Body.Close()

		var payload struct {
			Method string `json:"method"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("decode proxy request: %v", err)
		}

		proxy.mu.Lock()
		proxy.methods = append(proxy.methods, payload.Method)
		proxy.mu.Unlock()

		req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, proxy.upstream, bytes.NewReader(body))
		if err != nil {
			t.Fatalf("forward proxy request: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("proxy upstream request: %v", err)
		}
		defer resp.Body.Close()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(resp.StatusCode)
		if _, err := io.Copy(w, resp.Body); err != nil {
			t.Fatalf("copy proxy response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	proxy.URL = server.URL
	proxy.server = server
	return proxy
}

func (p *rpcMethodProxy) assertSawMethod(t *testing.T, method string) {
	t.Helper()

	p.mu.Lock()
	defer p.mu.Unlock()
	for _, seen := range p.methods {
		if seen == method {
			return
		}
	}
	t.Fatalf("expected proxy to observe rpc method %q, got %v", method, p.methods)
}

func (p *rpcMethodProxy) assertNoMethod(t *testing.T, method string) {
	t.Helper()

	p.mu.Lock()
	defer p.mu.Unlock()
	for _, seen := range p.methods {
		if seen == method {
			t.Fatalf("expected proxy not to observe rpc method %q, got %v", method, p.methods)
		}
	}
}
