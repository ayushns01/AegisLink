package app

import (
	limitsmodule "github.com/ayushns01/aegislink/chain/aegislink/x/limits"
	limitskeeper "github.com/ayushns01/aegislink/chain/aegislink/x/limits/keeper"
	pausermodule "github.com/ayushns01/aegislink/chain/aegislink/x/pauser"
	pauserkeeper "github.com/ayushns01/aegislink/chain/aegislink/x/pauser/keeper"
	registrymodule "github.com/ayushns01/aegislink/chain/aegislink/x/registry"
	registrykeeper "github.com/ayushns01/aegislink/chain/aegislink/x/registry/keeper"
)

type App struct {
	Config         Config
	RegistryKeeper *registrykeeper.Keeper
	LimitsKeeper   *limitskeeper.Keeper
	PauserKeeper   *pauserkeeper.Keeper
	modules        []string
}

func New() *App {
	registryKeeper := registrykeeper.NewKeeper()
	limitsKeeper := limitskeeper.NewKeeper()
	pauserKeeper := pauserkeeper.NewKeeper()

	registryAppModule := registrymodule.NewAppModule(registryKeeper)
	limitsAppModule := limitsmodule.NewAppModule(limitsKeeper)
	pauserAppModule := pausermodule.NewAppModule(pauserKeeper)

	return &App{
		Config:         DefaultConfig(),
		RegistryKeeper: registryKeeper,
		LimitsKeeper:   limitsKeeper,
		PauserKeeper:   pauserKeeper,
		modules: []string{
			registryAppModule.Name(),
			limitsAppModule.Name(),
			pauserAppModule.Name(),
		},
	}
}

func (a *App) ModuleNames() []string {
	modules := make([]string, len(a.modules))
	copy(modules, a.modules)
	return modules
}
