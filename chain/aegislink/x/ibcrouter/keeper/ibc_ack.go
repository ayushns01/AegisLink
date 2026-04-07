package keeper

func (k *Keeper) ReceiveIBCAck(transferID string, success bool, reason string) (TransferRecord, error) {
	if success {
		return k.AcknowledgeSuccess(transferID)
	}
	return k.AcknowledgeFailure(transferID, reason)
}
