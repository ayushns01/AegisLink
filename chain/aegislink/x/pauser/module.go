package pauser

import pauserkeeper "github.com/ayushns01/aegislink/chain/aegislink/x/pauser/keeper"

const ModuleName = "pauser"

type AppModule struct {
	keeper *pauserkeeper.Keeper
}

func NewAppModule(k *pauserkeeper.Keeper) AppModule {
	return AppModule{keeper: k}
}

func (m AppModule) Name() string {
	return ModuleName
}

func (m AppModule) Keeper() *pauserkeeper.Keeper {
	return m.keeper
}
