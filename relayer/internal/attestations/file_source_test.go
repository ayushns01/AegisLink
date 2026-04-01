package attestations

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestFileVoteSourceReturnsMatchingVotes(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "votes.json")
	payload := `{
  "votes": [
    {"message_id":"message-1","payload_hash":"digest-1","signer":"signer-1","expiry":120},
    {"message_id":"message-1","payload_hash":"digest-1","signer":"signer-2","expiry":140},
    {"message_id":"message-1","payload_hash":"digest-2","signer":"signer-3","expiry":160}
  ]
}`
	if err := os.WriteFile(path, []byte(payload), 0o644); err != nil {
		t.Fatalf("write votes fixture: %v", err)
	}

	source := NewFileVoteSource(path)
	votes, err := source.Votes(context.Background(), "message-1", "digest-1")
	if err != nil {
		t.Fatalf("load votes: %v", err)
	}
	if len(votes) != 2 {
		t.Fatalf("expected 2 votes, got %d", len(votes))
	}
	if votes[0].Signer != "signer-1" || votes[1].Signer != "signer-2" {
		t.Fatalf("expected matching votes in file order, got %#v", votes)
	}
}
