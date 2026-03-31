package keeper

import (
	"errors"
	"testing"
)

func TestAssertNotPausedRejectsPausedFlow(t *testing.T) {
	keeper := NewKeeper()

	if err := keeper.Pause("bridge.inbound"); err != nil {
		t.Fatalf("expected pause to succeed, got %v", err)
	}

	err := keeper.AssertNotPaused("bridge.inbound")
	if !errors.Is(err, ErrFlowPaused) {
		t.Fatalf("expected paused flow error, got %v", err)
	}
}

func TestUnpauseClearsPausedFlow(t *testing.T) {
	keeper := NewKeeper()

	if err := keeper.Pause("bridge.inbound"); err != nil {
		t.Fatalf("expected pause to succeed, got %v", err)
	}
	if err := keeper.Unpause("bridge.inbound"); err != nil {
		t.Fatalf("expected unpause to succeed, got %v", err)
	}
	if err := keeper.AssertNotPaused("bridge.inbound"); err != nil {
		t.Fatalf("expected flow to be active after unpause, got %v", err)
	}
}
