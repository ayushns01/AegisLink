package keeper

import (
	"errors"
	"strings"

	"github.com/ayushns01/aegislink/chain/aegislink/internal/sdkstore"
	registrytypes "github.com/ayushns01/aegislink/chain/aegislink/x/registry/types"
	storetypes "cosmossdk.io/store/types"
)

var (
	ErrAssetAlreadyExists = errors.New("asset already exists")
	ErrAssetNotFound      = errors.New("asset not found")
)

type Keeper struct {
	assets     map[string]registrytypes.Asset
	stateStore *sdkstore.JSONStateStore
}

func NewKeeper() *Keeper {
	return &Keeper{
		assets: make(map[string]registrytypes.Asset),
	}
}

func NewStoreKeeper(multiStore storetypes.CommitMultiStore, key *storetypes.KVStoreKey) (*Keeper, error) {
	stateStore, err := sdkstore.NewJSONStateStore(multiStore, key)
	if err != nil {
		return nil, err
	}

	keeper := NewKeeper()
	keeper.stateStore = stateStore

	var assets []registrytypes.Asset
	if err := stateStore.Load(&assets); err != nil {
		return nil, err
	}
	if err := keeper.ImportAssets(assets); err != nil {
		return nil, err
	}

	return keeper, nil
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
	return k.persist()
}

func (k *Keeper) GetAsset(assetID string) (registrytypes.Asset, bool) {
	asset, ok := k.assets[assetKey(assetID)]
	return asset, ok
}

func (k *Keeper) DisableAsset(assetID string) error {
	return k.setAssetEnabled(assetID, false)
}

func (k *Keeper) EnableAsset(assetID string) error {
	return k.setAssetEnabled(assetID, true)
}

func (k *Keeper) setAssetEnabled(assetID string, enabled bool) error {
	key := assetKey(assetID)
	asset, ok := k.assets[key]
	if !ok {
		return ErrAssetNotFound
	}

	asset.Enabled = enabled
	k.assets[key] = asset
	return k.persist()
}

func (k *Keeper) ExportAssets() []registrytypes.Asset {
	assets := make([]registrytypes.Asset, 0, len(k.assets))
	for _, asset := range k.assets {
		assets = append(assets, canonicalAsset(asset))
	}
	return assets
}

func (k *Keeper) ImportAssets(assets []registrytypes.Asset) error {
	k.assets = make(map[string]registrytypes.Asset, len(assets))
	for _, asset := range assets {
		if err := k.RegisterAsset(asset); err != nil {
			return err
		}
	}
	return k.persist()
}

func (k *Keeper) persist() error {
	if k.stateStore == nil {
		return nil
	}
	return k.stateStore.Save(k.ExportAssets())
}

func (k *Keeper) Flush() error {
	return k.persist()
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
