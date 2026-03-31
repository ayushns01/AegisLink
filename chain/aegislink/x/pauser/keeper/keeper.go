package keeper

import (
	"errors"
	"strings"
)

var (
	ErrInvalidFlow = errors.New("invalid flow")
	ErrFlowPaused  = errors.New("flow is paused")
)

type Keeper struct {
	paused map[string]bool
}

func NewKeeper() *Keeper {
	return &Keeper{
		paused: make(map[string]bool),
	}
}

func (k *Keeper) Pause(flow string) error {
	key, err := flowKey(flow)
	if err != nil {
		return err
	}
	k.paused[key] = true
	return nil
}

func (k *Keeper) Unpause(flow string) error {
	key, err := flowKey(flow)
	if err != nil {
		return err
	}
	delete(k.paused, key)
	return nil
}

func (k *Keeper) IsPaused(flow string) bool {
	key, err := flowKey(flow)
	if err != nil {
		return false
	}
	return k.paused[key]
}

func (k *Keeper) AssertNotPaused(flow string) error {
	if k.IsPaused(flow) {
		return ErrFlowPaused
	}
	return nil
}

func flowKey(flow string) (string, error) {
	key := strings.TrimSpace(flow)
	if key == "" {
		return "", ErrInvalidFlow
	}
	return key, nil
}
