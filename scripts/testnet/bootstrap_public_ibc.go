package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type ibcManifest struct {
	Enabled             bool               `json:"enabled"`
	SourceChainID       string             `json:"source_chain_id"`
	DestinationChainID  string             `json:"destination_chain_id"`
	Provider            string             `json:"provider"`
	WalletPrefix        string             `json:"wallet_prefix"`
	ChannelID           string             `json:"channel_id"`
	PortID              string             `json:"port_id"`
	RouteID             string             `json:"route_id"`
	AllowedMemoPrefixes []string           `json:"allowed_memo_prefixes,omitempty"`
	AllowedActionTypes  []string           `json:"allowed_action_types,omitempty"`
	Assets              []ibcManifestAsset `json:"assets"`
	Notes               string             `json:"notes,omitempty"`
}

type ibcManifestAsset struct {
	AssetID          string `json:"asset_id"`
	SourceDenom      string `json:"source_denom"`
	DestinationDenom string `json:"destination_denom"`
}

type publicIBCRegistryFile struct {
	Assets []publicIBCRegistryAsset `json:"assets"`
}

type publicIBCRegistryAsset struct {
	AssetID string `json:"asset_id"`
	Denom   string `json:"denom"`
}

func main() {
	if err := runBootstrapPublicIBC(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runBootstrapPublicIBC() error {
	flags := flag.NewFlagSet("bootstrap_public_ibc", flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	registryPath := flags.String("bridge-registry-file", "", "path to bridge-assets.json")
	outputPath := flags.String("output", "", "path to public IBC manifest")
	sourceChainID := flags.String("source-chain-id", "", "source AegisLink chain id")
	destinationChainID := flags.String("destination-chain-id", "", "destination IBC chain id")
	channelID := flags.String("channel-id", "", "destination channel id")
	routeID := flags.String("route-id", "", "route profile identifier")
	provider := flags.String("provider", "hermes", "operator provider label")
	walletPrefix := flags.String("wallet-prefix", "osmo", "destination wallet prefix")
	portID := flags.String("port-id", "transfer", "IBC port identifier")
	memoPrefixes := flags.String("memo-prefixes", "swap:,stake:", "comma-separated allowed memo prefixes")
	actionTypes := flags.String("action-types", "swap,stake", "comma-separated allowed action types")
	enabled := flags.Bool("enabled", false, "enable the manifest for live IBC usage")
	if err := flags.Parse(os.Args[1:]); err != nil {
		return err
	}

	if strings.TrimSpace(*registryPath) == "" {
		return fmt.Errorf("missing --bridge-registry-file")
	}
	if strings.TrimSpace(*outputPath) == "" {
		return fmt.Errorf("missing --output")
	}
	if strings.TrimSpace(*sourceChainID) == "" {
		return fmt.Errorf("missing --source-chain-id")
	}
	if strings.TrimSpace(*destinationChainID) == "" {
		return fmt.Errorf("missing --destination-chain-id")
	}
	if strings.TrimSpace(*channelID) == "" {
		return fmt.Errorf("missing --channel-id")
	}
	if strings.TrimSpace(*routeID) == "" {
		return fmt.Errorf("missing --route-id")
	}

	registryPayload, err := loadPublicIBCRegistry(*registryPath)
	if err != nil {
		return err
	}
	if len(registryPayload.Assets) == 0 {
		return fmt.Errorf("bridge registry %s does not contain any assets", filepath.Clean(*registryPath))
	}

	manifest := ibcManifest{
		Enabled:             *enabled,
		SourceChainID:       strings.TrimSpace(*sourceChainID),
		DestinationChainID:  strings.TrimSpace(*destinationChainID),
		Provider:            strings.TrimSpace(*provider),
		WalletPrefix:        strings.TrimSpace(*walletPrefix),
		ChannelID:           strings.TrimSpace(*channelID),
		PortID:              strings.TrimSpace(*portID),
		RouteID:             strings.TrimSpace(*routeID),
		AllowedMemoPrefixes: splitCSV(*memoPrefixes),
		AllowedActionTypes:  splitCSV(*actionTypes),
		Assets:              make([]ibcManifestAsset, 0, len(registryPayload.Assets)),
		Notes:               "Phase K public IBC bootstrap scaffold. Verify live Hermes connectivity and real Osmosis channel metadata before claiming public wallet delivery.",
	}

	for _, asset := range registryPayload.Assets {
		assetID := strings.TrimSpace(asset.AssetID)
		sourceDenom := strings.TrimSpace(asset.Denom)
		if assetID == "" || sourceDenom == "" {
			return fmt.Errorf("bridge registry %s contains an incomplete asset entry", filepath.Clean(*registryPath))
		}
		manifest.Assets = append(manifest.Assets, ibcManifestAsset{
			AssetID:          assetID,
			SourceDenom:      sourceDenom,
			DestinationDenom: "ibc/" + sourceDenom,
		})
	}

	if err := os.MkdirAll(filepath.Dir(filepath.Clean(*outputPath)), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.WriteFile(filepath.Clean(*outputPath), data, 0o644); err != nil {
		return err
	}

	return json.NewEncoder(os.Stdout).Encode(map[string]any{
		"status":               "bootstrapped",
		"manifest_path":        filepath.Clean(*outputPath),
		"bridge_registry_file": filepath.Clean(*registryPath),
		"route_id":             manifest.RouteID,
		"destination_chain_id": manifest.DestinationChainID,
		"asset_count":          len(manifest.Assets),
		"enabled":              manifest.Enabled,
	})
}

func loadPublicIBCRegistry(path string) (publicIBCRegistryFile, error) {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return publicIBCRegistryFile{}, err
	}
	var payload publicIBCRegistryFile
	if err := json.Unmarshal(data, &payload); err != nil {
		return publicIBCRegistryFile{}, err
	}
	return payload, nil
}

func splitCSV(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
