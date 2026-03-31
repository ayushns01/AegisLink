package limits

import limitskeeper "github.com/ayushns01/aegislink/chain/aegislink/x/limits/keeper"

const ModuleName = "limits"

type AppModule struct {
	keeper *limitskeeper.Keeper
}

func NewAppModule(k *limitskeeper.Keeper) AppModule {
	return AppModule{keeper: k}
}

func (m AppModule) Name() string {
	return ModuleName
}

func (m AppModule) Keeper() *limitskeeper.Keeper {
	return m.keeper
}
