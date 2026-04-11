package evm

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"time"

	bridgetypes "github.com/ayushns01/aegislink/chain/aegislink/x/bridge/types"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/decred/dcrd/dcrec/secp256k1/v4/ecdsa"
	"golang.org/x/crypto/sha3"
)

const releaseSelector = "3418e653"

type RPCReleaseTarget struct {
	rpcURL                  string
	gatewayAddress          string
	releaseSignerPrivateKey string
	releaseSignerAddress    string
	client                  *http.Client
}

type rpcTransactionReceipt struct {
	TransactionHash string `json:"transactionHash"`
	Status          string `json:"status"`
}

func NewRPCReleaseTarget(rpcURL, gatewayAddress string) *RPCReleaseTarget {
	return NewRPCReleaseTargetWithSigner(rpcURL, gatewayAddress, "", "")
}

func NewRPCReleaseTargetWithSigner(rpcURL, gatewayAddress, releaseSignerPrivateKey, releaseSignerAddress string) *RPCReleaseTarget {
	return &RPCReleaseTarget{
		rpcURL:                  strings.TrimSpace(rpcURL),
		gatewayAddress:          strings.TrimSpace(gatewayAddress),
		releaseSignerPrivateKey: strings.TrimSpace(releaseSignerPrivateKey),
		releaseSignerAddress:    strings.TrimSpace(releaseSignerAddress),
		client:                  http.DefaultClient,
	}
}

func (t *RPCReleaseTarget) ReleaseWithdrawal(ctx context.Context, request ReleaseRequest) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if t == nil || t.rpcURL == "" || t.gatewayAddress == "" {
		return "", ErrReleaseUnavailable
	}

	data, err := encodeReleaseCalldata(request)
	if err != nil {
		return "", err
	}

	if t.releaseSignerPrivateKey != "" {
		txHash, err := t.releaseWithdrawalWithSigner(ctx, request, data)
		if err != nil {
			return "", err
		}
		return txHash, nil
	}

	sender, err := t.senderAccount(ctx)
	if err != nil {
		return "", err
	}
	txHashRaw, err := t.rpcCall(ctx, "eth_sendTransaction", []any{map[string]any{
		"from": sender,
		"to":   t.gatewayAddress,
		"data": data,
	}})
	if err != nil {
		return "", err
	}

	var txHash string
	if err := json.Unmarshal(txHashRaw, &txHash); err != nil {
		return "", err
	}
	if strings.TrimSpace(txHash) == "" {
		return "", errors.New("missing transaction hash")
	}
	if err := t.waitForReceipt(ctx, txHash); err != nil {
		return "", err
	}
	return txHash, nil
}

func (t *RPCReleaseTarget) releaseWithdrawalWithSigner(ctx context.Context, request ReleaseRequest, data string) (string, error) {
	sender, err := t.releaseSenderAddress()
	if err != nil {
		return "", err
	}
	chainID, err := t.chainID(ctx)
	if err != nil {
		return "", err
	}
	nonce, err := t.transactionNonce(ctx, sender)
	if err != nil {
		return "", err
	}
	gasPrice, err := t.gasPrice(ctx)
	if err != nil {
		return "", err
	}
	rawTx, err := signLegacyTransaction(chainID, nonce, gasPrice, big.NewInt(500000), t.gatewayAddress, data, t.releaseSignerPrivateKey)
	if err != nil {
		return "", err
	}
	txHashRaw, err := t.rpcCall(ctx, "eth_sendRawTransaction", []any{rawTx})
	if err != nil {
		return "", err
	}
	var txHash string
	if err := json.Unmarshal(txHashRaw, &txHash); err != nil {
		return "", err
	}
	if strings.TrimSpace(txHash) == "" {
		return "", errors.New("missing transaction hash")
	}
	if err := t.waitForReceipt(ctx, txHash); err != nil {
		return "", err
	}
	return txHash, nil
}

func (t *RPCReleaseTarget) releaseSenderAddress() (string, error) {
	if trimmed := strings.TrimSpace(t.releaseSignerAddress); trimmed != "" {
		return trimmed, nil
	}
	if strings.TrimSpace(t.releaseSignerPrivateKey) == "" {
		return "", errors.New("missing release signer private key")
	}
	return bridgetypes.SignerAddressFromPrivateKeyHex(t.releaseSignerPrivateKey)
}

func (t *RPCReleaseTarget) senderAccount(ctx context.Context) (string, error) {
	raw, err := t.rpcCall(ctx, "eth_accounts", []any{})
	if err != nil {
		return "", err
	}
	var accounts []string
	if err := json.Unmarshal(raw, &accounts); err != nil {
		return "", err
	}
	if len(accounts) == 0 {
		return "", errors.New("no unlocked ethereum accounts available")
	}
	return accounts[0], nil
}

func (t *RPCReleaseTarget) chainID(ctx context.Context) (*big.Int, error) {
	raw, err := t.rpcCall(ctx, "eth_chainId", []any{})
	if err != nil {
		return nil, err
	}
	var encoded string
	if err := json.Unmarshal(raw, &encoded); err != nil {
		return nil, err
	}
	return parseHexBigIntString(encoded)
}

func (t *RPCReleaseTarget) transactionNonce(ctx context.Context, sender string) (*big.Int, error) {
	raw, err := t.rpcCall(ctx, "eth_getTransactionCount", []any{sender, "pending"})
	if err != nil {
		return nil, err
	}
	var encoded string
	if err := json.Unmarshal(raw, &encoded); err != nil {
		return nil, err
	}
	return parseHexBigIntString(encoded)
}

func (t *RPCReleaseTarget) gasPrice(ctx context.Context) (*big.Int, error) {
	raw, err := t.rpcCall(ctx, "eth_gasPrice", []any{})
	if err != nil {
		return nil, err
	}
	var encoded string
	if err := json.Unmarshal(raw, &encoded); err != nil {
		return nil, err
	}
	return parseHexBigIntString(encoded)
}

func (t *RPCReleaseTarget) waitForReceipt(ctx context.Context, txHash string) error {
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if err := ctx.Err(); err != nil {
			return err
		}

		raw, err := t.rpcCall(ctx, "eth_getTransactionReceipt", []any{txHash})
		if err != nil {
			return err
		}
		if string(raw) == "null" {
			time.Sleep(100 * time.Millisecond)
			continue
		}

		var receipt rpcTransactionReceipt
		if err := json.Unmarshal(raw, &receipt); err != nil {
			return err
		}
		status, err := parseHexStringUint64(receipt.Status)
		if err != nil {
			return fmt.Errorf("parse release receipt status: %w", err)
		}
		if status != 1 {
			return fmt.Errorf("release transaction %s reverted with status %s", txHash, receipt.Status)
		}
		return nil
	}
	return fmt.Errorf("timed out waiting for release transaction %s", txHash)
}

func encodeReleaseCalldata(request ReleaseRequest) (string, error) {
	asset, err := encodeAddressWord(request.AssetAddress)
	if err != nil {
		return "", fmt.Errorf("encode asset address: %w", err)
	}
	recipient, err := encodeAddressWord(request.Recipient)
	if err != nil {
		return "", fmt.Errorf("encode recipient address: %w", err)
	}
	messageID, err := encodeBytes32Word(request.MessageID)
	if err != nil {
		return "", fmt.Errorf("encode message id: %w", err)
	}

	static := make([]byte, 0, 32*6)
	static = append(static, asset...)
	static = append(static, recipient...)
	static = append(static, encodeBigIntWord(request.Amount)...)
	static = append(static, messageID...)
	static = append(static, encodeUint64Word(request.Deadline)...)
	static = append(static, encodeBigIntWord(big.NewInt(32*6))...)

	dynamic := encodeDynamicBytes(request.Signature)
	return "0x" + releaseSelector + hex.EncodeToString(append(static, dynamic...)), nil
}

func encodeAddressWord(value string) ([]byte, error) {
	decoded, err := decodeHexBytes(value, 20)
	if err != nil {
		return nil, err
	}
	word := make([]byte, 32)
	copy(word[12:], decoded)
	return word, nil
}

func encodeBytes32Word(value string) ([]byte, error) {
	return decodeHexBytes(value, 32)
}

func encodeBigIntWord(value *big.Int) []byte {
	word := make([]byte, 32)
	if value == nil || value.Sign() <= 0 {
		return word
	}
	encoded := value.Bytes()
	copy(word[32-len(encoded):], encoded)
	return word
}

func encodeUint64Word(value uint64) []byte {
	return encodeBigIntWord(new(big.Int).SetUint64(value))
}

func encodeDynamicBytes(value []byte) []byte {
	payload := make([]byte, 0, 32+len(value)+31)
	payload = append(payload, encodeBigIntWord(big.NewInt(int64(len(value))))...)
	payload = append(payload, value...)
	if rem := len(payload) % 32; rem != 0 {
		payload = append(payload, make([]byte, 32-rem)...)
	}
	return payload
}

func decodeHexBytes(value string, expectedLen int) ([]byte, error) {
	trimmed := strings.TrimPrefix(strings.TrimSpace(value), "0x")
	if len(trimmed) != expectedLen*2 {
		return nil, fmt.Errorf("expected %d-byte hex value, got %q", expectedLen, value)
	}
	decoded, err := hex.DecodeString(trimmed)
	if err != nil {
		return nil, err
	}
	return decoded, nil
}

func signLegacyTransaction(chainID, nonce, gasPrice, gasLimit *big.Int, to, data, privateKeyHex string) (string, error) {
	privateKey, err := parsePrivateKeyHex(privateKeyHex)
	if err != nil {
		return "", err
	}
	encodedTo, err := decodeHexBytes(to, 20)
	if err != nil {
		return "", fmt.Errorf("encode recipient: %w", err)
	}

	signingPayload := rlpEncodeList(
		rlpEncodeBigInt(nonce),
		rlpEncodeBigInt(gasPrice),
		rlpEncodeBigInt(gasLimit),
		rlpEncodeBytes(encodedTo),
		rlpEncodeBigInt(big.NewInt(0)),
		rlpEncodeBytes(mustDecodeHex(data)),
		rlpEncodeBigInt(chainID),
		rlpEncodeBigInt(big.NewInt(0)),
		rlpEncodeBigInt(big.NewInt(0)),
	)
	digest := keccak256(signingPayload)
	sig := ecdsa.SignCompact(privateKey, digest, false)
	if len(sig) != 65 {
		return "", fmt.Errorf("unexpected compact signature length %d", len(sig))
	}
	recoveryCode := sig[0] - 27
	v := new(big.Int).Mul(chainID, big.NewInt(2))
	v.Add(v, big.NewInt(int64(35+recoveryCode)))
	signedPayload := rlpEncodeList(
		rlpEncodeBigInt(nonce),
		rlpEncodeBigInt(gasPrice),
		rlpEncodeBigInt(gasLimit),
		rlpEncodeBytes(encodedTo),
		rlpEncodeBigInt(big.NewInt(0)),
		rlpEncodeBytes(mustDecodeHex(data)),
		rlpEncodeBigInt(v),
		rlpEncodeBigInt(new(big.Int).SetBytes(sig[1:33])),
		rlpEncodeBigInt(new(big.Int).SetBytes(sig[33:65])),
	)
	return "0x" + hex.EncodeToString(signedPayload), nil
}

func mustDecodeHex(value string) []byte {
	trimmed := strings.TrimPrefix(strings.TrimSpace(value), "0x")
	if trimmed == "" {
		return nil
	}
	decoded, err := hex.DecodeString(trimmed)
	if err != nil {
		panic(err)
	}
	return decoded
}

func rlpEncodeBigInt(value *big.Int) []byte {
	if value == nil || value.Sign() <= 0 {
		return []byte{0x80}
	}
	return rlpEncodeBytes(value.Bytes())
}

func rlpEncodeBytes(value []byte) []byte {
	if len(value) == 1 && value[0] < 0x80 {
		return append([]byte(nil), value[0])
	}
	if len(value) <= 55 {
		out := make([]byte, 1+len(value))
		out[0] = byte(0x80 + len(value))
		copy(out[1:], value)
		return out
	}
	length := rlpEncodeLength(len(value), 0xb7)
	return append(length, value...)
}

func rlpEncodeList(values ...[]byte) []byte {
	payloadLen := 0
	for _, value := range values {
		payloadLen += len(value)
	}
	payload := make([]byte, 0, payloadLen)
	for _, value := range values {
		payload = append(payload, value...)
	}
	if len(payload) <= 55 {
		out := make([]byte, 1+len(payload))
		out[0] = byte(0xc0 + len(payload))
		copy(out[1:], payload)
		return out
	}
	return append(rlpEncodeLength(len(payload), 0xf7), payload...)
}

func rlpEncodeLength(length int, offset byte) []byte {
	if length == 0 {
		return []byte{offset}
	}
	encoded := make([]byte, 0, 9)
	value := length
	for value > 0 {
		encoded = append([]byte{byte(value & 0xff)}, encoded...)
		value >>= 8
	}
	return append([]byte{offset + byte(len(encoded))}, encoded...)
}

func keccak256(data []byte) []byte {
	digest := sha3.NewLegacyKeccak256()
	_, _ = digest.Write(data)
	return digest.Sum(nil)
}

func parsePrivateKeyHex(privateKeyHex string) (*secp256k1.PrivateKey, error) {
	trimmed := strings.TrimPrefix(strings.TrimSpace(privateKeyHex), "0x")
	if trimmed == "" {
		return nil, errors.New("missing release signer private key")
	}
	decoded, err := hex.DecodeString(trimmed)
	if err != nil {
		return nil, fmt.Errorf("decode release signer private key: %w", err)
	}
	privateKey := secp256k1.PrivKeyFromBytes(decoded)
	if privateKey.Key.IsZero() {
		return nil, errors.New("release signer private key is zero")
	}
	return privateKey, nil
}

func (t *RPCReleaseTarget) rpcCall(ctx context.Context, method string, params []any) (json.RawMessage, error) {
	payload, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  method,
		"params":  params,
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.rpcURL, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(req)
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
