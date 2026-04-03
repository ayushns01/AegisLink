package ibcrouter

import ibcrouterkeeper "github.com/ayushns01/aegislink/chain/aegislink/x/ibcrouter/keeper"

const ModuleName = "ibcrouter"

type AppModule struct {
	keeper *ibcrouterkeeper.Keeper
}

func NewAppModule(k *ibcrouterkeeper.Keeper) AppModule {
	return AppModule{keeper: k}
}

func (m AppModule) Name() string {
	return ModuleName
}

func (m AppModule) Keeper() *ibcrouterkeeper.Keeper {
	return m.keeper
}
