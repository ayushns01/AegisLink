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
)

const releaseSelector = "3418e653"

type RPCReleaseTarget struct {
	rpcURL         string
	gatewayAddress string
	client         *http.Client
}

type rpcTransactionReceipt struct {
	TransactionHash string `json:"transactionHash"`
	Status          string `json:"status"`
}

func NewRPCReleaseTarget(rpcURL, gatewayAddress string) *RPCReleaseTarget {
	return &RPCReleaseTarget{
		rpcURL:         strings.TrimSpace(rpcURL),
		gatewayAddress: strings.TrimSpace(gatewayAddress),
		client:         http.DefaultClient,
	}
}

func (t *RPCReleaseTarget) ReleaseWithdrawal(ctx context.Context, request ReleaseRequest) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if t == nil || t.rpcURL == "" || t.gatewayAddress == "" {
		return "", ErrReleaseUnavailable
	}

	sender, err := t.senderAccount()
	if err != nil {
		return "", err
	}
	data, err := encodeReleaseCalldata(request)
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

func (t *RPCReleaseTarget) senderAccount() (string, error) {
	raw, err := t.rpcCall(context.Background(), "eth_accounts", []any{})
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
