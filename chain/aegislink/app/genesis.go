package app

import (
	"errors"
	"fmt"
	"strings"
)

func (g Genesis) Validate() error {
	if strings.TrimSpace(g.AppName) == "" {
		return errors.New("genesis app name is required")
	}
	if strings.TrimSpace(g.ChainID) == "" {
		return errors.New("genesis chain id is required")
	}
	if len(g.Modules) == 0 {
		return errors.New("genesis modules are required")
	}
	if len(g.AllowedSigners) == 0 {
		return errors.New("genesis allowed signers are required")
	}
	if g.RequiredThreshold == 0 {
		return errors.New("genesis required threshold must be greater than zero")
	}
	if int(g.RequiredThreshold) > len(g.AllowedSigners) {
		return fmt.Errorf("genesis threshold %d exceeds configured signers %d", g.RequiredThreshold, len(g.AllowedSigners))
	}

	seenModules := make(map[string]struct{}, len(g.Modules))
	for _, moduleName := range g.Modules {
		moduleName = strings.TrimSpace(moduleName)
		if moduleName == "" {
			return errors.New("genesis module name must not be empty")
		}
		if _, exists := seenModules[moduleName]; exists {
			return fmt.Errorf("duplicate genesis module %q", moduleName)
		}
		seenModules[moduleName] = struct{}{}
	}

	seenSigners := make(map[string]struct{}, len(g.AllowedSigners))
	for _, signer := range g.AllowedSigners {
		signer = strings.TrimSpace(signer)
		if signer == "" {
			return errors.New("genesis signer must not be empty")
		}
		if _, exists := seenSigners[signer]; exists {
			return fmt.Errorf("duplicate genesis signer %q", signer)
		}
		seenSigners[signer] = struct{}{}
	}

	return nil
}
