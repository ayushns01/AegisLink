package keeper

import (
	"errors"
	"strings"

	storetypes "cosmossdk.io/store/types"
	"github.com/ayushns01/aegislink/chain/aegislink/internal/sdkstore"
	registrytypes "github.com/ayushns01/aegislink/chain/aegislink/x/registry/types"
)

var (
	ErrAssetAlreadyExists = errors.New("asset already exists")
	ErrAssetNotFound      = errors.New("asset not found")
)

type Keeper struct {
	assets      map[string]registrytypes.Asset
	prefixStore *sdkstore.JSONPrefixStore
	legacyStore *sdkstore.JSONStateStore
}

const assetPrefix = "asset"

func NewKeeper() *Keeper {
	return &Keeper{
		assets: make(map[string]registrytypes.Asset),
	}
}

func NewStoreKeeper(multiStore storetypes.CommitMultiStore, key *storetypes.KVStoreKey) (*Keeper, error) {
	prefixStore, err := sdkstore.NewJSONPrefixStore(multiStore, key)
	if err != nil {
		return nil, err
	}
	stateStore, err := sdkstore.NewJSONStateStore(multiStore, key)
	if err != nil {
		return nil, err
	}

	keeper := NewKeeper()
	keeper.prefixStore = prefixStore
	keeper.legacyStore = stateStore

	if prefixStore.HasAny(assetPrefix) {
		if err := keeper.loadFromPrefixStore(); err != nil {
			return nil, err
		}
		return keeper, nil
	}
	if stateStore.HasState() {
		var assets []registrytypes.Asset
		if err := stateStore.Load(&assets); err != nil {
			return nil, err
		}
		if err := keeper.ImportAssets(assets); err != nil {
			return nil, err
		}
	}

	return keeper, nil
}

func (k *Keeper) RegisterAsset(asset registrytypes.Asset) error {
	stored := canonicalAsset(asset)
	stored = deriveAssetMetadata(stored)
	if err := stored.ValidateBasic(); err != nil {
		return err
	}

	key := assetKey(stored.AssetID)
	if _, exists := k.assets[key]; exists {
		return ErrAssetAlreadyExists
	}
	if err := k.ensureDestinationDenomAvailable(stored, key); err != nil {
		return err
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
		stored := deriveAssetMetadata(canonicalAsset(asset))
		if err := stored.ValidateBasic(); err != nil {
			return err
		}
		key := assetKey(stored.AssetID)
		if err := k.ensureDestinationDenomAvailable(stored, key); err != nil {
			return err
		}
		k.assets[key] = stored
	}
	return k.persist()
}

func (k *Keeper) persist() error {
	if k.prefixStore == nil {
		return nil
	}
	if err := k.prefixStore.ClearPrefix(assetPrefix); err != nil {
		return err
	}
	for key, asset := range k.assets {
		if err := k.prefixStore.Save(assetPrefix, key, canonicalAsset(asset)); err != nil {
			return err
		}
	}
	return k.prefixStore.Commit()
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
	asset.SourceAssetAddress = strings.TrimSpace(asset.SourceAssetAddress)
	asset.SourceContract = strings.TrimSpace(asset.SourceContract)
	asset.SourceAssetKind = registrytypes.SourceAssetKind(strings.TrimSpace(string(asset.SourceAssetKind)))
	asset.Denom = strings.TrimSpace(asset.Denom)
	asset.DestinationDenom = strings.TrimSpace(asset.DestinationDenom)
	asset.DisplayName = strings.TrimSpace(asset.DisplayName)
	asset.DisplaySymbol = strings.TrimSpace(asset.DisplaySymbol)
	if asset.DisplaySymbol == "" {
		asset.DisplaySymbol = asset.DisplayName
	}
	if asset.SourceAssetAddress == "" {
		asset.SourceAssetAddress = asset.SourceContract
	}
	if asset.SourceContract == "" {
		asset.SourceContract = asset.SourceAssetAddress
	}
	if asset.SourceAssetKind == "" && asset.SourceAssetAddress != "" {
		asset.SourceAssetKind = registrytypes.SourceAssetKindERC20
	}
	return asset
}

func deriveAssetMetadata(asset registrytypes.Asset) registrytypes.Asset {
	asset.DestinationDenom = deriveDestinationDenom(asset)
	asset.Denom = asset.DestinationDenom
	return asset
}

func (k *Keeper) ensureDestinationDenomAvailable(asset registrytypes.Asset, key string) error {
	for existingKey, existingAsset := range k.assets {
		if existingKey == key {
			continue
		}
		if strings.EqualFold(existingAsset.DestinationDenom, asset.DestinationDenom) {
			return ErrAssetAlreadyExists
		}
	}
	return nil
}

func deriveDestinationDenom(asset registrytypes.Asset) string {
	prefix := assetPrefixFromAssetID(asset.AssetID)
	if prefix == "" {
		prefix = "asset"
	}

	switch asset.SourceAssetKind {
	case registrytypes.SourceAssetKindNativeETH:
		return "u" + prefix
	case registrytypes.SourceAssetKindERC20:
		symbol := assetSuffixFromAssetID(asset.AssetID)
		if symbol == "" {
			symbol = normalizedSymbol(asset.DisplaySymbol)
		}
		if symbol == "" {
			symbol = "token"
		}
		return "u" + prefix + symbol
	default:
		return "u" + prefix + normalizedSymbol(asset.DisplaySymbol)
	}
}

func assetPrefixFromAssetID(assetID string) string {
	trimmed := strings.TrimSpace(assetID)
	if trimmed == "" {
		return ""
	}
	if prefix, _, ok := strings.Cut(trimmed, "."); ok {
		return normalizedSymbol(prefix)
	}
	return normalizedSymbol(trimmed)
}

func assetSuffixFromAssetID(assetID string) string {
	trimmed := strings.TrimSpace(assetID)
	if trimmed == "" {
		return ""
	}
	if _, suffix, ok := strings.Cut(trimmed, "."); ok {
		return normalizedSymbol(suffix)
	}
	return ""
}

func normalizedSymbol(value string) string {
	return strings.ToLower(strings.TrimSpace(strings.NewReplacer(".", "", "-", "", "_", "").Replace(value)))
}

func (k *Keeper) loadFromPrefixStore() error {
	k.assets = make(map[string]registrytypes.Asset)
	return k.prefixStore.LoadAll(assetPrefix, func() any {
		return &registrytypes.Asset{}
	}, func(_ string, value any) error {
		asset := deriveAssetMetadata(canonicalAsset(*(value.(*registrytypes.Asset))))
		if err := asset.ValidateBasic(); err != nil {
			return err
		}
		key := assetKey(asset.AssetID)
		if err := k.ensureDestinationDenomAvailable(asset, key); err != nil {
			return err
		}
		k.assets[key] = asset
		return nil
	})
}
