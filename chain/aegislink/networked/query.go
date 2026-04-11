package networked

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	bankkeeper "github.com/ayushns01/aegislink/chain/aegislink/x/bank/keeper"
	bridgekeeper "github.com/ayushns01/aegislink/chain/aegislink/x/bridge/keeper"
	rpchttp "github.com/cometbft/cometbft/rpc/client/http"
)

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
