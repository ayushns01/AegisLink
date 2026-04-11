package networked

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	aegisapp "github.com/ayushns01/aegislink/chain/aegislink/app"
)

type Config struct {
	HomeDir      string
	RPCAddress   string
	GRPCAddress  string
	ReadyFile    string
	TickInterval time.Duration
}

func ResolveConfig(cfg Config) (Config, aegisapp.Config, error) {
	appCfg, err := aegisapp.ResolveConfig(aegisapp.Config{
		HomeDir: strings.TrimSpace(cfg.HomeDir),
	})
	if err != nil {
		return Config{}, aegisapp.Config{}, err
	}
	if _, err := aegisapp.LoadGenesis(appCfg.GenesisPath); err != nil {
		return Config{}, aegisapp.Config{}, err
	}

	cfg.HomeDir = appCfg.HomeDir
	if strings.TrimSpace(cfg.RPCAddress) == "" {
		cfg.RPCAddress = "127.0.0.1:26657"
	}
	if strings.TrimSpace(cfg.GRPCAddress) == "" {
		cfg.GRPCAddress = "127.0.0.1:9090"
	}
	if strings.TrimSpace(cfg.ReadyFile) == "" {
		cfg.ReadyFile = filepath.Join(appCfg.HomeDir, "data", "demo-node-ready.json")
	}
	if strings.TrimSpace(cfg.HomeDir) == "" {
		return Config{}, aegisapp.Config{}, fmt.Errorf("missing home dir")
	}

	return cfg, appCfg, nil
}
