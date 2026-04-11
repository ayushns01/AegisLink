package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type rlyManifest struct {
	SourceChainID      string             `json:"source_chain_id"`
	DestinationChainID string             `json:"destination_chain_id"`
	ChannelID          string             `json:"channel_id"`
	PortID             string             `json:"port_id"`
	RouteID            string             `json:"route_id"`
	Assets             []rlyManifestAsset `json:"assets"`
}

type rlyManifestAsset struct {
	AssetID          string `json:"asset_id"`
	SourceDenom      string `json:"source_denom"`
	DestinationDenom string `json:"destination_denom"`
}

type chainMetadata struct {
	ChainName    string `json:"chain_name"`
	ChainID      string `json:"chain_id"`
	Bech32Prefix string `json:"bech32_prefix"`
	Fees         struct {
		FeeTokens []struct {
			Denom            string  `json:"denom"`
			FixedMinGasPrice float64 `json:"fixed_min_gas_price"`
		} `json:"fee_tokens"`
	} `json:"fees"`
	APIs struct {
		RPC []struct {
			Address string `json:"address"`
		} `json:"rpc"`
		GRPC []struct {
			Address string `json:"address"`
		} `json:"grpc"`
	} `json:"apis"`
}

type generatedPath struct {
	PathName string      `json:"path_name"`
	Src      pathEnd     `json:"src"`
	Dst      pathEnd     `json:"dst"`
	Manifest rlyManifest `json:"manifest"`
}

type pathEnd struct {
	ChainID      string `json:"chain_id"`
	ClientID     string `json:"client_id"`
	ConnectionID string `json:"connection_id"`
	PortID       string `json:"port_id"`
	ChannelID    string `json:"channel_id"`
}

type sourceReadyState struct {
	RPCAddress      string   `json:"rpc_address"`
	CometRPCAddress string   `json:"comet_rpc_address"`
	GRPCAddress     string   `json:"grpc_address"`
	CoreStoreKeys   []string `json:"core_store_keys"`
}

func main() {
	if err := runBootstrapRlyPath(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runBootstrapRlyPath() error {
	flags := flag.NewFlagSet("bootstrap_rly_path", flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	manifestPath := flags.String("manifest-file", "", "path to the public IBC manifest")
	destinationMetadataPath := flags.String("destination-metadata-file", "", "path to chain-registry-style destination metadata")
	outputDir := flags.String("output-dir", "", "directory for generated rly files")
	sourceReadyFile := flags.String("source-ready-file", "", "path to a demo-node ready-state file")
	sourceRPC := flags.String("source-rpc-addr", "", "AegisLink demo-node RPC address")
	sourceRPCWS := flags.String("source-rpc-ws-addr", "", "AegisLink demo-node websocket address")
	sourceGRPC := flags.String("source-grpc-addr", "", "AegisLink demo-node gRPC address")
	sourceKeyName := flags.String("source-key-name", "aegislink-demo", "rly key name for AegisLink")
	destKeyName := flags.String("destination-key-name", "osmosis-demo", "rly key name for the destination chain")
	destPrefixOverride := flags.String("destination-account-prefix", "", "override destination account prefix")
	destGasDenomOverride := flags.String("destination-gas-price-denom", "", "override destination gas price denom")
	destGasAmountOverride := flags.String("destination-gas-price-amount", "", "override destination gas price amount")
	pathName := flags.String("path-name", "", "generated path name")
	sourcePrefix := flags.String("source-account-prefix", "cosmos", "AegisLink bech32 prefix")
	sourceGasDenom := flags.String("source-gas-price-denom", "ueth", "AegisLink gas price denom")
	sourceGasAmount := flags.String("source-gas-price-amount", "0.0", "AegisLink gas price amount")
	if err := flags.Parse(os.Args[1:]); err != nil {
		return err
	}

	sourceRPCValue, sourceRPCWSValue, sourceGRPCValue, err := resolveSourceEndpoints(
		strings.TrimSpace(*sourceReadyFile),
		strings.TrimSpace(*sourceRPC),
		strings.TrimSpace(*sourceRPCWS),
		strings.TrimSpace(*sourceGRPC),
	)
	if err != nil {
		return err
	}

	required := map[string]string{
		"--manifest-file":             strings.TrimSpace(*manifestPath),
		"--destination-metadata-file": strings.TrimSpace(*destinationMetadataPath),
		"--output-dir":                strings.TrimSpace(*outputDir),
		"--source-rpc-addr":           sourceRPCValue,
		"--source-rpc-ws-addr":        sourceRPCWSValue,
		"--source-grpc-addr":          sourceGRPCValue,
	}
	for flagName, value := range required {
		if value == "" {
			return fmt.Errorf("missing %s", flagName)
		}
	}

	manifest, err := loadRlyManifest(*manifestPath)
	if err != nil {
		return err
	}
	if len(manifest.Assets) == 0 {
		return fmt.Errorf("manifest %s does not include any assets", filepath.Clean(*manifestPath))
	}
	metadata, err := loadChainMetadata(*destinationMetadataPath)
	if err != nil {
		return err
	}

	destPrefix := strings.TrimSpace(*destPrefixOverride)
	if destPrefix == "" {
		destPrefix = strings.TrimSpace(metadata.Bech32Prefix)
	}
	if destPrefix == "" {
		return fmt.Errorf("destination metadata %s is missing bech32_prefix", filepath.Clean(*destinationMetadataPath))
	}

	destGasDenom := strings.TrimSpace(*destGasDenomOverride)
	destGasAmount := strings.TrimSpace(*destGasAmountOverride)
	if len(metadata.Fees.FeeTokens) > 0 {
		if destGasDenom == "" {
			destGasDenom = strings.TrimSpace(metadata.Fees.FeeTokens[0].Denom)
		}
		if destGasAmount == "" {
			destGasAmount = strconv.FormatFloat(metadata.Fees.FeeTokens[0].FixedMinGasPrice, 'f', -1, 64)
		}
	}
	if destGasDenom == "" || destGasAmount == "" {
		return fmt.Errorf("destination gas price metadata is incomplete")
	}

	destRPC := firstAPIAddress(metadata.APIs.RPC)
	destGRPC := firstAPIAddress(metadata.APIs.GRPC)
	if destRPC == "" || destGRPC == "" {
		return fmt.Errorf("destination metadata %s must include both rpc and grpc addresses", filepath.Clean(*destinationMetadataPath))
	}

	if strings.TrimSpace(*pathName) == "" {
		*pathName = manifest.RouteID
	}

	configDir := filepath.Clean(*outputDir)
	pathsDir := filepath.Join(configDir, "paths")
	if err := os.MkdirAll(pathsDir, 0o755); err != nil {
		return err
	}

	configPath := filepath.Join(configDir, "config.yaml")
	configBody := renderRlyConfig(renderRlyConfigParams{
		SourceChainID:        manifest.SourceChainID,
		SourceRPC:            sourceRPCValue,
		SourceRPCWS:          sourceRPCWSValue,
		SourceGRPC:           sourceGRPCValue,
		SourceKeyName:        strings.TrimSpace(*sourceKeyName),
		SourceAccountPrefix:  strings.TrimSpace(*sourcePrefix),
		SourceGasPrices:      strings.TrimSpace(*sourceGasAmount) + strings.TrimSpace(*sourceGasDenom),
		DestinationChainID:   manifest.DestinationChainID,
		DestinationRPC:       destRPC,
		DestinationGRPC:      destGRPC,
		DestinationKeyName:   strings.TrimSpace(*destKeyName),
		DestinationPrefix:    destPrefix,
		DestinationGasPrices: destGasAmount + destGasDenom,
	})
	if err := os.WriteFile(configPath, []byte(configBody), 0o644); err != nil {
		return err
	}

	path := generatedPath{
		PathName: strings.TrimSpace(*pathName),
		Src: pathEnd{
			ChainID:      manifest.SourceChainID,
			ClientID:     "07-tendermint-aegislink-demo",
			ConnectionID: "connection-aegislink-demo",
			PortID:       manifest.PortID,
			ChannelID:    manifest.ChannelID,
		},
		Dst: pathEnd{
			ChainID:      manifest.DestinationChainID,
			ClientID:     "07-tendermint-osmosis-demo",
			ConnectionID: "connection-osmosis-demo",
			PortID:       manifest.PortID,
			ChannelID:    manifest.ChannelID,
		},
		Manifest: manifest,
	}
	pathBytes, err := json.MarshalIndent(path, "", "  ")
	if err != nil {
		return err
	}
	pathBytes = append(pathBytes, '\n')
	pathFile := filepath.Join(pathsDir, strings.TrimSpace(*pathName)+".json")
	if err := os.WriteFile(pathFile, pathBytes, 0o644); err != nil {
		return err
	}

	return json.NewEncoder(os.Stdout).Encode(map[string]any{
		"status":               "bootstrapped",
		"path_name":            strings.TrimSpace(*pathName),
		"config_path":          configPath,
		"path_file":            pathFile,
		"source_chain_id":      manifest.SourceChainID,
		"destination_chain_id": manifest.DestinationChainID,
		"destination_rpc":      destRPC,
		"destination_grpc":     destGRPC,
	})
}

func resolveSourceEndpoints(readyFilePath, sourceRPC, sourceRPCWS, sourceGRPC string) (string, string, string, error) {
	if readyFilePath != "" {
		ready, err := loadSourceReadyState(readyFilePath)
		if err != nil {
			return "", "", "", err
		}
		if !containsAllStrings(ready.CoreStoreKeys, "ibc", "transfer") {
			return "", "", "", fmt.Errorf("missing required source core store keys: need ibc and transfer in %s", filepath.Clean(readyFilePath))
		}
		readyRPC := strings.TrimSpace(ready.CometRPCAddress)
		if readyRPC == "" {
			readyRPC = strings.TrimSpace(ready.RPCAddress)
		}
		if sourceRPC == "" {
			sourceRPC = normalizeHTTPAddress(readyRPC)
		}
		if sourceRPCWS == "" {
			sourceRPCWS = deriveWebsocketAddress(readyRPC)
		}
		if sourceGRPC == "" {
			sourceGRPC = normalizeHTTPAddress(ready.GRPCAddress)
		}
	}
	return sourceRPC, sourceRPCWS, sourceGRPC, nil
}

func loadSourceReadyState(path string) (sourceReadyState, error) {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return sourceReadyState{}, err
	}
	var ready sourceReadyState
	if err := json.Unmarshal(data, &ready); err != nil {
		return sourceReadyState{}, err
	}
	return ready, nil
}

func loadRlyManifest(path string) (rlyManifest, error) {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return rlyManifest{}, err
	}
	var manifest rlyManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return rlyManifest{}, err
	}
	return manifest, nil
}

func normalizeHTTPAddress(address string) string {
	address = strings.TrimSpace(address)
	if address == "" {
		return ""
	}
	if strings.Contains(address, "://") {
		return address
	}
	return "http://" + address
}

func deriveWebsocketAddress(address string) string {
	address = strings.TrimSpace(address)
	if address == "" {
		return ""
	}
	if strings.HasPrefix(address, "ws://") || strings.HasPrefix(address, "wss://") {
		if strings.HasSuffix(address, "/websocket") {
			return address
		}
		return strings.TrimRight(address, "/") + "/websocket"
	}
	if strings.HasPrefix(address, "http://") {
		return "ws://" + strings.TrimPrefix(strings.TrimRight(address, "/"), "http://") + "/websocket"
	}
	if strings.HasPrefix(address, "https://") {
		return "wss://" + strings.TrimPrefix(strings.TrimRight(address, "/"), "https://") + "/websocket"
	}
	return "ws://" + strings.TrimRight(address, "/") + "/websocket"
}

func loadChainMetadata(path string) (chainMetadata, error) {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return chainMetadata{}, err
	}
	var metadata chainMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return chainMetadata{}, err
	}
	return metadata, nil
}

func containsAllStrings(values []string, required ...string) bool {
	for _, want := range required {
		found := false
		for _, value := range values {
			if value == want {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func firstAPIAddress(entries []struct {
	Address string `json:"address"`
}) string {
	for _, entry := range entries {
		if value := strings.TrimSpace(entry.Address); value != "" {
			return value
		}
	}
	return ""
}

type renderRlyConfigParams struct {
	SourceChainID       string
	SourceRPC           string
	SourceRPCWS         string
	SourceGRPC          string
	SourceKeyName       string
	SourceAccountPrefix string
	SourceGasPrices     string

	DestinationChainID   string
	DestinationRPC       string
	DestinationGRPC      string
	DestinationKeyName   string
	DestinationPrefix    string
	DestinationGasPrices string
}

func renderRlyConfig(params renderRlyConfigParams) string {
	return fmt.Sprintf(`global:
  api-listen-addr: :5183
  timeout: 10s
  memo: ""
chains:
  - id: %s
    type: cosmos
    value:
      key: %s
      chain-id: %s
      rpc-addr: %s
      websocket-addr: %s
      grpc-addr: %s
      account-prefix: %s
      keyring-backend: test
      gas-adjustment: 1.3
      gas-prices: %s
      debug: false
      timeout: 10s
      output-format: json
      sign-mode: direct
  - id: %s
    type: cosmos
    value:
      key: %s
      chain-id: %s
      rpc-addr: %s
      websocket-addr: ""
      grpc-addr: %s
      account-prefix: %s
      keyring-backend: test
      gas-adjustment: 1.3
      gas-prices: %s
      debug: false
      timeout: 10s
      output-format: json
      sign-mode: direct
`, params.SourceChainID, params.SourceKeyName, params.SourceChainID, params.SourceRPC, params.SourceRPCWS, params.SourceGRPC, params.SourceAccountPrefix, params.SourceGasPrices, params.DestinationChainID, params.DestinationKeyName, params.DestinationChainID, params.DestinationRPC, params.DestinationGRPC, params.DestinationPrefix, params.DestinationGasPrices)
}
