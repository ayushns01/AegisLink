package networked

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	bankkeeper "github.com/ayushns01/aegislink/chain/aegislink/x/bank/keeper"
	bridgecli "github.com/ayushns01/aegislink/chain/aegislink/x/bridge/client/cli"
	bridgekeeper "github.com/ayushns01/aegislink/chain/aegislink/x/bridge/keeper"
	rpchttp "github.com/cometbft/cometbft/rpc/client/http"
)

type SummaryView struct {
	AppName       string            `json:"app_name"`
	Modules       []string          `json:"modules"`
	Assets        int               `json:"assets"`
	Limits        int               `json:"limits"`
	PausedFlows   int               `json:"paused_flows"`
	CurrentHeight uint64            `json:"current_height"`
	Withdrawals   int               `json:"withdrawals"`
	SupplyByDenom map[string]string `json:"supply_by_denom"`
}

type WithdrawalView struct {
	Kind            string `json:"kind"`
	SourceChainID   string `json:"source_chain_id"`
	SourceContract  string `json:"source_contract"`
	SourceTxHash    string `json:"source_tx_hash"`
	SourceLogIndex  uint64 `json:"source_log_index"`
	Nonce           uint64 `json:"nonce"`
	MessageID       string `json:"message_id"`
	AssetID         string `json:"asset_id"`
	AssetAddress    string `json:"asset_address"`
	Amount          string `json:"amount"`
	Recipient       string `json:"recipient"`
	Deadline        uint64 `json:"deadline"`
	BlockHeight     uint64 `json:"block_height"`
	SignatureBase64 string `json:"signature_base64"`
}

type TransferView struct {
	TransferID         string `json:"transfer_id"`
	AssetID            string `json:"asset_id"`
	Amount             string `json:"amount"`
	Receiver           string `json:"receiver"`
	DestinationChainID string `json:"destination_chain_id"`
	ChannelID          string `json:"channel_id"`
	DestinationDenom   string `json:"destination_denom"`
	TimeoutHeight      uint64 `json:"timeout_height"`
	Memo               string `json:"memo"`
	Status             string `json:"status"`
	FailureReason      string `json:"failure_reason"`
}

func QueryBalances(ctx context.Context, cfg Config, address string) ([]bankkeeper.BalanceRecord, error) {
	var balances []bankkeeper.BalanceRecord
	if err := abciQueryJSON(ctx, cfg, "/balances", []byte(strings.TrimSpace(address)), &balances); err != nil {
		return nil, err
	}
	return balances, nil
}

func QueryTransfers(ctx context.Context, cfg Config) ([]TransferView, error) {
	var transfers []TransferView
	if err := abciQueryJSON(ctx, cfg, "/transfers", nil, &transfers); err != nil {
		return nil, err
	}
	return transfers, nil
}

func QuerySummary(ctx context.Context, cfg Config) (SummaryView, error) {
	var summary SummaryView
	if err := abciQueryJSON(ctx, cfg, "/summary", nil, &summary); err != nil {
		return SummaryView{}, err
	}
	return summary, nil
}

func QueryClaim(ctx context.Context, cfg Config, messageID string) (bridgekeeper.ClaimRecordSnapshot, bool, error) {
	var claim bridgekeeper.ClaimRecordSnapshot
	err := abciQueryJSON(ctx, cfg, "/claim", []byte(strings.TrimSpace(messageID)), &claim)
	if err != nil {
		if strings.Contains(err.Error(), "claim not found") {
			return bridgekeeper.ClaimRecordSnapshot{}, false, nil
		}
		return bridgekeeper.ClaimRecordSnapshot{}, false, err
	}
	return claim, true, nil
}

func QueryWithdrawals(ctx context.Context, cfg Config, fromHeight, toHeight uint64) ([]WithdrawalView, error) {
	payload, err := json.Marshal(map[string]uint64{
		"from_height": fromHeight,
		"to_height":   toHeight,
	})
	if err != nil {
		return nil, err
	}
	var withdrawals []WithdrawalView
	if err := abciQueryJSON(ctx, cfg, "/withdrawals", payload, &withdrawals); err != nil {
		return nil, err
	}
	return withdrawals, nil
}

func QueryBridgeSession(ctx context.Context, cfg Config, sourceTxHash string) (BridgeSessionView, error) {
	var view BridgeSessionView
	if err := getReadyJSON(ctx, cfg, "/bridge-status?sourceTxHash="+strings.TrimSpace(sourceTxHash), &view); err != nil {
		return BridgeSessionView{}, err
	}
	return view, nil
}

func readReadyState(cfg Config) (ReadyState, error) {
	resolved, _, err := ResolveConfig(cfg)
	if err != nil {
		return ReadyState{}, err
	}
	data, err := os.ReadFile(resolved.ReadyFile)
	if err != nil {
		return ReadyState{}, err
	}
	var ready ReadyState
	if err := json.Unmarshal(data, &ready); err != nil {
		return ReadyState{}, err
	}
	return ready, nil
}

func getReadyJSON(ctx context.Context, cfg Config, path string, target any) error {
	ready, err := readReadyState(cfg)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+strings.TrimSpace(ready.RPCAddress)+path, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return decodeHTTPFailure(resp, "query "+path)
	}
	if target == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(target)
}

func decodeHTTPFailure(resp *http.Response, action string) error {
	data, _ := io.ReadAll(resp.Body)
	if len(data) == 0 {
		return fmt.Errorf("%s failed with status %s", action, resp.Status)
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err == nil {
		if message, ok := payload["error"].(string); ok && strings.TrimSpace(message) != "" {
			return fmt.Errorf("%s failed: %s", action, message)
		}
	}
	return fmt.Errorf("%s failed with status %s: %s", action, resp.Status, strings.TrimSpace(string(data)))
}

func abciQueryJSON(ctx context.Context, cfg Config, path string, data []byte, target any) error {
	ready, err := readReadyState(cfg)
	if err != nil {
		return err
	}
	client, err := rpchttp.New("http://"+strings.TrimSpace(ready.CometRPCAddress), "/websocket")
	if err != nil {
		return err
	}
	resp, err := client.ABCIQuery(ctx, path, data)
	if err != nil {
		return err
	}
	if resp.Response.Code != 0 {
		return fmt.Errorf("abci query %s failed: %s", path, resp.Response.Log)
	}
	if target == nil || len(resp.Response.Value) == 0 {
		return nil
	}
	return json.Unmarshal(resp.Response.Value, target)
}

func withdrawalsFromRecords(records []bridgekeeper.WithdrawalRecord) []WithdrawalView {
	items := bridgecli.WithdrawalsResponse(records).Withdrawals
	withdrawals := make([]WithdrawalView, 0, len(items))
	for _, item := range items {
		withdrawals = append(withdrawals, WithdrawalView{
			Kind:            item.Kind,
			SourceChainID:   item.SourceChainId,
			SourceContract:  item.SourceContract,
			SourceTxHash:    item.SourceTxHash,
			SourceLogIndex:  item.SourceLogIndex,
			Nonce:           item.Nonce,
			MessageID:       item.MessageId,
			AssetID:         item.AssetId,
			AssetAddress:    item.AssetAddress,
			Amount:          item.Amount,
			Recipient:       item.Recipient,
			Deadline:        item.Deadline,
			BlockHeight:     item.BlockHeight,
			SignatureBase64: item.SignatureBase64,
		})
	}
	return withdrawals
}
