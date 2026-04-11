package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	aegisapp "github.com/ayushns01/aegislink/chain/aegislink/app"
	limittypes "github.com/ayushns01/aegislink/chain/aegislink/x/limits/types"
	registrytypes "github.com/ayushns01/aegislink/chain/aegislink/x/registry/types"
)

type registryFile struct {
	Assets []registryAssetRecord `json:"assets"`
}

type registryAssetRecord struct {
	AssetID            string `json:"asset_id"`
	SourceChainID      string `json:"source_chain_id"`
	SourceAssetKind    string `json:"source_asset_kind"`
	SourceAssetAddress string `json:"source_asset_address,omitempty"`
	SourceContract     string `json:"source_contract,omitempty"`
	Denom              string `json:"denom"`
	Decimals           uint32 `json:"decimals"`
	DisplayName        string `json:"display_name"`
	DisplaySymbol      string `json:"display_symbol"`
	Enabled            bool   `json:"enabled"`
}

var evmAddressPattern = regexp.MustCompile(`^0x[0-9a-fA-F]{40}$`)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	flags := flag.NewFlagSet("seed_public_bridge_assets", flag.ContinueOnError)
	flags.SetOutput(ioDiscard{})

	homeDir := flags.String("home", "", "AegisLink home directory")
	registryPath := flags.String("registry-file", "", "path to bridge-assets.json")
	windowSeconds := flags.Uint64("window-seconds", 600, "default rate-limit window seconds")
	nativeMaxAmount := flags.String("native-max-amount", "", "optional max amount override for native ETH assets")
	erc20MaxAmount := flags.String("erc20-max-amount", "", "optional max amount override for ERC-20 assets")
	if err := flags.Parse(os.Args[1:]); err != nil {
		return err
	}
	if strings.TrimSpace(*homeDir) == "" {
		return fmt.Errorf("missing --home")
	}
	if strings.TrimSpace(*registryPath) == "" {
		return fmt.Errorf("missing --registry-file")
	}

	payload, err := loadRegistryFile(*registryPath)
	if err != nil {
		return err
	}
	if len(payload.Assets) == 0 {
		return fmt.Errorf("registry file %s does not contain any assets", filepath.Clean(*registryPath))
	}

	cfg, err := aegisapp.ResolveConfig(aegisapp.Config{
		HomeDir: strings.TrimSpace(*homeDir),
	})
	if err != nil {
		return err
	}
	app, err := aegisapp.LoadWithConfig(cfg)
	if err != nil {
		return err
	}
	defer func() { _ = app.Close() }()

	seededAssets := make([]string, 0, len(payload.Assets))
	for _, asset := range payload.Assets {
		normalized := normalizeAsset(asset)
		if err := validateSeedableAsset(normalized); err != nil {
			return err
		}
		if err := ensureAsset(app, normalized); err != nil {
			return err
		}
		maxAmount, err := limitAmountForAsset(normalized, *nativeMaxAmount, *erc20MaxAmount)
		if err != nil {
			return err
		}
		if err := app.SetLimit(limittypes.RateLimit{
			AssetID:       normalized.AssetID,
			WindowSeconds: *windowSeconds,
			MaxAmount:     maxAmount,
		}); err != nil {
			return err
		}
		seededAssets = append(seededAssets, normalized.AssetID)
	}

	if err := app.Save(); err != nil {
		return err
	}

	return json.NewEncoder(os.Stdout).Encode(map[string]any{
		"status":         "seeded",
		"home_dir":       cfg.HomeDir,
		"registry_file":  filepath.Clean(*registryPath),
		"seeded_assets":  seededAssets,
		"window_seconds": *windowSeconds,
	})
}

func loadRegistryFile(path string) (registryFile, error) {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return registryFile{}, err
	}
	var payload registryFile
	if err := json.Unmarshal(data, &payload); err != nil {
		return registryFile{}, err
	}
	return payload, nil
}

func normalizeAsset(asset registryAssetRecord) registrytypes.Asset {
	normalized := registrytypes.Asset{
		AssetID:            strings.TrimSpace(asset.AssetID),
		SourceChainID:      strings.TrimSpace(asset.SourceChainID),
		SourceAssetKind:    registrytypes.SourceAssetKind(strings.TrimSpace(asset.SourceAssetKind)),
		SourceAssetAddress: strings.TrimSpace(asset.SourceAssetAddress),
		SourceContract:     strings.TrimSpace(asset.SourceContract),
		Denom:              strings.TrimSpace(asset.Denom),
		Decimals:           asset.Decimals,
		DisplayName:        strings.TrimSpace(asset.DisplayName),
		DisplaySymbol:      strings.TrimSpace(asset.DisplaySymbol),
		Enabled:            asset.Enabled,
	}
	if normalized.SourceContract == "" {
		normalized.SourceContract = normalized.SourceAssetAddress
	}
	return normalized
}

func validateSeedableAsset(asset registrytypes.Asset) error {
	if asset.SourceAssetKind == registrytypes.SourceAssetKindERC20 {
		if !evmAddressPattern.MatchString(asset.SourceAssetAddress) {
			return fmt.Errorf("invalid ERC-20 source asset address for %s", asset.AssetID)
		}
	}
	return asset.ValidateBasic()
}

func ensureAsset(app *aegisapp.App, asset registrytypes.Asset) error {
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
		existing.Decimals == expected.Decimals &&
		existing.DisplayName == expected.DisplayName &&
		existing.DisplaySymbol == expected.DisplaySymbol &&
		existing.Enabled == expected.Enabled
}

func limitAmountForAsset(asset registrytypes.Asset, nativeOverride string, erc20Override string) (*big.Int, error) {
	switch asset.SourceAssetKind {
	case registrytypes.SourceAssetKindNativeETH:
		if strings.TrimSpace(nativeOverride) != "" {
			return parseAmount(nativeOverride)
		}
		return scaledAmount(2, asset.Decimals), nil
	case registrytypes.SourceAssetKindERC20:
		if strings.TrimSpace(erc20Override) != "" {
			return parseAmount(erc20Override)
		}
		return scaledAmount(100, asset.Decimals), nil
	default:
		return nil, fmt.Errorf("unsupported asset kind %q for %s", asset.SourceAssetKind, asset.AssetID)
	}
}

func parseAmount(raw string) (*big.Int, error) {
	amount, ok := new(big.Int).SetString(strings.TrimSpace(raw), 10)
	if !ok || amount.Sign() <= 0 {
		return nil, fmt.Errorf("invalid amount %q", raw)
	}
	return amount, nil
}

func scaledAmount(units int64, decimals uint32) *big.Int {
	base := big.NewInt(units)
	scale := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil)
	return new(big.Int).Mul(base, scale)
}

type ioDiscard struct{}

func (ioDiscard) Write(p []byte) (int, error) {
	return len(p), nil
}
