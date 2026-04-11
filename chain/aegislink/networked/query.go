package networked

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	bankkeeper "github.com/ayushns01/aegislink/chain/aegislink/x/bank/keeper"
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
	ready, err := readReadyState(cfg)
	if err != nil {
		return nil, err
	}

	var balances []bankkeeper.BalanceRecord
	if err := getJSON(ctx, "http://"+ready.RPCAddress+"/balances?address="+url.QueryEscape(strings.TrimSpace(address)), &balances); err != nil {
		return nil, err
	}
	return balances, nil
}

func QueryTransfers(ctx context.Context, cfg Config) ([]TransferView, error) {
	ready, err := readReadyState(cfg)
	if err != nil {
		return nil, err
	}

	var transfers []TransferView
	if err := getJSON(ctx, "http://"+ready.RPCAddress+"/transfers", &transfers); err != nil {
		return nil, err
	}
	return transfers, nil
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

func getJSON(ctx context.Context, endpoint string, target any) error {
	requestCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d from %s", resp.StatusCode, endpoint)
	}
	return json.NewDecoder(resp.Body).Decode(target)
}
