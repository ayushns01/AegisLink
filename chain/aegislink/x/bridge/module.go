package bridge

import bridgekeeper "github.com/ayushns01/aegislink/chain/aegislink/x/bridge/keeper"

const ModuleName = "bridge"

type AppModule struct {
	keeper *bridgekeeper.Keeper
}

func NewAppModule(k *bridgekeeper.Keeper) AppModule {
	return AppModule{keeper: k}
}

func (m AppModule) Name() string {
	return ModuleName
}

func (m AppModule) Keeper() *bridgekeeper.Keeper {
	return m.keeper
}
