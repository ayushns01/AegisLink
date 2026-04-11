package evm

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"

	bridgetypes "github.com/ayushns01/aegislink/chain/aegislink/x/bridge/types"
)

const depositInitiatedTopic = "0xf9d606ceab83667329b57cc5f8977bd61f1496d82bb7a9d99586d24402a56a8c"
const maxDepositLogRange = uint64(10)

type RPCLogSource struct {
	rpcURL         string
	gatewayAddress string
	client         *http.Client
}

func NewRPCLogSource(rpcURL, gatewayAddress string) *RPCLogSource {
	return &RPCLogSource{
		rpcURL:         strings.TrimSpace(rpcURL),
		gatewayAddress: strings.TrimSpace(gatewayAddress),
		client:         http.DefaultClient,
	}
}

func (s *RPCLogSource) LatestBlock(ctx context.Context) (uint64, error) {
	if s == nil || s.rpcURL == "" {
		return 0, ErrSourceUnavailable
	}
	raw, err := s.rpcCall(ctx, "eth_blockNumber", []any{})
	if err != nil {
		return 0, err
	}
	return parseHexUint64(raw)
}

func (s *RPCLogSource) DepositEvents(ctx context.Context, fromBlock, toBlock uint64) ([]DepositEvent, error) {
	if s == nil || s.rpcURL == "" || s.gatewayAddress == "" {
		return nil, ErrSourceUnavailable
	}

	rawChainID, err := s.rpcCall(ctx, "eth_chainId", []any{})
	if err != nil {
		return nil, err
	}
	chainID, err := parseHexBigInt(rawChainID)
	if err != nil {
		return nil, err
	}

	events := make([]DepositEvent, 0)
	for start := fromBlock; start <= toBlock; {
		end := chunkedLogRangeEnd(start, toBlock)
		rawLogs, err := s.rpcCall(ctx, "eth_getLogs", []any{map[string]any{
			"fromBlock": toHexUint64(start),
			"toBlock":   toHexUint64(end),
			"address":   s.gatewayAddress,
			"topics":    []string{depositInitiatedTopic},
		}})
		if err != nil {
			return nil, err
		}

		var logs []rpcLog
		if err := json.Unmarshal(rawLogs, &logs); err != nil {
			return nil, err
		}

		for _, log := range logs {
			event, err := decodeDepositEvent(log, chainID.String())
			if err != nil {
				return nil, err
			}
			events = append(events, event)
		}
		if end == ^uint64(0) || end >= toBlock {
			break
		}
		start = end + 1
	}
	return events, nil
}

func chunkedLogRangeEnd(fromBlock, toBlock uint64) uint64 {
	if toBlock <= fromBlock {
		return toBlock
	}
	if maxDepositLogRange <= 1 {
		return fromBlock
	}
	maxEnd := fromBlock + maxDepositLogRange - 1
	if maxEnd < fromBlock || maxEnd > toBlock {
		return toBlock
	}
	return maxEnd
}

type rpcLog struct {
	Address         string   `json:"address"`
	Topics          []string `json:"topics"`
	Data            string   `json:"data"`
	BlockNumber     string   `json:"blockNumber"`
	TransactionHash string   `json:"transactionHash"`
	LogIndex        string   `json:"logIndex"`
}

func decodeDepositEvent(log rpcLog, sourceChainID string) (DepositEvent, error) {
	if len(log.Topics) < 4 {
		return DepositEvent{}, fmt.Errorf("deposit log missing topics")
	}
	blockNumber, err := parseHexStringUint64(log.BlockNumber)
	if err != nil {
		return DepositEvent{}, err
	}
	logIndex, err := parseHexStringUint64(log.LogIndex)
	if err != nil {
		return DepositEvent{}, err
	}
	nonce, err := parseHexStringUint64(log.Topics[3])
	if err != nil {
		return DepositEvent{}, err
	}

	asset, assetID, amount, recipient, expiry, err := decodeDepositData(log.Data)
	if err != nil {
		return DepositEvent{}, err
	}
	sourceAssetKind := bridgetypes.SourceAssetKindERC20
	if isZeroHexAddress(asset) {
		sourceAssetKind = bridgetypes.SourceAssetKindNativeETH
	}

	return DepositEvent{
		BlockNumber:     blockNumber,
		SourceChainID:   sourceChainID,
		SourceContract:  log.Address,
		TxHash:          log.TransactionHash,
		LogIndex:        logIndex,
		Nonce:           nonce,
		DepositID:       log.Topics[1],
		MessageID:       log.Topics[2],
		SourceAssetKind: sourceAssetKind,
		AssetAddress:    asset,
		AssetID:         assetID,
		Amount:          amount,
		Recipient:       recipient,
		Expiry:          expiry,
	}, nil
}

func decodeDepositData(data string) (string, string, *big.Int, string, uint64, error) {
	payload, err := hex.DecodeString(strings.TrimPrefix(data, "0x"))
	if err != nil {
		return "", "", nil, "", 0, err
	}
	if len(payload) < 32*5 {
		return "", "", nil, "", 0, fmt.Errorf("deposit data too short")
	}

	asset := "0x" + hex.EncodeToString(payload[12:32])
	assetOffset, err := wordToInt(payload[32:64])
	if err != nil {
		return "", "", nil, "", 0, err
	}
	amount := new(big.Int).SetBytes(payload[64:96])
	recipientOffset, err := wordToInt(payload[96:128])
	if err != nil {
		return "", "", nil, "", 0, err
	}
	expiry, err := wordToUint64(payload[128:160])
	if err != nil {
		return "", "", nil, "", 0, err
	}

	assetID, err := decodeABIString(payload, assetOffset)
	if err != nil {
		return "", "", nil, "", 0, err
	}
	recipient, err := decodeABIString(payload, recipientOffset)
	if err != nil {
		return "", "", nil, "", 0, err
	}
	return asset, assetID, amount, recipient, expiry, nil
}

func decodeABIString(payload []byte, offset int) (string, error) {
	if offset < 0 || offset+32 > len(payload) {
		return "", fmt.Errorf("invalid abi string offset")
	}
	length, err := wordToInt(payload[offset : offset+32])
	if err != nil {
		return "", err
	}
	start := offset + 32
	end := start + length
	if length < 0 || end > len(payload) {
		return "", fmt.Errorf("invalid abi string length")
	}
	return string(payload[start:end]), nil
}

func wordToInt(word []byte) (int, error) {
	bi := new(big.Int).SetBytes(word)
	if !bi.IsInt64() {
		return 0, fmt.Errorf("word exceeds int64")
	}
	return int(bi.Int64()), nil
}

func wordToUint64(word []byte) (uint64, error) {
	bi := new(big.Int).SetBytes(word)
	if !bi.IsUint64() {
		return 0, fmt.Errorf("word exceeds uint64")
	}
	return bi.Uint64(), nil
}

func parseHexUint64(raw json.RawMessage) (uint64, error) {
	value, err := parseHexBigInt(raw)
	if err != nil {
		return 0, err
	}
	if !value.IsUint64() {
		return 0, fmt.Errorf("hex value exceeds uint64")
	}
	return value.Uint64(), nil
}

func parseHexBigInt(raw json.RawMessage) (*big.Int, error) {
	var encoded string
	if err := json.Unmarshal(raw, &encoded); err != nil {
		return nil, err
	}
	return parseHexBigIntString(encoded)
}

func parseHexStringUint64(encoded string) (uint64, error) {
	value, err := parseHexBigIntString(encoded)
	if err != nil {
		return 0, err
	}
	if !value.IsUint64() {
		return 0, fmt.Errorf("hex value exceeds uint64")
	}
	return value.Uint64(), nil
}

func parseHexBigIntString(encoded string) (*big.Int, error) {
	trimmed := strings.TrimPrefix(encoded, "0x")
	if trimmed == "" {
		trimmed = "0"
	}
	value, ok := new(big.Int).SetString(trimmed, 16)
	if !ok {
		return nil, fmt.Errorf("invalid hex value %q", encoded)
	}
	return value, nil
}

func toHexUint64(value uint64) string {
	return fmt.Sprintf("0x%x", value)
}

func (s *RPCLogSource) rpcCall(ctx context.Context, method string, params []any) (json.RawMessage, error) {
	payload, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  method,
		"params":  params,
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.rpcURL, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
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
