package bank

import bankkeeper "github.com/ayushns01/aegislink/chain/aegislink/x/bank/keeper"

const ModuleName = "bank"

type AppModule struct {
	keeper *bankkeeper.Keeper
}

func NewAppModule(k *bankkeeper.Keeper) AppModule {
	return AppModule{keeper: k}
}

func (m AppModule) Name() string {
	return ModuleName
}

func (m AppModule) Keeper() *bankkeeper.Keeper {
	return m.keeper
}
