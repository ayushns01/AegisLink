package keeper

import (
	"fmt"
	"sort"
	"strings"

	bridgetypes "github.com/ayushns01/aegislink/chain/aegislink/x/bridge/types"
)

type SignerSet struct {
	Version     uint64   `json:"version"`
	Signers     []string `json:"signers"`
	Threshold   uint32   `json:"threshold"`
	ActivatedAt uint64   `json:"activated_at"`
	ExpiresAt   uint64   `json:"expires_at"`
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
		// Normalize before the duplicate check so that "0xABCD" and "0xabcd"
		// are correctly detected as the same address (they collapse to the same
		// key after normalizeSignerSet lowercases all entries).
		signer = strings.ToLower(strings.TrimSpace(signer))
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
		normalized.Signers = append(normalized.Signers, bridgetypes.NormalizeSignerAddress(signer))
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

func (k *Keeper) SignerSet(version uint64) (SignerSet, bool) {
	if version == 0 || k.signerSets == nil {
		return SignerSet{}, false
	}
	set, ok := k.signerSets[version]
	if !ok {
		return SignerSet{}, false
	}
	return set, true
}

func (k *Keeper) ExportSignerSets() []SignerSet {
	if len(k.signerSets) == 0 {
		return nil
	}
	sets := make([]SignerSet, 0, len(k.signerSets))
	for _, set := range k.signerSets {
		sets = append(sets, normalizeSignerSet(set))
	}
	sort.Slice(sets, func(i, j int) bool {
		return sets[i].Version < sets[j].Version
	})
	return sets
}
