package governance

import governancekeeper "github.com/ayushns01/aegislink/chain/aegislink/x/governance/keeper"

const ModuleName = "governance"

type AppModule struct {
	keeper *governancekeeper.Keeper
}

func NewAppModule(k *governancekeeper.Keeper) AppModule {
	return AppModule{keeper: k}
}

func (m AppModule) Name() string {
	return ModuleName
}

func (m AppModule) Keeper() *governancekeeper.Keeper {
	return m.keeper
}
