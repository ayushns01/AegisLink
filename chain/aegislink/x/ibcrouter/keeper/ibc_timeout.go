package keeper

func (k *Keeper) HandleIBCTimeout(transferID string) (TransferRecord, error) {
	return k.TimeoutTransfer(transferID)
}
