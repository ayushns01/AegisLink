package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	bridgemodule "github.com/ayushns01/aegislink/chain/aegislink/x/bridge"
	bridgetypes "github.com/ayushns01/aegislink/chain/aegislink/x/bridge/types"
	governancemodule "github.com/ayushns01/aegislink/chain/aegislink/x/governance"
	ibcroutermodule "github.com/ayushns01/aegislink/chain/aegislink/x/ibcrouter"
	limitsmodule "github.com/ayushns01/aegislink/chain/aegislink/x/limits"
	pausermodule "github.com/ayushns01/aegislink/chain/aegislink/x/pauser"
	registrymodule "github.com/ayushns01/aegislink/chain/aegislink/x/registry"
)

const AppName = "aegislink"
const DefaultChainID = "aegislink-local-1"
const DefaultRuntimeMode = "runtime-shell"
const RuntimeModeSDKStore = "sdk-store-runtime"

var ErrRuntimeAlreadyInitialized = errors.New("runtime already initialized")

type Config struct {
	AppName               string
	ChainID               string
	RuntimeMode           string
	HomeDir               string
	ConfigPath            string
	GenesisPath           string
	Modules               []string
	StatePath             string
	AllowedSigners        []string
	GovernanceAuthorities []string
	RequiredThreshold     uint32
}

type Genesis struct {
	AppName               string   `json:"app_name"`
	ChainID               string   `json:"chain_id"`
	Modules               []string `json:"modules"`
	AllowedSigners        []string `json:"allowed_signers"`
	GovernanceAuthorities []string `json:"governance_authorities"`
	RequiredThreshold     uint32   `json:"required_threshold"`
}

func DefaultConfig() Config {
	return Config{
		AppName:     AppName,
		ChainID:     DefaultChainID,
		RuntimeMode: DefaultRuntimeMode,
		HomeDir:     defaultHomeDir(),
		ConfigPath:  runtimeConfigPath(defaultHomeDir()),
		GenesisPath: runtimeGenesisPath(defaultHomeDir()),
		StatePath:   runtimeStatePath(defaultHomeDir()),
		Modules: []string{
			bridgemodule.ModuleName,
			registrymodule.ModuleName,
			limitsmodule.ModuleName,
			pausermodule.ModuleName,
			ibcroutermodule.ModuleName,
			governancemodule.ModuleName,
		},
		AllowedSigners:        bridgetypes.DefaultHarnessSignerAddresses()[:3],
		GovernanceAuthorities: []string{"guardian-1"},
		RequiredThreshold:     2,
	}
}

func ResolveConfig(cfg Config) (Config, error) {
	explicit := cfg
	cfg = normalizeConfig(cfg)

	stored, err := LoadConfig(cfg.ConfigPath)
	if err == nil {
		cfg = mergeConfig(stored, explicit)
		cfg = normalizeConfig(cfg)
	} else if !os.IsNotExist(err) {
		return Config{}, err
	}

	if err := validateConfig(cfg); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func InitHome(cfg Config, force bool) (Config, error) {
	cfg = normalizeConfig(cfg)
	if err := validateConfig(cfg); err != nil {
		return Config{}, err
	}

	configExists, err := fileExists(cfg.ConfigPath)
	if err != nil {
		return Config{}, err
	}
	genesisExists, err := fileExists(cfg.GenesisPath)
	if err != nil {
		return Config{}, err
	}
	stateExists, err := fileExists(cfg.StatePath)
	if err != nil {
		return Config{}, err
	}
	if !force && (configExists || genesisExists || stateExists) {
		return Config{}, ErrRuntimeAlreadyInitialized
	}

	if err := os.MkdirAll(filepath.Dir(cfg.ConfigPath), 0o755); err != nil {
		return Config{}, err
	}
	if cfg.RuntimeMode == RuntimeModeSDKStore {
		if err := os.MkdirAll(cfg.StatePath, 0o755); err != nil {
			return Config{}, err
		}
	} else {
		if err := os.MkdirAll(filepath.Dir(cfg.StatePath), 0o755); err != nil {
			return Config{}, err
		}
	}

	if err := writeConfigFile(cfg.ConfigPath, cfg); err != nil {
		return Config{}, err
	}
	if err := writeGenesisFile(cfg.GenesisPath, DefaultGenesis(cfg)); err != nil {
		return Config{}, err
	}
	if cfg.RuntimeMode == RuntimeModeSDKStore {
		runtime, err := newStoreRuntime(cfg)
		if err != nil {
			return Config{}, err
		}
		if err := runtime.db.Close(); err != nil {
			return Config{}, err
		}
	} else {
		if err := persistRuntimeState(cfg.StatePath, runtimeState{}); err != nil {
			return Config{}, err
		}
	}

	return cfg, nil
}

func LoadConfig(path string) (Config, error) {
	data, err := os.ReadFile(strings.TrimSpace(path))
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	return normalizeConfig(cfg), nil
}

func LoadGenesis(path string) (Genesis, error) {
	data, err := os.ReadFile(strings.TrimSpace(path))
	if err != nil {
		return Genesis{}, err
	}
	var genesis Genesis
	if err := json.Unmarshal(data, &genesis); err != nil {
		return Genesis{}, err
	}
	return genesis, nil
}

func DefaultGenesis(cfg Config) Genesis {
	cfg = normalizeConfig(cfg)
	return Genesis{
		AppName:               cfg.AppName,
		ChainID:               cfg.ChainID,
		Modules:               append([]string(nil), cfg.Modules...),
		AllowedSigners:        append([]string(nil), cfg.AllowedSigners...),
		GovernanceAuthorities: append([]string(nil), cfg.GovernanceAuthorities...),
		RequiredThreshold:     cfg.RequiredThreshold,
	}
}

func defaultHomeDir() string {
	return filepath.Join(os.TempDir(), "aegislinkd")
}

func runtimeConfigPath(homeDir string) string {
	return filepath.Join(homeDir, "config", "runtime.json")
}

func runtimeGenesisPath(homeDir string) string {
	return filepath.Join(homeDir, "config", "genesis.json")
}

func runtimeStatePath(homeDir string) string {
	return filepath.Join(homeDir, "data", "state.json")
}

func runtimeStorePath(homeDir string) string {
	return filepath.Join(homeDir, "data", "store")
}

func mergeConfig(base Config, override Config) Config {
	if override.AppName != "" {
		base.AppName = override.AppName
	}
	if override.ChainID != "" {
		base.ChainID = override.ChainID
	}
	if override.RuntimeMode != "" {
		base.RuntimeMode = override.RuntimeMode
	}
	if override.HomeDir != "" {
		base.HomeDir = override.HomeDir
	}
	if override.ConfigPath != "" {
		base.ConfigPath = override.ConfigPath
	}
	if override.GenesisPath != "" {
		base.GenesisPath = override.GenesisPath
	}
	if override.StatePath != "" {
		base.StatePath = override.StatePath
	}
	if len(override.Modules) > 0 {
		base.Modules = append([]string(nil), override.Modules...)
	}
	if len(override.AllowedSigners) > 0 {
		base.AllowedSigners = append([]string(nil), override.AllowedSigners...)
	}
	if len(override.GovernanceAuthorities) > 0 {
		base.GovernanceAuthorities = append([]string(nil), override.GovernanceAuthorities...)
	}
	if override.RequiredThreshold > 0 {
		base.RequiredThreshold = override.RequiredThreshold
	}
	return base
}

func writeConfigFile(path string, cfg Config) error {
	encoded, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, encoded, 0o644)
}

func writeGenesisFile(path string, genesis Genesis) error {
	encoded, err := json.MarshalIndent(genesis, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, encoded, 0o644)
}

func fileExists(path string) (bool, error) {
	_, err := os.Stat(strings.TrimSpace(path))
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func validateConfig(cfg Config) error {
	switch strings.TrimSpace(cfg.RuntimeMode) {
	case "", DefaultRuntimeMode, RuntimeModeSDKStore:
	default:
		return fmt.Errorf("unsupported runtime mode %q", cfg.RuntimeMode)
	}
	if strings.TrimSpace(cfg.HomeDir) == "" {
		return errors.New("home dir is required")
	}
	if strings.TrimSpace(cfg.ConfigPath) == "" {
		return errors.New("config path is required")
	}
	if strings.TrimSpace(cfg.GenesisPath) == "" {
		return errors.New("genesis path is required")
	}
	if strings.TrimSpace(cfg.StatePath) == "" {
		return errors.New("state path is required")
	}
	if len(cfg.AllowedSigners) == 0 {
		return errors.New("at least one allowed signer is required")
	}
	if len(cfg.GovernanceAuthorities) == 0 {
		return errors.New("at least one governance authority is required")
	}
	if cfg.RequiredThreshold == 0 {
		return errors.New("required threshold must be greater than zero")
	}
	if int(cfg.RequiredThreshold) > len(cfg.AllowedSigners) {
		return fmt.Errorf("required threshold %d exceeds configured signers %d", cfg.RequiredThreshold, len(cfg.AllowedSigners))
	}
	return nil
}
