package attestations

import (
	"context"
	"encoding/json"
	"os"
)

type FileVoteSource struct {
	path string
}

type persistedVotes struct {
	Votes []persistedVote `json:"votes"`
}

type persistedVote struct {
	MessageID   string `json:"message_id"`
	PayloadHash string `json:"payload_hash"`
	Signer      string `json:"signer"`
	Expiry      uint64 `json:"expiry"`
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
		votes = append(votes, Vote{
			Signer: vote.Signer,
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
