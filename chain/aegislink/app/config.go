package app

import (
	limitsmodule "github.com/ayushns01/aegislink/chain/aegislink/x/limits"
	pausermodule "github.com/ayushns01/aegislink/chain/aegislink/x/pauser"
	registrymodule "github.com/ayushns01/aegislink/chain/aegislink/x/registry"
)

const AppName = "aegislink"

type Config struct {
	AppName string
	Modules []string
}

func DefaultConfig() Config {
	return Config{
		AppName: AppName,
		Modules: []string{
			registrymodule.ModuleName,
			limitsmodule.ModuleName,
			pausermodule.ModuleName,
		},
	}
}
