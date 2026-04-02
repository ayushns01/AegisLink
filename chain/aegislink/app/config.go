package app

import (
	bridgemodule "github.com/ayushns01/aegislink/chain/aegislink/x/bridge"
	limitsmodule "github.com/ayushns01/aegislink/chain/aegislink/x/limits"
	pausermodule "github.com/ayushns01/aegislink/chain/aegislink/x/pauser"
	registrymodule "github.com/ayushns01/aegislink/chain/aegislink/x/registry"
)

const AppName = "aegislink"

type Config struct {
	AppName           string
	Modules           []string
	StatePath         string
	AllowedSigners    []string
	RequiredThreshold uint32
}

func DefaultConfig() Config {
	return Config{
		AppName: AppName,
		Modules: []string{
			bridgemodule.ModuleName,
			registrymodule.ModuleName,
			limitsmodule.ModuleName,
			pausermodule.ModuleName,
		},
		AllowedSigners:    []string{"relayer-1", "relayer-2", "relayer-3"},
		RequiredThreshold: 2,
	}
}
