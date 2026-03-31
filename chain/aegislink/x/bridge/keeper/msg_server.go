package keeper

import bridgetypes "github.com/ayushns01/aegislink/chain/aegislink/x/bridge/types"

type MsgServer struct {
	keeper *Keeper
}

func NewMsgServer(k *Keeper) MsgServer {
	return MsgServer{keeper: k}
}

func (s MsgServer) SubmitDepositClaim(claim bridgetypes.DepositClaim, attestation bridgetypes.Attestation) (ClaimResult, error) {
	return s.keeper.ExecuteDepositClaim(claim, attestation)
}
