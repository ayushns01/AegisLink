package networked

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"

	aegisapp "github.com/ayushns01/aegislink/chain/aegislink/app"
)

type DeliveryIntent struct {
	SourceTxHash string `json:"sourceTxHash"`
	Sender       string `json:"sender"`
	RouteID      string `json:"routeId"`
	AssetID      string `json:"assetId"`
	Amount       string `json:"amount"`
	Receiver     string `json:"receiver"`
}

func RegisterDeliveryIntent(cfg aegisapp.Config, intent DeliveryIntent) (DeliveryIntent, error) {
	intent, err := normalizeDeliveryIntent(intent)
	if err != nil {
		return DeliveryIntent{}, err
	}

	intents, err := loadDeliveryIntents(cfg)
	if err != nil {
		return DeliveryIntent{}, err
	}
	for index := range intents {
		if strings.EqualFold(intents[index].SourceTxHash, intent.SourceTxHash) {
			intents[index] = intent
			if err := saveDeliveryIntents(cfg, intents); err != nil {
				return DeliveryIntent{}, err
			}
			return intent, nil
		}
	}

	intents = append(intents, intent)
	slices.SortFunc(intents, func(left, right DeliveryIntent) int {
		return strings.Compare(left.SourceTxHash, right.SourceTxHash)
	})
	if err := saveDeliveryIntents(cfg, intents); err != nil {
		return DeliveryIntent{}, err
	}
	return intent, nil
}

func ListDeliveryIntents(cfg aegisapp.Config) ([]DeliveryIntent, error) {
	return loadDeliveryIntents(cfg)
}

func QueryDeliveryIntents(ctx context.Context, cfg Config) ([]DeliveryIntent, error) {
	var intents []DeliveryIntent
	if err := getReadyJSON(ctx, cfg, "/delivery-intents", &intents); err != nil {
		return nil, err
	}
	return intents, nil
}

func RegisterDeliveryIntentOverHTTP(ctx context.Context, cfg Config, intent DeliveryIntent) (DeliveryIntent, error) {
	ready, err := readReadyState(cfg)
	if err != nil {
		return DeliveryIntent{}, err
	}
	intent, err = normalizeDeliveryIntent(intent)
	if err != nil {
		return DeliveryIntent{}, err
	}
	body, err := json.Marshal(intent)
	if err != nil {
		return DeliveryIntent{}, err
	}
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		"http://"+strings.TrimSpace(ready.RPCAddress)+"/delivery-intents",
		strings.NewReader(string(body)),
	)
	if err != nil {
		return DeliveryIntent{}, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return DeliveryIntent{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return DeliveryIntent{}, decodeHTTPFailure(resp, "register delivery intent")
	}

	var stored DeliveryIntent
	if err := json.NewDecoder(resp.Body).Decode(&stored); err != nil {
		return DeliveryIntent{}, err
	}
	return stored, nil
}

func normalizeDeliveryIntent(intent DeliveryIntent) (DeliveryIntent, error) {
	intent.SourceTxHash = strings.TrimSpace(intent.SourceTxHash)
	intent.Sender = strings.TrimSpace(intent.Sender)
	intent.RouteID = strings.TrimSpace(intent.RouteID)
	intent.AssetID = strings.TrimSpace(intent.AssetID)
	intent.Amount = strings.TrimSpace(intent.Amount)
	intent.Receiver = strings.TrimSpace(intent.Receiver)

	switch {
	case intent.SourceTxHash == "":
		return DeliveryIntent{}, fmt.Errorf("missing source tx hash")
	case intent.Sender == "":
		return DeliveryIntent{}, fmt.Errorf("missing sender")
	case intent.RouteID == "":
		return DeliveryIntent{}, fmt.Errorf("missing route id")
	case intent.AssetID == "":
		return DeliveryIntent{}, fmt.Errorf("missing asset id")
	case intent.Amount == "":
		return DeliveryIntent{}, fmt.Errorf("missing amount")
	case intent.Receiver == "":
		return DeliveryIntent{}, fmt.Errorf("missing receiver")
	default:
		return intent, nil
	}
}

func deliveryIntentPath(cfg aegisapp.Config) string {
	return filepath.Join(strings.TrimSpace(cfg.HomeDir), "data", "delivery-intents.json")
}

func loadDeliveryIntents(cfg aegisapp.Config) ([]DeliveryIntent, error) {
	data, err := os.ReadFile(deliveryIntentPath(cfg))
	if err != nil {
		if os.IsNotExist(err) {
			return []DeliveryIntent{}, nil
		}
		return nil, err
	}
	var intents []DeliveryIntent
	if err := json.Unmarshal(data, &intents); err != nil {
		return nil, err
	}
	return intents, nil
}

func saveDeliveryIntents(cfg aegisapp.Config, intents []DeliveryIntent) error {
	path := deliveryIntentPath(cfg)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(intents, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}
