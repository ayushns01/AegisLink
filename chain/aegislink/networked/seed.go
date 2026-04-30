package networked

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strings"

	aegisapp "github.com/ayushns01/aegislink/chain/aegislink/app"
	ibcroutertypes "github.com/ayushns01/aegislink/chain/aegislink/x/ibcrouter/types"
	limittypes "github.com/ayushns01/aegislink/chain/aegislink/x/limits/types"
	registrytypes "github.com/ayushns01/aegislink/chain/aegislink/x/registry/types"
)

type SeedBridgeLimitPayload struct {
	AssetID       string `json:"asset_id"`
	WindowBlocks uint64 `json:"window_seconds"`
	MaxAmount     string `json:"max_amount"`
}

type SeedBridgeAssetsPayload struct {
	Assets []registrytypes.Asset    `json:"assets"`
	Limits []SeedBridgeLimitPayload `json:"limits"`
}

type SeedBridgeAssetsResult struct {
	Status        string   `json:"status"`
	Assets        int      `json:"assets"`
	Limits        int      `json:"limits"`
	SeededAssets  []string `json:"seeded_assets"`
	WindowBlocks []uint64 `json:"window_seconds,omitempty"`
}

type SetRouteProfilePayload struct {
	RouteID             string                      `json:"route_id"`
	DestinationChainID  string                      `json:"destination_chain_id"`
	ChannelID           string                      `json:"channel_id"`
	Enabled             bool                        `json:"enabled"`
	Assets              []ibcroutertypes.AssetRoute `json:"assets"`
	AllowedMemoPrefixes []string                    `json:"allowed_memo_prefixes,omitempty"`
	AllowedActionTypes  []string                    `json:"allowed_action_types,omitempty"`
}

type SetRouteProfileResult struct {
	RouteID             string                      `json:"route_id"`
	DestinationChainID  string                      `json:"destination_chain_id"`
	ChannelID           string                      `json:"channel_id"`
	Enabled             bool                        `json:"enabled"`
	Assets              []ibcroutertypes.AssetRoute `json:"assets"`
	AllowedMemoPrefixes []string                    `json:"allowed_memo_prefixes,omitempty"`
	AllowedActionTypes  []string                    `json:"allowed_action_types,omitempty"`
}

func SubmitSeedBridgeAssets(ctx context.Context, cfg Config, payload SeedBridgeAssetsPayload) (SeedBridgeAssetsResult, error) {
	if _, _, err := decodeSeedBridgeAssetsPayload(payload); err != nil {
		return SeedBridgeAssetsResult{}, err
	}
	ready, err := readReadyState(cfg)
	if err != nil {
		return SeedBridgeAssetsResult{}, err
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return SeedBridgeAssetsResult{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://"+strings.TrimSpace(ready.RPCAddress)+"/tx/seed-bridge-assets", bytes.NewReader(body))
	if err != nil {
		return SeedBridgeAssetsResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return SeedBridgeAssetsResult{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		var failure map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&failure); err == nil {
			if message, ok := failure["error"].(string); ok && strings.TrimSpace(message) != "" {
				return SeedBridgeAssetsResult{}, fmt.Errorf("seed bridge assets request failed: %s", message)
			}
		}
		return SeedBridgeAssetsResult{}, fmt.Errorf("seed bridge assets request failed with status %s", resp.Status)
	}
	var result SeedBridgeAssetsResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return SeedBridgeAssetsResult{}, err
	}
	return result, nil
}

func SubmitSetRouteProfile(ctx context.Context, cfg Config, payload SetRouteProfilePayload) (SetRouteProfileResult, error) {
	if _, err := decodeSetRouteProfilePayload(payload); err != nil {
		return SetRouteProfileResult{}, err
	}
	ready, err := readReadyState(cfg)
	if err != nil {
		return SetRouteProfileResult{}, err
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return SetRouteProfileResult{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://"+strings.TrimSpace(ready.RPCAddress)+"/tx/set-route-profile", bytes.NewReader(body))
	if err != nil {
		return SetRouteProfileResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return SetRouteProfileResult{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		var failure map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&failure); err == nil {
			if message, ok := failure["error"].(string); ok && strings.TrimSpace(message) != "" {
				return SetRouteProfileResult{}, fmt.Errorf("set route profile request failed: %s", message)
			}
		}
		return SetRouteProfileResult{}, fmt.Errorf("set route profile request failed with status %s", resp.Status)
	}
	var result SetRouteProfileResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return SetRouteProfileResult{}, err
	}
	return result, nil
}

func (n DemoNode) handleSeedBridgeAssets(w http.ResponseWriter, r *http.Request) error {
	if n.app == nil {
		return fmt.Errorf("seed bridge assets requires runtime app")
	}
	var payload SeedBridgeAssetsPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		return err
	}
	payload, parsedLimits, err := decodeSeedBridgeAssetsPayload(payload)
	if err != nil {
		return err
	}
	seededAssets := make([]string, 0, len(payload.Assets))
	windowSeconds := make([]uint64, 0, len(payload.Limits))
	for _, asset := range payload.Assets {
		if err := ensureSeededAsset(n.app, asset); err != nil {
			return err
		}
		seededAssets = append(seededAssets, asset.AssetID)
	}
	for _, limit := range payload.Limits {
		amount := parsedLimits[limit.AssetID]
		rateLimit := limittypes.RateLimit{
			AssetID:       limit.AssetID,
			WindowBlocks: limit.WindowBlocks,
			MaxAmount:     new(big.Int).Set(amount),
		}
		if err := n.app.SetLimit(rateLimit); err != nil {
			return err
		}
		windowSeconds = append(windowSeconds, limit.WindowBlocks)
	}
	if err := n.app.Save(); err != nil {
		return err
	}
	return json.NewEncoder(w).Encode(SeedBridgeAssetsResult{
		Status:        "seeded",
		Assets:        len(payload.Assets),
		Limits:        len(payload.Limits),
		SeededAssets:  seededAssets,
		WindowBlocks: windowSeconds,
	})
}

func (n DemoNode) handleSetRouteProfile(w http.ResponseWriter, r *http.Request) error {
	if n.app == nil {
		return fmt.Errorf("set route profile requires runtime app")
	}
	var payload SetRouteProfilePayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		return err
	}
	profile, err := decodeSetRouteProfilePayload(payload)
	if err != nil {
		return err
	}
	for _, asset := range profile.Assets {
		if _, ok := n.app.RegistryKeeper.GetAsset(asset.AssetID); !ok {
			return fmt.Errorf("asset %s is not registered in the AegisLink runtime", asset.AssetID)
		}
	}
	if err := n.app.SetRouteProfile(profile); err != nil {
		return err
	}
	if err := n.app.Save(); err != nil {
		return err
	}
	return json.NewEncoder(w).Encode(SetRouteProfileResult{
		RouteID:             profile.RouteID,
		DestinationChainID:  profile.DestinationChainID,
		ChannelID:           profile.ChannelID,
		Enabled:             profile.Enabled,
		Assets:              append([]ibcroutertypes.AssetRoute(nil), profile.Assets...),
		AllowedMemoPrefixes: append([]string(nil), profile.Policy.AllowedMemoPrefixes...),
		AllowedActionTypes:  append([]string(nil), profile.Policy.AllowedActionTypes...),
	})
}

func decodeSeedBridgeAssetsPayload(payload SeedBridgeAssetsPayload) (SeedBridgeAssetsPayload, map[string]*big.Int, error) {
	if len(payload.Assets) == 0 {
		return SeedBridgeAssetsPayload{}, nil, fmt.Errorf("missing bridge assets")
	}
	assetIDs := make(map[string]struct{}, len(payload.Assets))
	normalizedAssets := make([]registrytypes.Asset, 0, len(payload.Assets))
	for _, asset := range payload.Assets {
		asset.AssetID = strings.TrimSpace(asset.AssetID)
		asset.SourceChainID = strings.TrimSpace(asset.SourceChainID)
		asset.SourceAssetAddress = strings.TrimSpace(asset.SourceAssetAddress)
		asset.SourceContract = strings.TrimSpace(asset.SourceContract)
		asset.Denom = strings.TrimSpace(asset.Denom)
		asset.DisplayName = strings.TrimSpace(asset.DisplayName)
		asset.DisplaySymbol = strings.TrimSpace(asset.DisplaySymbol)
		if err := asset.ValidateBasic(); err != nil {
			return SeedBridgeAssetsPayload{}, nil, err
		}
		normalizedAssets = append(normalizedAssets, asset)
		assetIDs[asset.AssetID] = struct{}{}
	}
	parsedLimits := make(map[string]*big.Int, len(payload.Limits))
	normalizedLimits := make([]SeedBridgeLimitPayload, 0, len(payload.Limits))
	for _, limit := range payload.Limits {
		limit.AssetID = strings.TrimSpace(limit.AssetID)
		if limit.AssetID == "" {
			return SeedBridgeAssetsPayload{}, nil, fmt.Errorf("missing rate-limit asset id")
		}
		if limit.WindowBlocks == 0 {
			return SeedBridgeAssetsPayload{}, nil, fmt.Errorf("missing rate-limit window seconds for %s", limit.AssetID)
		}
		if _, ok := assetIDs[limit.AssetID]; !ok {
			return SeedBridgeAssetsPayload{}, nil, fmt.Errorf("rate limit references unknown asset %s", limit.AssetID)
		}
		amount, ok := new(big.Int).SetString(strings.TrimSpace(limit.MaxAmount), 10)
		if !ok || amount.Sign() <= 0 {
			return SeedBridgeAssetsPayload{}, nil, fmt.Errorf("invalid rate-limit amount %q for %s", limit.MaxAmount, limit.AssetID)
		}
		parsedLimits[limit.AssetID] = amount
		limit.MaxAmount = amount.String()
		normalizedLimits = append(normalizedLimits, limit)
	}
	return SeedBridgeAssetsPayload{
		Assets: normalizedAssets,
		Limits: normalizedLimits,
	}, parsedLimits, nil
}

func decodeSetRouteProfilePayload(payload SetRouteProfilePayload) (ibcroutertypes.RouteProfile, error) {
	profile := ibcroutertypes.RouteProfile{
		RouteID:            strings.TrimSpace(payload.RouteID),
		DestinationChainID: strings.TrimSpace(payload.DestinationChainID),
		ChannelID:          strings.TrimSpace(payload.ChannelID),
		Enabled:            payload.Enabled,
		Assets:             make([]ibcroutertypes.AssetRoute, 0, len(payload.Assets)),
		Policy: ibcroutertypes.RoutePolicy{
			AllowedMemoPrefixes: cleanSeedStrings(payload.AllowedMemoPrefixes),
			AllowedActionTypes:  cleanSeedStrings(payload.AllowedActionTypes),
		},
	}
	for _, asset := range payload.Assets {
		route := asset.Canonical()
		if err := route.ValidateBasic(); err != nil {
			return ibcroutertypes.RouteProfile{}, err
		}
		profile.Assets = append(profile.Assets, route)
	}
	if err := profile.ValidateBasic(); err != nil {
		return ibcroutertypes.RouteProfile{}, err
	}
	return profile.Canonical(), nil
}

func DecodeSetRouteProfilePayloadForCLI(payload SetRouteProfilePayload) (ibcroutertypes.RouteProfile, error) {
	return decodeSetRouteProfilePayload(payload)
}

func cleanSeedStrings(items []string) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func ensureSeededAsset(app *aegisapp.App, asset registrytypes.Asset) error {
	existing, ok := app.RegistryKeeper.GetAsset(asset.AssetID)
	if ok {
		if !sameSeededAsset(existing, asset) {
			return fmt.Errorf("asset %s already exists with different metadata", asset.AssetID)
		}
		return nil
	}
	return app.RegisterAsset(asset)
}

func sameSeededAsset(existing registrytypes.Asset, expected registrytypes.Asset) bool {
	return existing.AssetID == expected.AssetID &&
		existing.SourceChainID == expected.SourceChainID &&
		existing.SourceAssetKind == expected.SourceAssetKind &&
		strings.EqualFold(existing.SourceAssetAddress, expected.SourceAssetAddress) &&
		strings.EqualFold(existing.SourceContract, expected.SourceContract) &&
		existing.Denom == expected.Denom &&
		existing.Decimals == expected.Decimals &&
		existing.DisplayName == expected.DisplayName &&
		existing.DisplaySymbol == expected.DisplaySymbol &&
		existing.Enabled == expected.Enabled
}
