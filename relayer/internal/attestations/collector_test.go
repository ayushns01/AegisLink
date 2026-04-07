package attestations

import (
	"context"
	"errors"
	"testing"
)

func TestCollectorCollectBuildsThresholdAttestationDeterministically(t *testing.T) {
	t.Parallel()

	source := &stubSource{
		votes: []Vote{
			{Signer: "signer-2", Expiry: 120},
			{Signer: "signer-1", Expiry: 140},
			{Signer: "signer-2", Expiry: 130},
			{Signer: "signer-3", Expiry: 150},
		},
	}
	collector := NewCollector(source, 2, 1)

	attestation, err := collector.Collect(context.Background(), "message-1", "payload-1")
	if err != nil {
		t.Fatalf("expected collect to succeed, got error: %v", err)
	}

	if source.messageID != "message-1" || source.payloadHash != "payload-1" {
		t.Fatalf("collector queried unexpected payload: message=%q payload=%q", source.messageID, source.payloadHash)
	}

	if attestation.MessageID != "message-1" {
		t.Fatalf("expected message id to be preserved, got %q", attestation.MessageID)
	}
	if attestation.PayloadHash != "payload-1" {
		t.Fatalf("expected payload hash to be preserved, got %q", attestation.PayloadHash)
	}
	if attestation.Threshold != 2 {
		t.Fatalf("expected threshold 2, got %d", attestation.Threshold)
	}
	if attestation.Expiry != 140 {
		t.Fatalf("expected expiry to use the strongest threshold quorum, got %d", attestation.Expiry)
	}
	if attestation.SignerSetVersion != 1 {
		t.Fatalf("expected signer set version 1, got %d", attestation.SignerSetVersion)
	}

	wantSigners := []string{"signer-1", "signer-3"}
	if len(attestation.Signers) != len(wantSigners) {
		t.Fatalf("expected %d signers, got %d", len(wantSigners), len(attestation.Signers))
	}
	for i, want := range wantSigners {
		if attestation.Signers[i] != want {
			t.Fatalf("signer %d mismatch: want %q, got %q", i, want, attestation.Signers[i])
		}
	}
}

func TestCollectorCollectRejectsWhenThresholdNotMet(t *testing.T) {
	t.Parallel()

	source := &stubSource{
		votes: []Vote{
			{Signer: "signer-1", Expiry: 100},
		},
	}
	collector := NewCollector(source, 2, 1)

	_, err := collector.Collect(context.Background(), "message-1", "payload-1")
	if !errors.Is(err, ErrThresholdNotMet) {
		t.Fatalf("expected threshold error, got %v", err)
	}
}

type stubSource struct {
	votes       []Vote
	messageID   string
	payloadHash string
}

func (s *stubSource) Votes(_ context.Context, messageID, payloadHash string) ([]Vote, error) {
	s.messageID = messageID
	s.payloadHash = payloadHash
	return s.votes, nil
}
