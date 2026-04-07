package types

import (
	"errors"
	"fmt"
	"strings"
)

var ErrInvalidRouteProfile = errors.New("invalid route profile")

type AssetRoute struct {
	AssetID          string `json:"asset_id"`
	DestinationDenom string `json:"destination_denom"`
}

func (a AssetRoute) ValidateBasic() error {
	if strings.TrimSpace(a.AssetID) == "" {
		return fmt.Errorf("%w: missing asset id", ErrInvalidRouteProfile)
	}
	if strings.TrimSpace(a.DestinationDenom) == "" {
		return fmt.Errorf("%w: missing destination denom", ErrInvalidRouteProfile)
	}
	return nil
}

func (a AssetRoute) Canonical() AssetRoute {
	a.AssetID = strings.TrimSpace(a.AssetID)
	a.DestinationDenom = strings.TrimSpace(a.DestinationDenom)
	return a
}

type RoutePolicy struct {
	AllowedMemoPrefixes []string `json:"allowed_memo_prefixes"`
	AllowedActionTypes  []string `json:"allowed_action_types"`
}

func (p RoutePolicy) Canonical() RoutePolicy {
	if len(p.AllowedMemoPrefixes) > 0 {
		prefixes := make([]string, 0, len(p.AllowedMemoPrefixes))
		for _, prefix := range p.AllowedMemoPrefixes {
			prefixes = append(prefixes, strings.TrimSpace(prefix))
		}
		p.AllowedMemoPrefixes = prefixes
	}

	if len(p.AllowedActionTypes) > 0 {
		types := make([]string, 0, len(p.AllowedActionTypes))
		for _, actionType := range p.AllowedActionTypes {
			types = append(types, strings.TrimSpace(actionType))
		}
		p.AllowedActionTypes = types
	}
	return p
}

func (p RoutePolicy) AllowsMemo(memo string) bool {
	if len(p.AllowedMemoPrefixes) == 0 {
		return true
	}

	trimmedMemo := strings.TrimSpace(memo)
	for _, prefix := range p.Canonical().AllowedMemoPrefixes {
		if prefix == "" && trimmedMemo == "" {
			return true
		}
		if prefix != "" && strings.HasPrefix(trimmedMemo, prefix) {
			return true
		}
	}
	return false
}

func (p RoutePolicy) AllowsAction(memo string) bool {
	if len(p.AllowedActionTypes) == 0 {
		return true
	}

	actionType := routeActionType(memo)
	if actionType == "" {
		return true
	}
	for _, allowed := range p.Canonical().AllowedActionTypes {
		if allowed == actionType {
			return true
		}
	}
	return false
}

type RouteProfile struct {
	RouteID            string      `json:"route_id"`
	DestinationChainID string      `json:"destination_chain_id"`
	ChannelID          string      `json:"channel_id"`
	Enabled            bool        `json:"enabled"`
	Assets             []AssetRoute `json:"assets"`
	Policy             RoutePolicy `json:"policy"`
}

func (p RouteProfile) ValidateBasic() error {
	if strings.TrimSpace(p.RouteID) == "" {
		return fmt.Errorf("%w: missing route id", ErrInvalidRouteProfile)
	}
	if strings.TrimSpace(p.DestinationChainID) == "" {
		return fmt.Errorf("%w: missing destination chain id", ErrInvalidRouteProfile)
	}
	if strings.TrimSpace(p.ChannelID) == "" {
		return fmt.Errorf("%w: missing channel id", ErrInvalidRouteProfile)
	}
	if len(p.Assets) == 0 {
		return fmt.Errorf("%w: missing allowed assets", ErrInvalidRouteProfile)
	}
	for _, asset := range p.Assets {
		if err := asset.ValidateBasic(); err != nil {
			return err
		}
	}
	return nil
}

func (p RouteProfile) Canonical() RouteProfile {
	p.RouteID = strings.TrimSpace(p.RouteID)
	p.DestinationChainID = strings.TrimSpace(p.DestinationChainID)
	p.ChannelID = strings.TrimSpace(p.ChannelID)
	p.Policy = p.Policy.Canonical()
	if len(p.Assets) == 0 {
		return p
	}

	assets := make([]AssetRoute, 0, len(p.Assets))
	for _, asset := range p.Assets {
		assets = append(assets, asset.Canonical())
	}
	p.Assets = assets
	return p
}

func (p RouteProfile) AssetRoute(assetID string) (AssetRoute, bool) {
	trimmedAssetID := strings.TrimSpace(assetID)
	for _, asset := range p.Canonical().Assets {
		if asset.AssetID == trimmedAssetID {
			return asset, true
		}
	}
	return AssetRoute{}, false
}

func routeActionType(memo string) string {
	memo = strings.TrimSpace(memo)
	if memo == "" {
		return ""
	}
	prefix, _, _ := strings.Cut(memo, ":")
	return strings.TrimSpace(prefix)
}
