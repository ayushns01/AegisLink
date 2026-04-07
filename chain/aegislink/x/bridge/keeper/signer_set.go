package keeper

import (
	"fmt"
	"sort"
	"strings"
)

type SignerSet struct {
	Version     uint64
	Signers     []string
	Threshold   uint32
	ActivatedAt uint64
	ExpiresAt   uint64
}

func (s SignerSet) ValidateBasic() error {
	if s.Version == 0 {
		return fmt.Errorf("missing signer set version")
	}
	if len(s.Signers) == 0 {
		return fmt.Errorf("missing signers")
	}
	if s.Threshold == 0 {
		return fmt.Errorf("missing threshold")
	}
	if int(s.Threshold) > len(s.Signers) {
		return fmt.Errorf("threshold exceeds signer count")
	}
	if s.ExpiresAt != 0 && s.ExpiresAt < s.ActivatedAt {
		return fmt.Errorf("expiry before activation")
	}

	seen := make(map[string]struct{}, len(s.Signers))
	for _, signer := range s.Signers {
		signer = strings.TrimSpace(signer)
		if signer == "" {
			return fmt.Errorf("empty signer")
		}
		if _, ok := seen[signer]; ok {
			return fmt.Errorf("duplicate signer %q", signer)
		}
		seen[signer] = struct{}{}
	}

	return nil
}

func normalizeSignerSet(set SignerSet) SignerSet {
	normalized := SignerSet{
		Version:     set.Version,
		Threshold:   set.Threshold,
		ActivatedAt: set.ActivatedAt,
		ExpiresAt:   set.ExpiresAt,
		Signers:     make([]string, 0, len(set.Signers)),
	}
	for _, signer := range set.Signers {
		normalized.Signers = append(normalized.Signers, strings.TrimSpace(signer))
	}
	sort.Strings(normalized.Signers)
	return normalized
}

func (k *Keeper) UpsertSignerSet(set SignerSet) error {
	if err := set.ValidateBasic(); err != nil {
		return err
	}
	if k.signerSets == nil {
		k.signerSets = make(map[uint64]SignerSet)
	}
	k.signerSets[set.Version] = normalizeSignerSet(set)
	return k.persist()
}

func (k *Keeper) ActiveSignerSet() (SignerSet, error) {
	var (
		active SignerSet
		found  bool
	)

	for _, set := range k.signerSets {
		if set.ActivatedAt > k.currentHeight {
			continue
		}
		if set.ExpiresAt != 0 && k.currentHeight > set.ExpiresAt {
			continue
		}
		if !found || set.Version > active.Version {
			active = set
			found = true
		}
	}

	if !found {
		return SignerSet{}, ErrSignerSetInactive
	}
	return active, nil
}
