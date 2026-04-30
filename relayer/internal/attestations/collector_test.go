package attestations

import (
	"context"
	"errors"
	"sort"
	"testing"

	bridgetypes "github.com/ayushns01/aegislink/chain/aegislink/x/bridge/types"
	bridgetestutil "github.com/ayushns01/aegislink/chain/aegislink/x/bridge/types/testutil"
)

func TestCollectorCollectBuildsThresholdAttestationDeterministically(t *testing.T) {
	t.Parallel()

	source := &stubSource{
		votes: []Vote{
			{Signer: bridgetestutil.DefaultHarnessSignerAddresses()[1], Expiry: 120},
			{Signer: bridgetestutil.DefaultHarnessSignerAddresses()[0], Expiry: 140},
			{Signer: bridgetestutil.DefaultHarnessSignerAddresses()[1], Expiry: 130},
			{Signer: bridgetestutil.DefaultHarnessSignerAddresses()[2], Expiry: 150},
		},
	}
	collector := NewCollector(source, 2, 1, bridgetestutil.DefaultHarnessSignerPrivateKeys()[:3])

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

	wantSigners := []string{bridgetestutil.DefaultHarnessSignerAddresses()[0], bridgetestutil.DefaultHarnessSignerAddresses()[2]}
	sort.Strings(wantSigners)
	if len(attestation.Signers) != len(wantSigners) {
		t.Fatalf("expected %d signers, got %d", len(wantSigners), len(attestation.Signers))
	}
	for i, want := range wantSigners {
		if attestation.Signers[i] != want {
			t.Fatalf("signer %d mismatch: want %q, got %q", i, want, attestation.Signers[i])
		}
	}
	if len(attestation.Proofs) != 2 {
		t.Fatalf("expected 2 proofs, got %d", len(attestation.Proofs))
	}
}

func TestCollectorCollectRejectsWhenThresholdNotMet(t *testing.T) {
	t.Parallel()

	source := &stubSource{
		votes: []Vote{
			{Signer: bridgetestutil.DefaultHarnessSignerAddresses()[0], Expiry: 100},
		},
	}
	collector := NewCollector(source, 2, 1, bridgetestutil.DefaultHarnessSignerPrivateKeys()[:3])

	_, err := collector.Collect(context.Background(), "message-1", "payload-1")
	if !errors.Is(err, ErrThresholdNotMet) {
		t.Fatalf("expected threshold error, got %v", err)
	}
}

func TestCollectorCollectFallsBackToLocalSignerKeysWhenVoteFileIsEmpty(t *testing.T) {
	t.Parallel()

	source := &stubSource{}
	collector := NewCollector(source, 2, 1, bridgetestutil.DefaultHarnessSignerPrivateKeys()[:3])

	attestation, err := collector.Collect(context.Background(), "message-1", "payload-1")
	if err != nil {
		t.Fatalf("expected local signer fallback to succeed, got error: %v", err)
	}
	if attestation.Threshold != 2 {
		t.Fatalf("expected threshold 2, got %d", attestation.Threshold)
	}
	if attestation.Expiry == 0 {
		t.Fatalf("expected non-zero expiry, got %d", attestation.Expiry)
	}
	if len(attestation.Signers) != 2 || len(attestation.Proofs) != 2 {
		t.Fatalf("expected threshold-sized fallback attestation, got %+v", attestation)
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
