package attestations

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	bridgetypes "github.com/ayushns01/aegislink/chain/aegislink/x/bridge/types"
)

type FileVoteSource struct {
	path string
}

type persistedVotes struct {
	Votes []persistedVote `json:"votes"`
}

// persistedVote is a single oracle vote persisted to disk.
// Each vote MUST carry a Signature over voteSigningPayload so that the reader
// can verify the claimed Signer actually produced this vote.  Unsigned votes
// (empty Signature) are rejected.
type persistedVote struct {
	MessageID   string `json:"message_id"`
	PayloadHash string `json:"payload_hash"`
	Signer      string `json:"signer"`
	Expiry      uint64 `json:"expiry"`
	// Signature is an secp256k1 compact signature over voteSigningDigest.
	// It must be present; votes without a valid signature are silently dropped.
	Signature []byte `json:"signature"`
}

// voteSigningPayload builds the canonical byte payload that is hashed and
// signed by the oracle signer. Length-prefixing prevents field-boundary
// ambiguity attacks (the same technique used by Attestation.SigningDigest).
func voteSigningPayload(messageID, payloadHash string, expiry uint64) string {
	fields := []string{
		"aegislink.vote.v1",
		strings.TrimSpace(messageID),
		strings.ToLower(strings.TrimSpace(payloadHash)),
		fmt.Sprintf("%d", expiry),
	}
	encoded := make([]string, len(fields))
	for i, f := range fields {
		encoded[i] = fmt.Sprintf("%d:%s", len(f), f)
	}
	return strings.Join(encoded, "|")
}

// SignVote creates a persistedVote with a valid secp256k1 signature using the
// given private key hex. Call this when writing votes to the shared state file.
func SignVote(messageID, payloadHash string, expiry uint64, privateKeyHex string) (persistedVote, error) {
	address, err := bridgetypes.SignerAddressFromPrivateKeyHex(privateKeyHex)
	if err != nil {
		return persistedVote{}, fmt.Errorf("sign vote: derive signer address: %w", err)
	}

	payload := voteSigningPayload(messageID, payloadHash, expiry)
	sig, err := bridgetypes.SignRawPayload([]byte(payload), privateKeyHex)
	if err != nil {
		return persistedVote{}, fmt.Errorf("sign vote: %w", err)
	}

	return persistedVote{
		MessageID:   strings.TrimSpace(messageID),
		PayloadHash: strings.ToLower(strings.TrimSpace(payloadHash)),
		Signer:      address,
		Expiry:      expiry,
		Signature:   sig,
	}, nil
}

func NewFileVoteSource(path string) *FileVoteSource {
	return &FileVoteSource{path: path}
}

func (s *FileVoteSource) Votes(ctx context.Context, messageID, payloadHash string) ([]Vote, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s == nil || s.path == "" {
		return nil, nil
	}

	state, err := s.load()
	if err != nil {
		return nil, err
	}

	votes := make([]Vote, 0, len(state.Votes))
	for _, vote := range state.Votes {
		if vote.MessageID != messageID || vote.PayloadHash != payloadHash {
			continue
		}
		// Reject votes that carry no signature — they cannot be authenticated.
		if len(vote.Signature) == 0 {
			continue
		}
		// Verify the signature: recover the signer and compare to the claimed address.
		payload := voteSigningPayload(vote.MessageID, vote.PayloadHash, vote.Expiry)
		recovered, err := bridgetypes.RecoverSignerFromPayload([]byte(payload), vote.Signature)
		if err != nil {
			// Invalid signature — drop silently to not abort the whole batch.
			continue
		}
		if !strings.EqualFold(recovered, strings.TrimSpace(vote.Signer)) {
			// Signature valid but from a different key than claimed — drop.
			continue
		}
		votes = append(votes, Vote{
			Signer: recovered,
			Expiry: vote.Expiry,
		})
	}
	return votes, nil
}

func (s *FileVoteSource) load() (persistedVotes, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return persistedVotes{}, nil
		}
		return persistedVotes{}, err
	}

	var state persistedVotes
	if err := json.Unmarshal(data, &state); err != nil {
		return persistedVotes{}, err
	}
	return state, nil
}
