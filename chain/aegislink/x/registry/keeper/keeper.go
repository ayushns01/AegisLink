package keeper

import (
	"errors"
	"strings"

	registrytypes "github.com/ayushns01/aegislink/chain/aegislink/x/registry/types"
)

var (
	ErrAssetAlreadyExists = errors.New("asset already exists")
	ErrAssetNotFound      = errors.New("asset not found")
)

type Keeper struct {
	assets map[string]registrytypes.Asset
}

func NewKeeper() *Keeper {
	return &Keeper{
		assets: make(map[string]registrytypes.Asset),
	}
}

func (k *Keeper) RegisterAsset(asset registrytypes.Asset) error {
	if err := asset.ValidateBasic(); err != nil {
		return err
	}

	stored := canonicalAsset(asset)
	key := assetKey(stored.AssetID)
	if _, exists := k.assets[key]; exists {
		return ErrAssetAlreadyExists
	}

	k.assets[key] = stored
	return nil
}

func (k *Keeper) GetAsset(assetID string) (registrytypes.Asset, bool) {
	asset, ok := k.assets[assetKey(assetID)]
	return asset, ok
}

func (k *Keeper) DisableAsset(assetID string) error {
	key := assetKey(assetID)
	asset, ok := k.assets[key]
	if !ok {
		return ErrAssetNotFound
	}

	asset.Enabled = false
	k.assets[key] = asset
	return nil
}

func assetKey(assetID string) string {
	return strings.TrimSpace(assetID)
}

func canonicalAsset(asset registrytypes.Asset) registrytypes.Asset {
	asset.AssetID = strings.TrimSpace(asset.AssetID)
	asset.SourceChainID = strings.TrimSpace(asset.SourceChainID)
	asset.SourceContract = strings.TrimSpace(asset.SourceContract)
	asset.Denom = strings.TrimSpace(asset.Denom)
	asset.DisplayName = strings.TrimSpace(asset.DisplayName)
	return asset
}
