package keeper

import (
	"errors"
	"strconv"
	"testing"
)

func FuzzRouteRefundStateMachineNeverSkipsPending(f *testing.F) {
	f.Add(uint64(25_000_000), false, false)
	f.Add(uint64(25_000_000), true, true)

	f.Fuzz(func(t *testing.T, amountSeed uint64, useTimeout bool, doubleRefund bool) {
		t.Parallel()

		k := seededRouterKeeper(t)
		amount := mustAmount(strconv.FormatUint((amountSeed%100_000_000)+1, 10))

		transfer, err := k.InitiateTransfer("eth.usdc", amount, "osmo1recipient", 120, "swap:uosmo")
		if err != nil {
			t.Fatalf("initiate transfer: %v", err)
		}

		if _, err := k.MarkRefunded(transfer.TransferID); !errors.Is(err, ErrTransferNotRecoverable) {
			t.Fatalf("expected pending transfer refund to fail, got %v", err)
		}

		current, ok := exportedTransfer(k, transfer.TransferID)
		if !ok {
			t.Fatalf("expected transfer %q to exist", transfer.TransferID)
		}
		if current.Status != TransferStatusPending {
			t.Fatalf("expected transfer to remain pending after direct refund attempt, got %q", current.Status)
		}

		if useTimeout {
			current, err = k.TimeoutTransfer(transfer.TransferID)
		} else {
			current, err = k.AcknowledgeFailure(transfer.TransferID, "destination failure")
		}
		if err != nil {
			t.Fatalf("move transfer into recoverable state: %v", err)
		}
		if current.Status != TransferStatusTimedOut && current.Status != TransferStatusAckFailed {
			t.Fatalf("expected recoverable state, got %q", current.Status)
		}

		refunded, err := k.MarkRefunded(transfer.TransferID)
		if err != nil {
			t.Fatalf("refund recoverable transfer: %v", err)
		}
		if refunded.Status != TransferStatusRefunded {
			t.Fatalf("expected refunded status, got %q", refunded.Status)
		}

		if doubleRefund {
			if _, err := k.MarkRefunded(transfer.TransferID); !errors.Is(err, ErrTransferNotRecoverable) {
				t.Fatalf("expected second refund to fail, got %v", err)
			}
		}
	})
}

func exportedTransfer(k *Keeper, transferID string) (TransferRecord, bool) {
	for _, transfer := range k.ExportTransfers() {
		if transfer.TransferID == transferID {
			return transfer, true
		}
	}
	return TransferRecord{}, false
}
