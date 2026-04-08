package keeper

import "strings"

func canonicalAuthority(authority string) string {
	return strings.TrimSpace(authority)
}

func canonicalAuthoritySet(authorities []string) map[string]struct{} {
	set := make(map[string]struct{}, len(authorities))
	for _, authority := range authorities {
		authority = canonicalAuthority(authority)
		if authority == "" {
			continue
		}
		set[authority] = struct{}{}
	}
	return set
}

func (k *Keeper) authorize(authority string) (string, error) {
	authority = canonicalAuthority(authority)
	if authority == "" {
		return "", ErrUnauthorizedProposal
	}
	if _, ok := k.authorities[authority]; !ok {
		return "", ErrUnauthorizedProposal
	}
	return authority, nil
}
