package registry

import registrykeeper "github.com/ayushns01/aegislink/chain/aegislink/x/registry/keeper"

const ModuleName = "registry"

type AppModule struct {
	keeper *registrykeeper.Keeper
}

func NewAppModule(k *registrykeeper.Keeper) AppModule {
	return AppModule{keeper: k}
}

func (m AppModule) Name() string {
	return ModuleName
}

func (m AppModule) Keeper() *registrykeeper.Keeper {
	return m.keeper
}
