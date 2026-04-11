package networked

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	cmtcfg "github.com/cometbft/cometbft/config"
	"github.com/cometbft/cometbft/p2p"
	"github.com/cometbft/cometbft/privval"
	cmttypes "github.com/cometbft/cometbft/types"

	aegisapp "github.com/ayushns01/aegislink/chain/aegislink/app"
)

type CometNodeHome struct {
	RootDir                string
	NodeID                 string
	ConfigPath             string
	CometGenesisPath       string
	NodeKeyPath            string
	PrivValidatorKeyPath   string
	PrivValidatorStatePath string
	Config                 *cmtcfg.Config
}

func EnsureCometNodeHome(cfg Config) (CometNodeHome, error) {
	resolved, appCfg, err := ResolveConfig(cfg)
	if err != nil {
		return CometNodeHome{}, err
	}
	return ensureCometNodeHome(resolved, appCfg)
}

func ensureCometNodeHome(cfg Config, appCfg aegisapp.Config) (CometNodeHome, error) {
	cometCfg := cmtcfg.DefaultConfig().SetRoot(appCfg.HomeDir)
	cometCfg.Moniker = appCfg.AppName
	if strings.TrimSpace(cfg.CometRPCAddress) != "" {
		cometCfg.RPC.ListenAddress = normalizeTCPAddress(cfg.CometRPCAddress)
	}
	if strings.TrimSpace(cfg.ABCIAddress) != "" {
		cometCfg.ProxyApp = normalizeTCPAddress(cfg.ABCIAddress)
	}
	if strings.TrimSpace(cfg.P2PAddress) != "" {
		cometCfg.P2P.ListenAddress = normalizeTCPAddress(cfg.P2PAddress)
	}
	if cfg.TickInterval > 0 {
		cometCfg.Consensus.CreateEmptyBlocks = true
		cometCfg.Consensus.CreateEmptyBlocksInterval = cfg.TickInterval
	}
	if err := cometCfg.ValidateBasic(); err != nil {
		return CometNodeHome{}, fmt.Errorf("validate comet config: %w", err)
	}

	cmtcfg.EnsureRoot(appCfg.HomeDir)

	configPath := filepath.Join(appCfg.HomeDir, cmtcfg.DefaultConfigDir, cmtcfg.DefaultConfigFileName)
	cometGenesisPath := filepath.Join(appCfg.HomeDir, cmtcfg.DefaultConfigDir, "comet-genesis.json")
	nodeKeyPath := filepath.Join(appCfg.HomeDir, cmtcfg.DefaultConfigDir, cmtcfg.DefaultNodeKeyName)
	privValKeyPath := filepath.Join(appCfg.HomeDir, cmtcfg.DefaultConfigDir, cmtcfg.DefaultPrivValKeyName)
	privValStatePath := filepath.Join(appCfg.HomeDir, cmtcfg.DefaultDataDir, cmtcfg.DefaultPrivValStateName)
	cometCfg.Genesis = filepath.Join(cmtcfg.DefaultConfigDir, "comet-genesis.json")

	cmtcfg.WriteConfigFile(configPath, cometCfg)
	nodeKey, err := p2p.LoadOrGenNodeKey(nodeKeyPath)
	if err != nil {
		return CometNodeHome{}, fmt.Errorf("load or generate node key: %w", err)
	}
	filePV := privval.LoadOrGenFilePV(privValKeyPath, privValStatePath)
	pubKey, err := filePV.GetPubKey()
	if err != nil {
		return CometNodeHome{}, fmt.Errorf("read priv validator pubkey: %w", err)
	}
	genesis := &cmttypes.GenesisDoc{
		GenesisTime:     time.Now().UTC(),
		ChainID:         appCfg.ChainID,
		InitialHeight:   1,
		ConsensusParams: cmttypes.DefaultConsensusParams(),
		Validators: []cmttypes.GenesisValidator{
			{
				Address: filePV.GetAddress(),
				PubKey:  pubKey,
				Power:   1,
				Name:    appCfg.AppName + "-validator",
			},
		},
		AppState: []byte("{}"),
	}
	if err := genesis.ValidateAndComplete(); err != nil {
		return CometNodeHome{}, fmt.Errorf("validate comet genesis: %w", err)
	}
	genesis.SaveAs(cometGenesisPath)

	return CometNodeHome{
		RootDir:                appCfg.HomeDir,
		NodeID:                 string(nodeKey.ID()),
		ConfigPath:             configPath,
		CometGenesisPath:       cometGenesisPath,
		NodeKeyPath:            nodeKeyPath,
		PrivValidatorKeyPath:   privValKeyPath,
		PrivValidatorStatePath: privValStatePath,
		Config:                 cometCfg,
	}, nil
}

func normalizeTCPAddress(address string) string {
	address = strings.TrimSpace(address)
	if address == "" {
		return address
	}
	if strings.Contains(address, "://") {
		return address
	}
	return "tcp://" + address
}
