package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	aegisapp "github.com/ayushns01/aegislink/chain/aegislink/app"
	ibcroutertypes "github.com/ayushns01/aegislink/chain/aegislink/x/ibcrouter/types"
)

type seedPublicIBCManifest struct {
	Enabled             bool                 `json:"enabled"`
	SourceChainID       string               `json:"source_chain_id"`
	DestinationChainID  string               `json:"destination_chain_id"`
	Provider            string               `json:"provider"`
	WalletPrefix        string               `json:"wallet_prefix"`
	ChannelID           string               `json:"channel_id"`
	PortID              string               `json:"port_id"`
	RouteID             string               `json:"route_id"`
	AllowedMemoPrefixes []string             `json:"allowed_memo_prefixes"`
	AllowedActionTypes  []string             `json:"allowed_action_types"`
	Assets              []seedPublicIBCAsset `json:"assets"`
}

type seedPublicIBCAsset struct {
	AssetID          string `json:"asset_id"`
	SourceDenom      string `json:"source_denom"`
	DestinationDenom string `json:"destination_denom"`
}

func main() {
	if err := runSeedPublicIBCRoute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runSeedPublicIBCRoute() error {
	flags := flag.NewFlagSet("seed_public_ibc_route", flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	homeDir := flags.String("home", "", "AegisLink home directory")
	manifestPath := flags.String("manifest-file", "", "path to public IBC manifest")
	if err := flags.Parse(os.Args[1:]); err != nil {
		return err
	}
	if strings.TrimSpace(*homeDir) == "" {
		return fmt.Errorf("missing --home")
	}
	if strings.TrimSpace(*manifestPath) == "" {
		return fmt.Errorf("missing --manifest-file")
	}

	manifest, err := loadSeedPublicIBCManifest(*manifestPath)
	if err != nil {
		return err
	}
	if strings.TrimSpace(manifest.RouteID) == "" {
		return fmt.Errorf("manifest %s is missing route_id", filepath.Clean(*manifestPath))
	}
	if strings.TrimSpace(manifest.DestinationChainID) == "" {
		return fmt.Errorf("manifest %s is missing destination_chain_id", filepath.Clean(*manifestPath))
	}
	if strings.TrimSpace(manifest.ChannelID) == "" {
		return fmt.Errorf("manifest %s is missing channel_id", filepath.Clean(*manifestPath))
	}
	if len(manifest.Assets) == 0 {
		return fmt.Errorf("manifest %s does not contain any assets", filepath.Clean(*manifestPath))
	}

	cfg, err := aegisapp.ResolveConfig(aegisapp.Config{HomeDir: strings.TrimSpace(*homeDir)})
	if err != nil {
		return err
	}
	app, err := aegisapp.LoadWithConfig(cfg)
	if err != nil {
		return err
	}
	defer func() { _ = app.Close() }()

	if sourceChainID := strings.TrimSpace(manifest.SourceChainID); sourceChainID != "" && sourceChainID != app.Config.ChainID {
		return fmt.Errorf("manifest source chain id %q does not match home chain id %q", sourceChainID, app.Config.ChainID)
	}

	profile := ibcroutertypes.RouteProfile{
		RouteID:            strings.TrimSpace(manifest.RouteID),
		DestinationChainID: strings.TrimSpace(manifest.DestinationChainID),
		ChannelID:          strings.TrimSpace(manifest.ChannelID),
		Enabled:            manifest.Enabled,
		Assets:             make([]ibcroutertypes.AssetRoute, 0, len(manifest.Assets)),
		Policy: ibcroutertypes.RoutePolicy{
			AllowedMemoPrefixes: cleanStrings(manifest.AllowedMemoPrefixes),
			AllowedActionTypes:  cleanStrings(manifest.AllowedActionTypes),
		},
	}
	for _, asset := range manifest.Assets {
		assetID := strings.TrimSpace(asset.AssetID)
		if assetID == "" {
			return fmt.Errorf("manifest %s contains an asset without asset_id", filepath.Clean(*manifestPath))
		}
		if _, ok := app.RegistryKeeper.GetAsset(assetID); !ok {
			return fmt.Errorf("asset %s is not registered in the AegisLink runtime", assetID)
		}
		destinationDenom := strings.TrimSpace(asset.DestinationDenom)
		if destinationDenom == "" {
			return fmt.Errorf("manifest %s contains an asset without destination_denom", filepath.Clean(*manifestPath))
		}
		profile.Assets = append(profile.Assets, ibcroutertypes.AssetRoute{
			AssetID:          assetID,
			DestinationDenom: destinationDenom,
		})
	}
	if err := app.SetRouteProfile(profile); err != nil {
		return err
	}
	if err := app.Save(); err != nil {
		return err
	}

	return json.NewEncoder(os.Stdout).Encode(map[string]any{
		"status":               "seeded",
		"home_dir":             cfg.HomeDir,
		"manifest_file":        filepath.Clean(*manifestPath),
		"route_id":             profile.RouteID,
		"destination_chain_id": profile.DestinationChainID,
		"enabled":              profile.Enabled,
		"asset_count":          len(profile.Assets),
	})
}

func loadSeedPublicIBCManifest(path string) (seedPublicIBCManifest, error) {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return seedPublicIBCManifest{}, err
	}
	var manifest seedPublicIBCManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return seedPublicIBCManifest{}, err
	}
	return manifest, nil
}

func cleanStrings(items []string) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
