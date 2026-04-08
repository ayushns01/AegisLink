package attestations

import (
	"context"
	"errors"
	"sort"
	"strings"

	bridgetypes "github.com/ayushns01/aegislink/chain/aegislink/x/bridge/types"
)

var ErrThresholdNotMet = errors.New("attestation threshold not met")

type Vote struct {
	Signer string
	Expiry uint64
}

type VoteSource interface {
	Votes(context.Context, string, string) ([]Vote, error)
}

type Collector struct {
	source           VoteSource
	threshold        uint32
	signerSetVersion uint64
	signerKeys       map[string]string
}

func NewCollector(source VoteSource, threshold uint32, signerSetVersion uint64, signerKeys []string) *Collector {
	if signerSetVersion == 0 {
		signerSetVersion = 1
	}
	keyLookup := make(map[string]string, len(signerKeys))
	for _, privateKeyHex := range signerKeys {
		address, err := bridgetypes.SignerAddressFromPrivateKeyHex(privateKeyHex)
		if err != nil {
			continue
		}
		keyLookup[address] = privateKeyHex
	}
	return &Collector{source: source, threshold: threshold, signerSetVersion: signerSetVersion, signerKeys: keyLookup}
}

func (c *Collector) Collect(ctx context.Context, messageID, payloadHash string) (bridgetypes.Attestation, error) {
	if c == nil || c.source == nil {
		return bridgetypes.Attestation{}, ErrThresholdNotMet
	}
	if err := ctx.Err(); err != nil {
		return bridgetypes.Attestation{}, err
	}

	votes, err := c.source.Votes(ctx, messageID, payloadHash)
	if err != nil {
		return bridgetypes.Attestation{}, err
	}

	unique := make(map[string]uint64, len(votes))
	for _, vote := range votes {
		signer := strings.TrimSpace(vote.Signer)
		if signer == "" {
			continue
		}
		if expiry, ok := unique[signer]; !ok || vote.Expiry > expiry {
			unique[signer] = vote.Expiry
		}
	}
	if c.threshold == 0 || uint32(len(unique)) < c.threshold {
		return bridgetypes.Attestation{}, ErrThresholdNotMet
	}

	type signerVote struct {
		signer string
		expiry uint64
	}
	ranked := make([]signerVote, 0, len(unique))
	for signer, signerExpiry := range unique {
		ranked = append(ranked, signerVote{signer: signer, expiry: signerExpiry})
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].expiry != ranked[j].expiry {
			return ranked[i].expiry > ranked[j].expiry
		}
		return ranked[i].signer < ranked[j].signer
	})

	signers := make([]string, 0, c.threshold)
	proofs := make([]bridgetypes.AttestationProof, 0, c.threshold)
	var expiry uint64
	for _, vote := range ranked[:c.threshold] {
		signer := bridgetypes.NormalizeSignerAddress(vote.signer)
		_, ok := c.signerKeys[signer]
		if !ok {
			return bridgetypes.Attestation{}, ErrThresholdNotMet
		}
		signers = append(signers, signer)
		if expiry == 0 || vote.expiry < expiry {
			expiry = vote.expiry
		}
		proofs = append(proofs, bridgetypes.AttestationProof{Signer: signer})
	}
	sort.Strings(signers)

	attestation := bridgetypes.Attestation{
		MessageID:        messageID,
		PayloadHash:      payloadHash,
		Signers:          signers,
		Proofs:           proofs,
		Threshold:        c.threshold,
		Expiry:           expiry,
		SignerSetVersion: c.signerSetVersion,
	}
	for i, signer := range signers {
		privateKeyHex := c.signerKeys[signer]
		proof, err := bridgetypes.SignAttestationWithPrivateKeyHex(attestation, privateKeyHex)
		if err != nil {
			return bridgetypes.Attestation{}, err
		}
		proofs[i] = proof
	}
	attestation.Proofs = proofs
	return attestation, nil
}
